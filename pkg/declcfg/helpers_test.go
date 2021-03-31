package declcfg

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/operator-framework/operator-registry/pkg/model"
	"github.com/operator-framework/operator-registry/pkg/property"
)

func buildValidDeclarativeConfig(includeUnrecognized bool) DeclarativeConfig {
	a001 := newTestBundle("anakin", "0.0.1",
		withChannel("light", ""),
		withChannel("dark", ""),
	)
	a010 := newTestBundle("anakin", "0.1.0",
		withChannel("light", testBundleName("anakin", "0.0.1")),
		withChannel("dark", testBundleName("anakin", "0.0.1")),
	)
	a011 := newTestBundle("anakin", "0.1.1",
		withChannel("dark", testBundleName("anakin", "0.0.1")),
		withSkips(testBundleName("anakin", "0.1.0")),
	)
	b1 := newTestBundle("boba-fett", "1.0.0",
		withChannel("mando", ""),
	)
	b2 := newTestBundle("boba-fett", "2.0.0",
		withChannel("mando", testBundleName("boba-fett", "1.0.0")),
	)

	var others []Meta
	if includeUnrecognized {
		others = []Meta{
			{Schema: "custom.1", Blob: json.RawMessage(`{"schema": "custom.1"}`)},
			{Schema: "custom.2", Blob: json.RawMessage(`{"schema": "custom.2"}`)},
			{Schema: "custom.3", Package: "anakin", Blob: json.RawMessage(`{
				"schema": "custom.3",
				"package": "anakin",
				"myField": "foobar"
			}`)},
			{Schema: "custom.3", Package: "boba-fett", Blob: json.RawMessage(`{
				"schema": "custom.3",
				"package": "boba-fett",
				"myField": "foobar"
			}`)},
		}
	}

	return DeclarativeConfig{
		Packages: []Package{
			newTestPackage("anakin", "dark", svgSmallCircle),
			newTestPackage("boba-fett", "mando", svgBigCircle),
		},
		Bundles: []Bundle{
			a001, a010, a011,
			b1, b2,
		},
		Others: others,
	}
}

type bundleOpt func(*Bundle)

func withChannel(name, replaces string) func(*Bundle) {
	return func(b *Bundle) {
		b.Properties = append(b.Properties, property.MustBuildChannel(name, replaces))
	}
}

func withSkips(name string) func(*Bundle) {
	return func(b *Bundle) {
		b.Properties = append(b.Properties, property.MustBuildSkips(name))
	}
}

func newTestBundle(packageName, version string, opts ...bundleOpt) Bundle {
	csvJson := fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, testBundleName(packageName, version))
	b := Bundle{
		Schema:  schemaBundle,
		Name:    testBundleName(packageName, version),
		Package: packageName,
		Image:   testBundleImage(packageName, version),
		Properties: []property.Property{
			property.MustBuildPackage(packageName, version),
			property.MustBuildBundleObjectRef(filepath.Join("objects", testBundleName(packageName, version)+".csv.yaml")),
			property.MustBuildBundleObjectData([]byte(`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`)),
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
	return b
}

const (
	svgSmallCircle = `<svg viewBox="0 0 100 100"><circle cx="25" cy="25" r="25"/></svg>`
	svgBigCircle   = `<svg viewBox="0 0 100 100"><circle cx="50" cy="50" r="50"/></svg>`
)

func newTestPackage(packageName, defaultChannel, svgData string) Package {
	p := Package{
		Schema:         schemaPackage,
		Name:           packageName,
		DefaultChannel: defaultChannel,
		Icon:           &Icon{Data: []byte(svgData), MediaType: "image/svg+xml"},
		Description:    testPackageDescription(packageName),
	}
	return p
}

func buildTestModel() model.Model {
	return model.Model{
		"anakin":    buildAnakinPkgModel(),
		"boba-fett": buildBobaFettPkgModel(),
	}
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

	for _, chName := range []string{"light", "dark"} {
		ch := &model.Channel{
			Package: pkg,
			Name:    chName,
			Bundles: map[string]*model.Bundle{},
		}
		pkg.Channels[ch.Name] = ch
	}
	pkg.DefaultChannel = pkg.Channels["dark"]

	versions := map[string][]property.Channel{
		"0.0.1": {{Name: "light"}, {Name: "dark"}},
		"0.1.0": {
			{Name: "light", Replaces: testBundleName(pkgName, "0.0.1")},
			{Name: "dark", Replaces: testBundleName(pkgName, "0.0.1")},
		},
		"0.1.1": {{Name: "dark", Replaces: testBundleName(pkgName, "0.0.1")}},
	}
	for version, channels := range versions {
		csvJson := fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, testBundleName(pkgName, version))
		crdJson := `{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`
		props := []property.Property{
			property.MustBuildPackage(pkgName, version),
			property.MustBuildBundleObjectRef(filepath.Join("objects", testBundleName(pkgName, version)+".csv.yaml")),
			property.MustBuildBundleObjectData([]byte(crdJson)),
		}
		for _, channel := range channels {
			props = append(props, property.MustBuild(&channel))
			ch := pkg.Channels[channel.Name]
			bName := testBundleName(pkgName, version)
			bImage := testBundleImage(pkgName, version)
			skips := []string{}
			if version == "0.1.1" {
				skip := testBundleName(pkgName, "0.1.0")
				skips = append(skips, skip)
				props = append(props, property.MustBuildSkips(skip))
			}

			props = append(props)

			bundle := &model.Bundle{
				Package:    pkg,
				Channel:    ch,
				Name:       bName,
				Image:      bImage,
				Replaces:   channel.Replaces,
				Skips:      skips,
				Properties: props,
				RelatedImages: []model.RelatedImage{{
					Name:  "bundle",
					Image: testBundleImage(pkgName, version),
				}},
				CsvJSON: csvJson,
				Objects: []string{
					csvJson,
					crdJson,
				},
			}
			ch.Bundles[bName] = bundle
		}
	}
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
	ch := &model.Channel{
		Package: pkg,
		Name:    "mando",
		Bundles: map[string]*model.Bundle{},
	}
	pkg.Channels[ch.Name] = ch
	pkg.DefaultChannel = ch

	versions := map[string][]property.Channel{
		"1.0.0": {{Name: "mando"}},
		"2.0.0": {{Name: "mando", Replaces: testBundleName(pkgName, "1.0.0")}},
	}
	for version, channels := range versions {
		csvJson := fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, testBundleName(pkgName, version))
		crdJson := `{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`
		props := []property.Property{
			property.MustBuildPackage(pkgName, version),
			property.MustBuildBundleObjectRef(filepath.Join("objects", testBundleName(pkgName, version)+".csv.yaml")),
			property.MustBuildBundleObjectData([]byte(crdJson)),
		}
		for _, channel := range channels {
			props = append(props, property.MustBuild(&channel))
			ch := pkg.Channels[channel.Name]
			bName := testBundleName(pkgName, version)
			bImage := testBundleImage(pkgName, version)
			bundle := &model.Bundle{
				Package:    pkg,
				Channel:    ch,
				Name:       bName,
				Image:      bImage,
				Replaces:   channel.Replaces,
				Properties: props,
				RelatedImages: []model.RelatedImage{{
					Name:  "bundle",
					Image: testBundleImage(pkgName, version),
				}},
				CsvJSON: csvJson,
				Objects: []string{
					csvJson,
					crdJson,
				},
			}
			ch.Bundles[bName] = bundle
		}
	}
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
