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

func TestValidateBundleFormat(t *testing.T) {
	dir := "./testdata/validate/valid_bundle/"

	logger := logrus.NewEntry(logrus.New())

	validator := imageValidator{
		logger: logger,
	}

	err := validator.ValidateBundleFormat(dir)
	require.NoError(t, err)
}

func TestValidateBundle_InvalidRegistryVersion(t *testing.T) {
	dir := "./testdata/validate/invalid_annotations_bundle"

	logger := logrus.NewEntry(logrus.New())

	validator := imageValidator{
		logger: logger,
	}

	err := validator.ValidateBundleFormat(dir)
	require.Error(t, err)
	var validationError ValidationError
	isValidationErr := errors.As(err, &validationError)
	require.True(t, isValidationErr)
	require.Equal(t, len(validationError.Errors), 1)
}

func TestValidateBundleContent(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())

	validator := imageValidator{
		logger: logger,
	}

	var table = []struct {
		description string
		mediaType   string
		directory   string
		numErrors   int
		errString   string
	}{
		{
			description: "registryv1 bundle/invalid csv",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_manifests_bundle/invalid_csv/",
			numErrors:   1,
			errString:   "install modes not found",
		},
		{
			description: "registryv1 bundle/invalid crd",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_manifests_bundle/invalid_crd/",
			numErrors:   1,
			errString:   "must contain unique version name",
		},
		{
			description: "registryv1 bundle/invalid sa",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_manifests_bundle/invalid_sa/",
			numErrors:   1,
			errString:   "json: cannot unmarshal number into Go struct field ObjectMeta.metadata.namespace of type string",
		},
		{
			description: "registryv1 bundle/invalid type",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_manifests_bundle/invalid_type/",
			numErrors:   1,
			errString:   "ResourceQuota is not supported type for registryV1 bundle",
		},
		{
			description: "valid registryv1 bundle",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/valid_bundle/manifests/",
			numErrors:   0,
		},
	}

	for i, tt := range table {
		fmt.Println(tt.directory)
		err := validator.ValidateBundleContent(tt.directory)
		var validationError ValidationError
		if err != nil {
			isValidationErr := errors.As(err, &validationError)
			require.True(t, isValidationErr)
		}
		require.Len(t, validationError.Errors, tt.numErrors, table[i].description)
		if len(validationError.Errors) > 0 {
			e := validationError.Errors[0]
			require.Contains(t, e.Error(), tt.errString)
		}
	}
}
