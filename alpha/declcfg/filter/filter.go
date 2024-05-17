package filter

import (
	"context"
	"slices"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

// NewPackageFilter returns a CatalogFilter that keeps package, channel, bundle, deprecation, and other
// meta objects for the given package names. All objects for all other packages are removed.
func NewPackageFilter(keepPackages ...string) declcfg.CatalogFilter {
	return &packageFilter{keepPackages: sets.New[string](keepPackages...)}
}

type packageFilter struct {
	keepPackages sets.Set[string]
}

func (f *packageFilter) FilterCatalog(_ context.Context, fbc *declcfg.DeclarativeConfig) (*declcfg.DeclarativeConfig, error) {
	if fbc == nil {
		return nil, nil
	}
	fbc.Packages = slices.DeleteFunc(fbc.Packages, func(pkg declcfg.Package) bool { return !f.keepPackages.Has(pkg.Name) })
	fbc.Channels = slices.DeleteFunc(fbc.Channels, func(channel declcfg.Channel) bool { return !f.keepPackages.Has(channel.Package) })
	fbc.Bundles = slices.DeleteFunc(fbc.Bundles, func(bundle declcfg.Bundle) bool { return !f.keepPackages.Has(bundle.Package) })
	fbc.Deprecations = slices.DeleteFunc(fbc.Deprecations, func(deprecation declcfg.Deprecation) bool { return !f.keepPackages.Has(deprecation.Package) })
	fbc.Others = slices.DeleteFunc(fbc.Others, func(other declcfg.Meta) bool { return !f.keepPackages.Has(other.Package) })
	return fbc, nil
}

func (f *packageFilter) KeepMeta(meta *declcfg.Meta) bool {
	packageName := meta.Package
	if meta.Schema == "olm.package" {
		packageName = meta.Name
	}
	return f.keepPackages.Has(packageName)
}
