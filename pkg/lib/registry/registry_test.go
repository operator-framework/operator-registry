package registry

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/internal/model"
	"github.com/operator-framework/operator-registry/internal/property"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

func fakeBundlePathFromName(name string) string {
	return fmt.Sprintf("%s-path", name)
}

func newQuerier(bundles []*model.Bundle) *registry.Querier {
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
	return registry.NewQuerier(pkgs)
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
			querier: newQuerier([]*model.Bundle{
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
			querier: newQuerier([]*model.Bundle{
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
			querier:     newQuerier(nil),
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
			querier: newQuerier([]*model.Bundle{
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

func TestCheckForBundles(t *testing.T) {
	type step struct {
		bundles []string
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
					bundles: []string{
						"unorderedReplaces/1.0.0",
						"unorderedReplaces/1.1.0",
						"unorderedReplaces/1.2.0",
					},
					action: actionAdd,
				},
			},
			wantErr: fmt.Errorf("added bundle unorderedReplaces/1.0.0 pruned from package testpkg, channel stable: this may be due to incorrect channel head (unorderedReplaces.1.1.0)"),
		},
		{
			description: "ignoreDeprecated",
			steps: []step{
				{
					bundles: []string{
						"ignoreDeprecated/1.0.0",
						"ignoreDeprecated/1.1.0",
						"ignoreDeprecated/1.2.0",
					},
					action: actionAdd,
				},
				{
					bundles: []string{
						"ignoreDeprecated/1.1.0",
					},
					action: actionDeprecate,
				},
				{
					bundles: []string{
						"ignoreDeprecated/1.0.0",
						"ignoreDeprecated/1.1.0",
					},
					action: actionOverwrite,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
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
					for _, deprecate := range step.bundles {
						require.NoError(t, load.DeprecateBundle(deprecate))
					}
				case actionAdd:
					refs := map[image.Reference]string{}
					for _, b := range step.bundles {
						refs[image.SimpleReference(b)] = fmt.Sprintf("testdata/%s", b)
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
					for _, b := range step.bundles {
						to := image.SimpleReference(b)
						from := fmt.Sprintf("testdata/%s", b)
						refs[to] = from
						img, err := registry.NewImageInput(to, from)
						require.NoError(t, err)
						if _, ok := overwriteRefs[img.Bundle.Package]; ok {
							overwriteRefs[img.Bundle.Package] = map[image.Reference]string{}
						}
						overwriteRefs[img.Bundle.Package][to] = from
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
