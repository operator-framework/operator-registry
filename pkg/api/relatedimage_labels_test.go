package api

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"

	"github.com/operator-framework/operator-registry/alpha/model"
)

func TestGetRelatedImagesLabels(t *testing.T) {
	type spec struct {
		name     string
		csvJSON  string
		expected []model.RelatedImage
	}
	specs := []spec{
		{
			name:    "WithLabels",
			csvJSON: `{"spec":{"relatedImages":[{"name":"lightsaber","image":"quay.io/anakin/lightsaber:v0.1.0","labels":{"feature":"duel"}}]}}`,
			expected: []model.RelatedImage{{
				Name:   "lightsaber",
				Image:  "quay.io/anakin/lightsaber:v0.1.0",
				Labels: map[string]string{"feature": "duel"},
			}},
		},
		{
			name:    "WithoutLabels",
			csvJSON: `{"spec":{"relatedImages":[{"name":"lightsaber","image":"quay.io/anakin/lightsaber:v0.1.0"}]}}`,
			expected: []model.RelatedImage{{
				Name:  "lightsaber",
				Image: "quay.io/anakin/lightsaber:v0.1.0",
			}},
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			actual, err := getRelatedImages(s.csvJSON)
			require.NoError(t, err)
			require.Equal(t, s.expected, actual)
		})
	}
}

func TestConvertModelRelatedImagesToCSVRelatedImagesLabels(t *testing.T) {
	actual := convertModelRelatedImagesToCSVRelatedImages([]model.RelatedImage{
		{Name: "lightsaber", Image: "quay.io/anakin/lightsaber:v0.1.0", Labels: map[string]string{"feature": "duel"}},
		{Name: "podracer", Image: "quay.io/anakin/podracer:v0.1.0"},
	})
	require.Equal(t, []v1alpha1.RelatedImage{
		{Name: "lightsaber", Image: "quay.io/anakin/lightsaber:v0.1.0", Labels: map[string]string{"feature": "duel"}},
		{Name: "podracer", Image: "quay.io/anakin/podracer:v0.1.0"},
	}, actual)
}
