package validation

import (
	"github.com/operator-framework/api/pkg/validation/errors"
	interfaces "github.com/operator-framework/api/pkg/validation/interfaces"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

var RegistryBundleValidator interfaces.Validator = interfaces.ValidatorFunc(validateBundles)

func validateBundles(objs ...interface{}) (results []errors.ManifestResult) {
	for _, obj := range objs {
		switch v := obj.(type) {
		case *registry.Bundle:
			results = append(results, validateBundle(v))
		}
	}
	return results
}

func validateBundle(bundle *registry.Bundle) (result errors.ManifestResult) {
	// obtain the CSV... an error here means at least one file in the bundle
	// resulted in an error (but CSV is not necessarily the cause of the failure)
	_, err := bundle.ClusterServiceVersion()
	if err != nil {
		result.Add(errors.ErrInvalidParse("error getting bundle CSV", err))
		return result
	}

	// Add registry Bundle validation logic here,
	// but DO NOT duplicate what is already being done in
	// `BundleValidator` found in github.com/operator-framework/api/pkg/validation

	return result
}
