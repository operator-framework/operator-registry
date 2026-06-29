package declcfg

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/property"
)

func TestValidate(t *testing.T) {
	type spec struct {
		name      string
		cfg       DeclarativeConfig
		assertion require.ErrorAssertionFunc
	}

	specs := []spec{
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
			name:      "Success/ValidModelWithMultipleChannels",
			assertion: require.NoError,
			cfg:       buildValidDeclarativeConfig(validDeclarativeConfigSpec{}),
		},
		{
			name:      "Success/ValidModelWithDeprecations",
			assertion: require.NoError,
			cfg:       buildValidDeclarativeConfig(validDeclarativeConfigSpec{IncludeDeprecations: true}),
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

		// Package errors
		{
			name:      "Error/PackageNoName",
			assertion: hasError(`config contains package with no name`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("", "alpha", svgSmallCircle)},
				Bundles:  []Bundle{{Name: "foo.v0.1.0"}},
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
			},
		},

		// Channel errors
		{
			name:      "Error/ChannelMissingPackageName",
			assertion: hasError(`unknown package "" for channel "alpha"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
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
			name:      "Error/ChannelMissingName",
			assertion: hasError(`package contains channel with no name`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Error/DuplicateChannel",
			assertion: hasError(`duplicate channel "alpha"`),
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
			name:      "Error/ChannelDuplicateEntry",
			assertion: hasErrorContaining(`duplicate entry "foo.v0.1.0"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha",
					ChannelEntry{Name: "foo.v0.1.0"},
					ChannelEntry{Name: "foo.v0.1.0"},
				)},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},

		// Bundle errors
		{
			name:      "Error/BundleMissingPackageName",
			assertion: hasError(`package name must be set`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []Bundle{{Name: "foo.v0.1.0"}},
			},
		},
		{
			name:      "Error/BundleUnknownPackage",
			assertion: hasError(`unknown package "bar"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []Bundle{newTestBundle("bar", "0.1.0")},
			},
		},
		{
			name:      "Error/DuplicateBundle",
			assertion: hasError(`duplicate bundle`),
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
			name:      "Error/BundleInvalidProperties",
			assertion: hasErrorContaining(`parse properties`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Properties = append(b.Properties, property.Property{
						Type:  "olm.foo",
						Value: json.RawMessage("{"),
					})
				})},
			},
		},
		{
			name:      "Error/BundleMissingPackageProperty",
			assertion: hasError(`must have exactly 1 "olm.package" property, found 0`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0", withNoProperties())},
			},
		},
		{
			name:      "Error/BundleMultiplePackageProperty",
			assertion: hasError(`must have exactly 1 "olm.package" property, found 2`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Properties = []property.Property{
						property.MustBuildPackage("foo", "0.1.0"),
						property.MustBuildPackage("foo", "0.1.0"),
					}
				})},
			},
		},
		{
			name:      "Error/BundlePackageMismatch",
			assertion: hasError(`package "foo" does not match "olm.package" property "foooperator"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Properties = []property.Property{
						property.MustBuildPackage("foooperator", "0.1.0"),
					}
				})},
			},
		},
		{
			name:      "Error/BundleInvalidVersion",
			assertion: hasErrorContaining(`error parsing version`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Properties = []property.Property{
						property.MustBuildPackage("foo", "0.1.0.1"),
					}
				})},
			},
		},
		{
			name:      "Error/BundleMissingChannel",
			assertion: hasError(`not found in any channel entries`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha")},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
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
			// Regression: validate that entries from all channels are checked, not just the last.
			// A package with two channels where the first channel's entry has no bundle blob must
			// be rejected even when the second channel's entries are fully satisfied.
			name:      "Error/ChannelEntryWithoutBundleMultiChannel",
			assertion: hasError(`no olm.bundle blobs found in package "foo" for olm.channel entries [foo.v0.1.0]`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "beta", svgSmallCircle)},
				Channels: []Channel{
					newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"}),
					newTestChannel("foo", "beta", ChannelEntry{Name: testBundleName("foo", "0.2.0")}),
				},
				Bundles: []Bundle{newTestBundle("foo", "0.2.0")},
			},
		},
		{
			name:      "Error/BundleWithoutChannelEntry",
			assertion: hasError(`not found in any channel entries`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.2.0")},
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

		// Image validation
		{
			name:      "Error/BundleImageInvalidPullSpec",
			assertion: hasErrorContaining("invalid image pull spec"),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: testBundleName("foo", "0.1.0")})},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Image = "quay.io/operator-framework/foo-bundle@ssha256:abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234"
				})},
			},
		},
		{
			name:      "Error/BundleRelatedImageInvalidPullSpec",
			assertion: hasErrorContaining("invalid image pull spec"),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: testBundleName("foo", "0.1.0")})},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.RelatedImages = []RelatedImage{
						{Name: "bundle", Image: testBundleImage("foo", "0.1.0")},
						{Name: "operator", Image: "quay.io/operator-framework/my-operator@ssha256:abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234"},
					}
				})},
			},
		},
		{
			name:      "Success/BundleImageValidSha256Digest",
			assertion: require.NoError,
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: testBundleName("foo", "0.1.0")})},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Image = "quay.io/operator-framework/foo-bundle@sha256:abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234"
					b.RelatedImages = []RelatedImage{
						{Name: "bundle", Image: "quay.io/operator-framework/foo-bundle@sha256:abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234"},
					}
				})},
			},
		},

		// Release/normalization errors
		{
			name:      "Success/ValidBundleReleaseVersion",
			assertion: require.NoError,
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo-v0.1.0-alpha.1.0.0"})},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Properties = []property.Property{
						property.MustBuildPackageRelease("foo", "0.1.0", "alpha.1.0.0"),
					}
					b.Name = "foo-v0.1.0-alpha.1.0.0"
				})},
			},
		},
		{
			name:      "Error/InvalidBundleNormalizedName",
			assertion: hasError(`name "foo.v0.1.0-alpha.1.0.0" does not match normalized name "foo-v0.1.0-alpha.1.0.0"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0-alpha.1.0.0"})},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Properties = []property.Property{
						property.MustBuildPackageRelease("foo", "0.1.0", "alpha.1.0.0"),
					}
					b.Name = "foo.v0.1.0-alpha.1.0.0"
				})},
			},
		},
		{
			name:      "Error/BundleReleaseWithBuildMetadata",
			assertion: hasError(`cannot use build metadata in version with a release version`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0+alpha.1.0.0-0.0.1"})},
				Bundles: []Bundle{newTestBundle("foo", "0.1.0", func(b *Bundle) {
					b.Properties = []property.Property{
						property.MustBuildPackageRelease("foo", "0.1.0+alpha.1.0.0", "0.0.1"),
					}
					b.Name = "foo.v0.1.0+alpha.1.0.0-0.0.1"
				})},
			},
		},

		// Deprecation errors
		{
			name:      "Error/Deprecation/UnspecifiedPackage",
			assertion: hasError(`package name must be set for deprecation item 0`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
				Deprecations: []Deprecation{
					{Schema: SchemaDeprecation},
				},
			},
		},
		{
			name:      "Error/Deprecation/OutOfBoundsBundle",
			assertion: hasError(`cannot deprecate bundle "foo.v2.0.0": bundle not found`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
				Deprecations: []Deprecation{
					{
						Schema:  SchemaDeprecation,
						Package: "foo",
						Entries: []DeprecationEntry{
							{Reference: PackageScopedReference{Schema: SchemaBundle, Name: "foo.v2.0.0"}, Message: "deprecated"},
						},
					},
				},
			},
		},
		{
			name:      "Error/Deprecation/OutOfBoundsPackage",
			assertion: hasError(`cannot apply deprecations to an unknown package "nyarl"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
				Deprecations: []Deprecation{
					{Schema: SchemaDeprecation, Package: "nyarl"},
				},
			},
		},
		{
			name:      "Error/Deprecation/MultiplePerPackage",
			assertion: hasError(`expected a maximum of one deprecation per package: "foo"`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
				Deprecations: []Deprecation{
					{Schema: SchemaDeprecation, Package: "foo", Entries: []DeprecationEntry{
						{Reference: PackageScopedReference{Schema: SchemaChannel, Name: "alpha"}, Message: "deprecated"},
					}},
					{Schema: SchemaDeprecation, Package: "foo", Entries: []DeprecationEntry{
						{Reference: PackageScopedReference{Schema: SchemaBundle, Name: "foo.v0.1.0"}, Message: "deprecated"},
					}},
				},
			},
		},
		{
			name:      "Error/Deprecation/BadRefSchema",
			assertion: hasErrorContaining(`object schema unknown`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
				Deprecations: []Deprecation{
					{Schema: SchemaDeprecation, Package: "foo", Entries: []DeprecationEntry{
						{Reference: PackageScopedReference{Schema: "badschema", Name: "foo.v2.0.0"}, Message: "deprecated"},
					}},
				},
			},
		},
		{
			name:      "Error/Deprecation/DuplicateRef",
			assertion: hasErrorContaining(`duplicate deprecation entry`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
				Deprecations: []Deprecation{
					{Schema: SchemaDeprecation, Package: "foo", Entries: []DeprecationEntry{
						{Reference: PackageScopedReference{Schema: SchemaBundle, Name: "foo.v0.1.0"}, Message: "bad"},
						{Reference: PackageScopedReference{Schema: SchemaBundle, Name: "foo.v0.1.0"}, Message: "bad"},
					}},
				},
			},
		},

		// Bundle image/objects
		{
			name:      "Error/BundleMissingImageAndObjects",
			assertion: hasError(`bundle image must be set`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: testBundleName("foo", "0.1.0")})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0", withNoBundleImage(), withNoBundleData())},
			},
		},

		// Deprecation message validation
		{
			name:      "Error/Deprecation/EmptyMessage",
			assertion: hasErrorContaining(`must have a message`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "alpha", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: "foo.v0.1.0"})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
				Deprecations: []Deprecation{
					{Schema: SchemaDeprecation, Package: "foo", Entries: []DeprecationEntry{
						{Reference: PackageScopedReference{Schema: SchemaBundle, Name: "foo.v0.1.0"}, Message: ""},
					}},
				},
			},
		},

		// Default channel errors
		{
			name:      "Error/PackageMissingDefaultChannel",
			assertion: hasErrorContaining(`default channel must be set`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "bar", ChannelEntry{Name: testBundleName("foo", "0.1.0")})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Error/PackageNonExistentDefaultChannel",
			assertion: hasErrorContaining(`not found in channels list`),
			cfg: DeclarativeConfig{
				Packages: []Package{newTestPackage("foo", "nonexistent", svgSmallCircle)},
				Channels: []Channel{newTestChannel("foo", "alpha", ChannelEntry{Name: testBundleName("foo", "0.1.0")})},
				Bundles:  []Bundle{newTestBundle("foo", "0.1.0")},
			},
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			err := Validate(s.cfg)
			s.assertion(t, err)
		})
	}
}

func TestValidateChannelGraph(t *testing.T) {
	type spec struct {
		name      string
		ch        Channel
		assertion require.ErrorAssertionFunc
	}
	specs := []spec{
		{
			name: "Success/Valid",
			ch: newTestChannel("anakin", "dark",
				ChannelEntry{Name: "anakin.v0.0.1"},
				ChannelEntry{Name: "anakin.v0.0.2", Skips: []string{"anakin.v0.0.1"}},
				ChannelEntry{Name: "anakin.v0.0.3", Skips: []string{"anakin.v0.0.2"}},
				ChannelEntry{Name: "anakin.v0.0.4", Replaces: "anakin.v0.0.3"},
			),
			assertion: require.NoError,
		},
		{
			name: "Success/SimpleChain",
			ch: newTestChannel("foo", "stable",
				ChannelEntry{Name: "foo.v0.0.1"},
				ChannelEntry{Name: "foo.v0.0.2", Replaces: "foo.v0.0.1"},
				ChannelEntry{Name: "foo.v0.0.3", Replaces: "foo.v0.0.2"},
			),
			assertion: require.NoError,
		},
		{
			name:      "Error/EmptyChannel",
			ch:        newTestChannel("foo", "empty"),
			assertion: hasError(`channel must contain at least one bundle`),
		},
		{
			name: "Error/CycleNoHops",
			ch: newTestChannel("anakin", "dark",
				ChannelEntry{Name: "anakin.v0.0.4", Replaces: "anakin.v0.0.4"},
				ChannelEntry{Name: "anakin.v0.0.5", Replaces: "anakin.v0.0.4"},
			),
			assertion: hasError(`detected cycle in replaces chain of upgrade graph: anakin.v0.0.4 -> anakin.v0.0.4`),
		},
		{
			name: "Error/CycleMultipleHops",
			ch: newTestChannel("anakin", "dark",
				ChannelEntry{Name: "anakin.v0.0.1", Replaces: "anakin.v0.0.3"},
				ChannelEntry{Name: "anakin.v0.0.2", Replaces: "anakin.v0.0.1"},
				ChannelEntry{Name: "anakin.v0.0.3", Replaces: "anakin.v0.0.2"},
				ChannelEntry{Name: "anakin.v0.0.4", Replaces: "anakin.v0.0.3"},
			),
			assertion: hasError(`detected cycle in replaces chain of upgrade graph: anakin.v0.0.3 -> anakin.v0.0.2 -> anakin.v0.0.1 -> anakin.v0.0.3`),
		},
		{
			name: "Error/Stranded",
			ch: newTestChannel("anakin", "dark",
				ChannelEntry{Name: "anakin.v0.0.1"},
				ChannelEntry{Name: "anakin.v0.0.2", Replaces: "anakin.v0.0.1"},
				ChannelEntry{Name: "anakin.v0.0.3", Skips: []string{"anakin.v0.0.2"}},
			),
			assertion: hasError(`channel contains one or more stranded bundles: anakin.v0.0.1`),
		},
		{
			name: "Error/SkippedReplacesStranded",
			ch: newTestChannel("anakin", "dark",
				ChannelEntry{Name: "anakin.v0.0.1"},
				ChannelEntry{Name: "anakin.v0.0.2", Replaces: "anakin.v0.0.1"},
				ChannelEntry{Name: "anakin.v0.0.3", Replaces: "anakin.v0.0.2", Skips: []string{"anakin.v0.0.2"}},
			),
			assertion: hasError(`channel contains one or more stranded bundles: anakin.v0.0.1`),
		},
		{
			name: "Error/MultipleHeads",
			ch: newTestChannel("foo", "stable",
				ChannelEntry{Name: "foo.v0.0.1"},
				ChannelEntry{Name: "foo.v0.0.2"},
			),
			assertion: hasErrorContaining(`multiple channel heads found in graph`),
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			err := validateChannelGraph(s.ch)
			s.assertion(t, err)
		})
	}
}

func hasError(expectedError string) require.ErrorAssertionFunc {
	return func(t require.TestingT, actualError error, args ...interface{}) {
		if stdt, ok := t.(*testing.T); ok {
			stdt.Helper()
		}
		errsToCheck := []error{actualError}
		for len(errsToCheck) > 0 {
			var err error
			err, errsToCheck = errsToCheck[0], errsToCheck[1:]
			if err == nil {
				continue
			}
			var verr *validationError
			if errors.As(err, &verr) {
				if verr.message == expectedError {
					return
				}
				errsToCheck = append(errsToCheck, verr.subErrors...)
			} else if expectedError == err.Error() {
				return
			}
		}
		t.Errorf("expected error to be or contain suberror `%s`, got `%s`", expectedError, actualError)
		t.FailNow()
	}
}

func hasErrorContaining(substring string) require.ErrorAssertionFunc {
	return func(t require.TestingT, actualError error, args ...interface{}) {
		if stdt, ok := t.(*testing.T); ok {
			stdt.Helper()
		}
		require.Error(t, actualError)
		require.Contains(t, actualError.Error(), substring, "expected error to contain %q", substring)
	}
}
