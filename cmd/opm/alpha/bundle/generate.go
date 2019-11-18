package bundle

import (
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// newBundleGenerateCmd returns a command that will generate operator bundle
// annotations.yaml metadata
func newBundleGenerateCmd() *cobra.Command {
	bundleGenerateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate operator bundle metadata and Dockerfile",
		Long: `The "opm alpha bundle generate" command will generate operator
        bundle metadata if needed and a Dockerfile to build Operator bundle image.

        $ opm alpha bundle generate --directory /test/0.1.0/ --package test-operator \
		--channels stable,beta --default stable

		Note:
		* All manifests yaml must be in the same directory.
        `,
		RunE: generateFunc,
	}

	bundleGenerateCmd.Flags().StringVarP(&dirBuildArgs, "directory", "d", "", "The directory where bundle manifests for a specific version are located.")
	if err := bundleGenerateCmd.MarkFlagRequired("directory"); err != nil {
		log.Fatalf("Failed to mark `directory` flag for `generate` subcommand as required")
	}

	bundleGenerateCmd.Flags().StringVarP(&packageNameArgs, "package", "p", "", "The name of the package that bundle image belongs to")
	if err := bundleGenerateCmd.MarkFlagRequired("package"); err != nil {
		log.Fatalf("Failed to mark `package` flag for `generate` subcommand as required")
	}

	bundleGenerateCmd.Flags().StringVarP(&channelsArgs, "channels", "c", "", "The list of channels that bundle image belongs to")
	if err := bundleGenerateCmd.MarkFlagRequired("channels"); err != nil {
		log.Fatalf("Failed to mark `channels` flag for `generate` subcommand as required")
	}

	bundleGenerateCmd.Flags().StringVarP(&channelDefaultArgs, "default", "e", "", "The default channel for the bundle image")

	return bundleGenerateCmd
}

func generateFunc(cmd *cobra.Command, args []string) error {
	err := bundle.GenerateFunc(dirBuildArgs, packageNameArgs, channelsArgs, channelDefaultArgs, true)
	if err != nil {
		return err
	}

	return nil
}
