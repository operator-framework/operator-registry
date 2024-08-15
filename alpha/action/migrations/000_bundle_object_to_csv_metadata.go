package migrations

import (
	"encoding/json"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func bundleObjectToCSVMetadata(cfg *declcfg.DeclarativeConfig) error {
	convertBundleObjectToCSVMetadata := func(b *declcfg.Bundle) error {
		if b.Image == "" || b.CsvJSON == "" {
			return nil
		}

		var csv v1alpha1.ClusterServiceVersion
		if err := json.Unmarshal([]byte(b.CsvJSON), &csv); err != nil {
			return err
		}

		props := b.Properties[:0]
		for _, p := range b.Properties {
			switch p.Type {
			case property.TypeBundleObject:
				// Get rid of the bundle objects
			case property.TypeCSVMetadata:
				// If this bundle already has a CSV metadata
				// property, we won't mutate the bundle at all.
				return nil
			default:
				// Keep all of the other properties
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
