package render

import (
	"io"
	"log"
	"os"
	"text/template"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/declcfg/filter"
	configv1alpha1 "github.com/operator-framework/operator-registry/alpha/declcfg/filter/config/v1alpha1"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

func NewCmd(showAlphaHelp bool) *cobra.Command {
	var (
		render           action.Render
		output           string
		imageRefTemplate string

		filterFile   string
		keepPackages []string
	)
	cmd := &cobra.Command{
		Use:   "render [catalog-image | catalog-directory | bundle-image | bundle-directory | sqlite-file]...",
		Short: "Generate a stream of file-based catalog objects from catalogs and bundles",
		Long: `Generate a stream of file-based catalog objects to stdout from the provided
catalog images, file-based catalog directories, bundle images, and sqlite
database files.
`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			render.Refs = args

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
			// returned from render.Run and logged as fatal errors.
			logrus.SetOutput(io.Discard)

			reg, err := util.CreateCLIRegistry(cmd)
			if err != nil {
				log.Fatal(err)
			}
			defer reg.Destroy()
			render.Registry = reg

			if imageRefTemplate != "" {
				tmpl, err := template.New("image-ref-template").Parse(imageRefTemplate)
				if err != nil {
					log.Fatalf("invalid image reference template: %v", err)
				}
				render.ImageRefTemplate = tmpl
			}

			if filterFile != "" {
				filterLogger := logrus.NewEntry(logrus.New())
				filterer, err := filterFromFile(filterFile, filterLogger)
				if err != nil {
					log.Fatal(err)
				}
				render.Filter = filterer
			} else if len(keepPackages) > 0 {
				render.Filter = filter.NewPackageFilter(keepPackages...)
			}

			cfg, err := render.Run(cmd.Context())
			if err != nil {
				log.Fatal(err)
			}

			if err := write(*cfg, os.Stdout); err != nil {
				log.Fatal(err)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format of the streamed file-based catalog objects (json|yaml)")
	cmd.Flags().BoolVar(&render.Migrate, "migrate", false, "Perform migrations on the rendered FBC")

	// Alpha flags
	cmd.Flags().StringVar(&imageRefTemplate, "alpha-image-ref-template", "", "When bundle image reference information is unavailable, populate it with this template")

	// Filter-related flags. These are mutually exclusive.
	cmd.Flags().StringVar(&filterFile, "alpha-filter-config", "", "Path to a filter configuration file")
	cmd.Flags().StringSliceVar(&keepPackages, "alpha-keep-packages", nil, "Only include packages with the given name(s) in the rendered FBC")
	cmd.MarkFlagsMutuallyExclusive("alpha-filter-config", "alpha-keep-packages")

	if showAlphaHelp {
		cmd.Long += `
If rendering sources that do not carry bundle image reference information
(e.g. bundle directories), the --alpha-image-ref-template flag can be used to
generate image references for the rendered file-based catalog objects.
This is useful when generating a catalog with image references prior to
those images actually existing. Available template variables are:
  - {{.Package}} : the package name the bundle belongs to
  - {{.Name}}    : the name of the bundle (for registry+v1 bundles, this is the CSV name)
  - {{.Version}} : the version of the bundle

The --alpha-filter-config and --alpha-keep-packages flags can be used to filter
the rendered file-based catalog objects. This is useful when you are only interested
in a subset of the packages in the source catalogs and bundles.
`
	}
	cmd.Long += "\n" + sqlite.DeprecationMessage
	return cmd
}

func filterFromFile(filterFilePath string, log *logrus.Entry) (declcfg.CatalogFilter, error) {
	if filterFilePath == "" {
		return nil, nil
	}

	filterFile, err := os.Open(filterFilePath)
	if err != nil {
		return nil, err
	}

	// There is currently only one supported format, so just try that directly.
	// If, in the future, we add a different filter configuration API, we should
	// parse the type meta, and then switch on it to choose the correct config
	// loader function.
	cfg, err := configv1alpha1.LoadFilterConfiguration(filterFile)
	if err != nil {
		return nil, err
	}
	return configv1alpha1.NewFilter(*cfg, configv1alpha1.WithLogger(log)), nil
}
