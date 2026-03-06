package validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

func TestValidateBundle(t *testing.T) {
	var table = []struct {
		description string
		directory   string
		hasError    bool
		errString   string
	}{
		{
			description: "registryv1 bundle/valid bundle",
			directory:   "./testdata/valid_bundle",
			hasError:    false,
		},
		{
			description: "registryv1 bundle/invalid bundle",
			directory:   "./testdata/invalid_bundle",
			hasError:    true,
			errString:   "owned CRD etcdclusters.etcd.database.coreos.com/v1beta2 not found in bundle",
		},
		{
			description: "registryv1 bundle/invalid bundle 2",
			directory:   "./testdata/invalid_bundle_2",
			hasError:    false, // The below error seems to be a warning and not an error in bundle.go line 65
			errString:   `CRD etcdclusters.etcd.database.coreos.com/v1beta2 is present in bundle "test" but not defined in CSV`,
		},
	}

	for _, tt := range table {
		// Read all files in manifests directory
		items, err := os.ReadDir(tt.directory)
		require.NoError(t, err, "Unable to read directory: %s", tt.description)

		unstObjs := make([]*unstructured.Unstructured, 0, len(items))

		for _, item := range items {
			fileWithPath := filepath.Join(tt.directory, item.Name())
			data, err := os.ReadFile(fileWithPath)
			require.NoError(t, err, "Unable to read file: %s", fileWithPath)

			dec := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 30)
			k8sFile := &unstructured.Unstructured{}
			err = dec.Decode(k8sFile)
			require.NoError(t, err, "Unable to decode file: %s", fileWithPath)

			unstObjs = append(unstObjs, k8sFile)
		}

		// Validate the bundle object
		bundle := registry.NewBundle("test", &registry.Annotations{}, unstObjs...)
		results := BundleValidator.Validate(bundle)

		if len(results) > 0 {
			require.Equal(t, tt.hasError, results[0].HasError(), "%s: %s", tt.description, results[0])
			if results[0].HasError() {
				require.Contains(t, results[0].Errors[0].Error(), tt.errString)
			}
		}
	}
}
