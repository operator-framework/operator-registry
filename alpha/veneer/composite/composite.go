package composite

import (
	"context"
	"path"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

type BuilderMap map[string]Builder

type CatalogBuilderMap map[string]BuilderConfig

type BuilderConfig struct {
	Builders        BuilderMap
	ContainerConfig ContainerConfig
}

type Veneer struct {
	CatalogBuilders CatalogBuilderMap
}

// TODO: update this to use the new builder map

func (v *Veneer) Render(ctx context.Context, config *CompositeConfig) (map[string]*declcfg.DeclarativeConfig, error) {
	// this should probably return a mapping of output destination --> DeclarativeConfig
	catalogs := map[string]*declcfg.DeclarativeConfig{}
	for _, component := range config.Components {
		if builderCfg, ok := v.CatalogBuilders[component.Name]; ok {

			if builder, ok := builderCfg.Builders[component.Strategy.Veneer.Schema]; ok {
				// run the builder corresponding to the schema
				bcfg, outPath, err := builder.Build(ctx, component.Strategy.Veneer, builderCfg.ContainerConfig)
				if err != nil {
					return nil, err
				}

				// append the config
				catalogs[path.Join(component.Destination.Path, outPath)] = bcfg
			}
			// TODO: Add error return
		}
		// TODO: Add error return
	}
	return catalogs, nil
}
