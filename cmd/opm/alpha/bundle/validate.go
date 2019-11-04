package bundle

import (
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newBundleValidateCmd() *cobra.Command {
	bundleValidateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate bundle image",
		Long: `The "opm alpha bundle validate" command will validate bundle image
    from a remote source to determine if its format and content information are
    accurate.

        $ opm alpha bundle validate --tag quay.io/test/test-operator:latest \
		--image-builder docker`,
		RunE: validateFunc,
	}

	bundleValidateCmd.Flags().StringVarP(&tagBuildArgs, "tag", "t", "",
		"The path of a registry to pull from, image name and its tag that present the bundle image (e.g. quay.io/test/test-operator:latest)")
	if err := bundleValidateCmd.MarkFlagRequired("tag"); err != nil {
		log.Fatalf("Failed to mark `tag` flag for `validate` subcommand as required")
	}

	bundleValidateCmd.Flags().StringVarP(&imageBuilderArgs, "image-builder", "b", "docker", "Tool to build container images. One of: [docker, podman]")

	return bundleValidateCmd
}

func validateFunc(cmd *cobra.Command, args []string) error {
	err := bundle.ValidateFunc(tagBuildArgs, imageBuilderArgs)
	if err != nil {
		return err
	}

	return nil
}
