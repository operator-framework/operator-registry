package action

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/blang/semver/v4"
	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/model"
	"github.com/operator-framework/operator-registry/pkg/image"
)

type Diff struct {
	Registry image.Registry

	OldRefs []string
	NewRefs []string
	// SkipDependencies directs Run() to not include dependencies
	// of bundles included in the diff if true.
	SkipDependencies bool

	IncludeConfig DiffIncludeConfig
	// IncludeAdditively catalog objects specified in IncludeConfig.
	IncludeAdditively bool
	// HeadsOnly is the mode that selects the head of the channels only.
	HeadsOnly bool

	Logger *logrus.Entry
}

func (diff Diff) Run(ctx context.Context) (*declcfg.DeclarativeConfig, error) {
	if err := diff.validate(); err != nil {
		return nil, err
	}

	// Disallow bundle refs.
	mask := RefDCDir | RefDCImage | RefSqliteFile | RefSqliteImage

	// Heads-only mode does not require an old ref, so there may be nothing to render.
	var oldModel model.Model
	if len(diff.OldRefs) != 0 {
		oldRender := Render{Refs: diff.OldRefs, Registry: diff.Registry, AllowedRefMask: mask}
		oldCfg, err := oldRender.Run(ctx)
		if err != nil {
			if errors.Is(err, ErrNotAllowed) {
				return nil, fmt.Errorf("%w (diff does not permit direct bundle references)", err)
			}
			return nil, fmt.Errorf("error rendering old refs: %v", err)
		}
		oldModel, err = declcfg.ConvertToModel(*oldCfg)
		if err != nil {
			return nil, fmt.Errorf("error converting old declarative config to model: %v", err)
		}
	}

	newRender := Render{Refs: diff.NewRefs, Registry: diff.Registry, AllowedRefMask: mask}
	newCfg, err := newRender.Run(ctx)
	if err != nil {
		if errors.Is(err, ErrNotAllowed) {
			return nil, fmt.Errorf("%w (diff does not permit direct bundle references)", err)
		}
		return nil, fmt.Errorf("error rendering new refs: %v", err)
	}
	newModel, err := declcfg.ConvertToModel(*newCfg)
	if err != nil {
		return nil, fmt.Errorf("error converting new declarative config to model: %v", err)
	}

	g := &declcfg.DiffGenerator{
		Logger:            diff.Logger,
		SkipDependencies:  diff.SkipDependencies,
		Includer:          convertIncludeConfigToIncluder(diff.IncludeConfig),
		IncludeAdditively: diff.IncludeAdditively,
		HeadsOnly:         diff.HeadsOnly,
	}
	diffModel, err := g.Run(oldModel, newModel)
	if err != nil {
		return nil, fmt.Errorf("error generating diff: %v", err)
	}

	cfg := declcfg.ConvertFromModel(diffModel)
	return &cfg, nil
}

func (p Diff) validate() error {
	if len(p.NewRefs) == 0 {
		return fmt.Errorf("no new refs to diff")
	}
	return nil
}

// DiffIncludeConfig configures Diff.Run() to include a set of packages,
// channels, and/or bundles/versions in the output DeclarativeConfig.
// These override other diff mechanisms. For example, if running in
// heads-only mode but package "foo" channel "stable" is specified,
// the entire "stable" channel (all channel bundles) is added to the output.
type DiffIncludeConfig struct {
	// Packages to include.
	Packages []DiffIncludePackage `json:"packages" yaml:"packages"`
}

// DiffIncludePackage contains a name (required) and channels and/or versions
// (optional) to include in the diff. The full package is only included if no channels
// or versions are specified.
type DiffIncludePackage struct {
	// Name of package.
	Name string `json:"name" yaml:"name"`
	// Channels to include.
	Channels []DiffIncludeChannel `json:"channels,omitempty" yaml:"channels,omitempty"`
	// Versions to include. All channels containing these versions
	// are parsed for an upgrade graph.
	Versions []semver.Version `json:"versions,omitempty" yaml:"versions,omitempty"`
	// Bundles are bundle names to include. All channels containing these bundles
	// are parsed for an upgrade graph.
	// Set this field only if the named bundle has no semantic version metadata.
	Bundles []string `json:"bundles,omitempty" yaml:"bundles,omitempty"`
	// Semver range of versions to include. All channels containing these versions
	// are parsed for an upgrade graph. If the channels don't contain these versions,
	// they will be ignored. This range can only be used with package exclusively
	// and cannot combined with `Range` in `DiffIncludeChannel`.
	// Range setting is mutually exclusive with channel versions/bundles/range settings.
	Range string `json:"range,omitempty" yaml:"range,omitempty"`
}

// DiffIncludeChannel contains a name (required) and versions (optional)
// to include in the diff. The full channel is only included if no versions are specified.
type DiffIncludeChannel struct {
	// Name of channel.
	Name string `json:"name" yaml:"name"`
	// Versions to include.
	Versions []semver.Version `json:"versions,omitempty" yaml:"versions,omitempty"`
	// Bundles are bundle names to include.
	// Set this field only if the named bundle has no semantic version metadata.
	Bundles []string `json:"bundles,omitempty" yaml:"bundles,omitempty"`
	// Semver range of versions to include in the channel. If the channel don't contain
	// these versions, an error will be raised. This range can only be used with
	// channel exclusively and cannot combined with `Range` in `DiffIncludePackage`.
	// Range setting is mutually exclusive with Versions and Bundles settings.
	Range string `json:"range,omitempty" yaml:"range,omitempty"`
}

// LoadDiffIncludeConfig loads a (YAML or JSON) DiffIncludeConfig from r.
func LoadDiffIncludeConfig(r io.Reader) (c DiffIncludeConfig, err error) {
	dec := yaml.NewYAMLOrJSONDecoder(r, 8)
	if err := dec.Decode(&c); err != nil {
		return DiffIncludeConfig{}, err
	}

	if len(c.Packages) == 0 {
		return c, fmt.Errorf("must specify at least one package in include config")
	}

	var errs []error
	for pkgI, pkg := range c.Packages {
		if pkg.Name == "" {
			errs = append(errs, fmt.Errorf("package at index %v requires a name", pkgI))
			continue
		}
		if pkg.Range != "" && (len(pkg.Versions) != 0 || len(pkg.Bundles) != 0) {
			errs = append(errs, fmt.Errorf("package %q contains invalid settings: range and versions and/or bundles are mutually exclusive", pkg.Name))
		}
		if pkg.Range != "" {
			_, err := semver.ParseRange(pkg.Range)
			if err != nil {
				errs = append(errs, fmt.Errorf("package %q has an invalid version range %s", pkg.Name, pkg.Range))
			}
		}
		for chI, ch := range pkg.Channels {
			if ch.Name == "" {
				errs = append(errs, fmt.Errorf("package %s: channel at index %v requires a name", pkg.Name, chI))
				continue
			}
			if ch.Range == "" {
				continue
			}
			if ch.Range != "" && (len(ch.Versions) != 0 || len(ch.Bundles) != 0) {
				errs = append(errs, fmt.Errorf("package %q: channel %q contains invalid settings: range and versions and/or bundles are mutually exclusive", pkg.Name, ch.Name))
			}
			if pkg.Range != "" && ch.Range != "" {
				errs = append(errs, fmt.Errorf("version range settings in package %q and in channel %q must be mutually exclusive", pkg.Name, ch.Name))
			}
			_, err := semver.ParseRange(ch.Range)
			if err != nil {
				errs = append(errs, fmt.Errorf("package %s: channel %q has an invalid version range %s", pkg.Name, ch.Name, pkg.Range))
			}
		}
	}
	return c, utilerrors.NewAggregate(errs)
}

func convertIncludeConfigToIncluder(c DiffIncludeConfig) (includer declcfg.DiffIncluder) {
	includer.Packages = make([]declcfg.DiffIncludePackage, len(c.Packages))
	for pkgI, cpkg := range c.Packages {
		pkg := &includer.Packages[pkgI]
		pkg.Name = cpkg.Name
		pkg.AllChannels.Versions = cpkg.Versions
		pkg.AllChannels.Bundles = cpkg.Bundles
		if cpkg.Range != "" {
			pkg.Range, _ = semver.ParseRange(cpkg.Range)
		}

		if len(cpkg.Channels) != 0 {
			pkg.Channels = make([]declcfg.DiffIncludeChannel, len(cpkg.Channels))
			for chI, cch := range cpkg.Channels {
				ch := &pkg.Channels[chI]
				ch.Name = cch.Name
				ch.Versions = cch.Versions
				ch.Bundles = cch.Bundles
				if cch.Range != "" {
					ch.Range, _ = semver.ParseRange(cch.Range)
				}
			}
		}
	}
	return includer
}
