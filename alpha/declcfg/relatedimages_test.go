package declcfg

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/model"
)

func TestRelatedImageLabelsModelRoundTrip(t *testing.T) {
	labels := map[string]string{"feature": "duel"}

	b := newTestBundle("foo", "0.1.0")
	b.RelatedImages = append(b.RelatedImages, RelatedImage{
		Name:   "lightsaber",
		Image:  "quay.io/anakin/lightsaber:v0.1.0",
		Labels: labels,
	})
	cfg := DeclarativeConfig{
		Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
		Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
		Bundles:  []Bundle{b},
	}

	m, err := ConvertToModel(cfg)
	require.NoError(t, err)
	require.Equal(t, []model.RelatedImage{
		{Name: "bundle", Image: "foo-bundle:v0.1.0"},
		{Name: "lightsaber", Image: "quay.io/anakin/lightsaber:v0.1.0", Labels: labels},
	}, m["foo"].Channels["alpha"].Bundles["foo.v0.1.0"].RelatedImages)

	actual := ConvertFromModel(m)
	require.Len(t, actual.Bundles, 1)
	require.Equal(t, b.RelatedImages, actual.Bundles[0].RelatedImages)
}

func TestRelatedImageLabelsRoundTrip(t *testing.T) {
	type spec struct {
		name     string
		in       string
		expected []RelatedImage
	}
	specs := []spec{
		{
			name: "WithLabels",
			in: `{
    "schema": "olm.bundle",
    "name": "anakin.v0.0.1",
    "package": "anakin",
    "image": "quay.io/anakin/bundle:v0.0.1",
    "relatedImages": [
        {
            "name": "lightsaber",
            "image": "quay.io/anakin/lightsaber:v0.0.1",
            "labels": {
                "feature": "duel",
                "olm.operatorframework.io/optional": "true"
            }
        }
    ]
}`,
			expected: []RelatedImage{{
				Name:  "lightsaber",
				Image: "quay.io/anakin/lightsaber:v0.0.1",
				Labels: map[string]string{
					"feature":                           "duel",
					"olm.operatorframework.io/optional": "true",
				},
			}},
		},
		{
			name: "WithoutLabels",
			in: `{
    "schema": "olm.bundle",
    "name": "anakin.v0.0.1",
    "package": "anakin",
    "image": "quay.io/anakin/bundle:v0.0.1",
    "relatedImages": [
        {
            "name": "lightsaber",
            "image": "quay.io/anakin/lightsaber:v0.0.1"
        }
    ]
}`,
			expected: []RelatedImage{{
				Name:  "lightsaber",
				Image: "quay.io/anakin/lightsaber:v0.0.1",
			}},
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			cfg, err := LoadReader(strings.NewReader(s.in))
			require.NoError(t, err)
			require.Len(t, cfg.Bundles, 1)
			require.Equal(t, s.expected, cfg.Bundles[0].RelatedImages)

			for _, tc := range []struct {
				format string
				write  func(DeclarativeConfig, *bytes.Buffer) error
			}{
				{"json", func(c DeclarativeConfig, buf *bytes.Buffer) error { return WriteJSON(c, buf) }},
				{"yaml", func(c DeclarativeConfig, buf *bytes.Buffer) error { return WriteYAML(c, buf) }},
			} {
				t.Run(tc.format, func(t *testing.T) {
					buf := &bytes.Buffer{}
					require.NoError(t, tc.write(*cfg, buf))

					roundTripped, err := LoadReader(bytes.NewReader(buf.Bytes()))
					require.NoError(t, err)
					require.Len(t, roundTripped.Bundles, 1)
					require.Equal(t, s.expected, roundTripped.Bundles[0].RelatedImages)

					if s.expected[0].Labels == nil {
						require.NotContains(t, buf.String(), "labels")
					}
				})
			}
		})
	}
}
