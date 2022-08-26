package rendergraph

import (
	"io"
	"log"
	"os"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var (
		render  action.Render
		minEdge string
	)
	cmd := &cobra.Command{
		Use:   "render-graph [index-image | fbc-dir | bundle-image]",
		Short: "Generate mermaid-formatted view of upgrade graph of operators in an index",
		Long:  `Generate mermaid-formatted view of upgrade graphs of operators in an index`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// The bundle loading impl is somewhat verbose, even on the happy path,
			// so discard all logrus default logger logs. Any important failures will be
			// returned from render.Run and logged as fatal errors.
			logrus.SetOutput(io.Discard)

			registry, err := util.CreateCLIRegistry(cmd)
			if err != nil {
				log.Fatal(err)
			}

			render.Refs = args
			render.AllowedRefMask = action.RefBundleImage | action.RefDCImage | action.RefDCDir // all non-sqlite
			render.Registry = registry

			cfg, err := render.Run(cmd.Context())
			if err != nil {
				log.Fatal(err)
			}

			if err := declcfg.WriteMermaidChannels(*cfg, os.Stdout, minEdge); err != nil {
				log.Fatal(err)
			}
		},
	}
	cmd.Flags().StringVar(&minEdge, "minimum-edge", "", "the channel edge to be used as the lower bound of the set of edges composing the upgrade graph")
	return cmd
}
