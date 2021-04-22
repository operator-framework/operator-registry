package render

import (
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/internal/action"
	"github.com/operator-framework/operator-registry/internal/declcfg"
)

func NewCmd() *cobra.Command {
	var (
		render action.Render
		output string
	)
	cmd := &cobra.Command{
		Use:   "render <index-or-bundle-image1> <index-or-bundle-image2> <index-or-bundle-imageN>",
		Short: "Generate declarative config blobs from the provided index and bundle images",
		Args:  cobra.MinimumNArgs(1),
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

			// TODO(joelanford): What's this for?
			logrus.SetOutput(ioutil.Discard)

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
