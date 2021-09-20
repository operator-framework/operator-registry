package registry

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/operator-framework/operator-registry/alpha/model"
	"github.com/operator-framework/operator-registry/alpha/property"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
	"github.com/operator-framework/operator-registry/pkg/sqlite/sqlitefakes"
)

func fakeBundlePathFromName(name string) string {
	return fmt.Sprintf("%s-path", name)
}

func newQuerier(t *testing.T, bundles []*model.Bundle) *registry.Querier {
	t.Helper()
	pkgs := map[string]*model.Package{}
	channels := map[string]map[string]*model.Channel{}

	for _, b := range bundles {
		if len(b.Image) == 0 {
			b.Image = fakeBundlePathFromName(b.Name)
		}
		channelName := b.Channel.Name
		packageName := b.Package.Name
		if _, ok := pkgs[packageName]; !ok {
			pkgs[packageName] = &model.Package{
				Name: packageName,
			}
			channels[packageName] = map[string]*model.Channel{
				channelName: {
					Package: pkgs[packageName],
					Name:    channelName,
					Bundles: map[string]*model.Bundle{b.Name: b},
				},
			}
			pkgs[packageName].Channels = channels[packageName]
			pkgs[packageName].DefaultChannel = channels[packageName][channelName]
		}

		if _, ok := channels[packageName][channelName]; !ok {
			channels[packageName][channelName] = &model.Channel{
				Package: pkgs[packageName],
				Name:    channelName,
				Bundles: map[string]*model.Bundle{b.Name: b},
			}
			pkgs[packageName].Channels[channelName] = channels[packageName][channelName]
		}
		b.Package = pkgs[packageName]
		b.Channel = channels[packageName][channelName]
		var pkgPropertyFound bool
		for _, p := range b.Properties {
			if p.Type == property.TypePackage {
				pkgPropertyFound = true
				break
			}
		}
		if !pkgPropertyFound {
			pkgJson, _ := json.Marshal(property.Package{
				PackageName: b.Package.Name,
				Version:     b.Name,
			})
			b.Properties = append(b.Properties, property.Property{
				Type:  property.TypePackage,
				Value: pkgJson,
			})
		}
	}
	reg, err := registry.NewQuerier(pkgs)
	require.NoError(t, err)
	return reg
}

func TestCheckForBundlePaths(t *testing.T) {
	type testResult struct {
		err     error
		found   []string
		missing []string
	}

	tests := []struct {
		description string
		querier     registry.GRPCQuery
		checkPaths  []string
		expected    testResult
	}{
		{
			description: "BundleListPresent",
			querier: newQuerier(t, []*model.Bundle{
				{
					Package: &model.Package{Name: "pkg-0"},
					Channel: &model.Channel{Name: "stable"},
					Name:    "csv-a",
				},
				{
					Package: &model.Package{Name: "pkg-0"},
					Channel: &model.Channel{Name: "alpha"},
					Name:    "csv-b",
				},
			}),
			checkPaths: []string{
				fakeBundlePathFromName("csv-a"),
			},
			expected: testResult{
				err:     nil,
				found:   []string{fakeBundlePathFromName("csv-a")},
				missing: nil,
			},
		},
		{
			description: "BundleListPartiallyMissing",
			querier: newQuerier(t, []*model.Bundle{
				{
					Package: &model.Package{Name: "pkg-0"},
					Channel: &model.Channel{Name: "stable"},
					Name:    "csv-a",
				},
				{
					Package: &model.Package{Name: "pkg-0"},
					Channel: &model.Channel{Name: "alpha"},
					Name:    "csv-b",
				},
			}),
			checkPaths: []string{
				fakeBundlePathFromName("csv-a"),
				fakeBundlePathFromName("missing"),
			},
			expected: testResult{
				err:     fmt.Errorf("target bundlepaths for deprecation missing from registry: %v", []string{fakeBundlePathFromName("missing")}),
				found:   []string{fakeBundlePathFromName("csv-a")},
				missing: []string{fakeBundlePathFromName("missing")},
			},
		},
		{
			description: "EmptyRegistry",
			querier:     newQuerier(t, nil),
			checkPaths: []string{
				fakeBundlePathFromName("missing"),
			},
			expected: testResult{
				err:     nil,
				missing: []string{fakeBundlePathFromName("missing")},
			},
		},
		{
			description: "EmptyDeprecateList",
			querier: newQuerier(t, []*model.Bundle{
				{
					Package: &model.Package{Name: "pkg-0"},
					Channel: &model.Channel{Name: "stable"},
					Name:    "csv-a",
				},
			}),
			checkPaths: []string{},
			expected: testResult{
				err:     nil,
				found:   []string{},
				missing: nil,
			},
		},
		{
			description: "InvalidQuerier",
			querier:     registry.NewEmptyQuerier(),
			checkPaths:  []string{fakeBundlePathFromName("missing")},
			expected: testResult{
				err:     errors.New("empty querier: cannot list bundles"),
				found:   []string{},
				missing: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			found, missing, err := checkForBundlePaths(tt.querier, tt.checkPaths)
			if qc, ok := tt.querier.(*registry.Querier); ok {
				defer qc.Close()
			}
			if tt.expected.err != nil {
				require.EqualError(t, err, tt.expected.err.Error())
				return
			}
			require.NoError(t, err)

			require.EqualValues(t, tt.expected.found, found)
			require.EqualValues(t, tt.expected.missing, missing)
		})
	}
}

func TestUnpackImage(t *testing.T) {
	type testResult struct {
		dstImage string
		err      error
	}
	tests := []struct {
		description    string
		registryImages []string
		srcImage       image.Reference
		expected       testResult
	}{
		{
			description:    "unpackFS",
			registryImages: []string{"image"},
			srcImage:       image.SimpleReference("image"),
			expected: testResult{
				dstImage: "image",
				err:      nil,
			},
		},
		{
			description:    "missingImage",
			registryImages: []string{},
			srcImage:       image.SimpleReference("missing"),
			expected: testResult{
				dstImage: "",
				err:      errors.New("not found"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			images := map[image.Reference]*image.MockImage{}
			for _, i := range tt.registryImages {
				images[image.SimpleReference(i)] = &image.MockImage{
					FS: fstest.MapFS{},
				}
			}
			ref, _, cleanup, err := unpackImage(context.TODO(), &image.MockRegistry{RemoteImages: images}, tt.srcImage)
			if cleanup != nil {
				cleanup()
			}

			if tt.expected.err != nil {
				require.EqualError(t, err, tt.expected.err.Error())
				return
			}
			require.NoError(t, err)
			require.EqualValues(t, ref, tt.expected.dstImage)
		})
	}
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func CreateTestDb(t *testing.T) (*sql.DB, func()) {
	dbName := fmt.Sprintf("test-%d.db", rand.Int())

	db, err := sqlite.Open(dbName)
	require.NoError(t, err)

	return db, func() {
		defer func() {
			if err := os.Remove(dbName); err != nil {
				t.Fatal(err)
			}
		}()
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func newUnpackedTestBundle(dir, name string, csvSpec json.RawMessage, annotations registry.Annotations) (string, func(), error) {
	bundleDir := filepath.Join(dir, fmt.Sprintf("%s-%s", annotations.PackageName, name))
	cleanup := func() {
		os.RemoveAll(bundleDir)
	}
	if err := os.Mkdir(bundleDir, 0755); err != nil {
		return bundleDir, cleanup, err
	}
	if err := os.Mkdir(filepath.Join(bundleDir, bundle.ManifestsDir), 0755); err != nil {
		return bundleDir, cleanup, err
	}
	if err := os.Mkdir(filepath.Join(bundleDir, bundle.MetadataDir), 0755); err != nil {
		return bundleDir, cleanup, err
	}
	if len(csvSpec) == 0 {
		csvSpec = json.RawMessage(`{}`)
	}

	rawCSV, err := json.Marshal(registry.ClusterServiceVersion{
		TypeMeta: v1.TypeMeta{
			Kind: sqlite.ClusterServiceVersionKind,
		},
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
		Spec: csvSpec,
	})
	if err != nil {
		return bundleDir, cleanup, err
	}

	rawObj := unstructured.Unstructured{}
	if err := json.Unmarshal(rawCSV, &rawObj); err != nil {
		return bundleDir, cleanup, err
	}
	rawObj.SetCreationTimestamp(v1.Time{})

	jsonout, err := rawObj.MarshalJSON()
	out, err := yaml.JSONToYAML(jsonout)
	if err != nil {
		return bundleDir, cleanup, err
	}
	if err := ioutil.WriteFile(filepath.Join(bundleDir, bundle.ManifestsDir, "csv.yaml"), out, 0666); err != nil {
		return bundleDir, cleanup, err
	}

	out, err = yaml.Marshal(registry.AnnotationsFile{Annotations: annotations})
	if err != nil {
		return bundleDir, cleanup, err
	}
	if err := ioutil.WriteFile(filepath.Join(bundleDir, bundle.MetadataDir, "annotations.yaml"), out, 0666); err != nil {
		return bundleDir, cleanup, err
	}
	return bundleDir, cleanup, nil
}

type bundleDir struct {
	csvSpec     json.RawMessage
	annotations registry.Annotations
}

func TestPackagesFromUnpackedRefs(t *testing.T) {
	tests := []struct {
		description string
		bundles     map[string]bundleDir
		expected    map[string]registry.Package
		wantErr     bool
	}{
		{
			description: "InvalidBundle/Empty",
			bundles: map[string]bundleDir{
				"bundle-empty": {},
			},
			wantErr: true,
		},
		{
			description: "LoadPartialGraph",
			bundles: map[string]bundleDir{
				"testoperator-1": {
					csvSpec: json.RawMessage(`{"version":"1.1.0","replaces":"1.0.0"}`),
					annotations: registry.Annotations{
						PackageName:        "testpkg-1",
						Channels:           "alpha",
						DefaultChannelName: "stable",
					},
				},
				"testoperator-2": {
					csvSpec: json.RawMessage(`{"version":"2.1.0"}`),
					annotations: registry.Annotations{
						PackageName:        "testpkg-2",
						Channels:           "stable,alpha",
						DefaultChannelName: "stable",
					},
				},
			},
			expected: map[string]registry.Package{
				"testpkg-1": {
					Name: "testpkg-1",
					Channels: map[string]registry.Channel{
						"alpha": {
							Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
								registry.BundleKey{
									BundlePath: fakeBundlePathFromName("testoperator-1"),
									Version:    "1.1.0",
									CsvName:    "testoperator-1",
								}: nil,
							},
						},
					},
				},
				"testpkg-2": {
					Name: "testpkg-2",
					Channels: map[string]registry.Channel{
						"alpha": {
							Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
								registry.BundleKey{
									BundlePath: fakeBundlePathFromName("testoperator-2"),
									Version:    "2.1.0",
									CsvName:    "testoperator-2",
								}: nil,
							},
						},
						"stable": {
							Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
								registry.BundleKey{
									BundlePath: fakeBundlePathFromName("testoperator-2"),
									Version:    "2.1.0",
									CsvName:    "testoperator-2",
								}: nil,
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			tmpdir, err := os.MkdirTemp(".", "tmpdir-*")
			defer os.RemoveAll(tmpdir)
			require.NoError(t, err)
			refs := map[image.Reference]string{}
			for name, b := range tt.bundles {
				dir, _, err := newUnpackedTestBundle(tmpdir, name, b.csvSpec, b.annotations)
				require.NoError(t, err)
				refs[image.SimpleReference(fakeBundlePathFromName(name))] = dir
			}
			pkg, err := packagesFromUnpackedRefs(refs)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.EqualValues(t, tt.expected, pkg)
		})
	}
}

func TestCheckForBundles(t *testing.T) {
	type step struct {
		bundles map[string]bundleDir
		action  int
	}
	const (
		actionAdd = iota
		actionDeprecate
		actionOverwrite
	)
	tests := []struct {
		description string
		steps       []step
		wantErr     error
		init        func() (*sql.DB, func())
	}{
		{
			// 1.1.0 -> 1.0.0         pruned    channel 1
			//        		\-> 1.2.0 ok        channel 2
			description: "partialPruning",
			steps: []step{
				{
					bundles: map[string]bundleDir{
						"unorderedReplaces-1.1.0": {
							csvSpec: json.RawMessage(`{"version":"1.1.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "stable,alpha",
								DefaultChannelName: "stable",
							},
						},
						"unorderedReplaces-1.0.0": {
							csvSpec: json.RawMessage(`{"version":"1.0.0","replaces":"unorderedReplaces-1.1.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "stable,alpha",
								DefaultChannelName: "stable",
							},
						},
						"unorderedReplaces-1.2.0": {
							csvSpec: json.RawMessage(`{"version":"1.2.0","replaces":"unorderedReplaces-1.0.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "alpha",
								DefaultChannelName: "stable",
							},
						},
					},
					action: actionAdd,
				},
			},
			wantErr: fmt.Errorf("added bundle unorderedReplaces-1.0.0 pruned from package testpkg, channel stable: this may be due to incorrect channel head (unorderedReplaces-1.1.0)"),
		},
		{
			description: "ignoreDeprecated",
			steps: []step{
				{
					bundles: map[string]bundleDir{
						"ignoreDeprecated-1.0.0": {
							csvSpec: json.RawMessage(`{"version":"1.0.0"}`),
							annotations: registry.Annotations{
								PackageName: "testpkg",
								Channels:    "stable",
							},
						},
						"ignoreDeprecated-1.1.0": {
							csvSpec: json.RawMessage(`{"version":"1.1.0","replaces":"ignoreDeprecated-1.0.0"}`),
							annotations: registry.Annotations{
								PackageName: "testpkg",
								Channels:    "stable",
							},
						},
						"ignoreDeprecated-1.2.0": {
							csvSpec: json.RawMessage(`{"version":"1.2.0","replaces":"ignoreDeprecated-1.1.0"}`),
							annotations: registry.Annotations{
								PackageName: "testpkg",
								Channels:    "stable",
							},
						},
					},
					action: actionAdd,
				},
				{
					bundles: map[string]bundleDir{
						"ignoreDeprecated-1.1.0": {},
					},
					action: actionDeprecate,
				},
				{
					bundles: map[string]bundleDir{
						"ignoreDeprecated-1.0.0": {
							csvSpec: json.RawMessage(`{"version":"1.0.0"}`),
							annotations: registry.Annotations{
								PackageName: "testpkg",
								Channels:    "stable",
							},
						},
						"ignoreDeprecated-1.1.0": {
							csvSpec: json.RawMessage(`{"version":"1.1.0","replaces":"ignoreDeprecated-1.0.0"}`),
							annotations: registry.Annotations{
								PackageName: "testpkg",
								Channels:    "stable",
							},
						},
					},
					action: actionOverwrite,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			tmpdir, err := os.MkdirTemp(".", "tmpdir-*")
			defer os.RemoveAll(tmpdir)
			db, cleanup := CreateTestDb(t)
			defer cleanup()
			load, err := sqlite.NewSQLLiteLoader(db)
			require.NoError(t, err)
			require.NoError(t, load.Migrate(context.TODO()))
			query := sqlite.NewSQLLiteQuerierFromDb(db)
			graphLoader, err := sqlite.NewSQLGraphLoaderFromDB(db)
			require.NoError(t, err)

			for _, step := range tt.steps {
				switch step.action {
				case actionDeprecate:
					for deprecate := range step.bundles {
						require.NoError(t, load.DeprecateBundle(deprecate))
					}
				case actionAdd:
					refs := map[image.Reference]string{}
					for name, b := range step.bundles {
						dir, _, err := newUnpackedTestBundle(tmpdir, name, b.csvSpec, b.annotations)
						require.NoError(t, err)
						refs[image.SimpleReference(name)] = dir
					}
					require.NoError(t, registry.NewDirectoryPopulator(
						load,
						graphLoader,
						query,
						refs,
						nil,
						false).Populate(registry.ReplacesMode))

					err = checkForBundles(context.TODO(), query, graphLoader, refs)
					if tt.wantErr == nil {
						require.NoError(t, err)
						return
					}
					require.EqualError(t, err, tt.wantErr.Error())

				case actionOverwrite:
					overwriteRefs := map[string]map[image.Reference]string{}
					refs := map[image.Reference]string{}
					for name, b := range step.bundles {
						dir, _, err := newUnpackedTestBundle(tmpdir, name, b.csvSpec, b.annotations)
						require.NoError(t, err)
						to := image.SimpleReference(name)
						refs[image.SimpleReference(name)] = dir
						refs[to] = dir
						img, err := registry.NewImageInput(to, dir)
						require.NoError(t, err)
						if _, ok := overwriteRefs[img.Bundle.Package]; ok {
							overwriteRefs[img.Bundle.Package] = map[image.Reference]string{}
						}
						overwriteRefs[img.Bundle.Package][to] = dir
					}
					require.NoError(t, registry.NewDirectoryPopulator(
						load,
						graphLoader,
						query,
						nil,
						overwriteRefs,
						true).Populate(registry.ReplacesMode))

					err = checkForBundles(context.TODO(), query, graphLoader, refs)
					if tt.wantErr == nil {
						require.NoError(t, err)
						return
					}
					require.EqualError(t, err, tt.wantErr.Error())
				}
			}
		})
	}
}

func TestDeprecated(t *testing.T) {
	deprecated := map[string]bool{
		"deprecatedBundle": true,
		"otherBundle":      false,
	}
	q := &sqlitefakes.FakeQuerier{
		QueryContextStub: func(ctx context.Context, query string, args ...interface{}) (sqlite.RowScanner, error) {
			bundleName := args[2].(string)
			if len(bundleName) == 0 {
				return nil, fmt.Errorf("empty bundle name")
			}
			hasNext := true
			return &sqlitefakes.FakeRowScanner{ScanStub: func(args ...interface{}) error {
				if deprecated[bundleName] {
					*args[0].(*sql.NullString) = sql.NullString{
						String: registry.DeprecatedType,
						Valid:  true,
					}
					*args[1].(*sql.NullString) = sql.NullString{
						Valid: true,
					}
				}
				return nil
			},
				NextStub: func() bool {
					if hasNext {
						hasNext = false
						return true
					}
					return false
				},
			}, nil
		},
	}

	querier := sqlite.NewSQLLiteQuerierFromDBQuerier(q)

	_, err := isDeprecated(context.TODO(), querier, registry.BundleKey{})
	require.Error(t, err)

	for b := range deprecated {
		isDeprecated, err := isDeprecated(context.TODO(), querier, registry.BundleKey{BundlePath: b})
		require.NoError(t, err)
		require.Equal(t, deprecated[b], isDeprecated)
	}
}
