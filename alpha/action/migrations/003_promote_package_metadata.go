package migrations

import (
	"encoding/json"

	"github.com/Masterminds/semver/v3"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func promotePackageMetadata(cfg *declcfg.DeclarativeConfig) error {
	metadataByPackage := map[string]promotedMetadata{}
	for i := range cfg.Bundles {
		b := &cfg.Bundles[i]

		csvMetadata, csvMetadataIdx, err := getCsvMetadata(b)
		if err != nil {
			return err
		}
		if csvMetadata == nil {
			continue
		}

		cur, ok := metadataByPackage[b.Package]
		if !ok || compareRegistryV1Semver(cur.version, b.Version) < 0 {
			metadataByPackage[b.Package] = promotedCSVMetadata(b.Version, csvMetadata)
		}

		csvMetadata.DisplayName = ""
		delete(csvMetadata.Annotations, "description")
		csvMetadata.Provider = v1alpha1.AppLink{}
		csvMetadata.Maintainers = nil
		csvMetadata.Links = nil
		csvMetadata.Keywords = nil

		newCSVMetadata, err := json.Marshal(csvMetadata)
		if err != nil {
			return err
		}
		b.Properties[csvMetadataIdx] = property.Property{
			Type:  property.TypeCSVMetadata,
			Value: newCSVMetadata,
		}
	}

	for i := range cfg.Packages {
		pkg := &cfg.Packages[i]
		metadata, ok := metadataByPackage[pkg.Name]
		if !ok {
			continue
		}
		pkg.DisplayName = metadata.displayName
		pkg.ShortDescription = metadata.shortDescription
		pkg.Provider = metadata.provider
		pkg.Maintainers = metadata.maintainers
		pkg.Links = metadata.links
		pkg.Keywords = metadata.keywords
	}
	return nil
}

func getCsvMetadata(b *declcfg.Bundle) (*property.CSVMetadata, int, error) {
	for i, p := range b.Properties {
		if p.Type != property.TypeCSVMetadata {
			continue
		}
		var csvMetadata property.CSVMetadata
		if err := json.Unmarshal(p.Value, &csvMetadata); err != nil {
			return nil, -1, err
		}
		return &csvMetadata, i, nil
	}
	return nil, -1, nil
}

func compareRegistryV1Semver(a, b *semver.Version) int {
	if v := a.Compare(b); v != 0 {
		return v
	}
	aPre := semver.New(0, 0, 0, a.Metadata(), "")
	bPre := semver.New(0, 0, 0, b.Metadata(), "")
	return aPre.Compare(bPre)
}

type promotedMetadata struct {
	version *semver.Version

	displayName      string
	shortDescription string
	provider         v1alpha1.AppLink
	maintainers      []v1alpha1.Maintainer
	links            []v1alpha1.AppLink
	keywords         []string
}

func promotedCSVMetadata(version *semver.Version, metadata *property.CSVMetadata) promotedMetadata {
	return promotedMetadata{
		version:          version,
		displayName:      metadata.DisplayName,
		shortDescription: metadata.Annotations["description"],
		provider:         metadata.Provider,
		maintainers:      metadata.Maintainers,
		links:            metadata.Links,
		keywords:         metadata.Keywords,
	}
}
