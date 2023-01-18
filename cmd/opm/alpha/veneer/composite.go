package veneer

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/alpha/veneer/composite"
)

func newCompositeVeneerRenderCmd() *cobra.Command {
	var (
		veneer        composite.Veneer
		output        string
		containerTool string
		validate      bool
		compositeFile string
		catalogFile   string
	)
	cmd := &cobra.Command{
		Use: "composite composite-veneer-file",
		Short: `Generate a file-based catalog from a single 'composite veneer' file
When FILE is '-' or not provided, the veneer is read from standard input`,
		Long: `Generate a file-based catalog from a single 'composite veneer' file
When FILE is '-' or not provided, the veneer is read from standard input`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			catalogData, err := os.Open(catalogFile)
			if err != nil {
				log.Fatalf("opening catalog config file %q: %s", catalogFile, err)
			}
			defer catalogData.Close()

			// get catalog configurations
			catalogConfig := &composite.CatalogConfig{}
			catalogDoc := json.RawMessage{}
			catalogDecoder := yaml.NewYAMLOrJSONDecoder(catalogData, 4096)
			err = catalogDecoder.Decode(&catalogDoc)
			if err != nil {
				log.Fatalf("decoding catalog config: %s", err)
			}
			err = json.Unmarshal(catalogDoc, catalogConfig)
			if err != nil {
				log.Fatalf("unmarshalling catalog config: %s", err)
			}

			if catalogConfig.Schema != composite.CatalogSchema {
				log.Fatalf("catalog configuration file has unknown schema, should be %q", composite.CatalogSchema)
			}

			catalogBuilderMap := make(composite.CatalogBuilderMap)

			// setup the builders for each catalog
			for _, catalog := range catalogConfig.Catalogs {
				if _, ok := catalogBuilderMap[catalog.Name]; !ok {
					builderMap := make(composite.BuilderMap)
					for _, schema := range catalog.Builders {
						builder, err := builderForSchema(schema, composite.BuilderConfig{
							ContainerCfg: composite.ContainerConfig{
								ContainerTool: containerTool,
								BaseImage:     catalog.Destination.BaseImage,
								WorkingDir:    catalog.Destination.WorkingDir,
							},
							OutputType: output,
						})
						if err != nil {
							log.Fatalf("getting builder %q for catalog %q: %s", schema, catalog.Name, err)
						}
						builderMap[schema] = builder
					}
					catalogBuilderMap[catalog.Name] = builderMap
				}
			}

			veneer.CatalogBuilders = catalogBuilderMap

			compositeData, err := os.Open(compositeFile)
			if err != nil {
				log.Fatalf("opening composite config file %q: %s", compositeFile, err)
			}
			defer compositeData.Close()

			// parse data to composite config
			compositeConfig := &composite.CompositeConfig{}
			compositeDoc := json.RawMessage{}
			compositeDecoder := yaml.NewYAMLOrJSONDecoder(compositeData, 4096)
			err = compositeDecoder.Decode(&compositeDoc)
			if err != nil {
				log.Fatalf("decoding composite config: %s", err)
			}
			err = json.Unmarshal(compositeDoc, compositeConfig)
			if err != nil {
				log.Fatalf("unmarshalling composite config: %s", err)
			}

			if compositeConfig.Schema != composite.CompositeSchema {
				log.Fatalf("%q has unknown schema, should be %q", compositeFile, composite.CompositeSchema)
			}

			err = veneer.Render(cmd.Context(), compositeConfig, validate)
			if err != nil {
				log.Fatalf("rendering the composite veneer: %s", err)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format (json|yaml)")
	// TODO: Should we lock this flag to either docker or podman?
	cmd.Flags().StringVar(&containerTool, "container-tool", "docker", "container tool to be used when rendering veneers (should be an equivalent replacement to docker - similar to podman)")
	cmd.Flags().BoolVar(&validate, "validate", true, "whether or not the created FBC should be validated (i.e 'opm validate')")
	cmd.Flags().StringVarP(&compositeFile, "composite-config", "c", "catalog/config.yaml", "File to use as the composite configuration file")
	cmd.Flags().StringVarP(&catalogFile, "catalog-config", "f", "catalogs.yaml", "File to use as the catalog configuration file")
	return cmd
}

func builderForSchema(schema string, builderCfg composite.BuilderConfig) (composite.Builder, error) {
	var builder composite.Builder
	switch schema {
	case composite.BasicVeneerBuilderSchema:
		builder = composite.NewBasicBuilder(builderCfg)
	case composite.SemverVeneerBuilderSchema:
		builder = composite.NewSemverBuilder(builderCfg)
	case composite.RawVeneerBuilderSchema:
		builder = composite.NewRawBuilder(builderCfg)
	case composite.CustomVeneerBuilderSchema:
		builder = composite.NewCustomBuilder(builderCfg)
	default:
		return nil, fmt.Errorf("unknown schema %q", schema)
	}

	return builder, nil
}
