package configmap

import (
	"os"
	"strings"
	"testing"

	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		assertFunc func(t *testing.T, bundleGot *api.Bundle)
	}{
		{
			name:   "BundleWithCsvAndCrd",
			source: "testdata/bundle.cm.yaml",
			assertFunc: func(t *testing.T, bundleGot *api.Bundle) {
				csvGot := bundleGot.GetCsvJson()
				assert.NotNil(t, csvGot)
				assert.Equal(t, "etcdoperator.v0.6.1", bundleGot.GetCsvName())

				crdListGot := bundleGot.GetObject()
				// 1 CSV + 1 CRD = 2 objects
				assert.Equal(t, 2, len(crdListGot))
			},
		},
		{
			name:   "BundleWithBuiltInKubeTypes",
			source: "testdata/bundle-with-kube-resources.cm.yaml",
			assertFunc: func(t *testing.T, bundleGot *api.Bundle) {
				objects := bundleGot.GetObject()
				assert.NotNil(t, objects)
				assert.Equal(t, 1, len(objects))

				unst := getUnstructured(t, objects[0])
				assert.True(t, unst.GetKind() == "Foo")
			},
		},
		{
			name:   "BundleWithMultipleCsvs",
			source: "testdata/bundle-with-multiple-csvs.cm.yaml",
			assertFunc: func(t *testing.T, bundleGot *api.Bundle) {
				csvGot := bundleGot.GetCsvJson()
				assert.NotNil(t, csvGot)

				unst := getUnstructured(t, csvGot)
				assert.True(t, unst.GetName() == "first" || unst.GetName() == "second")
			},
		},
		{
			name:   "BundleWithBadResource",
			source: "testdata/bundle-with-bad-resource.cm.yaml",
			assertFunc: func(t *testing.T, bundleGot *api.Bundle) {
				csvGot := bundleGot.GetCsvJson()
				assert.NotNil(t, csvGot)
			},
		},
		{
			name:   "BundleWithAll",
			source: "testdata/bundle-with-all.yaml",
			assertFunc: func(t *testing.T, bundleGot *api.Bundle) {
				csvGot := bundleGot.GetCsvJson()
				assert.NotNil(t, csvGot)
				unst := getUnstructured(t, csvGot)
				assert.True(t, unst.GetName() == "kiali-operator.v1.4.2")

				objects := bundleGot.GetObject()
				// 2 CRDs + 1 CSV == 3 objects
				assert.Equal(t, 3, len(objects))
			},
		},
		{
			name:   "BundleWithNoDefaultChannel",
			source: "testdata/bundle-with-no-default-channel.yaml",
			assertFunc: func(t *testing.T, bundleGot *api.Bundle) {
				csvGot := bundleGot.GetCsvJson()
				assert.NotNil(t, csvGot)
				unst := getUnstructured(t, csvGot)
				assert.True(t, unst.GetName() == "kiali-operator.v1.4.2")

				objects := bundleGot.GetObject()
				assert.Equal(t, 3, len(objects))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := loadfromFile(t, tt.source)

			loader := NewBundleLoader()
			bundleGot, errGot := loader.Load(cm)

			assert.NoError(t, errGot)
			assert.NotNil(t, bundleGot)

			if tt.assertFunc != nil {
				tt.assertFunc(t, bundleGot)
			}
		})
	}
}

func loadfromFile(t *testing.T, path string) *corev1.ConfigMap {
	reader, err := os.Open(path)
	require.NoError(t, err, "unable to load from file %s", path)

	decoder := yaml.NewYAMLOrJSONDecoder(reader, 30)
	bundle := &corev1.ConfigMap{}
	err = decoder.Decode(bundle)
	require.NoError(t, err, "could not decode into configmap, file=%s", path)

	return bundle
}

func getUnstructured(t *testing.T, str string) *unstructured.Unstructured {
	dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(str), 1)
	unst := &unstructured.Unstructured{}
	err := dec.Decode(unst)
	assert.NoError(t, err)
	return unst
}
