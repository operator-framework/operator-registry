package migrations

import (
	"encoding/json"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func bundleObjectToCSVMetadata(cfg *declcfg.DeclarativeConfig) error {
	convertBundleObjectToCSVMetadata := func(b *declcfg.Bundle) error {
		foundCSVMetadata := false
		for pi := range b.Properties {
			if b.Properties[pi].Type != property.TypeCSVMetadata {
				continue
			}

			var metadata property.CSVMetadata
			if err := json.Unmarshal(b.Properties[pi].Value, &metadata); err != nil {
				return err
			}
			b.Properties[pi] = property.MustBuild(&metadata)
			foundCSVMetadata = true
		}
		if foundCSVMetadata {
			// If this bundle already has a CSV metadata property, don't mutate
			// anything other than minifying that property.
			return nil
		}
		if b.Image == "" || b.CsvJSON == "" {
			return nil
		}

		var csv v1alpha1.ClusterServiceVersion
		if err := json.Unmarshal([]byte(b.CsvJSON), &csv); err != nil {
			return err
		}

		props := b.Properties[:0]
		for _, p := range b.Properties {
			if p.Type != property.TypeBundleObject {
				props = append(props, p)
			}
		}
		b.Properties = append(props, property.MustBuildCSVMetadata(csv))
		return nil
	}

	for bi := range cfg.Bundles {
		if err := convertBundleObjectToCSVMetadata(&cfg.Bundles[bi]); err != nil {
			return err
		}
	}
	return nil
}
