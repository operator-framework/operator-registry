package declcfg

import (
	"encoding/json"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/model"
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
				Bundles:  []Bundle{newTestBundle("bar", "0.1.0")},
			},
		},
		{
			name:      "Error/BundleMissingChannel",
			assertion: hasError(`package "foo", bundle "foo.v0.1.0" not found in any channel entries`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Error/BundleInvalidProperties",
			assertion: hasError(`parse properties for bundle "foo.v0.1.0": parse property[2] of type "olm.foo": unexpected end of JSON input`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Properties = append(b.Properties, property.Property{
						Type:  "olm.foo",
						Value: json.RawMessage("{"),
					})
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
			assertion: hasError(`error parsing bundle "foo.v0.1.0" version "0.1.0.1": Invalid character(s) found in patch number "0.1"`),
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
			name:      "Error/BundleMissingVersion",
			assertion: hasError(`error parsing bundle "foo.v" version "": Version string empty`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []Bundle{newTestBundle("foo", "", func(b *Bundle) {})},
			},
		},
		{
			name: "Error/PackageMissingDefaultChannel",
			assertion: hasError(`invalid index:
└── invalid package "foo":
    └── default channel must be set`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "bar", ChannelEntry{Name: testBundleName("foo", "0.1.0")})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
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
				Channels: []Channel{newTestChannel("foo", "bar")},
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
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: testBundleName("foo", "0.1.0")})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0", withNoBundleImage())},
			},
		},
		{
			name:      "Error/ChannelEntryWithoutBundle",
			assertion: hasError(`no olm.bundle blobs found in package "foo" for olm.channel entries [foo.v0.1.0]`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
			},
		},
		{
			name:      "Error/BundleWithoutChannelEntry",
			assertion: hasError(`package "foo", bundle "foo.v0.2.0" not found in any channel entries`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.2.0")},
			},
		},
		{
			name:      "Error/ChannelMissingName",
			assertion: hasError(`package "foo" contains channel with no name`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.2.0")},
			},
		},
		{
			name:      "Error/ChannelMissingPackageName",
			assertion: hasError(`unknown package "" for channel "alpha"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.2.0")},
			},
		},
		{
			name:      "Error/ChannelNonExistentPackage",
			assertion: hasError(`unknown package "non-existent" for channel "alpha"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("non-existent", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Error/ChannelDuplicateEntry",
			assertion: hasError(`invalid package "foo", channel "alpha": duplicate entry "foo.v0.1.0"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha",
					ChannelEntry{Name: "foo.v0.1.0"},
					ChannelEntry{Name: "foo.v0.1.0"},
				)},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Error/DuplicatePackage",
			assertion: hasError(`duplicate package "foo"`),
			cfg: DeclarativeConfig{
				Packages: []Package{
					newTestPackage("foo", "alpha", svgSmallCircle),
					newTestPackage("foo", "alpha", svgSmallCircle),
				},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Error/PackageBreaksRFC1123",
			assertion: hasError(`invalid package name "foo.bar": [must not contain dots]`),
			cfg: DeclarativeConfig{
				Packages: []Package{
					newTestPackage("foo.bar", "alpha", svgSmallCircle),
				},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Error/DuplicateChannel",
			assertion: hasError(`package "foo" has duplicate channel "alpha"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{
					newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"}),
					newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"}),
				},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Error/DuplicateBundle",
			assertion: hasError(`package "foo" has duplicate bundle "foo.v0.1.0"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles: []Bundle{
					newTestBundle("foo", "0.1.0"),
					newTestBundle("foo", "0.1.0"),
				},
			},
		},
		{
			name:      "Success/ValidModel",
			assertion: require.NoError,
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Success/ValidModelWithChannelProperties",
			assertion: require.NoError,
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{
					addChannelProperties(
						newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"}),
						[]property.Property{
							{Type: "user", Value: json.RawMessage("{\"group\":\"xyz.com\",\"name\":\"account\"}")},
						},
					),
				},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Success/ValidModelWithPackageProperties",
			assertion: require.NoError,
			cfg: DeclarativeConfig{
				Packages: []Package{
					addPackageProperties(
						newTestPackage("foo", "alpha", svgSmallCircle),
						[]property.Property{
							{Type: "owner", Value: json.RawMessage("{\"group\":\"abc.com\",\"name\":\"admin\"}")},
						},
					),
				},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Error/Deprecation/UnspecifiedPackage",
			assertion: hasError(`package name must be set for deprecation item 0`),
			cfg: DeclarativeConfig{
				Packages: []Package{
					addPackageProperties(
						newTestPackage("foo", "alpha", svgSmallCircle),
						[]property.Property{
							{Type: "owner", Value: json.RawMessage("{\"group\":\"abc.com\",\"name\":\"admin\"}")},
						},
					),
				},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
				Deprecations: []Deprecation{
					{Schema: SchemaDeprecation},
				},
			},
		},
		{
			name:      "Error/Deprecation/OutOfBoundsBundle",
			assertion: hasError(`cannot deprecate bundle "foo.v2.0.0" for package "foo": bundle not found`),
			cfg: DeclarativeConfig{
				Packages: []Package{
					addPackageProperties(
						newTestPackage("foo", "alpha", svgSmallCircle),
						[]property.Property{
							{Type: "owner", Value: json.RawMessage("{\"group\":\"abc.com\",\"name\":\"admin\"}")},
						},
					),
				},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
				Deprecations: []Deprecation{
					{
						Schema:  SchemaDeprecation,
						Package: "foo",
						Entries: []DeprecationEntry{
							{Reference: PackageScopedReference{Schema: SchemaBundle, Name: "foo.v2.0.0"}, Message: "foo.v2.0.0 doesn't exist in the first place"},
						},
					},
				},
			},
		},
		{
			name:      "Error/Deprecation/OutOfBoundsPackage",
			assertion: hasError(`cannot apply deprecations to an unknown package "nyarl"`),
			cfg: DeclarativeConfig{
				Packages: []Package{
					addPackageProperties(
						newTestPackage("foo", "alpha", svgSmallCircle),
						[]property.Property{
							{Type: "owner", Value: json.RawMessage("{\"group\":\"abc.com\",\"name\":\"admin\"}")},
						},
					),
				},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
				Deprecations: []Deprecation{
					{
						Schema:  SchemaDeprecation,
						Package: "nyarl",
					},
				},
			},
		},
		{
			name:      "Error/Deprecation/MultiplePerPackage",
			assertion: hasError(`expected a maximum of one deprecation per package: "foo"`),
			cfg: DeclarativeConfig{
				Packages: []Package{
					addPackageProperties(
						newTestPackage("foo", "alpha", svgSmallCircle),
						[]property.Property{
							{Type: "owner", Value: json.RawMessage("{\"group\":\"abc.com\",\"name\":\"admin\"}")},
						},
					),
				},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
				Deprecations: []Deprecation{
					{
						Schema:  SchemaDeprecation,
						Package: "foo",
						Entries: []DeprecationEntry{
							{Reference: PackageScopedReference{Schema: SchemaChannel, Name: "alpha"}, Message: "no more alpha channel"},
						},
					},
					{
						Schema:  SchemaDeprecation,
						Package: "foo",
						Entries: []DeprecationEntry{
							{Reference: PackageScopedReference{Schema: SchemaBundle, Name: "foo.v0.1.0"}, Message: "foo.v0.1.0 is dead.  do another thing"},
						},
					},
				},
			},
		},
		{
			name:      "Error/Deprecation/BadRefSchema",
			assertion: hasError(`cannot deprecate object declcfg.PackageScopedReference{Schema:"badschema", Name:"foo.v2.0.0"} referenced by entry 0 for package "foo": object schema unknown`),
			cfg: DeclarativeConfig{
				Packages: []Package{
					addPackageProperties(
						newTestPackage("foo", "alpha", svgSmallCircle),
						[]property.Property{
							{Type: "owner", Value: json.RawMessage("{\"group\":\"abc.com\",\"name\":\"admin\"}")},
						},
					),
				},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
				Deprecations: []Deprecation{
					{
						Schema:  SchemaDeprecation,
						Package: "foo",
						Entries: []DeprecationEntry{
							{Reference: PackageScopedReference{Schema: "badschema", Name: "foo.v2.0.0"}, Message: "foo.v2.0.0 doesn't exist in the first place"},
						},
					},
				},
			},
		},
		{
			name:      "Error/Deprecation/DuplicateRef",
			assertion: hasError(`duplicate deprecation entry declcfg.PackageScopedReference{Schema:"olm.bundle", Name:"foo.v0.1.0"} for package "foo"`),
			cfg: DeclarativeConfig{
				Packages: []Package{
					addPackageProperties(
						newTestPackage("foo", "alpha", svgSmallCircle),
						[]property.Property{
							{Type: "owner", Value: json.RawMessage("{\"group\":\"abc.com\",\"name\":\"admin\"}")},
						},
					),
				},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
				Deprecations: []Deprecation{
					{
						Schema:  SchemaDeprecation,
						Package: "foo",
						Entries: []DeprecationEntry{
							{Reference: PackageScopedReference{Schema: SchemaBundle, Name: "foo.v0.1.0"}, Message: "foo.v0.1.0 is bad"},
							{Reference: PackageScopedReference{Schema: SchemaBundle, Name: "foo.v0.1.0"}, Message: "foo.v0.1.0 is bad"},
						},
					},
				},
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

func TestConvertToModelBundle(t *testing.T) {
	cfg := DeclarativeConfig{
		Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
		Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
		Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
	}
	m, err := ConvertToModel(cfg)
	require.NoError(t, err)

	pkg, ok := m["foo"]
	require.True(t, ok, "expected package 'foo' to be present")
	ch, ok := pkg.Channels["alpha"]
	require.True(t, ok, "expected channel 'alpha' to be present")
	b, ok := ch.Bundles["foo.v0.1.0"]
	require.True(t, ok, "expected bundle 'foo.v0.1.0' to be present")

	assert.Equal(t, pkg, b.Package)
	assert.Equal(t, ch, b.Channel)
	assert.Equal(t, "foo.v0.1.0", b.Name)
	assert.Equal(t, "foo-bundle:v0.1.0", b.Image)
	assert.Equal(t, "", b.Replaces)
	assert.Nil(t, b.Skips)
	assert.Equal(t, "", b.SkipRange)
	assert.Len(t, b.Properties, 3)
	assert.Equal(t, []model.RelatedImage{{Name: "bundle", Image: "foo-bundle:v0.1.0"}}, b.RelatedImages)
	assert.Nil(t, b.Deprecation)
	assert.Len(t, b.Objects, 2)
	assert.NotEmpty(t, b.CsvJSON)
	assert.NotNil(t, b.PropertiesP)
	assert.Len(t, b.PropertiesP.BundleObjects, 2)
	assert.Len(t, b.PropertiesP.Packages, 1)
	assert.Equal(t, semver.MustParse("0.1.0"), b.Version)

}

func TestConvertToModelRoundtrip(t *testing.T) {
	expected := buildValidDeclarativeConfig(validDeclarativeConfigSpec{IncludeUnrecognized: true, IncludeDeprecations: false}) // TODO: turn on deprecation when we have model-->declcfg conversion

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
		if actualError != nil && actualError.Error() == expectedError {
			return
		}
		t.Errorf("expected error to be `%s`, got `%s`", expectedError, actualError)
		t.FailNow()
	}
}
