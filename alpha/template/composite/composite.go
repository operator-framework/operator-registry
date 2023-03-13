package composite

import (
	"context"
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/image"
)

type BuilderMap map[string]Builder

type CatalogBuilderMap map[string]BuilderMap

type Template struct {
	CatalogBuilders CatalogBuilderMap
	Registry        image.Registry
}

// TODO(everettraven): do we need the context here? If so, how should it be used?
func (t *Template) Render(ctx context.Context, config *CompositeConfig, validate bool) error {
	// TODO(everettraven): should we return aggregated errors?
	for _, component := range config.Components {
		if builderMap, ok := t.CatalogBuilders[component.Name]; ok {
			if builder, ok := builderMap[component.Strategy.Template.Schema]; ok {
				// run the builder corresponding to the schema
				err := builder.Build(ctx, t.Registry, component.Destination.Path, component.Strategy.Template)
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
			} else {
				return fmt.Errorf("building component %q: no builder found for template schema %q", component.Name, component.Strategy.Template.Schema)
			}
		} else {
			allowedComponents := []string{}
			for k := range t.CatalogBuilders {
				allowedComponents = append(allowedComponents, k)
			}
			return fmt.Errorf("building component %q: component does not exist in the catalog configuration. Available components are: %s", component.Name, allowedComponents)
		}
	}
	return nil
}
