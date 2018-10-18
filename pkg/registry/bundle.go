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

func ProvidedAPIs(objs []*unstructured.Unstructured) (map[APIKey]struct{}, error) {
	provided := map[APIKey]struct{}{}
	for _, o := range objs {
		if o.GetObjectKind().GroupVersionKind().Kind == "CustomResourceDefinition" {
			crd := &apiextensions.CustomResourceDefinition{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.UnstructuredContent(), crd); err != nil {
				return nil, err
			}
			for _, v := range crd.Spec.Versions {
				provided[APIKey{Group: crd.Spec.Group, Version: v.Name, Kind: crd.Spec.Names.Kind}] = struct{}{}
			}
			if crd.Spec.Version != "" {
				provided[APIKey{Group: crd.Spec.Group, Version: crd.Spec.Version, Kind: crd.Spec.Names.Kind}] = struct{}{}
			}
		}

		//TODO: APIServiceDefinitions
	}
	return provided, nil
}

func AllProvidedAPIsInBundle(csv *v1alpha1.ClusterServiceVersion, bundleAPIs map[APIKey]struct{}) error {
	shouldExist := make(map[APIKey]struct{}, len(csv.Spec.CustomResourceDefinitions.Owned)+len(csv.Spec.APIServiceDefinitions.Owned))
	for _, crdDef := range csv.Spec.CustomResourceDefinitions.Owned {
		parts := strings.SplitAfterN(crdDef.Name, ".", 2)
		shouldExist[APIKey{parts[1], crdDef.Version, crdDef.Kind}] = struct{}{}
	}
	//TODO: APIServiceDefinitions
	for key := range shouldExist {
		if _, ok := bundleAPIs[key]; !ok {
			return fmt.Errorf("couldn't find %v in bundle", key)
		}
	}
	return nil
}
