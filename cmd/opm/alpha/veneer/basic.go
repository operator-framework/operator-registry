package veneer

import (
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/grokspawn/api/pkg/lib/declcfg"
	"github.com/operator-framework/operator-registry/alpha/veneer/basic"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
)

func newBasicVeneerRenderCmd() *cobra.Command {
	var (
		veneer basic.Veneer
		output string
	)
	cmd := &cobra.Command{
		Use: "basic basic-veneer-file",
		Short: `Generate a file-based catalog from a single 'basic veneer' file
When FILE is '-' or not provided, the veneer is read from standard input`,
		Long: `Generate a file-based catalog from a single 'basic veneer' file
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

			reg, err := util.CreateCLIRegistry(cmd)
			if err != nil {
				log.Fatalf("creating containerd registry: %v", err)
			}
			defer reg.Destroy()

			veneer.Registry = reg

			// only taking first file argument
			cfg, err := veneer.Render(cmd.Context(), data)
			if err != nil {
				log.Fatal(err)
			}

			if err := write(*cfg, os.Stdout); err != nil {
				log.Fatal(err)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format (json|yaml)")
	return cmd
}
