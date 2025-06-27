package migrations

import (
	"encoding/json"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

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

		// Skip objects that have no olm.csv.metadata property
		if csvMetadata == nil {
			continue
		}

		// Keep track of the metadata from the highest versioned bundle from each package.
		cur, ok := metadataByPackage[b.Package]
		if !ok || compareRegistryV1Semver(cur.version, b.Version) < 0 {
			metadataByPackage[b.Package] = promotedCSVMetadata(b.Version, csvMetadata)
		}

		// Delete the package-level metadata from the olm.csv.metadata object and
		// update the bundle properties to use the new slimmed-down revision of it.
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

	// Update each olm.package object to include the metadata we extracted from
	// bundles in the first loop.
	for i := range cfg.Packages {
		pkg := &cfg.Packages[i]
		metadata, ok := metadataByPackage[pkg.Name]
		if !ok {
			continue
		}
		pkg.DisplayName = metadata.displayName
		pkg.ShortDescription = shortenDescription(metadata.shortDescription)
		if metadata.provider.Name != "" || metadata.provider.URL != "" {
			pkg.Provider = &metadata.provider
		}
		pkg.Maintainers = metadata.maintainers
		pkg.Links = metadata.links
		pkg.Keywords = slices.DeleteFunc(metadata.keywords, func(s string) bool {
			// Delete keywords that are empty strings
			return s == ""
		})
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

func shortenDescription(input string) string {
	const maxLen = 256
	input = strings.TrimSpace(input)

	// If the input is already under the limit return it.
	if utf8.RuneCountInString(input) <= maxLen {
		return input
	}

	// Chop off everything after the first paragraph.
	if idx := strings.Index(input, "\n\n"); idx != -1 {
		input = strings.TrimSpace(input[:idx])
	}

	// If we're _now_ under the limit, return the first paragraph.
	if utf8.RuneCountInString(input) <= maxLen {
		return input
	}

	// If the first paragraph is still over the limit, we'll have to truncate.
	// We'll truncate at the last word boundary that still allows an ellipsis
	// to fit within the maximum length. But if there are no word boundaries
	// (veeeeery unlikely), we'll hard truncate mid-word.
	input = input[:maxLen-3]
	if truncatedIdx := strings.LastIndexFunc(input, unicode.IsSpace); truncatedIdx != -1 {
		return input[:truncatedIdx] + "..."
	}
	return input + "..."
}
