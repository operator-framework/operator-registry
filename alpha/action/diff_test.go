package action

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
)

func TestDiff(t *testing.T) {
	type spec struct {
		name        string
		diff        Diff
		expectedCfg *declcfg.DeclarativeConfig
		assertion   require.ErrorAssertionFunc
	}

	registry, err := newDiffRegistry()
	require.NoError(t, err)

	specs := []spec{
		{
			name: "Success/Latest",
			diff: Diff{
				Registry: registry,
				OldRefs:  []string{filepath.Join("testdata", "index-declcfgs", "old")},
				NewRefs:  []string{filepath.Join("testdata", "index-declcfgs", "latest")},
			},
			expectedCfg: loadDirFS(t, indicesDir, filepath.Join("testdata", "index-declcfgs", "exp-latest")),
			assertion:   require.NoError,
		},
		{
			name: "Success/HeadsOnly",
			diff: Diff{
				Registry:  registry,
				NewRefs:   []string{filepath.Join("testdata", "index-declcfgs", "latest")},
				HeadsOnly: true,
			},
			expectedCfg: loadDirFS(t, indicesDir, filepath.Join("testdata", "index-declcfgs", "exp-headsonly")),
			assertion:   require.NoError,
		},
		{
			name: "Success/IncludePackage",
			diff: Diff{
				Registry: registry,
				NewRefs:  []string{filepath.Join("testdata", "index-declcfgs", "latest")},
				IncludeConfig: DiffIncludeConfig{
					Packages: []DiffIncludePackage{{Name: "baz"}},
				},
				IncludeAdditively: true,
				HeadsOnly:         true,
			},
			expectedCfg: loadDirFS(t, indicesDir, filepath.Join("testdata", "index-declcfgs", "exp-include-pkg")),
			assertion:   require.NoError,
		},
		{
			name: "Success/IncludeChannel",
			diff: Diff{
				Registry: registry,
				NewRefs:  []string{filepath.Join("testdata", "index-declcfgs", "latest")},
				IncludeConfig: DiffIncludeConfig{
					Packages: []DiffIncludePackage{
						{
							Name:     "baz",
							Channels: []DiffIncludeChannel{{Name: "stable"}},
						},
					},
				},
				IncludeAdditively: true,
				HeadsOnly:         true,
			},
			expectedCfg: loadDirFS(t, indicesDir, filepath.Join("testdata", "index-declcfgs", "exp-include-channel")),
			assertion:   require.NoError,
		},
		{
			name: "Success/IncludeVersion",
			diff: Diff{
				Registry: registry,
				NewRefs:  []string{filepath.Join("testdata", "index-declcfgs", "latest")},
				IncludeConfig: DiffIncludeConfig{
					Packages: []DiffIncludePackage{
						{
							Name:     "baz",
							Versions: []semver.Version{semver.MustParse("1.0.0")},
						},
					},
				},
				IncludeAdditively: true,
				HeadsOnly:         true,
			},
			expectedCfg: loadDirFS(t, indicesDir, filepath.Join("testdata", "index-declcfgs", "exp-include-channel")),
			assertion:   require.NoError,
		},
		{
			name: "Success/IncludeBundle",
			diff: Diff{
				Registry: registry,
				NewRefs:  []string{filepath.Join("testdata", "index-declcfgs", "latest")},
				IncludeConfig: DiffIncludeConfig{
					Packages: []DiffIncludePackage{
						{
							Name:    "baz",
							Bundles: []string{"baz.v1.0.0"},
						},
					},
				},
				IncludeAdditively: true,
				HeadsOnly:         true,
			},
			expectedCfg: loadDirFS(t, indicesDir, filepath.Join("testdata", "index-declcfgs", "exp-include-channel")),
			assertion:   require.NoError,
		},
		{
			name: "Success/IncludeSameVersionAndBundle",
			diff: Diff{
				Registry: registry,
				NewRefs:  []string{filepath.Join("testdata", "index-declcfgs", "latest")},
				IncludeConfig: DiffIncludeConfig{
					Packages: []DiffIncludePackage{
						{
							Name:     "baz",
							Versions: []semver.Version{semver.MustParse("1.0.0")},
							Bundles:  []string{"baz.v1.0.0"},
						},
					},
				},
				IncludeAdditively: true,
				HeadsOnly:         true,
			},
			expectedCfg: loadDirFS(t, indicesDir, filepath.Join("testdata", "index-declcfgs", "exp-include-channel")),
			assertion:   require.NoError,
		},
		{
			name: "Fail/NewBundleImage",
			diff: Diff{
				Registry:  registry,
				NewRefs:   []string{"test.registry/foo-operator/foo-bundle:v0.1.0"},
				HeadsOnly: true,
			},
			assertion: func(t require.TestingT, err error, _ ...interface{}) {
				if !assert.Error(t, err) {
					require.Fail(t, "expected an error")
				}
				if !errors.Is(err, ErrNotAllowed) {
					require.Fail(t, "err is not ErrNotAllowed", err)
				}
			},
		},
		{
			name: "Fail/OldBundleImage",
			diff: Diff{
				Registry: registry,
				OldRefs:  []string{"test.registry/foo-operator/foo-bundle:v0.1.0"},
				NewRefs:  []string{filepath.Join("testdata", "index-declcfgs", "latest")},
			},
			assertion: func(t require.TestingT, err error, _ ...interface{}) {
				if !assert.Error(t, err) {
					require.Fail(t, "expected an error")
				}
				if !errors.Is(err, ErrNotAllowed) {
					require.Fail(t, "err is not ErrNotAllowed", err)
				}
			},
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			actualCfg, actualErr := s.diff.Run(context.Background())
			s.assertion(t, actualErr)
			require.Equal(t, s.expectedCfg, actualCfg)
		})
	}
}

func TestLoadDiffIncludeConfig(t *testing.T) {
	type spec struct {
		name             string
		input            string
		expectedCfg      DiffIncludeConfig
		expectedIncluder declcfg.DiffIncluder
		assertion        require.ErrorAssertionFunc
	}

	specs := []spec{
		{
			name: "Success/Basic",
			input: `
packages:
- name: foo
`,
			expectedCfg: DiffIncludeConfig{
				Packages: []DiffIncludePackage{{Name: "foo"}},
			},
			expectedIncluder: declcfg.DiffIncluder{
				Packages: []declcfg.DiffIncludePackage{{Name: "foo"}},
			},
			assertion: require.NoError,
		},
		{
			name: "Success/MultiPackage",
			input: `
packages:
- name: foo
  channels:
  - name: stable
    bundles:
    - foo.v0.3.0
    versions:
    - 0.1.0
    - 0.2.0
  versions:
  - 1.0.0
- name: bar
  channels:
  - name: stable
    versions:
    - 0.1.0
  versions:
  - 1.0.0
  bundles:
  - bar.v1.2.0
`,
			expectedCfg: DiffIncludeConfig{
				Packages: []DiffIncludePackage{
					{
						Name: "foo",
						Channels: []DiffIncludeChannel{
							{
								Name:     "stable",
								Versions: []semver.Version{semver.MustParse("0.1.0"), semver.MustParse("0.2.0")},
								Bundles:  []string{"foo.v0.3.0"},
							},
						},
						Versions: []semver.Version{semver.MustParse("1.0.0")},
					},
					{
						Name: "bar",
						Channels: []DiffIncludeChannel{
							{Name: "stable", Versions: []semver.Version{
								semver.MustParse("0.1.0"),
							}},
						},
						Versions: []semver.Version{semver.MustParse("1.0.0")},
						Bundles:  []string{"bar.v1.2.0"},
					},
				},
			},
			expectedIncluder: declcfg.DiffIncluder{
				Packages: []declcfg.DiffIncludePackage{
					{
						Name: "foo",
						Channels: []declcfg.DiffIncludeChannel{
							{
								Name:     "stable",
								Versions: []semver.Version{semver.MustParse("0.1.0"), semver.MustParse("0.2.0")},
								Bundles:  []string{"foo.v0.3.0"},
							},
						},
						AllChannels: declcfg.DiffIncludeChannel{
							Versions: []semver.Version{semver.MustParse("1.0.0")},
						},
					},
					{
						Name: "bar",
						Channels: []declcfg.DiffIncludeChannel{
							{Name: "stable", Versions: []semver.Version{
								semver.MustParse("0.1.0"),
							}},
						},
						AllChannels: declcfg.DiffIncludeChannel{
							Versions: []semver.Version{semver.MustParse("1.0.0")},
							Bundles:  []string{"bar.v1.2.0"},
						},
					},
				},
			},
			assertion: require.NoError,
		},
		{
			name:      "Fail/Empty",
			input:     ``,
			assertion: require.Error,
		},
		{
			name: "Fail/NoPackageName",
			input: `
packages:
- channels:
  - name: stable
    versions:
    - 0.1.0
`,
			assertion: require.Error,
		},
		{
			name: "Fail/NoChannelName",
			input: `
packages:
- name: foo
  channels:
  - versions:
    - 0.1.0
`,
			assertion: require.Error,
		},
		{
			name: "Fail/InvalidPackageRange",
			input: `
			{
			  "packages": [
			    {
			      "name": "foo",
			      "range": "test"
			    }
			  ]
			}`,
			assertion: require.Error,
		},
		{
			name: "Fail/InvalidChannelRange",
			input: `
			{
			  "packages": [
			    {
			      "name": "foo",
			      "channels": [
			        {
			          "name": "stable",
			          "range": "test"
			        }
			      ]
			    }
			  ]
			}`,
			assertion: require.Error,
		},
		{
			name: "Fail/InvalidRangeSetting/MixedRange&ChannelRange",
			input: `
			{
			  "packages": [
			    {
			      "name": "foo",
			      "range": "test",
			      "channels": [
			        {
			          "name": "stable",
			          "range": "test"
			        }
			      ]
			    }
			  ]
			}`,
			assertion: require.Error,
		},
		{
			name: "Fail/InvalidRangeSetting/MixedRange&OtherVersions",
			input: `
			{
			  "packages": [
			    {
			      "name": "foo",
			      "channels": [
			        {
			          "name": "stable",
			          "range": ">0.1.0",
			          "versions": [
			            "0.1.0"
			          ]
			        }
			      ]
			    }
			  ]
			}`,
			assertion: require.Error,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			actualCfg, err := LoadDiffIncludeConfig(bytes.NewBufferString(s.input))
			s.assertion(t, err)
			if err == nil {
				require.Equal(t, s.expectedCfg, actualCfg)
				require.Equal(t, s.expectedIncluder, convertIncludeConfigToIncluder(actualCfg))
			}
		})
	}
}

var (
	//go:embed testdata/foo-bundle-v0.1.0/manifests/*
	//go:embed testdata/foo-bundle-v0.1.0/metadata/*
	fooBundlev010 embed.FS
	//go:embed testdata/foo-bundle-v0.2.0/manifests/*
	//go:embed testdata/foo-bundle-v0.2.0/metadata/*
	fooBundlev020 embed.FS
	//go:embed testdata/foo-bundle-v0.3.0/manifests/*
	//go:embed testdata/foo-bundle-v0.3.0/metadata/*
	fooBundlev030 embed.FS
	//go:embed testdata/foo-bundle-v0.3.1/manifests/*
	//go:embed testdata/foo-bundle-v0.3.1/metadata/*
	fooBundlev031 embed.FS
	//go:embed testdata/bar-bundle-v0.1.0/manifests/*
	//go:embed testdata/bar-bundle-v0.1.0/metadata/*
	barBundlev010 embed.FS
	//go:embed testdata/bar-bundle-v0.2.0/manifests/*
	//go:embed testdata/bar-bundle-v0.2.0/metadata/*
	barBundlev020 embed.FS
	//go:embed testdata/bar-bundle-v1.0.0/manifests/*
	//go:embed testdata/bar-bundle-v1.0.0/metadata/*
	barBundlev100 embed.FS
	//go:embed testdata/baz-bundle-v1.0.0/manifests/*
	//go:embed testdata/baz-bundle-v1.0.0/metadata/*
	bazBundlev100 embed.FS
	//go:embed testdata/baz-bundle-v1.0.1/manifests/*
	//go:embed testdata/baz-bundle-v1.0.1/metadata/*
	bazBundlev101 embed.FS
	//go:embed testdata/baz-bundle-v1.1.0/manifests/*
	//go:embed testdata/baz-bundle-v1.1.0/metadata/*
	bazBundlev110 embed.FS
)

var bundleToFS = map[string]embed.FS{
	"test.registry/foo-operator/foo-bundle:v0.1.0": fooBundlev010,
	"test.registry/foo-operator/foo-bundle:v0.2.0": fooBundlev020,
	"test.registry/foo-operator/foo-bundle:v0.3.0": fooBundlev030,
	"test.registry/foo-operator/foo-bundle:v0.3.1": fooBundlev031,
	"test.registry/bar-operator/bar-bundle:v0.1.0": barBundlev010,
	"test.registry/bar-operator/bar-bundle:v0.2.0": barBundlev020,
	"test.registry/bar-operator/bar-bundle:v1.0.0": barBundlev100,
	"test.registry/baz-operator/baz-bundle:v1.0.0": bazBundlev100,
	"test.registry/baz-operator/baz-bundle:v1.0.1": bazBundlev101,
	"test.registry/baz-operator/baz-bundle:v1.1.0": bazBundlev110,
}

//go:embed testdata/index-declcfgs
var indicesDir embed.FS

func newDiffRegistry() (image.Registry, error) {
	subDeclcfgImage, err := fs.Sub(indicesDir, "testdata/index-declcfgs")
	if err != nil {
		return nil, err
	}
	reg := &image.MockRegistry{
		RemoteImages: map[image.Reference]*image.MockImage{
			image.SimpleReference("test.registry/catalog/index-declcfg:latest"): {
				Labels: map[string]string{containertools.ConfigsLocationLabel: "/latest/index.yaml"},
				FS:     subDeclcfgImage,
			},
			image.SimpleReference("test.registry/catalog/index-declcfg:old"): {
				Labels: map[string]string{containertools.ConfigsLocationLabel: "/old/index.yaml"},
				FS:     subDeclcfgImage,
			},
		},
	}

	for name, bfs := range bundleToFS {
		base := filepath.Base(name)
		pkg := base[:strings.Index(base, ":")]
		base = strings.ReplaceAll(base, ":", "-")
		subImage, err := fs.Sub(bfs, path.Join("testdata", base))
		if err != nil {
			return nil, err
		}
		reg.RemoteImages[image.SimpleReference(name)] = &image.MockImage{
			Labels: map[string]string{bundle.PackageLabel: pkg},
			FS:     subImage,
		}
	}

	return reg, nil
}

func loadDirFS(t *testing.T, parent fs.FS, dir string) *declcfg.DeclarativeConfig {
	sub, err := fs.Sub(parent, dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := declcfg.LoadFS(sub)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
