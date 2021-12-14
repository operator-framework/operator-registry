package configmap

import (
	"context"
	"os"
	"testing"

	"github.com/operator-framework/operator-registry/pkg/api"
	unstructuredlib "github.com/operator-framework/operator-registry/pkg/lib/unstructured"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	configMapName      = "test-configmap"
	configMapNamespace = "test-namespace"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		assertFunc func(t *testing.T, bundleGot *api.Bundle, filenamesGot []string)
	}{
		{
			name:   "BundleWithCsvAndCrd",
			source: "testdata/bundle.cm.yaml",
			assertFunc: func(t *testing.T, bundleGot *api.Bundle, filenamesGot []string) {
				csvGot := bundleGot.GetCsvJson()
				assert.NotNil(t, csvGot)
				assert.Equal(t, "etcdoperator.v0.6.1", bundleGot.GetCsvName())

				crdListGot := bundleGot.GetObject()
				// 1 CSV + 1 CRD = 2 objects
				assert.Equal(t, 2, len(crdListGot))
				assert.True(t, assert.ElementsMatch(t, []string{"crd.yaml", "csv.yaml"}, filenamesGot))
			},
		},
		{
			name:   "BundleWithBuiltInKubeTypes",
			source: "testdata/bundle-with-kube-resources.cm.yaml",
			assertFunc: func(t *testing.T, bundleGot *api.Bundle, filenamesGot []string) {
				objects := bundleGot.GetObject()
				assert.NotNil(t, objects)
				assert.Equal(t, 1, len(objects))

				unst, err := unstructuredlib.FromString(objects[0])
				assert.NoError(t, err)
				assert.True(t, unst.GetKind() == "Foo")

				assert.True(t, assert.ElementsMatch(t, []string{"my.kube.type.yaml"}, filenamesGot))
			},
		},
		{
			name:   "BundleWithMultipleCsvs",
			source: "testdata/bundle-with-multiple-csvs.cm.yaml",
			assertFunc: func(t *testing.T, bundleGot *api.Bundle, filenamesGot []string) {
				csvGot := bundleGot.GetCsvJson()
				assert.NotNil(t, csvGot)

				unst, err := unstructuredlib.FromString(csvGot)
				assert.NoError(t, err)
				assert.True(t, unst.GetName() == "first" || unst.GetName() == "second")

				assert.True(t, assert.ElementsMatch(t, []string{"first.csv.yaml", "second.csv.yaml"}, filenamesGot))
			},
		},
		{
			name:   "BundleWithBadResource",
			source: "testdata/bundle-with-bad-resource.cm.yaml",
			assertFunc: func(t *testing.T, bundleGot *api.Bundle, filenamesGot []string) {
				csvGot := bundleGot.GetCsvJson()
				assert.NotNil(t, csvGot)

			},
		},
		{
			name:   "BundleWithAll",
			source: "testdata/bundle-with-all.yaml",
			assertFunc: func(t *testing.T, bundleGot *api.Bundle, filenamesGot []string) {
				csvGot := bundleGot.GetCsvJson()
				assert.NotNil(t, csvGot)
				unst, err := unstructuredlib.FromString(csvGot)
				assert.NoError(t, err)
				assert.True(t, unst.GetName() == "kiali-operator.v1.4.2")

				objects := bundleGot.GetObject()
				// 2 CRDs + 1 CSV == 3 objects
				assert.Equal(t, 3, len(objects))

				// kiali.package.yaml not added as it is missing Kind
				assert.True(t, assert.ElementsMatch(t, []string{"kiali.crd.yaml", "kiali.monitoringdashboards.crd.yaml", "kiali.v1.4.2.clusterserviceversion.yaml"}, filenamesGot))
			},
		},
		{
			name:   "BundleWithNoDefaultChannel",
			source: "testdata/bundle-with-no-default-channel.yaml",
			assertFunc: func(t *testing.T, bundleGot *api.Bundle, filenamesGot []string) {
				csvGot := bundleGot.GetCsvJson()
				assert.NotNil(t, csvGot)
				unst, err := unstructuredlib.FromString(csvGot)
				assert.NoError(t, err)
				assert.True(t, unst.GetName() == "kiali-operator.v1.4.2")

				objects := bundleGot.GetObject()
				assert.Equal(t, 3, len(objects))

				// kiali.package.yaml not added as it is missing Kind
				assert.True(t, assert.ElementsMatch(t, []string{"kiali.crd.yaml", "kiali.monitoringdashboards.crd.yaml", "kiali.v1.4.2.clusterserviceversion.yaml"}, filenamesGot))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := loadfromFile(t, tt.source)

			loader := NewBundleLoader()
			bundleGot, filenamesGot, errGot := loader.Load(cm)

			assert.NoError(t, errGot)
			assert.NotNil(t, bundleGot)

			if tt.assertFunc != nil {
				tt.assertFunc(t, bundleGot, filenamesGot)
			}
		})
	}
}

func TestLoadWriteRead(t *testing.T) {
	tests := []struct {
		name   string
		source string
		gzip   bool
	}{
		{
			name:   "BundleUncompressed",
			source: "testdata/bundles/etcd.0.9.2/",
			gzip:   false,
		},
		{
			name:   "BundleCompressed",
			source: "testdata/bundles/etcd.0.9.2/",
			gzip:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: configMapNamespace,
				},
			}
			clientset := fake.NewSimpleClientset()
			clientset.CoreV1().ConfigMaps(configMapNamespace).Create(context.TODO(), cm, metav1.CreateOptions{})

			cmLoader := NewConfigMapLoaderWithClient(configMapName, configMapNamespace, tt.source, tt.gzip, clientset)
			err := cmLoader.Populate(1 << 20)
			assert.NoError(t, err)

			cm, err = clientset.CoreV1().ConfigMaps(configMapNamespace).Get(context.TODO(), configMapName, metav1.GetOptions{})
			assert.NoError(t, err)

			bundleLoader := NewBundleLoader()
			bundle, _, err := bundleLoader.Load(cm)

			expectedObjects, err := unstructuredlib.FromDir(tt.source + "manifests/")
			assert.NoError(t, err)

			bundleObjects, err := unstructuredlib.FromBundle(bundle)
			assert.NoError(t, err)

			assert.ElementsMatch(t, expectedObjects, bundleObjects)
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
