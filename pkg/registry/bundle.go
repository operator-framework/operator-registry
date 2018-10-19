package registry

import (
	"fmt"
	"strings"

	"github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// Scheme is the default instance of runtime.Scheme to which types in the Kubernetes API are already registered.
var Scheme = runtime.NewScheme()

// Codecs provides access to encoding and decoding for the scheme
var Codecs = serializer.NewCodecFactory(Scheme)

func DefaultYAMLDecoder() runtime.Decoder {
	return Codecs.UniversalDeserializer()
}

func init() {
	if err := v1alpha1.AddToScheme(Scheme); err != nil {
		panic(err)
	}

	if err := v1beta1.AddToScheme(Scheme); err != nil {
		panic(err)
	}
}

type Bundle struct {
	objs       []*unstructured.Unstructured
	csv        *v1alpha1.ClusterServiceVersion
	crds       []*apiextensions.CustomResourceDefinition
	cacheStale bool
}

func NewBundle(objs ...*unstructured.Unstructured) *Bundle {
	bundle := &Bundle{cacheStale:false}
	for _, o := range objs {
		bundle.Add(o)
	}
	return bundle
}

func (b *Bundle) Size() int {
	return len(b.objs)
}

func (b *Bundle) Add(obj *unstructured.Unstructured) {
	b.objs = append(b.objs, obj)
	b.cacheStale = true
}

func (b *Bundle) ClusterServiceVersion() (*v1alpha1.ClusterServiceVersion, error) {
	if err := b.cache(); err!=nil {
		return nil, err
	}
	return b.csv, nil
}

func (b *Bundle) CustomResourceDefinitions() ([]*apiextensions.CustomResourceDefinition, error) {
	if err := b.cache(); err!=nil {
		return nil, err
	}
	return b.crds, nil
}

func (b *Bundle) ProvidedAPIs() (map[APIKey]struct{}, error) {
	provided := map[APIKey]struct{}{}
	crds, err := b.CustomResourceDefinitions()
	if err != nil {
		return nil, err
	}
	for _, crd := range crds {
		for _, v := range crd.Spec.Versions {
			provided[APIKey{Group: crd.Spec.Group, Version: v.Name, Kind: crd.Spec.Names.Kind}] = struct{}{}
		}
		if crd.Spec.Version != "" {
			provided[APIKey{Group: crd.Spec.Group, Version: crd.Spec.Version, Kind: crd.Spec.Names.Kind}] = struct{}{}
		}
	}

	csv, err := b.ClusterServiceVersion()
	if err != nil {
		return nil, err
	}
	for _, api := range csv.Spec.APIServiceDefinitions.Owned {
		provided[APIKey{Group: api.Group, Version: api.Version, Kind: api.Kind}] = struct{}{}
	}
	return provided, nil
}

func (b *Bundle) AllProvidedAPIsInBundle() error {
	csv, err := b.ClusterServiceVersion()
	if err != nil {
		return err
	}
	bundleAPIs, err := b.ProvidedAPIs()
	if err != nil {
		return err
	}
	shouldExist := make(map[APIKey]struct{}, len(csv.Spec.CustomResourceDefinitions.Owned))
	for _, crdDef := range csv.Spec.CustomResourceDefinitions.Owned {
		parts := strings.SplitAfterN(crdDef.Name, ".", 2)
		shouldExist[APIKey{parts[1], crdDef.Version, crdDef.Kind}] = struct{}{}
	}
	for key := range shouldExist {
		if _, ok := bundleAPIs[key]; !ok {
			return fmt.Errorf("couldn't find %v in bundle. found: %v", key, bundleAPIs)
		}
	}
	// note: don't need to check bundle for extension apiserver types, which don't require extra bundle entries
	return nil
}

func (b *Bundle) Serialize() (csvName string, csvBytes []byte, bundleBytes []byte, err error) {
	csvCount := 0
	for _, obj := range b.objs {
		objBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
		if err != nil {
			return "", nil, nil, err
		}
		bundleBytes = append(bundleBytes, objBytes...)

		if obj.GetObjectKind().GroupVersionKind().Kind == "ClusterServiceVersion" {
			csvName = obj.GetName()
			csvBytes, err = runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
			if err != nil {
				return "", nil, nil, err
			}
			csvCount += 1
			if csvCount > 1 {
				return "", nil, nil, fmt.Errorf("two csvs found in one bundle")
			}
		}
	}

	return csvName, csvBytes, bundleBytes, nil
}

func (b *Bundle) cache() error {
	if !b.cacheStale {
		return nil
	}
	for _, o := range b.objs {
		if o.GetObjectKind().GroupVersionKind().Kind == "ClusterServiceVersion" {
			csv := &v1alpha1.ClusterServiceVersion{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.UnstructuredContent(), csv); err != nil {
				return err
			}
			b.csv = csv
			break
		}
	}

	if b.crds == nil {
		b.crds = []*apiextensions.CustomResourceDefinition{}
	}
	for _, o := range b.objs {
		if o.GetObjectKind().GroupVersionKind().Kind == "CustomResourceDefinition" {
			crd := &apiextensions.CustomResourceDefinition{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.UnstructuredContent(), crd); err != nil {
				return err
			}
			b.crds = append(b.crds, crd)
		}
	}

	b.cacheStale = false
	return nil
}
