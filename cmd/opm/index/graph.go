package index

import (
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/lib/indexer"
)

var graphLong = templates.LongDesc(`
		Render the package upgrade graphs from an index image (specified as an argument) to a graphviz DOT file (specified by the --output flag).
	`)

func newIndexGraphCmd() *cobra.Command {
	indexCmd := &cobra.Command{
		Use:   "graph <indexImage>",
		Short: "Visualize upgrade graphs from an index.",
		Long:  graphLong,
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if debug, _ := cmd.Flags().GetBool("debug"); debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil
		},
		RunE: runIndexGraphCmdFunc,
	}

	indexCmd.Flags().StringP("output", "o", "", "DOT file to write")
	indexCmd.Flags().Bool("debug", false, "enable debug logging")
	indexCmd.Flags().StringP("pull-tool", "p", "none", "tool to pull container images. One of: [none, docker, podman]")

	if err := indexCmd.MarkFlagRequired("output"); err != nil {
		logrus.Panic(err.Error())
	}

	if err := indexCmd.Flags().MarkHidden("debug"); err != nil {
		logrus.Panic(err.Error())
	}

	return indexCmd
}

func runIndexGraphCmdFunc(cmd *cobra.Command, args []string) error {
	indexImage := args[0]

	outputFilename, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}

	pullTool, err := cmd.Flags().GetString("pull-tool")
	if err != nil {
		return err
	}

	logger := logrus.WithFields(logrus.Fields{"index": indexImage})

	tmpdir, err := ioutil.TempDir("", "opm-index-graph-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	indexGrapher := indexer.NewIndexGrapher(
		containertools.NewContainerTool(pullTool, containertools.NoneTool),
		logger,
	)
	request := indexer.GraphFromIndexRequest{
		Index:      indexImage,
		OutputFile: outputFilename,
	}

	if err := indexGrapher.GraphFromIndex(request); err != nil {
		return err
	}
	return nil
}
