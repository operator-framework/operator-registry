package declcfg

import (
	"cmp"
	"slices"

	"k8s.io/apimachinery/pkg/util/sets"
)

func SortByPackage(cfg *DeclarativeConfig) {
	slices.SortFunc(cfg.Packages, func(a, b Package) int { return cmp.Compare(a.Name, b.Name) })
	slices.SortFunc(cfg.Channels, func(a, b Channel) int { return cmp.Compare(a.Package, b.Package) })
	slices.SortFunc(cfg.Bundles, func(a, b Bundle) int { return cmp.Compare(a.Package, b.Package) })

	slices.SortFunc(cfg.PackageV2s, func(a, b PackageV2) int { return cmp.Compare(a.Package, b.Package) })
	slices.SortFunc(cfg.PackageV2Icons, func(a, b PackageV2Icon) int { return cmp.Compare(a.Package, b.Package) })
	slices.SortFunc(cfg.PackageV2Metadatas, func(a, b PackageV2Metadata) int { return cmp.Compare(a.Package, b.Package) })

	slices.SortFunc(cfg.ChannelV2s, func(a, b ChannelV2) int { return cmp.Compare(a.Package, b.Package) })

	slices.SortFunc(cfg.BundleV2s, func(a, b BundleV2) int { return cmp.Compare(a.Package, b.Package) })
	slices.SortFunc(cfg.BundleV2Metadatas, func(a, b BundleV2Metadata) int { return cmp.Compare(a.Package, b.Package) })
	slices.SortFunc(cfg.BundleV2RelatedReferences, func(a, b BundleV2RelatedReferences) int { return cmp.Compare(a.Package, b.Package) })

	slices.SortFunc(cfg.Deprecations, func(a, b Deprecation) int { return cmp.Compare(a.Package, b.Package) })
	slices.SortFunc(cfg.Others, func(a, b Meta) int { return cmp.Compare(a.Package, b.Package) })
}

func OrganizeByPackage(cfg DeclarativeConfig) map[string]DeclarativeConfig {
	pkgNames := sets.New[string]()
	packagesByName := map[string][]Package{}
	for _, p := range cfg.Packages {
		pkgName := p.Name
		pkgNames.Insert(pkgName)
		packagesByName[pkgName] = append(packagesByName[pkgName], p)
	}
	channelsByPackage := map[string][]Channel{}
	for _, c := range cfg.Channels {
		pkgName := c.Package
		pkgNames.Insert(pkgName)
		channelsByPackage[pkgName] = append(channelsByPackage[pkgName], c)
	}
	bundlesByPackage := map[string][]Bundle{}
	for _, b := range cfg.Bundles {
		pkgName := b.Package
		pkgNames.Insert(pkgName)
		bundlesByPackage[pkgName] = append(bundlesByPackage[pkgName], b)
	}
	packageV2sByName := map[string][]PackageV2{}
	for _, p := range cfg.PackageV2s {
		pkgName := p.Package
		pkgNames.Insert(pkgName)
		packageV2sByName[pkgName] = append(packageV2sByName[pkgName], p)
	}
	packageIconsByPackage := map[string][]PackageV2Icon{}
	for _, pi := range cfg.PackageV2Icons {
		pkgName := pi.Package
		pkgNames.Insert(pkgName)
		packageIconsByPackage[pkgName] = append(packageIconsByPackage[pkgName], pi)
	}
	packageMetadataV2sByPackage := map[string][]PackageV2Metadata{}
	for _, pm := range cfg.PackageV2Metadatas {
		pkgName := pm.Package
		pkgNames.Insert(pkgName)
		packageMetadataV2sByPackage[pkgName] = append(packageMetadataV2sByPackage[pkgName], pm)
	}
	channelV2sByPackage := map[string][]ChannelV2{}
	for _, u := range cfg.ChannelV2s {
		pkgName := u.Package
		pkgNames.Insert(pkgName)
		channelV2sByPackage[pkgName] = append(channelV2sByPackage[pkgName], u)
	}
	bundleV2sByPackage := map[string][]BundleV2{}
	for _, b := range cfg.BundleV2s {
		pkgName := b.Package
		pkgNames.Insert(pkgName)
		bundleV2sByPackage[pkgName] = append(bundleV2sByPackage[pkgName], b)
	}
	bundleV2RelatedReferencesByPackage := map[string][]BundleV2RelatedReferences{}
	for _, b := range cfg.BundleV2RelatedReferences {
		pkgName := b.Package
		pkgNames.Insert(pkgName)
		bundleV2RelatedReferencesByPackage[pkgName] = append(bundleV2RelatedReferencesByPackage[pkgName], b)
	}
	bundleV2MetadatasByPackage := map[string][]BundleV2Metadata{}
	for _, b := range cfg.BundleV2Metadatas {
		pkgName := b.Package
		pkgNames.Insert(pkgName)
		bundleV2MetadatasByPackage[pkgName] = append(bundleV2MetadatasByPackage[pkgName], b)
	}
	othersByPackage := map[string][]Meta{}
	for _, o := range cfg.Others {
		pkgName := o.Package
		pkgNames.Insert(pkgName)
		othersByPackage[pkgName] = append(othersByPackage[pkgName], o)
	}
	deprecationsByPackage := map[string][]Deprecation{}
	for _, d := range cfg.Deprecations {
		pkgName := d.Package
		pkgNames.Insert(pkgName)
		deprecationsByPackage[pkgName] = append(deprecationsByPackage[pkgName], d)
	}

	fbcsByPackageName := make(map[string]DeclarativeConfig, len(pkgNames))
	for _, pkgName := range sets.List(pkgNames) {
		fbcsByPackageName[pkgName] = DeclarativeConfig{
			Packages: packagesByName[pkgName],
			Channels: channelsByPackage[pkgName],
			Bundles:  bundlesByPackage[pkgName],

			PackageV2s:                packageV2sByName[pkgName],
			PackageV2Icons:            packageIconsByPackage[pkgName],
			PackageV2Metadatas:        packageMetadataV2sByPackage[pkgName],
			ChannelV2s:                channelV2sByPackage[pkgName],
			BundleV2s:                 bundleV2sByPackage[pkgName],
			BundleV2RelatedReferences: bundleV2RelatedReferencesByPackage[pkgName],
			BundleV2Metadatas:         bundleV2MetadatasByPackage[pkgName],

			Deprecations: deprecationsByPackage[pkgName],
			Others:       othersByPackage[pkgName],
		}
	}
	return fbcsByPackageName
}
