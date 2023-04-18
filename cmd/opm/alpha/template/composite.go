package template

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/template/composite"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
)

func newCompositeTemplateCmd() *cobra.Command {
	var (
		output        string
		validate      bool
		compositeFile string
		catalogFile   string
	)
	cmd := &cobra.Command{
		Use: "composite",
		Short: `Generate file-based catalogs from a catalog configuration file 
and a 'composite template' file`,
		Long: `Generate file-based catalogs from a catalog configuration file 
and a 'composite template' file`,
		Args: cobra.MaximumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {

			switch output {
			case "yaml":
				// do nothing
			case "json":
				// do nothing
			default:
				log.Fatalf("invalid --output value %q, expected (json|yaml)", output)
			}

			reg, err := util.CreateCLIRegistry(cmd)
			if err != nil {
				log.Fatalf("creating containerd registry: %v", err)
			}
			defer reg.Destroy()

			// operator author's 'composite.yaml' file
			compositeReader, err := os.Open(compositeFile)
			if err != nil {
				log.Fatalf("opening composite config file %q: %v", compositeFile, err)
			}
			defer compositeReader.Close()

			// catalog maintainer's 'catalogs.yaml' file
			var tempCatalog io.ReadCloser
			catalogURI, err := url.ParseRequestURI(catalogFile)
			if err != nil {
				tempCatalog, err = os.Open(catalogFile)
				if err != nil {
					log.Fatalf("opening catalog config file %q: %v", catalogFile, err)
				}
				defer tempCatalog.Close()
			} else {
				tempResp, err := http.Get(catalogURI.String())
				if err != nil {
					log.Fatalf("fetching remote catalog config file %q: %v", catalogFile, err)
				}
				tempCatalog = tempResp.Body
				defer tempCatalog.Close()
			}

			template := composite.Template{
				Registry:         reg,
				CatalogFile:      tempCatalog,
				ContributionFile: compositeReader,
				OutputType:       output,
			}

			err = template.Render(cmd.Context(), validate)
			if err != nil {
				log.Fatalf("rendering the composite template: %v", err)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format (json|yaml)")
	cmd.Flags().BoolVar(&validate, "validate", true, "whether or not the created FBC should be validated (i.e 'opm validate')")
	cmd.Flags().StringVarP(&compositeFile, "composite-config", "c", "composite.yaml", "File to use as the composite configuration file")
	cmd.Flags().StringVarP(&catalogFile, "catalog-config", "f", "catalogs.yaml", "File to use as the catalog configuration file")
	return cmd
}
