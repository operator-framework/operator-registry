package filter

import (
	"context"
	"slices"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

var KeepAllMetas = declcfg.MetaFilter(declcfg.MetaFilterFunc(func(meta *declcfg.Meta) bool { return true }))

func NewPackageFilter(keepPackages ...string) declcfg.CatalogFilter {
	return &packageFilter{keepPackages: sets.New[string](keepPackages...)}
}

type packageFilter struct {
	keepPackages sets.Set[string]
}

func (f *packageFilter) FilterCatalog(_ context.Context, fbc *declcfg.DeclarativeConfig) (*declcfg.DeclarativeConfig, error) {
	slices.DeleteFunc(fbc.Packages, func(pkg declcfg.Package) bool {
		return !f.keepPackages.Has(pkg.Name)
	})
	slices.DeleteFunc(fbc.Channels, func(channel declcfg.Channel) bool {
		return !f.keepPackages.Has(channel.Package)
	})
	slices.DeleteFunc(fbc.Bundles, func(bundle declcfg.Bundle) bool {
		return !f.keepPackages.Has(bundle.Package)
	})
	slices.DeleteFunc(fbc.Deprecations, func(deprecation declcfg.Deprecation) bool { return !f.keepPackages.Has(deprecation.Package) })
	slices.DeleteFunc(fbc.Others, func(other declcfg.Meta) bool {
		return !f.keepPackages.Has(other.Package)
	})
	return fbc, nil
}

func (f *packageFilter) KeepMeta(meta *declcfg.Meta) bool {
	packageName := meta.Package
	if meta.Schema == "olm.package" {
		packageName = meta.Name
	}
	return f.keepPackages.Has(packageName)
}
