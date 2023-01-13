package veneer

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/veneer/composite"
)

func newCompositeVeneerRenderCmd() *cobra.Command {
	var (
		veneer        composite.Veneer
		output        string
		containerTool string
	)
	cmd := &cobra.Command{
		Use: "composite composite-veneer-file",
		Short: `Generate a file-based catalog from a single 'composite veneer' file
When FILE is '-' or not provided, the veneer is read from standard input`,
		Long: `Generate a file-based catalog from a single 'composite veneer' file
When FILE is '-' or not provided, the veneer is read from standard input`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Handle different input argument types
			// When no arguments or "-" is passed to the command,
			// assume input is coming from stdin
			// Otherwise open the file passed to the command
			data, source, err := openFileOrStdin(cmd, args)
			if err != nil {
				log.Fatalf("unable to open %q: %v", source, err)
			}
			defer data.Close()

			// get catalog configurations
			catalogConfig := &composite.CatalogConfig{}
			catalogDoc := json.RawMessage{}
			catalogDecoder := yaml.NewYAMLOrJSONDecoder(data, 4096)
			err = catalogDecoder.Decode(&catalogDoc)
			if err != nil {
				log.Fatalf("decoding catalog config: %s", err)
			}
			err = json.Unmarshal(catalogDoc, catalogConfig)
			if err != nil {
				log.Fatalf("unmarshalling catalog config: %s", err)
			}

			var write func(declcfg.DeclarativeConfig, io.Writer) error
			switch output {
			case "yaml":
				write = declcfg.WriteYAML
			case "json":
				write = declcfg.WriteJSON
			default:
				log.Fatalf("invalid --output value %q, expected (json|yaml)", output)
			}
			// The bundle loading impl is somewhat verbose, even on the happy path,
			// so discard all logrus default logger logs. Any important failures will be
			// returned from veneer.Render and logged as fatal errors.
			logrus.SetOutput(ioutil.Discard)

			// reg, err := util.CreateCLIRegistry(cmd)
			// if err != nil {
			// 	log.Fatalf("creating containerd registry: %v", err)
			// }
			// defer reg.Destroy()

			// veneer.Registry = reg

			catalogBuilderMap := make(composite.CatalogBuilderMap)

			// setup the builders for each catalog
			for _, catalog := range catalogConfig.Catalogs {
				if _, ok := catalogBuilderMap[catalog.Name]; !ok {
					builderMap := make(composite.BuilderMap)
					for _, schema := range catalog.Builders {
						builder, err := builderForSchema(schema)
						if err != nil {
							// TODO: make this much more descriptive
							log.Fatalf("getting builder: %s", err)
						}
						builderMap[schema] = builder
					}
					catalogBuilderMap[catalog.Name] = composite.BuilderConfig{
						Builders: builderMap,
						ContainerConfig: composite.ContainerConfig{
							ContainerTool: containerTool,
							BaseImage:     catalog.Destination.BaseImage,
							WorkingDir:    catalog.Destination.WorkingDir,
						},
					}
				}
			}

			veneer.CatalogBuilders = catalogBuilderMap

			compositeData, err := os.Open("catalog/config.yaml")
			if err != nil {
				log.Fatalf("opening catalog/config.yaml: %s", err)
			}

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

			// only taking first file argument
			cfgs, err := veneer.Render(cmd.Context(), compositeConfig)
			if err != nil {
				log.Fatalf("rendering the composite veneer: %s", err)
			}

			for key, cfg := range cfgs {
				// the key is the file to write to
				file, err := os.Create(key)
				if err != nil {
					log.Fatalf("creating output file: %s", err)
				}

				// should the composite commmand be responsible for the actual write
				// operations or should the builders be responsible for this?
				if err := write(*cfg, file); err != nil {
					log.Fatalf("writing to the output file: %s", err)
				}
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format (json|yaml)")
	cmd.Flags().StringVar(&containerTool, "container-tool", "docker", "container tool to be used when rendering veneers")
	return cmd
}

func builderForSchema(schema string) (composite.Builder, error) {
	var builder composite.Builder
	switch schema {
	case composite.BasicVeneerBuilderSchema:
		builder = composite.NewBasicBuilder()
	case composite.SemverVeneerBuilderSchema:
		builder = composite.NewSemverBuilder()
	case composite.RawVeneerBuilderSchema:
		builder = composite.NewRawBuilder()
	default:
		return nil, fmt.Errorf("unknown schema %q", schema)
	}

	return builder, nil
}
