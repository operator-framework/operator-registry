package v1alpha1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"strings"

	mmsemver "github.com/Masterminds/semver/v3"
	blangsemver "github.com/blang/semver/v4"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/model"
)

type filterOptions struct {
	Log *logrus.Entry
}

type FilterOption func(*filterOptions)

type filterer struct {
	pkgConfigs map[string]Package
	keeps      map[string]sets.Set[string]
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

func NewFilterer(config *FilterConfiguration, filterOpts ...FilterOption) declcfg.CatalogFilter {
	opts := filterOptions{
		Log: nullLogger(),
	}
	for _, opt := range filterOpts {
		opt(&opts)
	}
	pkgConfigs := make(map[string]Package, len(config.Packages))
	keeps := make(map[string]sets.Set[string], len(config.Packages))
	for _, pkg := range config.Packages {
		pkgConfigs[pkg.Name] = pkg
		channels := sets.New[string]()
		for _, ch := range pkg.Channels {
			channels.Insert(ch.Name)
		}
		keeps[pkg.Name] = channels
	}
	return &filterer{
		pkgConfigs: pkgConfigs,
		keeps:      keeps,
		opts:       opts,
	}
}

func (f *filterer) FilterCatalog(_ context.Context, fbc *declcfg.DeclarativeConfig) (*declcfg.DeclarativeConfig, error) {
	m, err := declcfg.ConvertToModel(*fbc)
	if err != nil {
		return nil, err
	}

	// first filter out packages
	f.filterPackages(m)

	// then filter out channels
	var errs []error
	for _, pkgConfig := range f.pkgConfigs {
		if err := f.filterChannels(m[pkgConfig.Name]); err != nil {
			errs = append(errs, fmt.Errorf("invalid filter configuration for package %q: %w", pkgConfig.Name, err))
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	filtered := declcfg.ConvertFromModel(m)
	return &filtered, nil
}

func (f *filterer) KeepMeta(meta *declcfg.Meta) bool {
	if len(f.keeps) == 0 {
		return true
	}

	packageName := meta.Package
	if meta.Schema == "olm.package" {
		packageName = meta.Name
	}

	_, ok := f.keeps[packageName]
	return ok
}

func (f *filterer) filterPackages(m model.Model) {
	for _, pkg := range m {
		if _, ok := f.keeps[pkg.Name]; !ok {
			delete(m, pkg.Name)
		}
	}
}

func (f *filterer) filterChannels(pkg *model.Package) error {
	// if no channels are set, then no channel filtering is needed
	pkgConfig, ok := f.pkgConfigs[pkg.Name]
	if !ok || len(pkgConfig.Channels) == 0 {
		return nil
	}

	// filter out channels that are not in the filter configuration
	maps.DeleteFunc(pkg.Channels, func(k string, _ *model.Channel) bool {
		channels, ok := f.keeps[pkg.Name]
		if ok && (channels.Len() == 0 || channels.Has(k)) {
			return false
		}
		return true
	})

	// filter out bundles that are not in the version range for the channels that are in the filter configuration
	var errs []error
	for _, channelConfig := range pkgConfig.Channels {
		if err := f.filterBundles(pkg.Channels[channelConfig.Name], channelConfig.VersionRange); err != nil {
			errs = append(errs, fmt.Errorf("error filtering bundles for channel %q in package %q: %w", channelConfig.Name, pkg.Name, err))
		}
	}

	// set and validate the default channel
	if err := setDefaultChannel(pkg, pkgConfig); err != nil {
		errs = append(errs, fmt.Errorf("invalid default channel configuration: %w", err))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func setDefaultChannel(pkg *model.Package, pkgConfig Package) error {
	// If the default channel was specified in the filter configuration, then we need to check if it exists after filtering.
	// If it does, then we update the model's default channel to the specified channel. Otherwise, we error.
	if pkgConfig.DefaultChannel != "" {
		configDefaultChannel, ok := pkg.Channels[pkgConfig.DefaultChannel]
		if !ok {
			return fmt.Errorf("specified default channel override %q does not exist", pkgConfig.DefaultChannel)
		}
		pkg.DefaultChannel = configDefaultChannel
		return nil
	}

	// At this point, we know that the default channel was not configured in the filter configuration for this package.
	// If the original default channel does not exist after filtering, error
	if _, ok := pkg.Channels[pkg.DefaultChannel.Name]; ok {
		return fmt.Errorf("the default channel %q was filtered out, a new default channel must be configured for this package", pkg.DefaultChannel.Name)
	}
	return nil
}

// filterBundles filters out bundles from the channel that do not fall within the version range.
//
// This is a bit tricky because we need to keep a single coherent channel head, which might mean including extra
// bundles that fall outside the version range. If this happens, we will emit a warning for each bundle that falls
// outside the range.
//
// We need to find the new head and tail bundle. We will count the number of bundles in the version range that are at or
// below each bundle in the replaces chain. In order to get the minimal set, we will keep track of the specific bundles
// that we have seen and only count them once.
//   - The new head will be the bundle with the most bundles at or below it. If multiple bundles have the same number
//     of version range matches at or below them, we will use the bundle lowest in the replaces chain.
//   - The tail will be the first bundle in the replaces chain whose version range match count is 0. The tail will not
//     be included in the filtered channel.
//
// If the entire channel is filtered out, we will emit an error.
func (f *filterer) filterBundles(ch *model.Channel, versionRange string) error {
	if versionRange == "" {
		return nil
	}
	logger := f.opts.Log.WithField("package", ch.Package.Name).WithField("channel", ch.Name)

	origHead, err := ch.Head()
	if err != nil {
		return fmt.Errorf("error getting head of channel %q: %v", ch.Name, err)
	}

	versionConstraints, err := mmsemver.NewConstraint(versionRange)
	if err != nil {
		return fmt.Errorf("invalid version range %q for channel %q: %v", versionRange, ch.Name, err)
	}

	seen := map[*model.Bundle]struct{}{}
	counts := map[*model.Bundle]int{}
	countUniqueTailBundlesInRange(origHead, versionConstraints, ch, seen, counts)
	maxCount := -1

	var head, tail *model.Bundle
	for cur := origHead; cur != nil; cur = ch.Bundles[cur.Replaces] {
		count := counts[cur]
		if count >= maxCount {
			head = cur
			maxCount = count
		}
		if count == 0 {
			tail = cur
			break
		}
	}

	// We how have head and tail, let's traverse head to tail and build a list of bundles to keep,
	// emitting a warning if anything in the replaces chain is not in the version range.
	bundles := map[string]*model.Bundle{}
	for cur := head; cur != tail; cur = ch.Bundles[cur.Replaces] {
		if !versionConstraints.Check(blangToMM(cur.Version)) {
			logger.Warnf("including bundle %q with version %q: it falls outside the specified range of %q but is required to ensure inclusion of all bundles in the range", cur.Name, cur.Version.String(), versionRange)
		}
		bundles[cur.Name] = cur
		for _, skip := range cur.Skips {
			if skipBundle, ok := ch.Bundles[skip]; ok {
				if versionConstraints.Check(blangToMM(skipBundle.Version)) {
					bundles[skipBundle.Name] = skipBundle
				}
			}
		}
	}
	if len(bundles) == 0 {
		return fmt.Errorf("invalid filter configuration: no bundles in channel %q for package %q matched the version range %q", ch.Name, ch.Package.Name, versionRange)
	}
	ch.Bundles = bundles
	return nil
}

func blangToMM(in blangsemver.Version) *mmsemver.Version {
	pres := make([]string, len(in.Pre))
	for i, p := range in.Pre {
		pres[i] = p.String()
	}
	return mmsemver.New(
		in.Major,
		in.Minor,
		in.Patch,
		strings.Join(pres, "."),
		strings.Join(in.Build, "."),
	)
}

// countUniqueTailBundlesInRange counts the number of bundles in the replaces chain of b that are in the version range
// that are unique to b, where "in the replaces chain" is defined as "b or any bundle that b skips, or any bundle in
// the replaces chain of b's replaces bundle"
func countUniqueTailBundlesInRange(b *model.Bundle, versionConstraints *mmsemver.Constraints, ch *model.Channel, seen map[*model.Bundle]struct{}, counts map[*model.Bundle]int) {
	selfCount := 0
	if _, ok := seen[b]; !ok && versionConstraints.Check(blangToMM(b.Version)) {
		seen[b] = struct{}{}
		selfCount++
	}
	for _, skip := range b.Skips {
		if skipBundle, ok := ch.Bundles[skip]; ok {
			if _, ok := seen[skipBundle]; !ok && versionConstraints.Check(blangToMM(skipBundle.Version)) {
				seen[skipBundle] = struct{}{}
				selfCount++
			}
		}
	}
	replaces, ok := ch.Bundles[b.Replaces]
	if !ok {
		counts[b] = selfCount
		return
	}
	countUniqueTailBundlesInRange(replaces, versionConstraints, ch, seen, counts)
	counts[b] = selfCount + counts[replaces]
	return
}
