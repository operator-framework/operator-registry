package migrations

import (
	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

func splitIcon(cfg *declcfg.DeclarativeConfig) error {
	splitIconFromPackage := func(p *declcfg.Package) error {
		if p.Icon == nil { //nolint:staticcheck
			return nil
		}

		// Make separate olm.icon object
		cfg.Icons = append(cfg.Icons, declcfg.Icon{
			Schema:    declcfg.SchemaIcon,
			Package:   p.Name,
			MediaType: p.Icon.MediaType, //nolint:staticcheck
			Data:      p.Icon.Data,      //nolint:staticcheck
		})

		// Delete original icon from olm.package object
		p.Icon = nil //nolint:staticcheck
		return nil
	}

	for i := range cfg.Packages {
		if err := splitIconFromPackage(&cfg.Packages[i]); err != nil {
			return err
		}
	}
	return nil
}
