package declcfg

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/model"
	"github.com/operator-framework/operator-registry/alpha/property"
)

type validDeclarativeConfigSpec struct {
	IncludeUnrecognized bool
	IncludeDeprecations bool
}

func buildValidDeclarativeConfig(spec validDeclarativeConfigSpec) DeclarativeConfig {
	a001 := newTestBundle("anakin", "0.0.1")
	a010 := newTestBundle("anakin", "0.1.0")
	a011 := newTestBundle("anakin", "0.1.1")
	b1 := newTestBundle("boba-fett", "1.0.0")
	b2 := newTestBundle("boba-fett", "2.0.0")

	var others []Meta
	if spec.IncludeUnrecognized {
		others = []Meta{
			{Schema: "custom.1", Blob: json.RawMessage(`{"schema": "custom.1"}`)},
			{Schema: "custom.2", Blob: json.RawMessage(`{"schema": "custom.2"}`)},
			{Schema: "custom.3", Package: "anakin", Blob: json.RawMessage(`{
				"myField": "foobar",
				"package": "anakin",
				"schema": "custom.3"
			}`)},
			{Schema: "custom.3", Package: "boba-fett", Blob: json.RawMessage(`{
				"myField": "foobar",
				"package": "boba-fett",
				"schema": "custom.3"
			}`)},
		}
	}

	var deprecations []Deprecation
	if spec.IncludeDeprecations {
		deprecations = []Deprecation{
			{
				Schema:  SchemaDeprecation,
				Package: "anakin",
				Entries: []DeprecationEntry{
					{
						Reference: PackageScopedReference{
							Schema: "olm.bundle",
							Name:   testBundleName("anakin", "0.0.1"),
						},
						Message: "This bundle version is deprecated",
					},
					{
						Reference: PackageScopedReference{
							Schema: "olm.channel",
							Name:   "light",
						},
						Message: "This channel is deprecated",
					},
					{
						Reference: PackageScopedReference{
							Schema: "olm.package",
						},
						Message: "This package is deprecated... there is another",
					},
				},
			},
		}
	}

	return DeclarativeConfig{
		Packages: []Package{
			newTestPackage("anakin", "dark", svgSmallCircle),
			newTestPackage("boba-fett", "mando", svgBigCircle),
		},
		Channels: []Channel{
			newTestChannel("anakin", "dark",
				ChannelEntry{
					Name: testBundleName("anakin", "0.0.1"),
				},
				ChannelEntry{
					Name:     testBundleName("anakin", "0.1.0"),
					Replaces: testBundleName("anakin", "0.0.1"),
				},
				ChannelEntry{
					Name:     testBundleName("anakin", "0.1.1"),
					Replaces: testBundleName("anakin", "0.0.1"),
					Skips:    []string{testBundleName("anakin", "0.1.0")},
				},
			),
			newTestChannel("anakin", "light",
				ChannelEntry{
					Name: testBundleName("anakin", "0.0.1"),
				},
				ChannelEntry{
					Name:     testBundleName("anakin", "0.1.0"),
					Replaces: testBundleName("anakin", "0.0.1"),
				},
			),
			newTestChannel("boba-fett", "mando",
				ChannelEntry{
					Name: testBundleName("boba-fett", "1.0.0"),
				},
				ChannelEntry{
					Name:     testBundleName("boba-fett", "2.0.0"),
					Replaces: testBundleName("boba-fett", "1.0.0"),
				},
			),
		},
		Bundles: []Bundle{
			a001, a010, a011,
			b1, b2,
		},
		Others:       others,
		Deprecations: deprecations,
	}
}

type bundleOpt func(*Bundle)

func withNoProperties() func(*Bundle) {
	return func(b *Bundle) {
		b.Properties = []property.Property{}
	}
}

func withNoBundleImage() func(*Bundle) {
	return func(b *Bundle) {
		b.Image = ""
	}
}

func withNoBundleData() func(*Bundle) {
	return func(b *Bundle) {
		b.Objects = []string{}
		b.CsvJSON = ""
	}
}

func newTestBundle(packageName, version string, opts ...bundleOpt) Bundle {
	csvJson := fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, testBundleName(packageName, version))
	b := Bundle{
		Schema:  SchemaBundle,
		Name:    testBundleName(packageName, version),
		Package: packageName,
		Image:   testBundleImage(packageName, version),
		Properties: []property.Property{
			property.MustBuildPackage(packageName, version),
			property.MustBuildBundleObject([]byte(csvJson)),
			property.MustBuildBundleObject([]byte(`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`)),
		},
		RelatedImages: []RelatedImage{
			{
				Name:  "bundle",
				Image: testBundleImage(packageName, version),
			},
		},
		CsvJSON: csvJson,
		Objects: []string{
			csvJson,
			`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`,
		},
	}
	for _, opt := range opts {
		opt(&b)
	}
	sort.Slice(b.Properties, func(i, j int) bool {
		if b.Properties[i].Type != b.Properties[j].Type {
			return b.Properties[i].Type < b.Properties[j].Type
		}
		return string(b.Properties[i].Value) < string(b.Properties[j].Value)
	})
	return b
}

const (
	svgSmallCircle = `<svg viewBox="0 0 100 100"><circle cx="25" cy="25" r="25"/></svg>`
	svgBigCircle   = `<svg viewBox="0 0 100 100"><circle cx="50" cy="50" r="50"/></svg>`
)

func newTestPackage(packageName, defaultChannel, svgData string) Package {
	p := Package{
		Schema:         SchemaPackage,
		Name:           packageName,
		DefaultChannel: defaultChannel,
		Icon:           &Icon{Data: []byte(svgData), MediaType: "image/svg+xml"},
		Description:    testPackageDescription(packageName),
	}
	return p
}

func addPackageProperties(in Package, p []property.Property) Package {
	in.Properties = p
	return in
}

func newTestChannel(packageName, channelName string, entries ...ChannelEntry) Channel {
	return Channel{
		Schema:  SchemaChannel,
		Name:    channelName,
		Package: packageName,
		Entries: entries,
	}
}

func addChannelProperties(in Channel, p []property.Property) Channel {
	in.Properties = p
	return in
}

func buildTestModel() model.Model {
	return model.Model{
		"anakin":    buildAnakinPkgModel(),
		"boba-fett": buildBobaFettPkgModel(),
	}
}

func getBundle(pkg *model.Package, ch *model.Channel, version, replaces string, skips ...string) *model.Bundle {
	return &model.Bundle{
		Package: pkg,
		Channel: ch,
		Name:    testBundleName(pkg.Name, version),
		Image:   testBundleImage(pkg.Name, version),
		Properties: []property.Property{
			property.MustBuildPackage(pkg.Name, version),
			property.MustBuildBundleObject([]byte(getCSVJson(pkg.Name, version))),
			property.MustBuildBundleObject([]byte(getCRDJSON())),
		},
		Replaces: replaces,
		Skips:    skips,
		RelatedImages: []model.RelatedImage{{
			Name:  "bundle",
			Image: testBundleImage(pkg.Name, version),
		}},
		CsvJSON: getCSVJson(pkg.Name, version),
		Objects: []string{
			getCSVJson(pkg.Name, version),
			getCRDJSON(),
		},
		Version: semver.MustParse(version),
	}
}

func getCSVJson(pkgName, version string) string {
	return fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, testBundleName(pkgName, version))
}

func getCRDJSON() string {
	return `{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`
}

func buildAnakinPkgModel() *model.Package {
	pkgName := "anakin"

	pkg := &model.Package{
		Name:        pkgName,
		Description: testPackageDescription(pkgName),
		Icon: &model.Icon{
			Data:      []byte(svgSmallCircle),
			MediaType: "image/svg+xml",
		},
		Channels: map[string]*model.Channel{},
	}

	light := &model.Channel{
		Package: pkg,
		Name:    "light",
		Bundles: map[string]*model.Bundle{},
	}

	dark := &model.Channel{
		Package: pkg,
		Name:    "dark",
		Bundles: map[string]*model.Bundle{},
	}
	light.Bundles[testBundleName(pkgName, "0.0.1")] = getBundle(pkg, light, "0.0.1", "")
	light.Bundles[testBundleName(pkgName, "0.1.0")] = getBundle(pkg, light, "0.1.0", testBundleName(pkgName, "0.0.1"))

	dark.Bundles[testBundleName(pkgName, "0.0.1")] = getBundle(pkg, dark, "0.0.1", "")
	dark.Bundles[testBundleName(pkgName, "0.1.0")] = getBundle(pkg, dark, "0.1.0", testBundleName(pkgName, "0.0.1"))
	dark.Bundles[testBundleName(pkgName, "0.1.1")] = getBundle(pkg, dark, "0.1.1", testBundleName(pkgName, "0.0.1"), testBundleName(pkgName, "0.1.0"))

	pkg.Channels["light"] = light
	pkg.Channels["dark"] = dark
	pkg.DefaultChannel = pkg.Channels["dark"]
	return pkg
}

func buildBobaFettPkgModel() *model.Package {
	pkgName := "boba-fett"
	pkg := &model.Package{
		Name:        pkgName,
		Description: testPackageDescription(pkgName),
		Icon: &model.Icon{
			Data:      []byte(svgBigCircle),
			MediaType: "image/svg+xml",
		},
		Channels: map[string]*model.Channel{},
	}
	mando := &model.Channel{
		Package: pkg,
		Name:    "mando",
		Bundles: map[string]*model.Bundle{},
	}
	mando.Bundles[testBundleName(pkgName, "1.0.0")] = getBundle(pkg, mando, "1.0.0", "")
	mando.Bundles[testBundleName(pkgName, "2.0.0")] = getBundle(pkg, mando, "2.0.0", testBundleName(pkgName, "1.0.0"))
	pkg.Channels["mando"] = mando
	pkg.DefaultChannel = mando
	return pkg
}

func testPackageDescription(pkg string) string {
	return fmt.Sprintf("%s operator", pkg)
}

func testBundleName(pkg, version string) string {
	return fmt.Sprintf("%s.v%s", pkg, version)
}

func testBundleImage(pkg, version string) string {
	return fmt.Sprintf("%s-bundle:v%s", pkg, version)
}

func equalsDeclarativeConfig(t *testing.T, expected, actual DeclarativeConfig) {
	t.Helper()
	removeJSONWhitespace(&expected)
	removeJSONWhitespace(&actual)

	assert.ElementsMatch(t, expected.Packages, actual.Packages)
	assert.ElementsMatch(t, expected.Others, actual.Others)

	// When comparing bundles, the order of properties doesn't matter.
	// Unfortunately, assert.ElementsMatch() only ignores ordering of
	// root elements, so we need to manually sort bundles and use
	// assert.ElementsMatch on the properties fields between
	// expected and actual.
	require.Equal(t, len(expected.Bundles), len(actual.Bundles))
	sort.SliceStable(expected.Bundles, func(i, j int) bool {
		return expected.Bundles[i].Name < expected.Bundles[j].Name
	})
	sort.SliceStable(actual.Bundles, func(i, j int) bool {
		return actual.Bundles[i].Name < actual.Bundles[j].Name
	})
	for i := range expected.Bundles {
		assert.ElementsMatch(t, expected.Bundles[i].Properties, actual.Bundles[i].Properties)
		expected.Bundles[i].Properties, actual.Bundles[i].Properties = nil, nil
		assert.Equal(t, expected.Bundles[i], actual.Bundles[i])
	}

	// In case new fields are added to the DeclarativeConfig struct in the future,
	// test that the rest is Equal.
	expected.Packages, actual.Packages = nil, nil
	expected.Bundles, actual.Bundles = nil, nil
	expected.Others, actual.Others = nil, nil
	assert.Equal(t, expected, actual)
}
