package migrations

import (
	"encoding/json"
	"slices"

	"github.com/Masterminds/semver/v3"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func promoteBundleVersion(cfg *declcfg.DeclarativeConfig) error {
	promoteVersion := func(b *declcfg.Bundle) error {
		// Promote the version from the olm.package property to the bundle field.
		for _, p := range b.Properties {
			if p.Type != property.TypePackage {
				continue
			}
			var pkg property.Package
			if err := json.Unmarshal(p.Value, &pkg); err != nil {
				return err
			}
			version, err := semver.StrictNewVersion(pkg.Version)
			if err != nil {
				return err
			}
			b.Version = version
		}

		// Delete the olm.package properties
		b.Properties = slices.DeleteFunc(b.Properties, func(p property.Property) bool {
			return p.Type == property.TypePackage
		})
		return nil
	}

	for i := range cfg.Bundles {
		if err := promoteVersion(&cfg.Bundles[i]); err != nil {
			return err
		}
	}
	return nil
}
