package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"

	mmsemver "github.com/Masterminds/semver/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

type filterOptions struct {
	Log *logrus.Entry
}

type FilterOption func(*filterOptions)

type filter struct {
	pkgConfigs map[string]Package
	chConfigs  map[string]map[string]Channel
	opts       filterOptions
}

func WithLogger(log *logrus.Entry) FilterOption {
	return func(opts *filterOptions) {
		opts.Log = log
	}
}

func nullLogger() *logrus.Entry {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return logrus.NewEntry(l)
}

func NewFilter(config FilterConfiguration, filterOpts ...FilterOption) declcfg.CatalogFilter {
	opts := filterOptions{
		Log: nullLogger(),
	}
	for _, opt := range filterOpts {
		opt(&opts)
	}
	pkgConfigs := make(map[string]Package, len(config.Packages))
	chConfigs := make(map[string]map[string]Channel, len(config.Packages))
	for _, pkg := range config.Packages {
		pkgConfigs[pkg.Name] = pkg
		pkgChannels, ok := chConfigs[pkg.Name]
		if !ok {
			pkgChannels = make(map[string]Channel)
		}
		for _, ch := range pkg.Channels {
			pkgChannels[ch.Name] = ch
		}
		chConfigs[pkg.Name] = pkgChannels
	}
	return &filter{
		pkgConfigs: pkgConfigs,
		chConfigs:  chConfigs,
		opts:       opts,
	}
}

func (f *filter) FilterCatalog(_ context.Context, fbc *declcfg.DeclarativeConfig) (*declcfg.DeclarativeConfig, error) {
	if fbc == nil {
		return nil, nil
	}
	fbc.Packages = slices.DeleteFunc(fbc.Packages, func(pkg declcfg.Package) bool {
		_, ok := f.chConfigs[pkg.Name]
		return !ok
	})
	fbc.Channels = slices.DeleteFunc(fbc.Channels, func(ch declcfg.Channel) bool {
		chSet, foundPackage := f.chConfigs[ch.Package]
		if !foundPackage {
			return true
		}
		if len(chSet) == 0 {
			return false
		}
		_, foundChannel := chSet[ch.Name]
		return !foundChannel
	})
	fbc.Bundles = slices.DeleteFunc(fbc.Bundles, func(b declcfg.Bundle) bool {
		_, ok := f.chConfigs[b.Package]
		return !ok
	})
	fbc.Deprecations = slices.DeleteFunc(fbc.Deprecations, func(d declcfg.Deprecation) bool {
		_, ok := f.chConfigs[d.Package]
		return !ok
	})
	fbc.Others = slices.DeleteFunc(fbc.Others, func(o declcfg.Meta) bool {
		_, ok := f.chConfigs[o.Package]
		return !ok && o.Package != ""
	})

	remainingChannels := make(map[string]sets.Set[string], len(fbc.Packages))
	for _, ch := range fbc.Channels {
		pkgChannels, ok := remainingChannels[ch.Package]
		if !ok {
			pkgChannels = sets.New[string]()
		}
		pkgChannels.Insert(ch.Name)
		remainingChannels[ch.Package] = pkgChannels
	}
	for i, pkg := range fbc.Packages {
		pkgConfig := f.pkgConfigs[pkg.Name]
		if err := setDefaultChannel(&fbc.Packages[i], pkgConfig, remainingChannels[pkg.Name]); err != nil {
			return nil, fmt.Errorf("invalid default channel configuration for package %q: %v", pkg.Name, err)
		}
	}

	getVersion := func(b declcfg.Bundle) (*mmsemver.Version, error) {
		for _, p := range b.Properties {
			if p.Type != property.TypePackage {
				continue
			}
			var pkg property.Package
			if err := json.Unmarshal(p.Value, &pkg); err != nil {
				return nil, err
			}
			return mmsemver.StrictNewVersion(pkg.Version)
		}
		return nil, fmt.Errorf("bundle %q in package %q has no package property", b.Name, b.Package)
	}

	versionMap := make(map[string]map[string]*mmsemver.Version)
	for _, b := range fbc.Bundles {
		v, err := getVersion(b)
		if err != nil {
			return nil, err
		}
		bundleVersions, ok := versionMap[b.Package]
		if !ok {
			bundleVersions = make(map[string]*mmsemver.Version)
		}
		bundleVersions[b.Name] = v
		versionMap[b.Package] = bundleVersions
	}

	keepBundles := map[string]sets.Set[string]{}
	for i, fbcCh := range fbc.Channels {
		keepEntries := sets.New[string]()
		chConfig, ok := f.chConfigs[fbcCh.Package][fbcCh.Name]
		if !ok || chConfig.VersionRange == "" {
			for _, e := range fbcCh.Entries {
				keepEntries.Insert(e.Name)
				keepEntries.Insert(e.Skips...)
				if e.Replaces != "" {
					keepEntries.Insert(e.Replaces)
				}
			}
		} else if chConfig.VersionRange != "" {
			versionRange, err := mmsemver.NewConstraint(chConfig.VersionRange)
			if err != nil {
				return nil, fmt.Errorf("error parsing version range: %v", err)
			}
			ch, err := newChannel(fbcCh, f.opts.Log)
			if err != nil {
				return nil, err
			}
			keepEntries = ch.filterByVersionRange(versionRange, versionMap[fbcCh.Package])
			if len(keepEntries) == 0 {
				return nil, fmt.Errorf("package %q channel %q has version range %q that results in an empty channel", fbcCh.Package, fbcCh.Name, chConfig.VersionRange)
			}
			fbc.Channels[i].Entries = slices.DeleteFunc(fbc.Channels[i].Entries, func(e declcfg.ChannelEntry) bool {
				return !keepEntries.Has(e.Name)
			})
		}

		if _, ok := keepBundles[fbcCh.Package]; !ok {
			keepBundles[fbcCh.Package] = sets.New[string]()
		}
		keepBundles[fbcCh.Package] = keepBundles[fbcCh.Package].Union(keepEntries)
	}

	fbc.Bundles = slices.DeleteFunc(fbc.Bundles, func(b declcfg.Bundle) bool {
		bundles, ok := keepBundles[b.Package]
		return ok && !bundles.Has(b.Name)
	})

	for i := range fbc.Deprecations {
		fbc.Deprecations[i].Entries = slices.DeleteFunc(fbc.Deprecations[i].Entries, func(e declcfg.DeprecationEntry) bool {
			if e.Reference.Schema == declcfg.SchemaBundle {
				bundles, ok := keepBundles[fbc.Deprecations[i].Package]
				return ok && !bundles.Has(e.Reference.Name)
			}
			if e.Reference.Schema == declcfg.SchemaChannel {
				channels, ok := remainingChannels[fbc.Deprecations[i].Package]
				return ok && !channels.Has(e.Reference.Name)
			}
			return false
		})
	}
	return fbc, nil
}

func (f *filter) KeepMeta(meta *declcfg.Meta) bool {
	if len(f.chConfigs) == 0 {
		return false
	}

	packageName := meta.Package
	if meta.Schema == "olm.package" {
		packageName = meta.Name
	}

	_, ok := f.chConfigs[packageName]
	return ok
}

func setDefaultChannel(pkg *declcfg.Package, pkgConfig Package, channels sets.Set[string]) error {
	// If both the FBC and package config leave the default channel unspecified, then we don't need to do anything.
	if pkg.DefaultChannel == "" && pkgConfig.DefaultChannel == "" {
		return nil
	}

	// If the default channel was specified in the filter configuration, then we need to check if it exists after filtering.
	// If it does, then we update the model's default channel to the specified channel. Otherwise, we error.
	if pkgConfig.DefaultChannel != "" {
		if !channels.Has(pkgConfig.DefaultChannel) {
			return fmt.Errorf("specified default channel override %q does not exist in the filtered output", pkgConfig.DefaultChannel)
		}
		pkg.DefaultChannel = pkgConfig.DefaultChannel
		return nil
	}

	// At this point, we know that the default channel was not configured in the filter configuration for this package.
	// If the original default channel does not exist after filtering, error
	if !channels.Has(pkg.DefaultChannel) {
		return fmt.Errorf("the default channel %q was filtered out, a new default channel must be configured for this package", pkg.DefaultChannel)
	}
	return nil
}
