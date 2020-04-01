package bundle

import (
	"github.com/sirupsen/logrus"
)

// BundleValidator provides a toolset for pulling and then validating
// bundle container images
type BundleValidator interface {
	// PullBundleImage takes imageBuilderArgs and an imageTag to pull, and a directory to push
	// the contents of the image
	PullBundleImage(imageBuilderArgs, imageTag string, directory string) error
	// Validate bundle takes a directory containing the contents of a bundle image
	// and validates that the format is correct
	ValidateBundleFormat(directory string) error
	// Validate bundle takes a directory containing the contents of a bundle image
	// and validates that the content is correct
	ValidateBundleContent(directory string) error
}

// NewBundleValidator is a constructor that returns an ImageValidator
func NewBundleValidator(logger *logrus.Entry) BundleValidator {
	return imageValidator{
		logger: logger,
	}
}
