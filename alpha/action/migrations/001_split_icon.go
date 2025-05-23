package migrations

import (
	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

func splitIcon(cfg *declcfg.DeclarativeConfig) error {
	splitIconFromPackage := func(p *declcfg.Package) error {
		if p.Icon == nil {
			return nil
		}

		cfg.Icons = append(cfg.Icons, declcfg.Icon{
			Schema:    declcfg.SchemaIcon,
			Package:   p.Name,
			MediaType: p.Icon.MediaType,
			Data:      p.Icon.Data,
		})
		p.Icon = nil
		return nil
	}

	for i := range cfg.Packages {
		if err := splitIconFromPackage(&cfg.Packages[i]); err != nil {
			return err
		}
	}
	return nil
}
