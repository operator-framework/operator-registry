package bundle

import (
	"errors"
	"fmt"
	"testing"

	"github.com/operator-framework/operator-registry/pkg/containertools/containertoolsfakes"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullBundle(t *testing.T) {
	tag := "quay.io/example/bundle:0.0.1"
	dir := "/tmp/dir"

	logger := logrus.NewEntry(logrus.New())

	mockImgReader := containertoolsfakes.FakeImageReader{}
	mockImgReader.GetImageDataReturns(nil)

	validator := imageValidator{
		imageReader: &mockImgReader,
		logger:      logger,
	}

	err := validator.PullBundleImage(tag, dir)
	require.NoError(t, err)
}

func TestPullBundle_Error(t *testing.T) {
	tag := "quay.io/example/bundle:0.0.1"
	dir := "/tmp/dir"

	expectedErr := fmt.Errorf("Unable to unpack image")

	logger := logrus.NewEntry(logrus.New())

	mockImgReader := containertoolsfakes.FakeImageReader{}
	mockImgReader.GetImageDataReturns(expectedErr)

	validator := imageValidator{
		imageReader: &mockImgReader,
		logger:      logger,
	}

	err := validator.PullBundleImage(tag, dir)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestValidateBundle(t *testing.T) {
	dir := "./testdata/validate/valid_bundle"

	logger := logrus.NewEntry(logrus.New())

	validator := imageValidator{
		logger: logger,
	}

	err := validator.ValidateBundle(dir)
	require.NoError(t, err)
}

func TestValidateBundle_InvalidRegistryVersion(t *testing.T) {
	dir := "./testdata/validate/invalid_annotations_bundle"

	logger := logrus.NewEntry(logrus.New())

	validator := imageValidator{
		logger: logger,
	}

	err := validator.ValidateBundle(dir)
	require.Error(t, err)
	var validationError ValidationError
	isValidationErr := errors.As(err, &validationError)
	require.True(t, isValidationErr)
	require.Equal(t, len(validationError.AnnotationErrors), 1)
}
