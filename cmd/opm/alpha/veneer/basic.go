package veneer

import (
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/veneer/basic"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
)

func newBasicVeneerRenderCmd() *cobra.Command {
	var (
		veneer basic.Veneer
		output string
	)
	cmd := &cobra.Command{
		Use:   "basic basic-veneer-file",
		Short: "Generate a declarative config blob from a single 'basic veneer' file",
		Long:  `Generate a declarative config blob from a single 'basic veneer' file, typified as a declarative configuration file where olm.bundle objects have no properties`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

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
			cfg, err := veneer.Render(cmd.Context(), args[0])
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
