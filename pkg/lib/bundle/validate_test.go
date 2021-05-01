package bundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

func TestValidateBundleFormatWithNonExistantDir(t *testing.T) {
	dir := "./xyzzy/"

	logger := logrus.NewEntry(logrus.New())

	validator := imageValidator{
		logger: logger,
	}

	err := validator.ValidateBundleFormat(dir)
	var validationError ValidationError
	if err != nil {
		isValidationErr := errors.As(err, &validationError)
		require.True(t, isValidationErr)
	}
	require.ElementsMatch(t, []error{
		&os.PathError{
			Op:   "open",
			Path: dir,
			Err:  syscall.Errno(0x2),
		},
		fmt.Errorf("Unable to locate manifests directory"),
		fmt.Errorf("Unable to locate metadata directory"),
	}, validationError.Errors)
}

func TestValidateBundleFormatDirWithoutExpectedSubDirectories(t *testing.T) {
	dir := "./testdata/"

	logger := logrus.NewEntry(logrus.New())

	validator := imageValidator{
		logger: logger,
	}

	err := validator.ValidateBundleFormat(dir)
	var validationError ValidationError
	if err != nil {
		isValidationErr := errors.As(err, &validationError)
		require.True(t, isValidationErr)
	}
	require.ElementsMatch(t, []error{
		fmt.Errorf("Unable to locate manifests directory"),
		fmt.Errorf("Unable to locate metadata directory"),
	}, validationError.Errors)
}

func TestValidateBundleFormatWithEmptySubDirectories(t *testing.T) {
	dir := "badmanifest"

	createDir(filepath.Join(dir, "manifests"))
	createDir(filepath.Join(dir, "metadata"))
	defer os.RemoveAll(dir)

	logger := logrus.NewEntry(logrus.New())

	validator := imageValidator{
		logger: logger,
	}

	err := validator.ValidateBundleFormat(dir)
	var validationError ValidationError
	if err != nil {
		isValidationErr := errors.As(err, &validationError)
		require.True(t, isValidationErr)
	}
	require.ElementsMatch(t, []error{
		fmt.Errorf("The directory badmanifest/manifests contains no yaml files"),
		fmt.Errorf("Could not find annotations file"),
	}, validationError.Errors)
}

func TestValidateBundleFormatWithMissingAnnotations(t *testing.T) {
	dir := "testdata/validate/missing_annotations"

	logger := logrus.NewEntry(logrus.New())

	validator := imageValidator{
		logger: logger,
	}

	err := validator.ValidateBundleFormat(dir)
	var validationError ValidationError
	if err != nil {
		isValidationErr := errors.As(err, &validationError)
		require.True(t, isValidationErr)
	}
	require.ElementsMatch(t, []error{
		fmt.Errorf(`Missing annotation "operators.operatorframework.io.bundle.mediatype.v1"`),
		fmt.Errorf(`Expecting annotation "operators.operatorframework.io.bundle.channels.v1" to have non-empty value`),
		fmt.Errorf(`Missing annotation "operators.operatorframework.io.bundle.manifests.v1"`),
		fmt.Errorf(`Missing annotation "operators.operatorframework.io.bundle.metadata.v1"`),
		fmt.Errorf(`Missing annotation "operators.operatorframework.io.bundle.package.v1"`),
		fmt.Errorf(`Missing annotation "operators.operatorframework.io.bundle.channels.v1"`),
		fmt.Errorf(`Expecting annotation "operators.operatorframework.io.bundle.mediatype.v1" to have value "registry+v1" instead of ""`),
	}, validationError.Errors)
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
		logger:   logger,
		optional: []string{"operatorhub,bundle-objects"},
	}

	var table = []struct {
		description string
		mediaType   string
		directory   string
		numErrors   int
		errStrings  []string
	}{
		{
			description: "registryv1 bundle/invalid csv",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_manifests_bundle/invalid_csv/",
			numErrors:   2,
			errStrings: []string{
				"install modes not found",
				"csv.Spec.Provider.Name not specified",
			},
		},
		{
			description: "registryv1 bundle/invalid crd",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_manifests_bundle/invalid_crd/",
			numErrors:   2,
			errStrings: []string{
				"must contain unique version names",
				`duplicate CRD "etcd.database.coreos.com/v1beta2, Kind=EtcdRestore" in bundle "etcdoperator.v0.9.4"`,
			},
		},
		{
			description: "registryv1 bundle/invalid sa",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_manifests_bundle/invalid_sa/",
			numErrors:   1,
			errStrings:  []string{"json: cannot unmarshal number into Go struct field ObjectMeta.metadata.namespace of type string"},
		},
		{
			description: "registryv1 bundle/invalid type",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_manifests_bundle/invalid_type/",
			numErrors:   1,
			errStrings:  []string{"ResourceQuota is not supported type for registryV1 bundle"},
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
			errStrings:  []string{`owned CRD "etcd.database.coreos.com/v1beta2, Kind=EtcdCluster" not found in bundle "etcdoperator.v0.9.4"`},
		},
		{
			description: "invalid registryv1 bundle/extra crd",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_manifests_bundle/invalid_bundle_2/",
			numErrors:   1,
			errStrings:  []string{`CRD "etcd.database.coreos.com/v1beta2, Kind=EtcdCluster" is present in bundle "etcdoperator.v0.9.4" but not defined in CSV`},
		},
		{
			description: "invalid registryv1 bundle/bad pdb",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_manifests_bundle/invalid_bundle_3/",
			numErrors:   1,
			errStrings:  []string{`minAvailable field cannot be set to 100%`},
		},
		{
			description: "invalid annotation names",
			mediaType:   RegistryV1Type,
			directory:   "./testdata/validate/invalid_manifests_bundle/invalid_annotation_name/",
			numErrors:   3,
			errStrings: []string{
				"provided annotation olm.operatorgroup uses wrong case and should be olm.operatorGroup instead",
				"provided annotation olm.operatornamespace uses wrong case and should be olm.operatorNamespace instead",
				"provided annotation olm.skiprange uses wrong case and should be olm.skipRange instead",
			},
		},
	}

	// doesActualErrorMatchExpected iterates through expected error messages and looks for substring match against actualError.
	// returns true for match, false otherwise
	doesActualErrorMatchExpected := func(actualError string, expectedSlice []string) bool {
		for _, expected := range expectedSlice {
			if strings.Contains(actualError, expected) {
				// found a substring match
				return true
			}
		}
		// no substring matches
		return false
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
		// convert each validation error to a string and check against all expected errStrings looking for a match
		for _, e := range validationError.Errors {
			require.True(t, doesActualErrorMatchExpected(e.Error(), tt.errStrings), table[i].description)
		}
	}
}
