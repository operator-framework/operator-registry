package render

import (
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

func NewCmd() *cobra.Command {
	var (
		render action.Render
		output string
	)
	cmd := &cobra.Command{
		Use:   "render [index-image | bundle-image | sqlite-file]...",
		Short: "Generate a declarative config blob from catalogs and bundles",
		Long: `Generate a declarative config blob from the provided index images, bundle images, and sqlite database files

` + sqlite.DeprecationMessage,
		Example: `
#
# Output declarative configuration view of an index-image in JSON format
#
$ opm render quay.io/operatorhubio/catalog:latest

#
# Output declarative configuration view of a bundle-image in YAML format
#
$ opm render quay.io/operatorhubio/ack-apigatewayv2-controller@sha256:14c507f2ecb4a64928bcfcf5897f4495d9988f4d7ff58f41e029359a9fe78c38

#
# Output channel graph of a catalog in mermaid format
#
$ opm render quay.io/operatorhubio/catalog:latest -o mermaid

#
# Output channel graph of a catalog and generate a scaled vector graphic (SVG) representation
# Note:  this pipeline filters out the comments about lacking skipRange support
#
$ opm render quay.io/operatorhubio/catalog:latest -o mermaid | \
    grep -Ev '^<!--.*$' | \
    docker run --rm -i -v "$PWD":/data ghcr.io/mermaid-js/mermaid-cli/mermaid-cli -o /data/operatorhubio-catalog.svg

# Note:  mermaid has a default maxTextSize of 30 000 characters.  To override this, generate a JSON-formatted initialization file for
# mermaid like this (using 300 000 for the limit):
$ cat << EOF > ./mermaid.json
{ "maxTextSize": 300000 }
EOF
# and then pass the file for initialization configuration, via the '-c' option, like:
$ opm render quay.io/operatorhubio/catalog:latest -o mermaid | \
    grep -Ev '^<!--.*$' | \
    docker run --rm -i -v "$PWD":/data ghcr.io/mermaid-js/mermaid-cli/mermaid-cli -c /data/mermaid.json -o /data/operatorhubio-catalog.svg


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
			logrus.SetOutput(ioutil.Discard)

			reg, err := util.CreateCLIRegistry(cmd)
			if err != nil {
				log.Fatal(err)
			}
			defer reg.Destroy()

			render.Registry = reg

			cfg, err := render.Run(cmd.Context())
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

func nullLogger() *logrus.Entry {
	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)
	return logrus.NewEntry(logger)
}
