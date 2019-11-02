package index

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/pkg/lib/indexer"
)

func newIndexAddCmd() *cobra.Command {
	indexCmd := &cobra.Command{
		Use:   "add",
		Short: "add an operator bundle to an index",
		Long:  `add an operator bundle to an index`,

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

	return indexCmd

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
		Generate: generate,
		FromIndex: fromIndex,
		BinarySourceImage: binaryImage,
		OutDockerfile: outDockerfile,
		Tag: tag,
		Bundles: bundles,
		Permissive: permissive,
	}

	err = indexAdder.AddToIndex(request)
	if err != nil {
		return err
	}

	return nil
}
