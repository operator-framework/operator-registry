package index

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/lib/indexer"
)

func newIndexPruneVersionCmd() *cobra.Command {
	indexCmd := &cobra.Command{
		Hidden: true,
		Use:    "prune-version",
		Short:  "prune an index of all but specified package versions",
		Long:   `prune an index of all but specified package versions`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			if debug, _ := cmd.Flags().GetBool("debug"); debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil
		},

		RunE: runIndexPruneVersionCmdFunc,
	}

	indexCmd.Flags().Bool("debug", false, "enable debug logging")
	indexCmd.Flags().Bool("generate", false, "if enabled, just creates the dockerfile and saves it to local disk")
	indexCmd.Flags().StringP("out-dockerfile", "d", "", "if generating the dockerfile, this flag is used to (optionally) specify a dockerfile name")
	indexCmd.Flags().StringP("from-index", "f", "", "index to prune")
	if err := indexCmd.MarkFlagRequired("from-index"); err != nil {
		logrus.Panic("Failed to set required `from-index` flag for `index prune`")
	}
	indexCmd.Flags().StringSliceP("package-versions", "p", nil, "comma separated list of package and versions to keep")
	if err := indexCmd.MarkFlagRequired("package-versions"); err != nil {
		logrus.Panic("Failed to set required `package-versions` flag for `index pruneversion`")
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

func runIndexPruneVersionCmdFunc(cmd *cobra.Command, args []string) error {
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

	packageVersions, err := cmd.Flags().GetStringSlice("package-versions")
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

	if containerTool == "none" {
		return fmt.Errorf("none is not a valid container-tool for index prune")
	}

	tag, err := cmd.Flags().GetString("tag")
	if err != nil {
		return err
	}

	permissive, err := cmd.Flags().GetBool("permissive")
	if err != nil {
		return err
	}

	skipTLS, err := cmd.Flags().GetBool("skip-tls")
	if err != nil {
		return err
	}

	logger := logrus.WithFields(logrus.Fields{"package-versions": packageVersions})

	logger.Info("pruning the index")

	indexPruner := indexer.NewIndexPruner(containertools.NewContainerTool(containerTool, containertools.PodmanTool), logger)

	request := indexer.PruneVersionFromIndexRequest{
		Generate:          generate,
		FromIndex:         fromIndex,
		BinarySourceImage: binaryImage,
		OutDockerfile:     outDockerfile,
		PackageVersions:   packageVersions,
		Tag:               tag,
		Permissive:        permissive,
		SkipTLS:           skipTLS,
	}

	err = indexPruner.PruneVersionFromIndex(request)
	if err != nil {
		return err
	}

	return nil
}
