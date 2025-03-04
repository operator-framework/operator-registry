package migrations

import (
	"encoding/json"
	"fmt"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func v2(cfg *declcfg.DeclarativeConfig) error {
	channelHeads, err := getChannelHeads(cfg)
	if err != nil {
		return fmt.Errorf("could not get channel heads: %w", err)
	}

	for _, pkgV1 := range cfg.Packages {
		cfg.PackageV2s = append(cfg.PackageV2s, packageToPackageV2(pkgV1))
		if pkgIconV2 := packageToPackageIconV2(pkgV1); pkgIconV2 != nil {
			cfg.PackageV2Icons = append(cfg.PackageV2Icons, *pkgIconV2)
		}
		pkgMetaV2, err := packageToPackageMetadataV2(pkgV1, channelHeads[channelKey{Package: pkgV1.Name, Channel: pkgV1.DefaultChannel}])
		if err != nil {
			return fmt.Errorf("could not get package metadata: %w", err)
		}
		cfg.PackageV2Metadatas = append(cfg.PackageV2Metadatas, *pkgMetaV2)
	}
	cfg.Packages = nil
	cfg.Bundles = nil
	return nil

	//
	//
	//uniqueBundles := map[string]*model.Bundle{}
	//m, err := declcfg.ConvertToModel(*cfg)
	//if err != nil {
	//	return err
	//}
	//cfg.Packages = nil
	//cfg.Bundles = nil
	//
	//for _, pkg := range m {
	//	head, err := pkg.DefaultChannel.Head()
	//	if err != nil {
	//		return err
	//	}
	//
	//	pkgV2 := packageToPackageV2(pkg)
	//
	//	if head.PropertiesP == nil || len(head.PropertiesP.CSVMetadatas) == 0 {
	//		return fmt.Errorf("no CSV metadata defined for package %s", pkg.Name)
	//	}
	//	csvMetadata := head.PropertiesP.CSVMetadatas[0]
	//
	//	packageAnnotations := map[string]string{
	//		"operators.openshift.io/capabilities":                          csvMetadata.Annotations["capabilities"],
	//		"operators.openshift.io/categories":                            csvMetadata.Annotations["categories"],
	//		"olm.operatorframework.io/olmv0-compatibility-default-channel": pkg.DefaultChannel.Name,
	//
	//		// TODO: these really probably belong at the bundle metadata level
	//		"operators.openshift.io/infrastructure-features": csvMetadata.Annotations["operators.openshift.io/infrastructure-features"],
	//		"operators.openshift.io/valid-subscription":      csvMetadata.Annotations["operators.openshift.io/valid-subscription"],
	//	}
	//
	//	v2p := declcfg.PackageV2{
	//		Schema:  declcfg.SchemaPackageV2,
	//		Package: pkg.Name,
	//	}
	//	cfg.PackageV2s = append(cfg.PackageV2s, v2p)
	//
	//	v2pkgMeta := declcfg.PackageV2Metadata{
	//		Schema:  declcfg.SchemaPackageV2,
	//		Package: pkg.Name,
	//
	//		DisplayName:      csvMetadata.DisplayName,
	//		ShortDescription: csvMetadata.Annotations["description"],
	//		LongDescription:  csvMetadata.Description,
	//		Keywords:         csvMetadata.Keywords,
	//		Links:            csvMetadata.Links,
	//		Provider:         csvMetadata.Provider,
	//		Maintainers:      csvMetadata.Maintainers,
	//		Annotations:      packageAnnotations,
	//	}
	//	cfg.PackageV2Metadatas = append(cfg.PackageV2Metadatas, v2pkgMeta)
	//
	//	if pkg.Icon != nil {
	//		v2i := declcfg.PackageV2Icon{
	//			Schema:    "olm.package.icon",
	//			Package:   pkg.Name,
	//			Data:      pkg.Icon.Data,
	//			MediaType: pkg.Icon.MediaType,
	//		}
	//		cfg.PackageV2Icons = append(cfg.PackageV2Icons, v2i)
	//	}
	//
	//	for _, ch := range pkg.Channels {
	//		for _, b := range ch.Bundles {
	//			uniqueBundles[b.Name] = b
	//		}
	//	}
	//}
	//
	//bundleRenames := make(map[string]string, len(uniqueBundles))
	//bundleVersions := make(map[string]semver.Version, len(uniqueBundles))
	//for _, b := range uniqueBundles {
	//	newName := fmt.Sprintf("%s.v%s", b.Package.Name, b.Version)
	//	bundleRenames[b.Name] = newName
	//	bundleVersions[b.Name] = b.Version
	//
	//	v2b := declcfg.BundleV2{
	//		Schema:  declcfg.SchemaBundleV2,
	//		Package: b.Package.Name,
	//		Name:    newName,
	//
	//		Version:   b.Version.String(),
	//		Release:   0,
	//		Reference: fmt.Sprintf("docker://%s", b.Image),
	//
	//		Constraints: map[string][]json.RawMessage{},
	//		Properties:  map[string][]json.RawMessage{},
	//	}
	//	for _, p := range b.Properties {
	//		if p.Type == property.TypePackage || p.Type == property.TypeBundleObject {
	//			continue
	//		}
	//		if isContraint(p) {
	//			v2b.Constraints[p.Type] = append(v2b.Constraints[p.Type], p.Value)
	//		} else {
	//			v2b.Properties[p.Type] = append(v2b.Properties[p.Type], p.Value)
	//		}
	//	}
	//	cfg.BundleV2s = append(cfg.BundleV2s, v2b)
	//
	//	if len(b.RelatedImages) > 0 {
	//		refs := sets.New[string]()
	//		for _, ri := range b.RelatedImages {
	//			if ri.Image != "" {
	//				refs.Insert(fmt.Sprintf("docker://%s", ri.Image))
	//			}
	//		}
	//		if refs.Len() > 0 {
	//			cfg.BundleV2RelatedReferences = append(cfg.BundleV2RelatedReferences, declcfg.BundleV2RelatedReferences{
	//				Schema:     declcfg.SchemaBundleV2RelatedReferences,
	//				Package:    b.Package.Name,
	//				Name:       newName,
	//				References: sets.List(refs),
	//			})
	//		}
	//	}
	//
	//	if b.PropertiesP != nil && len(b.PropertiesP.CSVMetadatas) > 0 {
	//		csvMetadata := b.PropertiesP.CSVMetadatas[0]
	//
	//		desiredAnnotations := map[string]string{
	//			"createdAt":      "operators.openshift.io/creationTimestamp",
	//			"repository":     "operators.openshift.io/repository",
	//			"support":        "operators.openshift.io/support",
	//			"containerImage": "operators.openshift.io/image",
	//		}
	//
	//		bundleAnnotations := map[string]string{}
	//		for fromKey, toKey := range desiredAnnotations {
	//			if value, ok := csvMetadata.Annotations[fromKey]; ok {
	//				bundleAnnotations[toKey] = value
	//			}
	//		}
	//		if len(bundleAnnotations) > 0 {
	//			v2BundleMetadata := declcfg.BundleV2Metadata{
	//				Schema:      declcfg.SchemaBundleV2Metadata,
	//				Package:     b.Package.Name,
	//				Name:        newName,
	//				Annotations: bundleAnnotations,
	//			}
	//			cfg.BundleV2Metadatas = append(cfg.BundleV2Metadatas, v2BundleMetadata)
	//		}
	//	}
	//}
	//
	//}
	//cfg.Channels = nil
	//
	//for depIdx := range cfg.Deprecations {
	//	for entryIdx := range cfg.Deprecations[depIdx].Entries {
	//		e := &cfg.Deprecations[depIdx].Entries[entryIdx]
	//		switch e.Reference.Schema {
	//		case declcfg.SchemaPackage:
	//			e.Reference.Schema = declcfg.SchemaPackageV2
	//		case declcfg.SchemaBundle:
	//			e.Reference.Schema = declcfg.SchemaBundleV2
	//		}
	//	}
	//}
	//return nil
}

func isContraint(p property.Property) bool {
	switch p.Type {
	case property.TypeConstraint, property.TypeGVKRequired, property.TypePackageRequired:
		return true
	}
	return false
}

type bundleKey struct {
	Package, Bundle string
}

type channelKey struct {
	Package, Channel string
}

func getChannelHeads(cfg *declcfg.DeclarativeConfig) (map[channelKey]*declcfg.Bundle, error) {
	channelHeadsKeys := map[bundleKey][]channelKey{}
	for _, ch := range cfg.Channels {
		head, err := getChannelHead(ch)
		if err != nil {
			return nil, fmt.Errorf("invalid channel %q in package %q: failed to get channel head: %w", ch.Name, ch.Package, err)
		}
		bk := bundleKey{Package: ch.Package, Bundle: head}
		ck := channelKey{Package: ch.Package, Channel: ch.Name}
		channelHeadsKeys[bk] = append(channelHeadsKeys[bk], ck)
	}
	channelHeads := map[channelKey]*declcfg.Bundle{}
	for i, b := range cfg.Bundles {
		bk := bundleKey{Package: b.Package, Bundle: b.Name}
		if channelKeys, ok := channelHeadsKeys[bk]; ok {
			for _, ck := range channelKeys {
				channelHeads[ck] = &cfg.Bundles[i]
			}
		}
	}
	return channelHeads, nil
}

func getChannelHead(ch declcfg.Channel) (string, error) {
	incoming := map[string]int{}
	for _, b := range ch.Entries {
		if b.Replaces != "" {
			incoming[b.Replaces]++
		}
		for _, skip := range b.Skips {
			incoming[skip]++
		}
	}
	heads := sets.New[string]()
	for _, b := range ch.Entries {
		if _, ok := incoming[b.Name]; !ok {
			heads.Insert(b.Name)
		}
	}
	if heads.Len() == 0 {
		return "", fmt.Errorf("no channel head found in graph")
	}
	if len(heads) > 1 {
		return "", fmt.Errorf("multiple channel heads found in graph: %s", strings.Join(sets.List(heads), ", "))
	}
	return heads.UnsortedList()[0], nil
}

func packageToPackageV2(in declcfg.Package) declcfg.PackageV2 {
	return declcfg.PackageV2{
		Schema:  declcfg.SchemaPackageV2,
		Package: in.Name,
	}
}
func packageToPackageMetadataV2(in declcfg.Package, defaultChannelHead *declcfg.Bundle) (*declcfg.PackageV2Metadata, error) {
	var csvMetadata *property.CSVMetadata
	if defaultChannelHead.CsvJSON != "" {
		var csvMetaTry property.CSVMetadata
		if err := json.Unmarshal([]byte(defaultChannelHead.CsvJSON), &csvMetaTry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal default channel head CSV: %w", err)
		}
		csvMetadata = &csvMetaTry
	} else {
		for _, p := range defaultChannelHead.Properties {
			var csvMetaTry property.CSVMetadata
			if p.Type == property.TypeCSVMetadata {
				if err := json.Unmarshal(p.Value, &csvMetaTry); err != nil {
					return nil, fmt.Errorf("failed to unmarshal default channel head property %q: %w", p.Type, err)
				}
				csvMetadata = &csvMetaTry
				break
			}
			if p.Type == property.TypeBundleObject {
				var pom v1.PartialObjectMetadata
				if err := json.Unmarshal(p.Value, &pom); err != nil {
					return nil, fmt.Errorf("failed to unmarshal default channel head property %q to determine object kind: %w", p.Type, err)
				}
				if pom.Kind != "ClusterServiceVersion" {
					continue
				}
				var csv v1alpha1.ClusterServiceVersion
				if err := json.Unmarshal(p.Value, &csv); err != nil {
					return nil, fmt.Errorf("failed to unmarshal default channel head property %q as CSV: %w", p.Type, err)
				}
				csvMetadataProp := property.MustBuildCSVMetadata(csv)
				if err := json.Unmarshal(csvMetadataProp.Value, &csvMetaTry); err != nil {
					return nil, fmt.Errorf("failed to unmarshal default channel head property %q as CSV Metadata property: %w", p.Type, err)
				}
				csvMetadata = &csvMetaTry
			}
		}
	}
	if csvMetadata == nil {
		return nil, nil
	}

	packageAnnotations := map[string]string{
		"operators.openshift.io/capabilities":                          csvMetadata.Annotations["capabilities"],
		"operators.openshift.io/categories":                            csvMetadata.Annotations["categories"],
		"olm.operatorframework.io/olmv0-compatibility-default-channel": in.DefaultChannel,

		// TODO: these really probably belong at the bundle metadata level
		"operators.openshift.io/infrastructure-features": csvMetadata.Annotations["operators.openshift.io/infrastructure-features"],
		"operators.openshift.io/valid-subscription":      csvMetadata.Annotations["operators.openshift.io/valid-subscription"],
	}

	return &declcfg.PackageV2Metadata{
		Schema:  declcfg.SchemaPackageV2Metadata,
		Package: in.Name,

		DisplayName:      csvMetadata.DisplayName,
		ShortDescription: csvMetadata.Annotations["description"],
		LongDescription:  csvMetadata.Description,
		Keywords:         csvMetadata.Keywords,
		Links:            csvMetadata.Links,
		Provider:         csvMetadata.Provider,
		Maintainers:      csvMetadata.Maintainers,
		Annotations:      packageAnnotations,
	}, nil
}
func packageToPackageIconV2(in declcfg.Package) *declcfg.PackageV2Icon {
	if in.Icon == nil {
		return nil
	}
	return &declcfg.PackageV2Icon{
		Schema:    declcfg.SchemaPackageV2Icon,
		Package:   in.Name,
		Data:      in.Icon.Data,
		MediaType: in.Icon.MediaType,
	}
}
