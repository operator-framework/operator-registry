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
		Example: `
#
# Output channel graph of a catalog in mermaid format
#
$ opm alpha render-graph quay.io/operatorhubio/catalog:latest

#
# Output channel graph of a catalog and generate a scaled vector graphic (SVG) representation
# Note:  this pipeline filters out the comments about lacking skipRange support
#
$ opm alpha render-graph quay.io/operatorhubio/catalog:latest | \
    grep -Ev '^<!--.*$' | \
    docker run --rm -i -v "$PWD":/data ghcr.io/mermaid-js/mermaid-cli/mermaid-cli -o /data/operatorhubio-catalog.svg

# Note:  mermaid has a default maxTextSize of 30 000 characters.  To override this, generate a JSON-formatted initialization file for
# mermaid like this (using 300 000 for the limit):
$ cat << EOF > ./mermaid.json
{ "maxTextSize": 300000 }
EOF
# and then pass the file for initialization configuration, via the '-c' option, like:
$ opm alpha render-graph quay.io/operatorhubio/catalog:latest | \
    grep -Ev '^<!--.*$' | \
    docker run --rm -i -v "$PWD":/data ghcr.io/mermaid-js/mermaid-cli/mermaid-cli -c /data/mermaid.json -o /data/operatorhubio-catalog.svg


		`,
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
