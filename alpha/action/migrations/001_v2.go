package migrations

import (
	"encoding/json"
	"fmt"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/model"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func v2(cfg *declcfg.DeclarativeConfig) error {
	uniqueBundles := map[string]*model.Bundle{}
	m, err := declcfg.ConvertToModel(*cfg)
	if err != nil {
		return err
	}
	cfg.Packages = nil
	cfg.Bundles = nil

	for _, pkg := range m {
		head, err := pkg.DefaultChannel.Head()
		if err != nil {
			return err
		}

		if head.PropertiesP == nil || len(head.PropertiesP.CSVMetadatas) == 0 {
			return fmt.Errorf("no CSV metadata defined for package %s", pkg.Name)
		}
		csvMetadata := head.PropertiesP.CSVMetadatas[0]

		packageAnnotations := map[string]string{
			"operators.openshift.io/capabilities":                          csvMetadata.Annotations["capabilities"],
			"operators.openshift.io/categories":                            csvMetadata.Annotations["categories"],
			"operators.openshift.io/infrastructure-features":               csvMetadata.Annotations["operators.openshift.io/infrastructure-features"],
			"operators.openshift.io/valid-subscription":                    csvMetadata.Annotations["operators.openshift.io/valid-subscription"],
			"olm.operatorframework.io/olmv0-compatibility-default-channel": pkg.DefaultChannel.Name,
		}

		v2p := declcfg.PackageV2{
			Schema:           "olm.package.v2",
			Package:          pkg.Name,
			DisplayName:      csvMetadata.DisplayName,
			ShortDescription: csvMetadata.Annotations["description"],
			LongDescription:  csvMetadata.Description,
			Keywords:         csvMetadata.Keywords,
			Links:            csvMetadata.Links,
			Provider:         csvMetadata.Provider,
			Maintainers:      csvMetadata.Maintainers,
			Annotations:      packageAnnotations,
		}
		cfg.PackageV2s = append(cfg.PackageV2s, v2p)

		if pkg.Icon != nil {
			v2i := declcfg.PackageIcon{
				Schema:    "olm.package.icon",
				Package:   pkg.Name,
				Data:      pkg.Icon.Data,
				MediaType: pkg.Icon.MediaType,
			}
			cfg.PackageIcons = append(cfg.PackageIcons, v2i)
		}

		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				uniqueBundles[b.Name] = b
			}
		}
	}

	bundleRenames := make(map[string]string, len(uniqueBundles))
	for _, b := range uniqueBundles {
		newName := fmt.Sprintf("%s.v%s", b.Package.Name, b.Version)
		v2b := declcfg.BundleV2{
			Schema:  "olm.bundle.v2",
			Package: b.Package.Name,
			Name:    newName,

			Version:           b.Version.String(),
			Release:           0,
			Reference:         fmt.Sprintf("docker://%s", b.Image),
			RelatedReferences: make([]string, 0, len(b.RelatedImages)),

			Constraints: map[string][]json.RawMessage{},
			Properties:  map[string][]json.RawMessage{},
		}
		bundleRenames[b.Name] = newName

		for _, ri := range b.RelatedImages {
			v2b.RelatedReferences = append(v2b.RelatedReferences, fmt.Sprintf("docker://%s", ri.Image))
		}

		for _, p := range b.Properties {
			if p.Type == property.TypePackage || p.Type == property.TypeBundleObject {
				continue
			}
			if isContraint(p) {
				v2b.Constraints[p.Type] = append(v2b.Constraints[p.Type], p.Value)
			} else {
				v2b.Properties[p.Type] = append(v2b.Properties[p.Type], p.Value)
			}
		}

		if b.PropertiesP != nil && len(b.PropertiesP.CSVMetadatas) > 0 {
			csvMetadata := b.PropertiesP.CSVMetadatas[0]

			desiredAnnotations := map[string]string{
				"createdAt":      "operators.openshift.io/creationTimestamp",
				"repository":     "operators.openshift.io/repository",
				"support":        "operators.openshift.io/support",
				"containerImage": "operators.openshift.io/image",
			}

			bundleAnnotations := map[string]string{}
			for fromKey, toKey := range desiredAnnotations {
				if value, ok := csvMetadata.Annotations[fromKey]; ok {
					bundleAnnotations[toKey] = value
				}
			}
			v2b.Annotations = bundleAnnotations
		}
		cfg.BundleV2s = append(cfg.BundleV2s, v2b)
	}

	for i, ch := range cfg.Channels {
		for j, entry := range ch.Entries {
			if newName, ok := bundleRenames[entry.Name]; ok {
				cfg.Channels[i].Entries[j].Name = newName
			}
			if newName, ok := bundleRenames[entry.Replaces]; ok {
				cfg.Channels[i].Entries[j].Replaces = newName
			}
			for k, skip := range entry.Skips {
				if newName, ok := bundleRenames[skip]; ok {
					cfg.Channels[i].Entries[j].Skips[k] = newName
				}
			}
		}
	}

	for depIdx := range cfg.Deprecations {
		for entryIdx := range cfg.Deprecations[depIdx].Entries {
			e := &cfg.Deprecations[depIdx].Entries[entryIdx]
			switch e.Reference.Schema {
			case declcfg.SchemaPackage:
				e.Reference.Schema = declcfg.SchemaPackageV2
			case declcfg.SchemaBundle:
				e.Reference.Schema = declcfg.SchemaBundleV2
			}
		}
	}
	return nil
}

func isContraint(p property.Property) bool {
	switch p.Type {
	case property.TypeConstraint, property.TypeGVKRequired, property.TypePackageRequired:
		return true
	}
	return false
}
