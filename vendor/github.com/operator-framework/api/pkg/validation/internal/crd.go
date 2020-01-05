package internal

import (
	"strings"

	"github.com/operator-framework/api/pkg/validation/errors"
	interfaces "github.com/operator-framework/api/pkg/validation/interfaces"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/install"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

var Scheme = scheme.Scheme

func init() {
	install.Install(Scheme)
}

var CRDValidator interfaces.Validator = interfaces.ValidatorFunc(validateCRDs)

func validateCRDs(objs ...interface{}) (results []errors.ManifestResult) {
	for _, obj := range objs {
		switch v := obj.(type) {
		case *v1beta1.CustomResourceDefinition:
			results = append(results, validateCRD(v))
		}
	}
	return results
}

func validateCRD(crd runtime.Object) (result errors.ManifestResult) {
	unversionedCRD := apiextensions.CustomResourceDefinition{}
	err := Scheme.Converter().Convert(&crd, &unversionedCRD, conversion.SourceToDest, nil)
	if err != nil {
		result.Add(errors.ErrInvalidParse("error converting versioned crd to unversioned crd", err))
		return result
	}
	gv := crd.GetObjectKind().GroupVersionKind().GroupVersion()
	result = validateCRDUnversioned(&unversionedCRD, gv)
	result.Name = unversionedCRD.GetName()
	return result
}

func validateCRDUnversioned(crd *apiextensions.CustomResourceDefinition, gv schema.GroupVersion) (result errors.ManifestResult) {
	errList := validation.ValidateCustomResourceDefinition(crd, gv)
	for _, err := range errList {
		if !strings.Contains(err.Field, "openAPIV3Schema") && !strings.Contains(err.Field, "status") {
			result.Add(errors.NewError(errors.ErrorType(err.Type), err.Error(), err.Field, err.BadValue))
		}
	}
	return result
}
