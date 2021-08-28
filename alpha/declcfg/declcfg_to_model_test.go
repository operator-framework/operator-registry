package declcfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/property"
)

func TestConvertToModel(t *testing.T) {
	type spec struct {
		name      string
		cfg       DeclarativeConfig
		assertion require.ErrorAssertionFunc
	}

	specs := []spec{
		{
			name:      "Error/PackageNoName",
			assertion: hasError(`config contains package with no name`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("", "alpha", svgSmallCircle)},
				Bundles:  []Bundle{{Name: "foo.v0.1.0"}},
			},
		},
		{
			name:      "Error/BundleMissingPackageName",
			assertion: hasError(`package name must be set for bundle "foo.v0.1.0"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []Bundle{{Name: "foo.v0.1.0"}},
			},
		},
		{
			name:      "Error/BundleUnknownPackage",
			assertion: hasError(`unknown package "bar" for bundle "bar.v0.1.0"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []Bundle{newTestBundle("bar", "0.1.0", withChannel("alpha", ""))},
			},
		},
		{
			name:      "Error/BundleMissingChannel",
			assertion: hasError(`package "foo" bundle "foo.v0.1.0" is missing channel information`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Error/BundleInvalidProperties",
			assertion: hasError(`parse properties for bundle "foo.v0.1.0": duplicate property of type "olm.channel" found with key "alpha"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0", withChannel("alpha", "1"), withChannel("alpha", "2"))},
			},
		},
		{
			name:      "Error/BundleMultipleSkipRanges",
			assertion: hasError(`package "foo" bundle "foo.v0.1.0" is invalid: multiple properties of type "olm.skipRange" not allowed`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", withChannel("alpha", ""), func(b *Bundle) {
					b.Properties = append(b.Properties,
						property.MustBuildSkipRange("<0.1.0"),
						property.MustBuildSkipRange("<=0.1.0"),
					)
				})},
			},
		},
		{
			name:      "Error/BundlePackageMismatch",
			assertion: hasError(`package "foo" does not match "olm.package" property "foooperator"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Properties = []property.Property{
						property.MustBuildPackage("foooperator", "0.1.0"),
					}
				})},
			},
		},
		{
			name:      "Error/BundleInvalidVersion",
			assertion: hasError(`error parsing bundle version: Invalid character(s) found in patch number "0.1"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Properties = []property.Property{
						property.MustBuildPackage("foo", "0.1.0.1"),
					}
				})},
			},
		},
		{
			name: "Error/PackageMissingDefaultChannel",
			assertion: hasError(`invalid index:
└── invalid package "foo":
    └── default channel must be set`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "", svgSmallCircle)},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0", withChannel("alpha", ""))},
			},
		},
		{
			name: "Error/PackageNonExistentDefaultChannel",
			assertion: hasError(`invalid index:
└── invalid package "foo":
    └── invalid channel "bar":
        └── channel must contain at least one bundle`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "bar", svgSmallCircle)},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0", withChannel("alpha", ""))},
			},
		},
		{
			name:      "Error/BundleMissingPackageProperty",
			assertion: hasError(`package "foo" bundle "foo.v0.1.0" must have exactly 1 "olm.package" property, found 0`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0", withNoProperties())},
			},
		},
		{
			name:      "Error/BundleMultiplePackageProperty",
			assertion: hasError(`package "foo" bundle "foo.v0.1.0" must have exactly 1 "olm.package" property, found 2`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Properties = []property.Property{
						property.MustBuildPackage("foo", "0.1.0"),
						property.MustBuildPackage("foo", "0.1.0"),
					}
				})},
			},
		},
		{
			name:      "Success/BundleWithDataButMissingImage",
			assertion: require.NoError,
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0", withChannel("alpha", ""), withNoBundleImage())},
			},
		},
		{
			name:      "Error/ChannelAndBundleChannel",
			assertion: hasError(`invalid package "foo", bundle "foo.v0.1.0": cannot use "olm.channel" properties with "olm.channel" blobs`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", LegacyChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0", withChannel("alpha", ""))},
			},
		},
		{
			name:      "Error/ChannelAndBundleSkips",
			assertion: hasError(`invalid package "foo", bundle "foo.v0.1.0": cannot use "olm.skips" properties with "olm.channel" blobs`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", LegacyChannelEntry{Name: "foo.v0.1.0"})},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Properties = append(b.Properties, property.MustBuildSkips("foo.v0.0.1"))
				})},
			},
		},
		{
			name:      "Error/ChannelAndBundleSkipRange",
			assertion: hasError(`invalid package "foo", bundle "foo.v0.1.0": cannot use "olm.skipRange" properties with "olm.channel" blobs`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", LegacyChannelEntry{Name: "foo.v0.1.0"})},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Properties = append(b.Properties, property.MustBuildSkipRange("<0.1.0"))
				})},
			},
		},
		{
			name:      "Error/ChannelEntryWithoutBundle",
			assertion: hasError(`no olm.bundle blobs found in package "foo" for olm.channel entries [foo.v0.1.0]`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", LegacyChannelEntry{Name: "foo.v0.1.0"})},
			},
		},
		{
			name:      "Error/BundleWithoutChannelEntry",
			assertion: hasError(`package "foo", bundle "foo.v0.2.0" not found in any channel entries`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", LegacyChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.2.0")},
			},
		},
		{
			name:      "Error/ChannelMissingName",
			assertion: hasError(`package "foo" contains channel with no name`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "", LegacyChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.2.0")},
			},
		},
		{
			name:      "Error/ChannelMissingPackageName",
			assertion: hasError(`unknown package "" for channel "alpha"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("", "alpha", LegacyChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.2.0")},
			},
		},
		{
			name:      "Error/ChannelMissingStrategy",
			assertion: hasError(`package "foo", channel "alpha" has no defined strategy`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{{Schema: schemaChannel, Package: "foo", Name: "alpha"}},
				Bundles:  []Bundle{newTestBundle("foo", "0.2.0")},
			},
		},
		{
			name:      "Error/ChannelNonExistentPackage",
			assertion: hasError(`unknown package "non-existent" for channel "alpha"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("non-existent", "alpha", LegacyChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Error/ChannelDuplicateEntry",
			assertion: hasError(`invalid package "foo", channel "alpha": duplicate entry "foo.v0.1.0"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha",
					LegacyChannelEntry{Name: "foo.v0.1.0"},
					LegacyChannelEntry{Name: "foo.v0.1.0"},
				)},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Success/WithChannel/ValidModel",
			assertion: require.NoError,
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", LegacyChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Success/WithoutChannel/ValidModel",
			assertion: require.NoError,
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", withChannel("alpha", ""), func(b *Bundle) {
					b.Properties = append(b.Properties,
						property.MustBuildSkipRange("<0.1.0"),
					)
				})},
			},
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			_, err := ConvertToModel(s.cfg)
			s.assertion(t, err)
		})
	}
}

func TestConvertToModelRoundtrip(t *testing.T) {
	expected := buildValidDeclarativeConfig(true)

	m, err := ConvertToModel(expected)
	require.NoError(t, err)
	actual := ConvertFromModel(m)

	removeJSONWhitespace(&expected)
	removeJSONWhitespace(&actual)

	assert.Equal(t, expected.Packages, actual.Packages)
	assert.Equal(t, expected.Bundles, actual.Bundles)
	assert.Len(t, actual.Others, 0, "expected unrecognized schemas not to make the roundtrip")
}

func hasError(expectedError string) require.ErrorAssertionFunc {
	return func(t require.TestingT, actualError error, args ...interface{}) {
		if stdt, ok := t.(*testing.T); ok {
			stdt.Helper()
		}
		if actualError.Error() == expectedError {
			return
		}
		t.Errorf("expected error to be `%s`, got `%s`", expectedError, actualError)
		t.FailNow()
	}
}
