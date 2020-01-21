package bundle

import (
	"io/ioutil"
	"os"
	"path/filepath"

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
accurate.`,
		Example: `$ opm alpha bundle validate --tag quay.io/test/test-operator:latest --image-builder docker`,
		RunE:    validateFunc,
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
	logger := log.WithFields(log.Fields{"container-tool": imageBuilderArgs})
	log.SetLevel(log.DebugLevel)

	imageValidator := bundle.NewImageValidator(imageBuilderArgs, logger)

	dir, err := ioutil.TempDir("", "bundle-")
	logger.Infof("Create a temp directory at %s", dir)
	if err != nil {
		return err
	}
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			logger.Error(err.Error())
		}
	}()

	err = imageValidator.PullBundleImage(tagBuildArgs, dir)
	if err != nil {
		return err
	}

	logger.Info("Unpacked image layers, validating bundle image format & contents")

	err = imageValidator.ValidateBundleFormat(dir)
	if err != nil {
		return err
	}

	err = imageValidator.ValidateBundleContent(filepath.Join(dir, bundle.ManifestsDir))
	if err != nil {
		return err
	}

	logger.Info("All validation tests have been completed successfully")

	return nil
}
