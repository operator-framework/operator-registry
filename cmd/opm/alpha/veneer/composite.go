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
		Use: "composite",
		Short: `Generate file-based catalogs from a catalog configuration file 
and a 'composite veneer' file`,
		Long: `Generate file-based catalogs from a catalog configuration file 
and a 'composite veneer' file`,
		Args: cobra.MaximumNArgs(0),
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

			wd, err := os.Getwd()
			if err != nil {
				log.Fatalf("getting current working directory: %w", err)
			}

			// setup the builders for each catalog
			setupFailed := false
			setupErrors := map[string][]string{}
			for _, catalog := range catalogConfig.Catalogs {
				errs := []string{}
				if catalog.Destination.BaseImage == "" {
					errs = append(errs, "destination.baseImage must not be an empty string")
				}

				if catalog.Destination.WorkingDir == "" {
					errs = append(errs, "destination.workingDir must not be an empty string")
				}

				// check for validation errors and skip builder creation if there are any errors
				if len(errs) > 0 {
					setupFailed = true
					setupErrors[catalog.Name] = errs
					continue
				}

				if _, ok := catalogBuilderMap[catalog.Name]; !ok {
					builderMap := make(composite.BuilderMap)
					for _, schema := range catalog.Builders {
						builder, err := builderForSchema(schema, composite.BuilderConfig{
							ContainerCfg: composite.ContainerConfig{
								ContainerTool: containerTool,
								BaseImage:     catalog.Destination.BaseImage,
								WorkingDir:    catalog.Destination.WorkingDir,
							},
							OutputType:       output,
							CurrentDirectory: wd,
						})
						if err != nil {
							log.Fatalf("getting builder %q for catalog %q: %s", schema, catalog.Name, err)
						}
						builderMap[schema] = builder
					}
					catalogBuilderMap[catalog.Name] = builderMap
				}
			}

			// if there were errors validating the catalog configuration then exit
			if setupFailed {
				//build the error message
				var errMsg string
				for cat, errs := range setupErrors {
					errMsg += fmt.Sprintf("\nCatalog %s:\n", cat)
					for _, err := range errs {
						errMsg += fmt.Sprintf("  - %s\n", err)
					}
				}
				log.Fatalf("catalog configuration file field validation failed: %s", errMsg)
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
	// TODO: Investigate ways to do this without using a cli tool like docker/podman
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
