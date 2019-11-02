package registry

import (
	"github.com/operator-framework/operator-registry/pkg/lib/registry"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newRegistryAddCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "add",
		Short: "add operator bundle to operator registry DB",
		Long:  `add operator bundle to operator registry DB`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			if debug, _ := cmd.Flags().GetBool("debug"); debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil
		},

		RunE: addFunc,
	}

	rootCmd.Flags().Bool("debug", false, "enable debug logging")
	rootCmd.Flags().StringP("database", "d", "bundles.db", "relative path to database file")
	rootCmd.Flags().StringSliceP("bundle-images", "b", []string{}, "comma separated list of links to bundle image")
	rootCmd.Flags().Bool("permissive", false, "allow registry load errors")
	rootCmd.Flags().StringP("container-tool", "c", "podman", "tool to interact with container images (save, build, etc.). One of: [docker, podman]")

	return rootCmd
}

func addFunc(cmd *cobra.Command, args []string) error {
	bundleImages, err := cmd.Flags().GetStringSlice("bundle-images")
	if err != nil {
		return err
	}

	fromFilename, err := cmd.Flags().GetString("database")
	if err != nil {
		return err
	}
	permissive, err := cmd.Flags().GetBool("permissive")
	if err != nil {
		return err
	}

	containerTool, err := cmd.Flags().GetString("container-tool")
	if err != nil {
		return err
	}

	request := registry.AddToRegistryRequest{
		Bundles: bundleImages,
		InputDatabase: fromFilename,
		Permissive: permissive,
		ContainerTool: containerTool,
	}

	logger := logrus.WithFields(logrus.Fields{"bundles": bundleImages})

	logger.Info("adding to the registry")

	registryAdder := registry.NewRegistryAdder(logger)

	err = registryAdder.AddToRegistry(request)
	if err != nil {
		return err
	}
	return nil
}
