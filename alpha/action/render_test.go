package action_test

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

func TestRender(t *testing.T) {
	type spec struct {
		name      string
		render    action.Render
		expectCfg *declcfg.DeclarativeConfig
		assertion require.ErrorAssertionFunc
	}

	reg, err := newRegistry()
	require.NoError(t, err)
	foov1csv, err := bundleImageV1.ReadFile("testdata/foo-bundle-v0.1.0/manifests/foo.v0.1.0.csv.yaml")
	require.NoError(t, err)
	foov1crd, err := bundleImageV1.ReadFile("testdata/foo-bundle-v0.1.0/manifests/foos.test.foo.crd.yaml")
	require.NoError(t, err)
	foov2csv, err := bundleImageV2.ReadFile("testdata/foo-bundle-v0.2.0/manifests/foo.v0.2.0.csv.yaml")
	require.NoError(t, err)
	foov2crd, err := bundleImageV2.ReadFile("testdata/foo-bundle-v0.2.0/manifests/foos.test.foo.crd.yaml")
	require.NoError(t, err)

	foov1csv, err = yaml.ToJSON(foov1csv)
	require.NoError(t, err)
	foov1crd, err = yaml.ToJSON(foov1crd)
	require.NoError(t, err)
	foov2csv, err = yaml.ToJSON(foov2csv)
	require.NoError(t, err)
	foov2crd, err = yaml.ToJSON(foov2crd)
	require.NoError(t, err)

	dir := t.TempDir()
	dbFile := filepath.Join(dir, "index.db")
	imageMap := map[image.Reference]string{
		image.SimpleReference("test.registry/foo-operator/foo-bundle:v0.1.0"): "testdata/foo-bundle-v0.1.0",
		image.SimpleReference("test.registry/foo-operator/foo-bundle:v0.2.0"): "testdata/foo-bundle-v0.2.0",
	}
	assert.NoError(t, generateSqliteFile(dbFile, imageMap))

	specs := []spec{
		{
			name: "Success/SqliteIndexImage",
			render: action.Render{
				Refs:     []string{"test.registry/foo-operator/foo-index-sqlite:v0.2.0"},
				Registry: reg,
			},
			expectCfg: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema:         "olm.package",
						Name:           "foo",
						DefaultChannel: "beta",
					},
				},
				Bundles: []declcfg.Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "test.registry/foo-operator/foo-bundle:v0.1.0",
						Properties: []property.Property{
							property.MustBuildChannel("beta", ""),
							property.MustBuildChannel("stable", ""),
							property.MustBuildGVK("test.foo", "v1", "Foo"),
							property.MustBuildGVKRequired("test.bar", "v1alpha1", "Bar"),
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("bar", "<0.1.0"),
							property.MustBuildSkipRange("<0.1.0"),
							property.MustBuildBundleObjectData(foov1csv),
							property.MustBuildBundleObjectData(foov1crd),
						},
						RelatedImages: []declcfg.RelatedImage{
							{
								Name:  "operator",
								Image: "test.registry/foo-operator/foo:v0.1.0",
							},
							{
								Image: "test.registry/foo-operator/foo-bundle:v0.1.0",
							},
						},
						CsvJSON: string(foov1csv),
						Objects: []string{string(foov1csv), string(foov1crd)},
					},
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "test.registry/foo-operator/foo-bundle:v0.2.0",
						Properties: []property.Property{
							property.MustBuildChannel("beta", "foo.v0.1.0"),
							property.MustBuildChannel("stable", "foo.v0.1.0"),
							property.MustBuildGVK("test.foo", "v1", "Foo"),
							property.MustBuildGVKRequired("test.bar", "v1alpha1", "Bar"),
							property.MustBuildPackage("foo", "0.2.0"),
							property.MustBuildPackageRequired("bar", "<0.1.0"),
							property.MustBuildSkipRange("<0.2.0"),
							property.MustBuildSkips("foo.v0.1.1"),
							property.MustBuildSkips("foo.v0.1.2"),
							property.MustBuildBundleObjectData(foov2csv),
							property.MustBuildBundleObjectData(foov2crd),
						},
						RelatedImages: []declcfg.RelatedImage{
							{
								Name:  "operator",
								Image: "test.registry/foo-operator/foo:v0.2.0",
							},
							{
								Image: "test.registry/foo-operator/foo-bundle:v0.2.0",
							},
						},
						CsvJSON: string(foov2csv),
						Objects: []string{string(foov2csv), string(foov2crd)},
					},
				},
			},
			assertion: require.NoError,
		},
		{
			name: "Success/SqliteFile",
			render: action.Render{
				Refs:     []string{dbFile},
				Registry: reg,
			},
			expectCfg: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema:         "olm.package",
						Name:           "foo",
						DefaultChannel: "beta",
					},
				},
				Bundles: []declcfg.Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "test.registry/foo-operator/foo-bundle:v0.1.0",
						Properties: []property.Property{
							property.MustBuildChannel("beta", ""),
							property.MustBuildChannel("stable", ""),
							property.MustBuildGVK("test.foo", "v1", "Foo"),
							property.MustBuildGVKRequired("test.bar", "v1alpha1", "Bar"),
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("bar", "<0.1.0"),
							property.MustBuildSkipRange("<0.1.0"),
							property.MustBuildBundleObjectData(foov1csv),
							property.MustBuildBundleObjectData(foov1crd),
						},
						RelatedImages: []declcfg.RelatedImage{
							{
								Name:  "operator",
								Image: "test.registry/foo-operator/foo:v0.1.0",
							},
							{
								Image: "test.registry/foo-operator/foo-bundle:v0.1.0",
							},
						},
						CsvJSON: string(foov1csv),
						Objects: []string{string(foov1csv), string(foov1crd)},
					},
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "test.registry/foo-operator/foo-bundle:v0.2.0",
						Properties: []property.Property{
							property.MustBuildChannel("beta", "foo.v0.1.0"),
							property.MustBuildChannel("stable", "foo.v0.1.0"),
							property.MustBuildGVK("test.foo", "v1", "Foo"),
							property.MustBuildGVKRequired("test.bar", "v1alpha1", "Bar"),
							property.MustBuildPackage("foo", "0.2.0"),
							property.MustBuildPackageRequired("bar", "<0.1.0"),
							property.MustBuildSkipRange("<0.2.0"),
							property.MustBuildSkips("foo.v0.1.1"),
							property.MustBuildSkips("foo.v0.1.2"),
							property.MustBuildBundleObjectData(foov2csv),
							property.MustBuildBundleObjectData(foov2crd),
						},
						RelatedImages: []declcfg.RelatedImage{
							{
								Name:  "operator",
								Image: "test.registry/foo-operator/foo:v0.2.0",
							},
							{
								Image: "test.registry/foo-operator/foo-bundle:v0.2.0",
							},
						},
						CsvJSON: string(foov2csv),
						Objects: []string{string(foov2csv), string(foov2crd)},
					},
				},
			},
			assertion: require.NoError,
		},
		{
			name: "Success/DeclcfgIndexImage",
			render: action.Render{
				Refs:     []string{"test.registry/foo-operator/foo-index-declcfg:v0.2.0"},
				Registry: reg,
			},
			expectCfg: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema:         "olm.package",
						Name:           "foo",
						DefaultChannel: "beta",
					},
				},
				Bundles: []declcfg.Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "test.registry/foo-operator/foo-bundle:v0.1.0",
						Properties: []property.Property{
							property.MustBuildChannel("beta", ""),
							property.MustBuildGVK("test.foo", "v1", "Foo"),
							property.MustBuildGVKRequired("test.bar", "v1alpha1", "Bar"),
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("bar", "<0.1.0"),
							property.MustBuildSkipRange("<0.1.0"),
							property.MustBuildBundleObjectData(foov1csv),
							property.MustBuildBundleObjectData(foov1crd),
						},
						RelatedImages: []declcfg.RelatedImage{
							{
								Name:  "operator",
								Image: "test.registry/foo-operator/foo:v0.1.0",
							},
							{
								Image: "test.registry/foo-operator/foo-bundle:v0.1.0",
							},
						},
						CsvJSON: string(foov1csv),
						Objects: []string{string(foov1csv), string(foov1crd)},
					},
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "test.registry/foo-operator/foo-bundle:v0.2.0",
						Properties: []property.Property{
							property.MustBuildChannel("beta", "foo.v0.1.0"),
							property.MustBuildChannel("stable", "foo.v0.1.0"),
							property.MustBuildGVK("test.foo", "v1", "Foo"),
							property.MustBuildGVKRequired("test.bar", "v1alpha1", "Bar"),
							property.MustBuildPackage("foo", "0.2.0"),
							property.MustBuildPackageRequired("bar", "<0.1.0"),
							property.MustBuildSkipRange("<0.2.0"),
							property.MustBuildSkips("foo.v0.1.1"),
							property.MustBuildSkips("foo.v0.1.2"),
							property.MustBuildBundleObjectData(foov2csv),
							property.MustBuildBundleObjectData(foov2crd),
						},
						RelatedImages: []declcfg.RelatedImage{
							{
								Name:  "operator",
								Image: "test.registry/foo-operator/foo:v0.2.0",
							},
							{
								Image: "test.registry/foo-operator/foo-bundle:v0.2.0",
							},
						},
						CsvJSON: string(foov2csv),
						Objects: []string{string(foov2csv), string(foov2crd)},
					},
				},
			},
			assertion: require.NoError,
		},
		{
			name: "Success/DeclcfgDirectory",
			render: action.Render{
				Refs:     []string{"testdata/foo-index-v0.2.0-declcfg"},
				Registry: reg,
			},
			expectCfg: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema:         "olm.package",
						Name:           "foo",
						DefaultChannel: "beta",
					},
				},
				Bundles: []declcfg.Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "test.registry/foo-operator/foo-bundle:v0.1.0",
						Properties: []property.Property{
							property.MustBuildChannel("beta", ""),
							property.MustBuildGVK("test.foo", "v1", "Foo"),
							property.MustBuildGVKRequired("test.bar", "v1alpha1", "Bar"),
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("bar", "<0.1.0"),
							property.MustBuildSkipRange("<0.1.0"),
							property.MustBuildBundleObjectData(foov1csv),
							property.MustBuildBundleObjectData(foov1crd),
						},
						RelatedImages: []declcfg.RelatedImage{
							{
								Name:  "operator",
								Image: "test.registry/foo-operator/foo:v0.1.0",
							},
							{
								Image: "test.registry/foo-operator/foo-bundle:v0.1.0",
							},
						},
						CsvJSON: string(foov1csv),
						Objects: []string{string(foov1csv), string(foov1crd)},
					},
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "test.registry/foo-operator/foo-bundle:v0.2.0",
						Properties: []property.Property{
							property.MustBuildChannel("beta", "foo.v0.1.0"),
							property.MustBuildChannel("stable", "foo.v0.1.0"),
							property.MustBuildGVK("test.foo", "v1", "Foo"),
							property.MustBuildGVKRequired("test.bar", "v1alpha1", "Bar"),
							property.MustBuildPackage("foo", "0.2.0"),
							property.MustBuildPackageRequired("bar", "<0.1.0"),
							property.MustBuildSkipRange("<0.2.0"),
							property.MustBuildSkips("foo.v0.1.1"),
							property.MustBuildSkips("foo.v0.1.2"),
							property.MustBuildBundleObjectData(foov2csv),
							property.MustBuildBundleObjectData(foov2crd),
						},
						RelatedImages: []declcfg.RelatedImage{
							{
								Name:  "operator",
								Image: "test.registry/foo-operator/foo:v0.2.0",
							},
							{
								Image: "test.registry/foo-operator/foo-bundle:v0.2.0",
							},
						},
						CsvJSON: string(foov2csv),
						Objects: []string{string(foov2csv), string(foov2crd)},
					},
				},
			},
			assertion: require.NoError,
		},
		{
			name: "Success/BundleImage",
			render: action.Render{
				Refs:     []string{"test.registry/foo-operator/foo-bundle:v0.2.0"},
				Registry: reg,
			},
			expectCfg: &declcfg.DeclarativeConfig{
				Bundles: []declcfg.Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "test.registry/foo-operator/foo-bundle:v0.2.0",
						Properties: []property.Property{
							property.MustBuildChannel("beta", "foo.v0.1.0"),
							property.MustBuildChannel("stable", "foo.v0.1.0"),
							property.MustBuildGVK("test.foo", "v1", "Foo"),
							property.MustBuildGVKRequired("test.bar", "v1alpha1", "Bar"),
							property.MustBuildPackage("foo", "0.2.0"),
							property.MustBuildPackageRequired("bar", "<0.1.0"),
							property.MustBuildSkipRange("<0.2.0"),
							property.MustBuildSkips("foo.v0.1.1"),
							property.MustBuildSkips("foo.v0.1.2"),
						},
						RelatedImages: []declcfg.RelatedImage{
							{
								Name:  "operator",
								Image: "test.registry/foo-operator/foo:v0.2.0",
							},
						},
					},
				},
			},
			assertion: require.NoError,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			actualCfg, actualErr := s.render.Run(context.Background())
			s.assertion(t, actualErr)
			require.Equal(t, s.expectCfg, actualCfg)
		})
	}
}

func TestAllowRefMask(t *testing.T) {
	type spec struct {
		name      string
		render    action.Render
		expectErr error
	}

	reg, err := newRegistry()
	require.NoError(t, err)

	dir := t.TempDir()
	dbFile := filepath.Join(dir, "index.db")
	imageMap := map[image.Reference]string{
		image.SimpleReference("test.registry/foo-operator/foo-bundle:v0.1.0"): "testdata/foo-bundle-v0.1.0",
		image.SimpleReference("test.registry/foo-operator/foo-bundle:v0.2.0"): "testdata/foo-bundle-v0.2.0",
	}
	assert.NoError(t, generateSqliteFile(dbFile, imageMap))

	specs := []spec{
		{
			name: "SqliteImage/Allowed",
			render: action.Render{
				Refs:           []string{"test.registry/foo-operator/foo-index-sqlite:v0.2.0"},
				Registry:       reg,
				AllowedRefMask: action.RefSqliteImage,
			},
			expectErr: nil,
		},
		{
			name: "SqliteImage/NotAllowed",
			render: action.Render{
				Refs:           []string{"test.registry/foo-operator/foo-index-sqlite:v0.2.0"},
				Registry:       reg,
				AllowedRefMask: action.RefDCImage | action.RefDCDir | action.RefSqliteFile | action.RefBundleImage,
			},
			expectErr: action.ErrNotAllowed,
		},
		{
			name: "SqliteFile/Allowed",
			render: action.Render{
				Refs:           []string{dbFile},
				Registry:       reg,
				AllowedRefMask: action.RefSqliteFile,
			},
			expectErr: nil,
		},
		{
			name: "SqliteFile/NotAllowed",
			render: action.Render{
				Refs:           []string{dbFile},
				Registry:       reg,
				AllowedRefMask: action.RefDCImage | action.RefDCDir | action.RefSqliteImage | action.RefBundleImage,
			},
			expectErr: action.ErrNotAllowed,
		},
		{
			name: "DeclcfgImage/Allowed",
			render: action.Render{
				Refs:           []string{"test.registry/foo-operator/foo-index-declcfg:v0.2.0"},
				Registry:       reg,
				AllowedRefMask: action.RefDCImage,
			},
			expectErr: nil,
		},
		{
			name: "DeclcfgImage/NotAllowed",
			render: action.Render{
				Refs:           []string{"test.registry/foo-operator/foo-index-declcfg:v0.2.0"},
				Registry:       reg,
				AllowedRefMask: action.RefDCDir | action.RefSqliteImage | action.RefSqliteFile | action.RefBundleImage,
			},
			expectErr: action.ErrNotAllowed,
		},
		{
			name: "DeclcfgDir/Allowed",
			render: action.Render{
				Refs:           []string{"testdata/foo-index-v0.2.0-declcfg"},
				Registry:       reg,
				AllowedRefMask: action.RefDCDir,
			},
			expectErr: nil,
		},
		{
			name: "DeclcfgDir/NotAllowed",
			render: action.Render{
				Refs:           []string{"testdata/foo-index-v0.2.0-declcfg"},
				Registry:       reg,
				AllowedRefMask: action.RefDCImage | action.RefSqliteImage | action.RefSqliteFile | action.RefBundleImage,
			},
			expectErr: action.ErrNotAllowed,
		},
		{
			name: "BundleImage/Allowed",
			render: action.Render{
				Refs:           []string{"test.registry/foo-operator/foo-bundle:v0.2.0"},
				Registry:       reg,
				AllowedRefMask: action.RefBundleImage,
			},
			expectErr: nil,
		},
		{
			name: "BundleImage/NotAllowed",
			render: action.Render{
				Refs:           []string{"test.registry/foo-operator/foo-bundle:v0.2.0"},
				Registry:       reg,
				AllowedRefMask: action.RefDCImage | action.RefDCDir | action.RefSqliteImage | action.RefSqliteFile,
			},
			expectErr: action.ErrNotAllowed,
		},
		{
			name: "All/Allowed",
			render: action.Render{
				Refs: []string{
					"test.registry/foo-operator/foo-index-sqlite:v0.2.0",
					dbFile,
					"test.registry/foo-operator/foo-index-declcfg:v0.2.0",
					"testdata/foo-index-v0.2.0-declcfg",
					"test.registry/foo-operator/foo-bundle:v0.2.0",
				},
				Registry: reg,
			},
			expectErr: nil,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			_, err := s.render.Run(context.Background())
			require.True(t, errors.Is(err, s.expectErr), "expected error %#v to be %#v", err, s.expectErr)
		})
	}
}

func TestAllowRefMaskAllowed(t *testing.T) {
	type spec struct {
		name   string
		mask   action.RefType
		pass   []action.RefType
		fail   []action.RefType
		expect bool
	}

	specs := []spec{
		{
			name: "Mask/All",
			mask: action.RefAll,
			pass: []action.RefType{
				action.RefDCImage,
				action.RefDCDir,
				action.RefSqliteImage,
				action.RefSqliteFile,
				action.RefBundleImage,
			},
			fail: []action.RefType{},
		},
		{
			name: "Mask/One",
			mask: action.RefDCImage,
			pass: []action.RefType{
				action.RefDCImage,
			},
			fail: []action.RefType{
				action.RefDCDir,
				action.RefSqliteImage,
				action.RefSqliteFile,
				action.RefBundleImage,
			},
		},
		{
			name: "Mask/Some",
			mask: action.RefDCImage | action.RefDCDir,
			pass: []action.RefType{
				action.RefDCImage,
				action.RefDCDir,
			},
			fail: []action.RefType{
				action.RefSqliteImage,
				action.RefSqliteFile,
				action.RefBundleImage,
			},
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			for _, c := range s.pass {
				actual := s.mask.Allowed(c)
				require.True(t, actual)
			}
			for _, c := range s.fail {
				actual := s.mask.Allowed(c)
				require.False(t, actual)
			}
		})
	}
}

//go:embed testdata/foo-bundle-v0.1.0/manifests/*
//go:embed testdata/foo-bundle-v0.1.0/metadata/*
var bundleImageV1 embed.FS

//go:embed testdata/foo-bundle-v0.2.0/manifests/*
//go:embed testdata/foo-bundle-v0.2.0/metadata/*
var bundleImageV2 embed.FS

//go:embed testdata/foo-index-v0.2.0-declcfg/foo/*
var declcfgImage embed.FS

func newRegistry() (image.Registry, error) {
	imageMap := map[image.Reference]string{
		image.SimpleReference("test.registry/foo-operator/foo-bundle:v0.1.0"): "testdata/foo-bundle-v0.1.0",
		image.SimpleReference("test.registry/foo-operator/foo-bundle:v0.2.0"): "testdata/foo-bundle-v0.2.0",
	}

	subSqliteImage, err := generateSqliteFS(imageMap)
	if err != nil {
		return nil, err
	}
	subDeclcfgImage, err := fs.Sub(declcfgImage, "testdata/foo-index-v0.2.0-declcfg")
	if err != nil {
		return nil, err
	}
	subBundleImageV1, err := fs.Sub(bundleImageV2, "testdata/foo-bundle-v0.1.0")
	if err != nil {
		return nil, err
	}
	subBundleImageV2, err := fs.Sub(bundleImageV2, "testdata/foo-bundle-v0.2.0")
	if err != nil {
		return nil, err
	}
	return &image.MockRegistry{
		RemoteImages: map[image.Reference]*image.MockImage{
			image.SimpleReference("test.registry/foo-operator/foo-index-sqlite:v0.2.0"): {
				Labels: map[string]string{
					containertools.DbLocationLabel: "/database/index.db",
				},
				FS: subSqliteImage,
			},
			image.SimpleReference("test.registry/foo-operator/foo-index-declcfg:v0.2.0"): {
				Labels: map[string]string{
					"operators.operatorframework.io.index.configs.v1": "/foo",
				},
				FS: subDeclcfgImage,
			},
			image.SimpleReference("test.registry/foo-operator/foo-bundle:v0.1.0"): {
				Labels: map[string]string{
					bundle.PackageLabel: "foo",
				},
				FS: subBundleImageV1,
			},
			image.SimpleReference("test.registry/foo-operator/foo-bundle:v0.2.0"): {
				Labels: map[string]string{
					bundle.PackageLabel: "foo",
				},
				FS: subBundleImageV2,
			},
		},
	}, nil
}

func generateSqliteFS(imageMap map[image.Reference]string) (fs.FS, error) {
	dir, err := os.MkdirTemp("", "opm-render-test-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	dbFile := filepath.Join(dir, "index.db")
	if err := generateSqliteFile(dbFile, imageMap); err != nil {
		return nil, err
	}

	dbData, err := os.ReadFile(dbFile)
	if err != nil {
		return nil, err
	}

	return &fstest.MapFS{
		"database/index.db": &fstest.MapFile{
			Data: dbData,
		},
	}, nil
}

func generateSqliteFile(path string, imageMap map[image.Reference]string) error {
	db, err := sqlite.Open(path)
	if err != nil {
		return err
	}
	defer db.Close()

	m, err := sqlite.NewSQLLiteMigrator(db)
	if err != nil {
		return err
	}
	if err := m.Migrate(context.Background()); err != nil {
		return err
	}

	graphLoader, err := sqlite.NewSQLGraphLoaderFromDB(db)
	if err != nil {
		return err
	}
	dbQuerier := sqlite.NewSQLLiteQuerierFromDb(db)

	loader, err := sqlite.NewSQLLiteLoader(db)
	if err != nil {
		return err
	}

	populator := registry.NewDirectoryPopulator(loader, graphLoader, dbQuerier, imageMap, nil, false)
	if err := populator.Populate(registry.ReplacesMode); err != nil {
		return err
	}
	return nil
}
