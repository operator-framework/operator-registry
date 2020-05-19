package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

func TestAddPackageChannels(t *testing.T) {
	type fields struct {
		bundles []*registry.Bundle
	}
	type args struct {
		pkgs []registry.PackageManifest
	}
	type expected struct {
		errs []error
		pkgs []string
	}
	tests := []struct {
		description string
		fields      fields
		args        args
		expected    expected
	}{
		{
			description: "DuplicateBundlesInPackage/DBDoesntLock",
			fields: fields{
				bundles: []*registry.Bundle{
					newBundle(t, "csv-a", "pkg-0", []string{"stable"}, newUnstructuredCSV(t, "csv-a", "")),
					newBundle(t, "csv-a", "pkg-0", []string{"stable"}, newUnstructuredCSV(t, "csv-a", "")),
					newBundle(t, "csv-b", "pkg-0", []string{"alpha"}, newUnstructuredCSV(t, "csv-b", "")),
					newBundle(t, "csv-c", "pkg-1", []string{"stable"}, newUnstructuredCSV(t, "csv-c", "")),
				},
			},
			args: args{
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-a",
							},
							{
								Name:           "alpha",
								CurrentCSVName: "csv-b",
							},
						},
						DefaultChannelName: "stable",
					},
					{
						PackageName: "pkg-1",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-c",
							},
						},
					},
				},
			},
			expected: expected{
				errs: make([]error, 2),
				pkgs: []string{
					"pkg-0",
					"pkg-1",
				},
			},
		},
		{
			description: "MissingReplacesInPackage/AggregatesAndContinues",
			fields: fields{
				bundles: []*registry.Bundle{
					newBundle(t, "csv-a", "pkg-0", []string{"stable"}, newUnstructuredCSV(t, "csv-a", "non-existant")),
					newBundle(t, "csv-b", "pkg-0", []string{"alpha"}, newUnstructuredCSV(t, "csv-b", "")),
					newBundle(t, "csv-c", "pkg-1", []string{"stable"}, newUnstructuredCSV(t, "csv-c", "")),
				},
			},
			args: args{
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-a",
							},
							{
								Name:           "alpha",
								CurrentCSVName: "csv-b",
							},
						},
						DefaultChannelName: "stable",
					},
					{
						PackageName: "pkg-1",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-c",
							},
						},
					},
				},
			},
			expected: expected{
				errs: []error{
					utilerrors.NewAggregate([]error{fmt.Errorf("csv-a specifies replacement that couldn't be found")}),
					nil,
				},
				pkgs: []string{
					"pkg-1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			db, cleanup := CreateTestDb(t)
			defer cleanup()
			store, err := NewSQLLiteLoader(db)
			require.NoError(t, err)
			err = store.Migrate(context.TODO())
			require.NoError(t, err)

			for _, bundle := range tt.fields.bundles {
				// Throw away any errors loading bundles (not testing this)
				store.AddOperatorBundle(bundle)
			}

			for i, pkg := range tt.args.pkgs {
				errs := store.AddPackageChannels(pkg)
				require.Equal(t, tt.expected.errs[i], errs, "expected %v, got %v", tt.expected.errs[i], errs)
			}

			// Ensure expected packages were loaded
			querier := NewSQLLiteQuerierFromDb(db)
			pkgs, err := querier.ListPackages(context.Background())
			require.NoError(t, err)
			t.Logf("%#v", tt.expected.pkgs)
			t.Logf("%#v", pkgs)
			require.ElementsMatch(t, tt.expected.pkgs, pkgs)
		})
	}
}

func TestAddOperatorBundleIgnoresEmptyImageReferences(t *testing.T) {
	require := require.New(t)

	db, cleanup := CreateTestDb(t)
	defer cleanup()

	store, err := NewSQLLiteLoader(db)
	require.NoError(err)

	err = store.Migrate(context.TODO())
	require.NoError(err)

	b := newBundle(t, "test-bundle", "test-package", nil, newUnstructuredCSV(t, "test-bundle", ""))
	b.BundleImage = ""

	err = store.AddOperatorBundle(b)
	require.NoError(err)

	querier := NewSQLLiteQuerierFromDb(db)
	images, err := querier.GetImagesForBundle(context.TODO(), "test-bundle")
	require.NoError(err)
	require.Empty(images)
}

func TestClearNonHeadBundles(t *testing.T) {
	db, cleanup := CreateTestDb(t)
	defer cleanup()
	store, err := NewSQLLiteLoader(db)
	require.NoError(t, err)
	err = store.Migrate(context.TODO())
	require.NoError(t, err)

	// Create a replaces chain that contains bundles with no bundle path
	pkg, channel := "pkg", "stable"
	channels := []string{"stable"}
	withoutPath := newBundle(t, "without-path", pkg, channels, newUnstructuredCSV(t, "without-path", ""))
	withPathInternal := newBundle(t, "with-path-internal", pkg, channels, newUnstructuredCSV(t, "with-path-internal", withoutPath.Name))
	withPathInternal.BundleImage = "this.is/agood@sha256:path"
	withPath := newBundle(t, "with-path", pkg, channels, newUnstructuredCSV(t, "with-path", withPathInternal.Name))
	withPath.BundleImage = "this.is/abetter@sha256:path"

	require.NoError(t, store.AddOperatorBundle(withoutPath))
	require.NoError(t, store.AddOperatorBundle(withPathInternal))
	require.NoError(t, store.AddOperatorBundle(withPath))
	err = store.AddPackageChannels(registry.PackageManifest{
		PackageName: pkg,
		Channels: []registry.PackageChannel{
			{
				Name:           channel,
				CurrentCSVName: withPath.Name,
			},
		},
		DefaultChannelName: channel,
	})
	require.NoError(t, err)

	// Clear everything but the default bundle
	require.NoError(t, store.ClearNonHeadBundles())

	// Internal node without bundle path should keep its manifests
	querier := NewSQLLiteQuerierFromDb(db)
	bundle, err := querier.GetBundle(context.Background(), pkg, channel, withoutPath.Name)
	require.NoError(t, err)
	require.NotNil(t, bundle)
	require.NotNil(t, bundle.Object)
	require.NotEmpty(t, bundle.CsvJson)

	// Internal node with bundle path should be cleared
	bundle, err = querier.GetBundle(context.Background(), pkg, channel, withPathInternal.Name)
	require.NoError(t, err)
	require.NotNil(t, bundle)
	require.Nil(t, bundle.Object)
	require.Empty(t, bundle.CsvJson)

	// Head of the default channel should keep its manifests
	bundle, err = querier.GetBundle(context.Background(), pkg, channel, withPath.Name)
	require.NoError(t, err)
	require.NotNil(t, bundle)
	require.NotNil(t, bundle.Object)
	require.NotEmpty(t, bundle.CsvJson)
}

func newUnstructuredCSV(t *testing.T, name, replaces string) *unstructured.Unstructured {
	csv := &registry.ClusterServiceVersion{}
	csv.TypeMeta.Kind = "ClusterServiceVersion"
	csv.SetName(name)
	csv.Spec = json.RawMessage(fmt.Sprintf(`{"replaces": "%s"}`, replaces))

	out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(csv)
	require.NoError(t, err)
	return &unstructured.Unstructured{Object: out}
}

func newBundle(t *testing.T, name, pkgName string, channels []string, objs ...*unstructured.Unstructured) *registry.Bundle {
	bundle := registry.NewBundle(name, pkgName, channels, objs...)

	// Bust the bundle cache to set the CSV and CRDs
	_, err := bundle.ClusterServiceVersion()
	require.NoError(t, err)

	return bundle
}
