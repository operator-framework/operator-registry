package template

import (
	"log"

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

			reg, err := util.CreateCLIRegistry(cmd)
			if err != nil {
				log.Fatalf("creating containerd registry: %v", err)
			}
			defer reg.Destroy()

			template := composite.Template{
				Registry:         reg,
				CatalogFile:      catalogFile,
				ContributionFile: compositeFile,
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
	cmd.Flags().StringVarP(&compositeFile, "composite-config", "c", "catalog/config.yaml", "File to use as the composite configuration file")
	cmd.Flags().StringVarP(&catalogFile, "catalog-config", "f", "catalogs.yaml", "File to use as the catalog configuration file")
	return cmd
}
