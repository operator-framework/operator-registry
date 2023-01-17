package composite

import (
	"context"
	"fmt"
)

type BuilderMap map[string]Builder

type CatalogBuilderMap map[string]BuilderMap

type Veneer struct {
	CatalogBuilders CatalogBuilderMap
}

// TODO(everettraven): do we need the context here? If so, how should it be used?
func (v *Veneer) Render(ctx context.Context, config *CompositeConfig, validate bool) error {
	// TODO(everettraven): should we return aggregated errors?
	for _, component := range config.Components {
		if builderMap, ok := v.CatalogBuilders[component.Name]; ok {
			if builder, ok := builderMap[component.Strategy.Veneer.Schema]; ok {
				// run the builder corresponding to the schema
				err := builder.Build(component.Destination.Path, component.Strategy.Veneer)
				if err != nil {
					return fmt.Errorf("building component %q: %w", component.Name, err)
				}

				if validate {
					// run the validation for the builder
					err = builder.Validate(component.Destination.Path)
					if err != nil {
						return fmt.Errorf("validating component %q: %w", component.Name, err)
					}
				}
				// TODO(everettraven): Should we remove the built FBC if validation fails?
			} else {
				return fmt.Errorf("building component %q: no builder found for veneer schema %q", component.Name, component.Strategy.Veneer.Schema)
			}
		} else {
			allowedComponents := []string{}
			for k := range v.CatalogBuilders {
				allowedComponents = append(allowedComponents, k)
			}
			return fmt.Errorf("building component %q: component does not exist in the catalog configuration. Available components are: %s", component.Name, allowedComponents)
		}
	}
	return nil
}
