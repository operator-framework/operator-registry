package index

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/operator-framework/operator-registry/pkg/lib/indexer"
)

var (
	addLong = templates.LongDesc(`
		Add operator bundles to an index.

		This command will add the given set of bundle images (specified by the --bundles option) to an index image (provided by the --from-index option).
	`)

	addExample = templates.Examples(`
		# Create an index image from scratch with a single bundle image
		%[1]s --bundles quay.io/operator-framework/operator-bundle-prometheus@sha256:a3ee653ffa8a0d2bbb2fabb150a94da6e878b6e9eb07defd40dc884effde11a0 --tag quay.io/operator-framework/monitoring:1.0.0

		# Add a single bundle image to an index image
		%[1]s --bundles quay.io/operator-framework/operator-bundle-prometheus:0.15.0 --from-index quay.io/operator-framework/monitoring:1.0.0 --tag quay.io/operator-framework/monitoring:1.0.1

		# Add multiple bundles to an index and generate a Dockerfile instead of an image
		%[1]s --bundles quay.io/operator-framework/operator-bundle-prometheus:0.15.0,quay.io/operator-framework/operator-bundle-prometheus:0.22.2 --generate
	`)
)

func addIndexAddCmd(parent *cobra.Command) {
	indexCmd := &cobra.Command{
		Use:   "add",
		Short: "Add operator bundles to an index.",
		Long:  addLong,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if debug, _ := cmd.Flags().GetBool("debug"); debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil
		},
		RunE: runIndexAddCmdFunc,
	}

	indexCmd.Flags().Bool("debug", false, "enable debug logging")
	indexCmd.Flags().Bool("generate", false, "if enabled, just creates the dockerfile and saves it to local disk")
	indexCmd.Flags().StringP("out-dockerfile", "d", "", "if generating the dockerfile, this flag is used to (optionally) specify a dockerfile name")
	indexCmd.Flags().StringP("from-index", "f", "", "previous index to add to")
	indexCmd.Flags().StringSliceP("bundles", "b", nil, "comma separated list of bundles to add")
	if err := indexCmd.MarkFlagRequired("bundles"); err != nil {
		logrus.Panic("Failed to set required `bundles` flag for `index add`")
	}
	indexCmd.Flags().StringP("binary-image", "i", "", "container image for on-image `opm` command")
	indexCmd.Flags().StringP("container-tool", "c", "podman", "tool to interact with container images (save, build, etc.). One of: [docker, podman]")
	indexCmd.Flags().StringP("tag", "t", "", "custom tag for container image being built")
	indexCmd.Flags().Bool("permissive", false, "allow registry load errors")

	if err := indexCmd.Flags().MarkHidden("debug"); err != nil {
		logrus.Panic(err.Error())
	}

	// Set the example after the parent has been set to get the correct command path
	parent.AddCommand(indexCmd)
	indexCmd.Example = fmt.Sprintf(addExample, indexCmd.CommandPath())

}

func runIndexAddCmdFunc(cmd *cobra.Command, args []string) error {
	generate, err := cmd.Flags().GetBool("generate")
	if err != nil {
		return err
	}

	outDockerfile, err := cmd.Flags().GetString("out-dockerfile")
	if err != nil {
		return err
	}

	fromIndex, err := cmd.Flags().GetString("from-index")
	if err != nil {
		return err
	}

	bundles, err := cmd.Flags().GetStringSlice("bundles")
	if err != nil {
		return err
	}

	binaryImage, err := cmd.Flags().GetString("binary-image")
	if err != nil {
		return err
	}

	containerTool, err := cmd.Flags().GetString("container-tool")
	if err != nil {
		return err
	}

	tag, err := cmd.Flags().GetString("tag")
	if err != nil {
		return err
	}

	permissive, err := cmd.Flags().GetBool("permissive")
	if err != nil {
		return err
	}

	logger := logrus.WithFields(logrus.Fields{"bundles": bundles})

	logger.Info("building the index")

	indexAdder := indexer.NewIndexAdder(containerTool, logger)

	request := indexer.AddToIndexRequest{
		Generate:          generate,
		FromIndex:         fromIndex,
		BinarySourceImage: binaryImage,
		OutDockerfile:     outDockerfile,
		Tag:               tag,
		Bundles:           bundles,
		Permissive:        permissive,
	}

	err = indexAdder.AddToIndex(request)
	if err != nil {
		return err
	}

	return nil
}
