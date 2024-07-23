package template

import (
	"io"
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/template/basic"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
)

func newBasicTemplateCmd() *cobra.Command {
	var (
		template basic.Template
	)
	cmd := &cobra.Command{
		Use: "basic basic-template-file",
		Short: `Generate a file-based catalog from a single 'basic template' file
When FILE is '-' or not provided, the template is read from standard input`,
		Long: `Generate a file-based catalog from a single 'basic template' file
When FILE is '-' or not provided, the template is read from standard input`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Handle different input argument types
			// When no arguments or "-" is passed to the command,
			// assume input is coming from stdin
			// Otherwise open the file passed to the command
			data, source, err := util.OpenFileOrStdin(cmd, args)
			if err != nil {
				log.Fatalf("unable to open %q: %v", source, err)
			}
			defer data.Close()

			var write func(declcfg.DeclarativeConfig, io.Writer) error
			output, err := cmd.Flags().GetString("output")
			if err != nil {
				log.Fatalf("unable to determine output format")
			}
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
			// returned from template.Render and logged as fatal errors.
			logrus.SetOutput(io.Discard)

			reg, err := util.CreateCLIRegistry(cmd)
			if err != nil {
				log.Fatalf("creating containerd registry: %v", err)
			}
			defer reg.Destroy()

			template.Registry = reg

			// only taking first file argument
			cfg, err := template.Render(cmd.Context(), data)
			if err != nil {
				log.Fatal(err)
			}

			if err := write(*cfg, os.Stdout); err != nil {
				log.Fatal(err)
			}
		},
	}

	cmd.Flags().IntVar(&template.MigrateStages, "migrate-stages", 0, "Number of migration stages to run; use -1 for all available stages")

	return cmd
}
