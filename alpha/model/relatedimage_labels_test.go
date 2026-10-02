package model

import (
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/property"
)

func TestRelatedImageValidateLabels(t *testing.T) {
	type spec struct {
		name      string
		labels    map[string]string
		assertion require.ErrorAssertionFunc
	}
	specs := []spec{
		{
			name:      "NoLabels",
			labels:    nil,
			assertion: require.NoError,
		},
		{
			name:      "EmptyLabels",
			labels:    map[string]string{},
			assertion: require.NoError,
		},
		{
			name: "ValidLabels",
			labels: map[string]string{
				"feature":                           "duel",
				"olm.operatorframework.io/optional": "true",
				"empty-value":                       "",
			},
			assertion: require.NoError,
		},
		{
			name:   "InvalidLabelKey",
			labels: map[string]string{"not a valid key": "duel"},
			assertion: hasErrorContaining(
				`invalid related image`,
				`invalid label key "not a valid key"`,
			),
		},
		{
			name:   "InvalidLabelKeyPrefix",
			labels: map[string]string{"not_a_domain/feature": "duel"},
			assertion: hasErrorContaining(
				`invalid label key "not_a_domain/feature"`,
			),
		},
		{
			name:   "InvalidLabelValue",
			labels: map[string]string{"feature": "not a valid value"},
			assertion: hasErrorContaining(
				`invalid label value "not a valid value" for key "feature"`,
			),
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			s.assertion(t, RelatedImage{Name: "foo", Image: "bar", Labels: s.labels}.Validate())
		})
	}
}

// TestBundleValidateRelatedImageLabels covers the `opm validate` path: bundle
// validation, not RelatedImage.Validate, is what runs against a loaded FBC.
func TestBundleValidateRelatedImageLabels(t *testing.T) {
	newBundle := func(relatedImages []RelatedImage) *Bundle {
		pkg, ch := makePackageChannelBundle()
		return &Bundle{
			Package: pkg,
			Channel: ch,
			Name:    "anakin.v0.0.1",
			Image:   "anakin-operator:v0.0.1",
			Version: semver.MustParse("0.0.1"),
			Properties: []property.Property{
				property.MustBuildPackage("anakin", "0.0.1"),
			},
			RelatedImages: relatedImages,
		}
	}

	t.Run("NoRelatedImages", func(t *testing.T) {
		require.NoError(t, newBundle(nil).Validate())
	})

	t.Run("RelatedImagesWithoutLabels", func(t *testing.T) {
		require.NoError(t, newBundle([]RelatedImage{{Name: "foo", Image: "bar"}}).Validate())
	})

	t.Run("RelatedImagesWithValidLabels", func(t *testing.T) {
		require.NoError(t, newBundle([]RelatedImage{
			{Name: "foo", Image: "bar", Labels: map[string]string{"feature": "duel"}},
		}).Validate())
	})

	t.Run("RelatedImageWithoutImageStillPasses", func(t *testing.T) {
		// Related images with an empty image reference exist in production
		// catalogs; adding label validation must not start rejecting them.
		require.NoError(t, newBundle([]RelatedImage{{Name: "foo"}}).Validate())
	})

	t.Run("RelatedImageWithInvalidLabels", func(t *testing.T) {
		err := newBundle([]RelatedImage{
			{Name: "foo", Image: "bar"},
			{Name: "baz", Image: "quux", Labels: map[string]string{"bad key": "bad value"}},
		}).Validate()
		require.Error(t, err)
		require.ErrorContains(t, err, `relatedImages[1]`)
		require.ErrorContains(t, err, `invalid label key "bad key"`)
		require.ErrorContains(t, err, `invalid label value "bad value" for key "bad key"`)
	})
}

func hasErrorContaining(substrings ...string) require.ErrorAssertionFunc {
	return func(t require.TestingT, actualError error, _ ...interface{}) {
		if stdt, ok := t.(*testing.T); ok {
			stdt.Helper()
		}
		require.Error(t, actualError)
		for _, s := range substrings {
			require.ErrorContains(t, actualError, s)
		}
	}
}
