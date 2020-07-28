package bundle

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestValidateBundleFormat(t *testing.T) {
	dir := "./testdata/validate/valid_bundle/"

	logger := logrus.NewEntry(logrus.New())

	validator := imageValidator{
		logger: logger,
	}

	err := validator.ValidateBundleFormat(dir)
	require.NoError(t, err)
}

func TestValidateBundleDependencies(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())

	validator := imageValidator{
		logger: logger,
	}

	var table = []struct {
		description string
		mediaType   string
		directory   string
		errs        []error
	}{
		{
			description: "registryv1 bundle/invalid gvk dependency",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_dependencies_bundle/invalid_gvk_dependency/",
			errs: []error{
				fmt.Errorf("couldn't parse dependency of type olm.gvk"),
			},
		},
		{
			description: "registryv1 bundle/invalid package dependency",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_dependencies_bundle/invalid_package_dependency/",
			errs: []error{
				fmt.Errorf("Invalid semver format version"),
				fmt.Errorf("Package version is empty"),
				fmt.Errorf("Package name is empty"),
			},
		},
		{
			description: "registryv1 bundle/invalid dependency type",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_dependencies_bundle/invalid_dependency_type/",
			errs: []error{
				fmt.Errorf("couldn't parse dependency of type olm.crd"),
			},
		},
		{
			description: "registryv1 bundle/invalid label type",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_dependencies_bundle/invalid_label_dependency/",
			errs: []error{
				fmt.Errorf("Label information is missing"),
			},
		},
	}

	for _, tt := range table {
		fmt.Println(tt.directory)
		err := validator.ValidateBundleFormat(tt.directory)
		var validationError ValidationError
		if err != nil {
			isValidationErr := errors.As(err, &validationError)
			require.True(t, isValidationErr)
		}
		require.ElementsMatch(t, tt.errs, validationError.Errors)
	}
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
		{
			description: "invalid registryv1 bundle/missing crd",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_manifests_bundle/invalid_bundle/",
			numErrors:   1,
			errString:   "owned CRD etcdclusters.etcd.database.coreos.com/v1beta2 not found in bundle",
		},
		{
			description: "invalid registryv1 bundle/extra crd",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_manifests_bundle/invalid_bundle_2/",
			numErrors:   1,
			errString:   `CRD etcdclusters.etcd.database.coreos.com/v1beta2 is present in bundle "etcdoperator.v0.9.4" but not defined in CSV`,
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
