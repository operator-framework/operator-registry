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
	err = reg.Wait(context.Background())
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
	isOverwrite bool
}

func TestCheckForBundles(t *testing.T) {
	type step struct {
		bundles  map[string]bundleDir
		action   int
		expected []*registry.Bundle // For testing pruning after deprecation
		wantErr  error
	}
	const (
		actionAdd = iota
		actionDeprecate
		actionOverwrite
	)
	tests := []struct {
		description string
		steps       []step
		init        func() (*sql.DB, func())
	}{
		{
			// 1.1.0 -> 1.2.0           ok      channel 1
			//        		\-> 1.2.0-1 pruned  channel 2
			description: "ErrorOnNewPrunedBundle",
			steps: []step{
				{
					bundles: map[string]bundleDir{
						"newPruned-1.1.0": {
							csvSpec: json.RawMessage(`{"version":"1.1.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "stable,alpha",
								DefaultChannelName: "stable",
							},
							version: "1.1.0",
						},
					},
					action: actionAdd,
				},
				{
					bundles: map[string]bundleDir{
						"newPruned-1.2.0": {
							csvSpec: json.RawMessage(`{"version":"1.2.0","replaces":"newPruned-1.1.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "alpha",
								DefaultChannelName: "stable",
							},
							version: "1.2.0",
						},
						"newPruned-1.2.0-1": {
							csvSpec: json.RawMessage(`{"version":"1.2.0-1","replaces":"newPruned-1.2.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "stable,alpha",
								DefaultChannelName: "stable",
							},
							version: "1.2.0-1",
						},
					},
					action:  actionAdd,
					wantErr: fmt.Errorf("add prunes bundle newPruned-1.2.0-1 (newPruned-1.2.0-1) from package testpkg, channel alpha: this may be due to incorrect channel head (newPruned-1.2.0, skips/replaces [newPruned-1.1.0]). Be aware that the head of the channel alpha where you are trying to add the newPruned-1.2.0-1 is newPruned-1.2.0. Upgrade graphs follows the Semantic Versioning 2.0.0 (https://semver.org/) which means that is not possible add new versions lower then the head of the channel"),
				},
			},
		},
		{
			description: "silentPruneForExistingBundle",
			steps: []step{
				{
					bundles: map[string]bundleDir{
						"silentPrune-1.0.0": {
							csvSpec: json.RawMessage(`{"version":"1.0.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "stable,alpha",
								DefaultChannelName: "stable",
							},
							version: "1.0.0",
						},
						"silentPrune-1.1.0": {
							csvSpec: json.RawMessage(`{"version":"1.1.0","replaces":"silentPrune-1.0.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "stable,alpha",
								DefaultChannelName: "stable",
							},
							version: "1.1.0",
						},
					},
					action: actionAdd,
				},
				{
					bundles: map[string]bundleDir{
						"silentPrune-1.2.0": {
							csvSpec: json.RawMessage(`{"version":"1.2.0","replaces":"silentPrune-1.0.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "alpha",
								DefaultChannelName: "stable",
							},
							version: "1.2.0",
						},
					},
					action: actionAdd,
				},
			},
		},
		{
			// 1.0.0 <- 1.0.1 <- 1.0.1-1 <- 1.0.2 (head)
			// No pruning despite chain being out of order for 1.0.1 <- 1.0.1-1
			description: "allowUnorderedWithMaxChannelHead",
			steps: []step{
				{
					bundles: map[string]bundleDir{
						"unorderedReplaces-1.0.0": {
							csvSpec: json.RawMessage(`{"version":"1.0.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "stable,alpha",
								DefaultChannelName: "stable",
							},
							version: "1.0.0",
						},
						"unorderedReplaces-1.1.0": {
							csvSpec: json.RawMessage(`{"version":"1.1.0","replaces":"unorderedReplaces-1.0.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "stable,alpha",
								DefaultChannelName: "stable",
							},
							version: "1.1.0",
						},
					},
					action: actionAdd,
				},
				{
					bundles: map[string]bundleDir{
						"unorderedReplaces-1.1.0-1": {
							csvSpec: json.RawMessage(`{"version":"1.1.0-1","replaces":"unorderedReplaces-1.1.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "alpha",
								DefaultChannelName: "stable",
							},
							version: "1.1.0-1",
						},
						"unorderedReplaces-1.2.0": {
							csvSpec: json.RawMessage(`{"version":"1.2.0","replaces":"unorderedReplaces-1.1.0-1"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "alpha",
								DefaultChannelName: "stable",
							},
							version: "1.2.0",
						},
					},
					action: actionAdd,
				},
			},
		},
		{
			// If a pruned bundle was deprecated, ignore
			description: "withDeprecated",
			steps: []step{
				{
					bundles: map[string]bundleDir{
						"withDeprecated-1.0.0": {
							csvSpec: json.RawMessage(`{"version":"1.0.0"}`),
							annotations: registry.Annotations{
								PackageName: "testpkg",
								Channels:    "stable",
							},
							version: "1.0.0",
						},
						"withDeprecated-1.1.0": {
							csvSpec: json.RawMessage(`{"version":"1.1.0","replaces":"withDeprecated-1.0.0"}`),
							annotations: registry.Annotations{
								PackageName: "testpkg",
								Channels:    "stable",
							},
							version: "1.1.0",
						},
						"withDeprecated-1.2.0": {
							csvSpec: json.RawMessage(`{"version":"1.2.0","replaces":"withDeprecated-1.1.0"}`),
							annotations: registry.Annotations{
								PackageName: "testpkg",
								Channels:    "stable",
							},
							version: "1.2.0",
						},
					},
					action: actionAdd,
				},
				{
					bundles: map[string]bundleDir{
						"withDeprecated-1.1.0": {},
					},
					action: actionDeprecate,
					expected: []*registry.Bundle{
						{
							Name:        "withDeprecated-1.1.0",
							Package:     "testpkg",
							Channels:    []string{"stable"},
							BundleImage: "withDeprecated-1.1.0",
						},
						{
							Name:        "withDeprecated-1.2.0",
							Package:     "testpkg",
							Channels:    []string{"stable"},
							BundleImage: "withDeprecated-1.2.0",
						},
					},
				},
				{
					bundles: map[string]bundleDir{
						"withDeprecated-1.2.0": {
							csvSpec: json.RawMessage(`{"version":"1.2.0","replaces":""}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "alpha",
								DefaultChannelName: "alpha",
							},
							version:     "1.2.0",
							isOverwrite: true,
						},
					},
					action: actionOverwrite,
				},
			},
		},
		{
			// bundle version should be immutable anyway, but only csv name is required to stay unchanged in overwrite
			description: "overwritePruning",
			steps: []step{
				{
					bundles: map[string]bundleDir{
						"withOverwrite-1.0.0": {
							csvSpec: json.RawMessage(`{"version":"1.0.0"}`),
							annotations: registry.Annotations{
								PackageName: "testpkg",
								Channels:    "stable",
							},
							version: "1.0.0",
						},
						"withOverwrite-1.1.0": {
							csvSpec: json.RawMessage(`{"version":"1.1.0","replaces":"withOverwrite-1.0.0"}`),
							annotations: registry.Annotations{
								PackageName: "testpkg",
								Channels:    "stable",
							},
							version: "1.1.0",
						},
					},
					action: actionAdd,
				},
				{
					bundles: map[string]bundleDir{
						"withOverwrite-1.1.0": {
							csvSpec: json.RawMessage(`{"version":"1.0.0-1","replaces":"withOverwrite-1.0.0"}`),
							annotations: registry.Annotations{
								PackageName: "testpkg",
								Channels:    "stable",
							},
							version:     "1.0.0-1",
							isOverwrite: true,
						},
						"withOverwrite-1.2.0": {
							csvSpec: json.RawMessage(`{"version":"1.2.0","replaces":"withOverwrite-1.1.0"}`),
							annotations: registry.Annotations{
								PackageName:        "testpkg",
								Channels:           "alpha",
								DefaultChannelName: "stable",
							},
							version: "1.2.0",
						},
					},
					action:  actionOverwrite,
					wantErr: fmt.Errorf("add prunes bundle withOverwrite-1.1.0 (withOverwrite-1.1.0-overwrite) from package testpkg, channel stable: this may be due to incorrect channel head (withOverwrite-1.0.0, skips/replaces []). Be aware that the head of the channel stable where you are trying to add the withOverwrite-1.1.0 is withOverwrite-1.0.0. Upgrade graphs follows the Semantic Versioning 2.0.0 (https://semver.org/) which means that is not possible add new versions lower then the head of the channel"),
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
				expected := []*registry.Bundle{}
				switch step.action {
				case actionDeprecate:
					for deprecate := range step.bundles {
						require.NoError(t, load.DeprecateBundle(deprecate))
					}
					expected = step.expected
				case actionAdd, actionOverwrite:
					overwriteRefs := map[string][]string{}
					refs := map[image.Reference]string{}
					for name, b := range step.bundles {
						dir, _, err := newUnpackedTestBundle(tmpdir, name, b.csvSpec, b.annotations, true)
						require.NoError(t, err)

						// refs to be added
						bundleImage := name

						// bundles to remove for overwrite. Only one per package is permitted.
						if step.action == actionOverwrite && b.isOverwrite {
							bundleImage += "-overwrite"
						}

						img, err := registry.NewImageInput(image.SimpleReference(bundleImage), dir)
						require.NoError(t, err)
						expected = append(expected, img.Bundle)

						if step.action == actionOverwrite && b.isOverwrite {
							overwriteRefs[img.Bundle.Package] = append(overwriteRefs[img.Bundle.Package], name)
						}
						refs[image.SimpleReference(bundleImage)] = dir

					}
					require.NoError(t, registry.NewDirectoryPopulator(
						load,
						graphLoader,
						query,
						refs,
						overwriteRefs).Populate(registry.ReplacesMode))
				}
				err = checkForBundles(context.TODO(), query, graphLoader, expected)
				if step.wantErr == nil {
					require.NoError(t, err, fmt.Sprintf("%d", step.action))
					continue
				}
				require.EqualError(t, err, step.wantErr.Error())
			}
		})
	}
}
