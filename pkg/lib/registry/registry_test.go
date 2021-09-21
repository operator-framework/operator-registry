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
	"github.com/operator-framework/operator-registry/pkg/registry/registryfakes"
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

func newUnpackedTestBundle(dir, name string, csvSpec json.RawMessage, annotations registry.Annotations, overwrite bool) (string, func(), error) {
	bundleDir := filepath.Join(dir, fmt.Sprintf("%s-%s", annotations.PackageName, name))
	cleanup := func() {
		os.RemoveAll(bundleDir)
	}

	if overwrite {
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
	version     string
	csvSpec     json.RawMessage
	annotations registry.Annotations
}

func TestCheckForBundles(t *testing.T) {
	type step struct {
		bundles map[string]bundleDir
		action  int
		expected map[string]*registry.Package
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
							version: "1.1.0",
						},
						"unorderedReplaces-1.0.0": {
							csvSpec: json.RawMessage(`{"version":"1.0.0","replaces":"unorderedReplaces-1.1.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "stable,alpha",
								DefaultChannelName: "stable",
							},
							version: "1.0.0",
						},
						"unorderedReplaces-1.2.0": {
							csvSpec: json.RawMessage(`{"version":"1.2.0","replaces":"unorderedReplaces-1.0.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "alpha",
								DefaultChannelName: "stable",
							},
							version: "1.2.0",
						},
					},
					action: actionAdd,
					expected: map[string]*registry.Package{
						"testpkg": {
							Name:           "testpkg",
							Channels:       map[string]registry.Channel{
								"alpha": {
									Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
										registry.BundleKey{
											BundlePath: "unorderedReplaces-1.0.0",
											Version:    "1.0.0",
											CsvName:    "unorderedReplaces-1.0.0",
										}: nil,
										registry.BundleKey{
											BundlePath: "unorderedReplaces-1.1.0",
											Version:    "1.1.0",
											CsvName:    "unorderedReplaces-1.1.0",
										}: nil,
										registry.BundleKey{
											BundlePath: "unorderedReplaces-1.2.0",
											Version:    "1.2.0",
											CsvName:    "unorderedReplaces-1.2.0",
										}: nil,
									},
								},
								"stable": {
									Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
										registry.BundleKey{
											BundlePath: "unorderedReplaces-1.0.0",
											Version:    "1.0.0",
											CsvName:    "unorderedReplaces-1.0.0",
										}: nil,
										registry.BundleKey{
											BundlePath: "unorderedReplaces-1.1.0",
											Version:    "1.1.0",
											CsvName:    "unorderedReplaces-1.1.0",
										}: nil,
									},
								},
							},
						},
					},
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
							version: "1.0.0",
						},
						"ignoreDeprecated-1.1.0": {
							csvSpec: json.RawMessage(`{"version":"1.1.0","replaces":"ignoreDeprecated-1.0.0"}`),
							annotations: registry.Annotations{
								PackageName: "testpkg",
								Channels:    "stable",
							},
							version: "1.1.0",
						},
						"ignoreDeprecated-1.2.0": {
							csvSpec: json.RawMessage(`{"version":"1.2.0","replaces":"ignoreDeprecated-1.1.0"}`),
							annotations: registry.Annotations{
								PackageName: "testpkg",
								Channels:    "stable",
							},
							version: "1.2.0",
						},
					},
					action: actionAdd,
					expected: map[string]*registry.Package{
						"testpkg": {
							Name:           "testpkg",
							Channels:       map[string]registry.Channel{
								"stable": {
									Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
										registry.BundleKey{
											BundlePath: "ignoreDeprecated-1.0.0",
											Version:    "1.0.0",
											CsvName:    "ignoreDeprecated-1.0.0",
										}: nil,
										registry.BundleKey{
											BundlePath: "ignoreDeprecated-1.1.0",
											Version:    "1.1.0",
											CsvName:    "ignoreDeprecated-1.1.0",
										}: nil,
										registry.BundleKey{
											BundlePath: "ignoreDeprecated-1.2.0",
											Version:    "1.2.0",
											CsvName:    "ignoreDeprecated-1.2.0",
										}: nil,
									},
								},
							},
						},
					},
				},
				{
					bundles: map[string]bundleDir{
						"ignoreDeprecated-1.1.0": {},
					},
					action: actionDeprecate,
					expected: map[string]*registry.Package{
						"testpkg": {
							Name:           "testpkg",
							Channels:       map[string]registry.Channel{
								"stable": {
									Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
										registry.BundleKey{
											BundlePath: "ignoreDeprecated-1.1.0",
											Version:    "1.1.0",
											CsvName:    "ignoreDeprecated-1.1.0",
										}: nil,
										registry.BundleKey{
											BundlePath: "ignoreDeprecated-1.2.0",
											Version:    "1.2.0",
											CsvName:    "ignoreDeprecated-1.2.0",
										}: nil,
									},
								},
							},
						},
					},
				},
				{
					bundles: map[string]bundleDir{
						"ignoreDeprecated-1.2.0": {
							csvSpec: json.RawMessage(`{"version":"1.2.0","replaces":""}`),
							annotations: registry.Annotations{
								PackageName: "testpkg",
								Channels:    "alpha",
								DefaultChannelName: "alpha",
							},
							version: "1.2.0",
						},
					},
					action: actionOverwrite,
					expected: map[string]*registry.Package{
						"testpkg": {
							Name:           "testpkg",
							Channels:       map[string]registry.Channel{
								"stable": {
									Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
										registry.BundleKey{
											BundlePath: "ignoreDeprecated-1.1.0",
											Version:    "1.1.0",
											CsvName:    "ignoreDeprecated-1.1.0",
										}: nil,
									},
								},
								"alpha": {
									Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
										registry.BundleKey{
											BundlePath: "ignoreDeprecated-1.2.0-overwrite",
											Version:    "1.2.0",
											CsvName:    "ignoreDeprecated-1.2.0",
										}: nil,
									},
								},
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
				case actionAdd, actionOverwrite:
					overwriteRefs := map[string][]string{}
					refs := map[image.Reference]string{}
					for name, b := range step.bundles {
						dir, _, err := newUnpackedTestBundle(tmpdir, name, b.csvSpec, b.annotations, true)
						require.NoError(t, err)

						// refs to be added
						bundleImage := name

						// bundles to remove for overwrite. Only one per package is permitted.
						if step.action == actionOverwrite {
							bundleImage += "-overwrite"
							img, err := registry.NewImageInput(image.SimpleReference(bundleImage), dir)
							require.NoError(t, err)
							overwriteRefs[img.Bundle.Package] = append(overwriteRefs[img.Bundle.Package], name)
						}
						refs[image.SimpleReference(bundleImage)] = dir

					}
					require.NoError(t, registry.NewDirectoryPopulator(
						load,
						graphLoader,
						query,
						refs,
						overwriteRefs,
						true).Populate(registry.ReplacesMode))

				}
				err = checkForBundles(context.TODO(), query, graphLoader, step.expected)
				if tt.wantErr == nil {
					require.NoError(t, err, fmt.Sprintf("%d", step.action))
					continue
				}
				require.EqualError(t, err, tt.wantErr.Error())
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

func TestExpectedGraphBundles(t *testing.T) {
	testBundle, err := registry.NewBundleFromStrings("testBundle", "0.0.1", "testPkg", "default", "default", "")
	require.NoError(t, err)
	testBundle.BundleImage = "testImage"
	testBundlePkg := &registry.Package{
		Name: "testPkg",
		Channels: map[string]registry.Channel{
			"default": {
				Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
					registry.BundleKey{
						BundlePath: "testImage",
						Version:    "0.0.1",
						CsvName:    "testBundle",
					}: nil,
				},
			},
		},
	}
	testBundleDifferentChannelPkg := &registry.Package{
		Name: "testPkg",
		Channels: map[string]registry.Channel{
			"alpha": {
				Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
					registry.BundleKey{
						BundlePath: "testImage",
						Version:    "0.0.1",
						CsvName:    "testBundle",
					}: nil,
				},
			},
		},
	}
	tests := []struct {
		description      string
		graphLoader      registry.GraphLoader
		bundles          []*registry.Bundle
		overwrite        bool
		wantErr          error
		wantGraphBundles map[string]*registry.Package
	}{
		{
			description: "GraphLoaderError",
			graphLoader: &registryfakes.FakeGraphLoader{GenerateStub: func(string) (*registry.Package, error) { return nil, fmt.Errorf("graphLoader error") }},
			bundles:     []*registry.Bundle{testBundle},
			wantErr:     fmt.Errorf("graphLoader error"),
		},
		{
			description: "NewPackage",
			graphLoader: &registryfakes.FakeGraphLoader{GenerateStub: func(string) (*registry.Package, error) { return nil, registry.ErrPackageNotInDatabase }},
			bundles:     []*registry.Bundle{testBundle},
			wantGraphBundles: map[string]*registry.Package{
				"testPkg": testBundlePkg,
			},
		},
		{
			description: "OverwriteWithoutFlag",
			graphLoader: &registryfakes.FakeGraphLoader{GenerateStub: func(string) (*registry.Package, error) { return testBundleDifferentChannelPkg, nil }},
			bundles:     []*registry.Bundle{testBundle},
			wantErr:     registry.BundleImageAlreadyAddedErr{ErrorString: fmt.Sprintf("Bundle %s already exists", testBundle.BundleImage)},
		},
		{
			description: "OverwriteWithFlag",
			graphLoader: &registryfakes.FakeGraphLoader{GenerateStub: func(string) (*registry.Package, error) { return testBundleDifferentChannelPkg, nil }},
			bundles:     []*registry.Bundle{testBundle},
			overwrite:   true,
			wantGraphBundles: map[string]*registry.Package{
				"testPkg": testBundlePkg,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			graphBundles, err := expectedGraphBundles(tt.bundles, tt.graphLoader, tt.overwrite)
			if tt.wantErr != nil {
				require.EqualError(t, err, tt.wantErr.Error())
				return
			}
			require.NoError(t, err)

			require.EqualValues(t, graphBundles, tt.wantGraphBundles)
		})
	}
}
