package action

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

// TestGetRelatedImagesLabels covers `opm render <bundle image>`: labels the
// operator author put on the CSV's relatedImages must survive into the
// olm.bundle blob.
func TestGetRelatedImagesLabels(t *testing.T) {
	csvSpec := `{
		"relatedImages": [
			{
				"name": "lightsaber",
				"image": "quay.io/anakin/lightsaber:v0.1.0",
				"labels": {"feature": "duel"}
			},
			{
				"name": "podracer",
				"image": "quay.io/anakin/podracer:v0.1.0"
			}
		]
	}`

	var spec map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(csvSpec), &spec))

	csv := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operators.coreos.com/v1alpha1",
		"kind":       "ClusterServiceVersion",
		"metadata":   map[string]interface{}{"name": "anakin.v0.1.0"},
		"spec":       spec,
	}}

	b := registry.NewBundle("anakin.v0.1.0", &registry.Annotations{PackageName: "anakin"}, csv)
	b.BundleImage = "quay.io/anakin/bundle:v0.1.0"

	relatedImages, err := getRelatedImages(b)
	require.NoError(t, err)
	require.Equal(t, []declcfg.RelatedImage{
		{Name: "lightsaber", Image: "quay.io/anakin/lightsaber:v0.1.0", Labels: map[string]string{"feature": "duel"}},
		{Name: "podracer", Image: "quay.io/anakin/podracer:v0.1.0"},
		{Image: "quay.io/anakin/bundle:v0.1.0"},
	}, relatedImages)
}
