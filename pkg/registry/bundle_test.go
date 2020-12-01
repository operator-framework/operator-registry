package registry

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

const (
	manifestDir = "./testdata/v1crd_bundle/manifests"
)

// TestV1CRDsInBundle tests that adding a v1 and v1beta1 CRD to a bundle is successful.
// The provided APIs and CRD objects in the created bundle are compared to those in a test manifest directory.
func TestV1CRDsInBundle(t *testing.T) {
	// create bundle from manifests that include a v1 CRD
	bundle := NewBundle("test", &Annotations{
		PackageName: "lib-bucket-provisioner",
		Channels:    "alpha",
	})

	// Read all files in manifests directory
	items, err := ioutil.ReadDir(manifestDir)
	if err != nil {
		t.Fatalf("reading manifests directory: %s", err)
	}

	// unmarshal objects into unstructured
	unstObjs := []*unstructured.Unstructured{}
	for _, item := range items {
		fileWithPath := filepath.Join(manifestDir, item.Name())
		data, err := ioutil.ReadFile(fileWithPath)
		if err != nil {
			t.Fatalf("reading manifests directory file %s: %s", fileWithPath, err)
		}

		dec := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 30)
		k8sFile := &unstructured.Unstructured{}
		err = dec.Decode(k8sFile)
		if err != nil {
			t.Fatalf("marshaling manifest into unstructured %s: %s", k8sFile, err)
		}

		t.Logf("added %s object", k8sFile.GroupVersionKind().String())
		unstObjs = append(unstObjs, k8sFile)
	}

	// add unstructured objects to test bundle
	for _, object := range unstObjs {
		bundle.Add(object)
	}

	// check provided APIs in bundle are what is expected
	expectedAPIs := map[APIKey]struct{}{
		APIKey{Group: "objectbucket.io", Version: "v1alpha1", Kind: "ObjectBucket", Plural: "objectbuckets"}:           {},
		APIKey{Group: "objectbucket.io", Version: "v1alpha1", Kind: "ObjectBucketClaim", Plural: "objectbucketclaims"}: {},
	}
	providedAPIs, err := bundle.ProvidedAPIs()
	t.Logf("provided CRDs: \n%#v", providedAPIs)

	if !reflect.DeepEqual(expectedAPIs, providedAPIs) {
		t.Fatalf("crds in bundle not provided: expected %#v got %#v", expectedAPIs, providedAPIs)
	}

	// check CRDs in bundle are what is expected
	// bundle contains one v1beta1 and one v1 CRD
	dec := serializer.NewCodecFactory(Scheme).UniversalDeserializer()
	crds, err := bundle.CustomResourceDefinitions()
	for _, crd := range crds {
		switch crd.(type) {
		case *apiextensionsv1.CustomResourceDefinition:
			// objectbuckets is the v1 CRD
			// confirm it is equal to the manifest
			objectbuckets := unstObjs[2]
			ob, err := objectbuckets.MarshalJSON()
			if err != nil {
				t.Fatalf("objectbuckets: %s", err)
			}
			c := &apiextensionsv1.CustomResourceDefinition{}
			if _, _, err = dec.Decode(ob, nil, c); err != nil {
				t.Fatalf("error decoding v1 CRD: %s", err)
			}
			if !reflect.DeepEqual(c, crds[0]) {
				t.Fatalf("v1 crd not equal: expected %#v got %#v", crds[0], c)
			}
		case *apiextensionsv1beta1.CustomResourceDefinition:
			// objectbucketclaims is the v1beta1 CRD
			// confirm it is equal to the manifest
			objectbucketclaims := unstObjs[1]
			ob, err := objectbucketclaims.MarshalJSON()
			if err != nil {
				t.Fatalf("objectbucketclaims: %s", err)
			}
			c := &apiextensionsv1beta1.CustomResourceDefinition{}
			if _, _, err = dec.Decode(ob, nil, c); err != nil {
				t.Fatalf("error decoding v1beta1 CRD: %s", err)
			}
			if !reflect.DeepEqual(c, crds[1]) {
				t.Fatalf("v1beta1 crd not equal: expected %#v got %#v", crds[1], c)
			}
		}
	}
}

func TestBundleImages(t *testing.T) {
	for _, tc := range []struct {
		Name     string
		Bundle   Bundle
		Includes []string
	}{
		{
			Name: "includes bundle image",
			Bundle: Bundle{
				BundleImage: "bundle-image",
			},
			Includes: []string{"bundle-image"},
		},
		{
			Name: "includes related images",
			Bundle: Bundle{
				csv: &ClusterServiceVersion{
					Spec: json.RawMessage(`{"relatedImages":[{"name":"one","image":"one-image"},{"name":"two","image":"two-image"}]}`),
				},
			},
			Includes: []string{"one-image", "two-image"},
		},
		{
			Name: "includes bundle image and related images",
			Bundle: Bundle{
				BundleImage: "bundle-image",
				csv: &ClusterServiceVersion{
					Spec: json.RawMessage(`{"relatedImages":[{"name":"one","image":"one-image"},{"name":"two","image":"two-image"}]}`),
				},
			},
			Includes: []string{"bundle-image", "one-image", "two-image"},
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)
			actual, err := tc.Bundle.Images()
			require.NoError(err)
			for _, each := range tc.Includes {
				require.Contains(actual, each)
			}
		})
	}
}
