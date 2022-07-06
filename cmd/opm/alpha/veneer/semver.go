package veneer

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/veneer/semver"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
	"github.com/spf13/cobra"
)

func newSemverCmd() *cobra.Command {
	output := ""
	cmd := &cobra.Command{
		Use:   "semver <filename>",
		Short: "Generate a file-based catalog from a single 'semver veneer' file",
		Long:  `Generate a file-based catalog from a single 'semver veneer' file`,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]

			var (
				err error
			)

			var write func(declcfg.DeclarativeConfig, io.Writer) error
			switch output {
			case "json":
				write = declcfg.WriteJSON
			case "yaml":
				write = declcfg.WriteYAML
			case "mermaid":
				write = declcfg.WriteMermaidChannels
			default:
				return fmt.Errorf("invalid output format %q", output)
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

			veneer := semver.Veneer{
				Ref: ref,
				Reg: reg,
			}
			out, err := veneer.Render(cmd.Context())
			if err != nil {
				log.Fatalf("semver %q: %v", ref, err)
			}

			if out != nil {
				if err := write(*out, os.Stdout); err != nil {
					log.Fatal(err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format (json|yaml|mermaid)")
	return cmd
}
