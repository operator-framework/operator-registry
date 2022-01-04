package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
					newBundle(t, "csv-a", "pkg-0", []string{"stable"}, newUnstructuredCSV(t, "csv-a", "csv-d")),
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
					utilerrors.NewAggregate([]error{fmt.Errorf("Invalid bundle csv-a, replaces nonexistent bundle csv-d")}),
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
			require.ElementsMatch(t, tt.expected.pkgs, pkgs)
		})
	}
}

func TestAddBundleSemver(t *testing.T) {
	// Create a test DB
	db, cleanup := CreateTestDb(t)
	defer cleanup()
	store, err := NewSQLLiteLoader(db)
	require.NoError(t, err)
	err = store.Migrate(context.TODO())
	require.NoError(t, err)
	graphLoader, err := NewSQLGraphLoaderFromDB(db)
	require.NoError(t, err)

	// Seed the db with a replaces-mode bundle/package
	replacesBundle := newBundle(t, "csv-a", "pkg-foo", []string{"stable"}, newUnstructuredCSV(t, "csv-a", ""))
	err = store.AddOperatorBundle(replacesBundle)
	require.NoError(t, err)

	err = store.AddPackageChannels(registry.PackageManifest{
		PackageName: "pkg-foo",
		Channels: []registry.PackageChannel{
			{
				Name:           "stable",
				CurrentCSVName: "csv-a",
			},
		},
		DefaultChannelName: "stable",
	})
	require.NoError(t, err)

	// Add semver bundles in non-semver order.
	bundles := []*registry.Bundle{
		newBundle(t, "csv-3", "pkg-0", []string{"stable"}, newUnstructuredCSVWithVersion(t, "csv-3", "0.3.0")),
		newBundle(t, "csv-1", "pkg-0", []string{"stable"}, newUnstructuredCSVWithVersion(t, "csv-1", "0.1.0")),
		newBundle(t, "csv-2", "pkg-0", []string{"stable"}, newUnstructuredCSVWithVersion(t, "csv-2", "0.2.0")),
	}
	for _, b := range bundles {
		graph, err := graphLoader.Generate(b.Package)
		require.Conditionf(t, func() bool {
			return err == nil || errors.Is(err, registry.ErrPackageNotInDatabase)
		}, "got unexpected error: %v", err)
		bundleLoader := registry.BundleGraphLoader{}
		updatedGraph, err := bundleLoader.AddBundleToGraph(b, graph, &registry.AnnotationsFile{Annotations: *b.Annotations}, false)
		require.NoError(t, err)
		err = store.AddBundleSemver(updatedGraph, b)
		require.NoError(t, err)
	}

	// Ensure bundles can be queried with expected replaces and skips values.
	querier := NewSQLLiteQuerierFromDb(db)
	gotBundles, err := querier.ListBundles(context.Background())
	require.NoError(t, err)
	replaces := map[string]string{}
	for _, b := range gotBundles {
		if b.PackageName != "pkg-0" {
			continue
		}
		require.Len(t, b.Skips, 0, "unexpected skips value(s) for bundle %q", b.CsvName)
		replaces[b.CsvName] = b.Replaces
	}
	require.Equal(t, map[string]string{
		"csv-3": "csv-2",
		"csv-2": "csv-1",
		"csv-1": "",
	}, replaces)
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

func newUnstructuredCSVWithSkips(t *testing.T, name, replaces string, skips ...string) *unstructured.Unstructured {
	csv := &registry.ClusterServiceVersion{}
	csv.TypeMeta.Kind = "ClusterServiceVersion"
	csv.SetName(name)
	allSkips, err := json.Marshal(skips)
	require.NoError(t, err)
	replacesSkips := fmt.Sprintf(`{"replaces": "%s", "skips": %s}`, replaces, string(allSkips))
	csv.Spec = json.RawMessage(replacesSkips)

	out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(csv)
	require.NoError(t, err)
	return &unstructured.Unstructured{Object: out}
}

func newUnstructuredCSVWithVersion(t *testing.T, name, version string) *unstructured.Unstructured {
	csv := &registry.ClusterServiceVersion{}
	csv.TypeMeta.Kind = "ClusterServiceVersion"
	csv.SetName(name)
	versionJson := fmt.Sprintf(`{"version": "%s"}`, version)
	csv.Spec = json.RawMessage(versionJson)

	out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(csv)
	require.NoError(t, err)
	return &unstructured.Unstructured{Object: out}
}

func newBundle(t *testing.T, name, pkgName string, channels []string, objs ...*unstructured.Unstructured) *registry.Bundle {
	bundle := registry.NewBundle(name, &registry.Annotations{
		PackageName: pkgName,
		Channels:    strings.Join(channels, ","),
	}, objs...)

	// Bust the bundle cache to set the CSV and CRDs
	_, err := bundle.ClusterServiceVersion()
	require.NoError(t, err)

	return bundle
}

func TestRMBundle(t *testing.T) {
	db, cleanup := CreateTestDb(t)
	defer cleanup()
	store, err := NewSQLLiteLoader(db)
	require.NoError(t, err)
	require.NoError(t, store.Migrate(context.Background()))
	tx, err := db.Begin()
	require.NoError(t, err)
	loader := store.(*sqlLoader)
	require.NoError(t, loader.rmBundle(tx, "non-existent"))
}

func TestDeprecationAwareLoader(t *testing.T) {
	withBundleImage := func(image string, bundle *registry.Bundle) *registry.Bundle {
		bundle.BundleImage = image
		return bundle
	}
	type fields struct {
		bundles         []*registry.Bundle
		pkgs            []registry.PackageManifest
		deprecatedPaths []string
	}
	type args struct {
		pkg string
	}
	type expected struct {
		err          error
		deprecated   map[string]struct{}
		nontruncated map[string]struct{}
	}
	tests := []struct {
		description string
		fields      fields
		args        args
		expected    expected
	}{
		{
			description: "NoDeprecation",
			fields: fields{
				bundles: []*registry.Bundle{
					withBundleImage("quay.io/my/bundle-a", newBundle(t, "csv-a", "pkg-0", []string{"stable"}, newUnstructuredCSV(t, "csv-a", ""))),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-a",
							},
						},
						DefaultChannelName: "stable",
					},
				},
				deprecatedPaths: []string{},
			},
			args: args{
				pkg: "pkg-0",
			},
			expected: expected{
				err:          nil,
				deprecated:   map[string]struct{}{},
				nontruncated: map[string]struct{}{},
			},
		},
		{
			description: "RemovePackage/DropsDeprecated",
			fields: fields{
				bundles: []*registry.Bundle{
					withBundleImage("quay.io/my/bundle-a", newBundle(t, "csv-a", "pkg-0", []string{"stable"}, newUnstructuredCSV(t, "csv-a", ""))),
					withBundleImage("quay.io/my/bundle-aa", newBundle(t, "csv-aa", "pkg-0", []string{"stable"}, newUnstructuredCSV(t, "csv-aa", "csv-a"))),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-aa",
							},
						},
						DefaultChannelName: "stable",
					},
				},
				deprecatedPaths: []string{
					"quay.io/my/bundle-a",
				},
			},
			args: args{
				pkg: "pkg-0",
			},
			expected: expected{
				err:          nil,
				deprecated:   map[string]struct{}{},
				nontruncated: map[string]struct{}{},
			},
		},
		{
			description: "RemovePackage/IgnoresOtherPackages",
			fields: fields{
				bundles: []*registry.Bundle{
					withBundleImage("quay.io/my/bundle-a", newBundle(t, "csv-a", "pkg-0", []string{"stable"}, newUnstructuredCSV(t, "csv-a", ""))),
					withBundleImage("quay.io/my/bundle-aa", newBundle(t, "csv-aa", "pkg-0", []string{"stable"}, newUnstructuredCSV(t, "csv-aa", "csv-a"))),
					withBundleImage("quay.io/my/bundle-b", newBundle(t, "csv-b", "pkg-1", []string{"stable"}, newUnstructuredCSV(t, "csv-b", ""))),
					withBundleImage("quay.io/my/bundle-bb", newBundle(t, "csv-bb", "pkg-1", []string{"stable"}, newUnstructuredCSV(t, "csv-bb", "csv-b"))),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-aa",
							},
						},
						DefaultChannelName: "stable",
					},
					{
						PackageName: "pkg-1",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-bb",
							},
						},
						DefaultChannelName: "stable",
					},
				},
				deprecatedPaths: []string{
					"quay.io/my/bundle-a",
					"quay.io/my/bundle-b",
				},
			},
			args: args{
				pkg: "pkg-0", // Should result in a alone being dropped from the deprecated table
			},
			expected: expected{
				err: nil,
				deprecated: map[string]struct{}{
					"csv-b": {},
				},
				nontruncated: map[string]struct{}{
					"csv-b:stable":  {},
					"csv-bb:stable": {},
				},
			},
		},
		{
			description: "DeprecateTruncate/DropsTruncated",
			fields: fields{
				bundles: []*registry.Bundle{
					withBundleImage("quay.io/my/bundle-a", newBundle(t, "csv-a", "pkg-0", []string{"stable"}, newUnstructuredCSV(t, "csv-a", ""))),
					withBundleImage("quay.io/my/bundle-aa", newBundle(t, "csv-aa", "pkg-0", []string{"stable"}, newUnstructuredCSV(t, "csv-aa", "csv-a"))),
					withBundleImage("quay.io/my/bundle-aaa", newBundle(t, "csv-aaa", "pkg-0", []string{"stable"}, newUnstructuredCSV(t, "csv-aaa", "csv-aa"))),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-aaa",
							},
						},
						DefaultChannelName: "stable",
					},
				},
				deprecatedPaths: []string{
					"quay.io/my/bundle-a",
					"quay.io/my/bundle-aa", // Should truncate a, dropping it from the deprecated table
				},
			},
			expected: expected{
				err: nil,
				deprecated: map[string]struct{}{
					"csv-aa": struct{}{}, // csv-b remains in the deprecated table since it has been truncated and hasn't been removed
				},
				nontruncated: map[string]struct{}{
					"csv-aa:stable":  {},
					"csv-aaa:stable": {},
				},
			},
		},
		{
			description: "DeprecateTruncateRemoveDeprecatedChannelHeadOnPackageRemoval",
			fields: fields{
				bundles: []*registry.Bundle{
					withBundleImage("quay.io/my/bundle-a", newBundle(t, "csv-a", "pkg-0", []string{"a"}, newUnstructuredCSV(t, "csv-a", ""))),
					withBundleImage("quay.io/my/bundle-aa", newBundle(t, "csv-aa", "pkg-0", []string{"a"}, newUnstructuredCSV(t, "csv-aa", "csv-a"))),
					withBundleImage("quay.io/my/bundle-b", newBundle(t, "csv-b", "pkg-0", []string{"b"}, newUnstructuredCSV(t, "csv-b", "csv-a"))),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "a",
								CurrentCSVName: "csv-aa",
							},
							{
								Name:           "b",
								CurrentCSVName: "csv-b",
							},
						},
						DefaultChannelName: "b",
					},
				},
				deprecatedPaths: []string{
					"quay.io/my/bundle-aa",
				},
			},
			args: args{
				pkg: "pkg-0",
			},
			expected: expected{
				err:        nil,
				deprecated: map[string]struct{}{},
			},
		},
		{
			description: "DeprecateChannelHead",
			fields: fields{
				bundles: []*registry.Bundle{
					withBundleImage("quay.io/my/bundle-a", newBundle(t, "csv-a", "pkg-0", []string{"a"}, newUnstructuredCSV(t, "csv-a", ""))),
					withBundleImage("quay.io/my/bundle-b", newBundle(t, "csv-b", "pkg-0", []string{"b"}, newUnstructuredCSV(t, "csv-b", "csv-a"))),
					withBundleImage("quay.io/my/bundle-aa", newBundle(t, "csv-aa", "pkg-0", []string{"a"}, newUnstructuredCSV(t, "csv-aa", "csv-a"))),
					withBundleImage("quay.io/my/bundle-aaa", newBundle(t, "csv-aaa", "pkg-0", []string{"a"}, newUnstructuredCSVWithSkips(t, "csv-aaa", "csv-aa", "csv-cc"))),
					withBundleImage("quay.io/my/bundle-cc", newBundle(t, "csv-cc", "pkg-0", []string{"c"}, newUnstructuredCSV(t, "csv-cc", "csv-c"))),
					withBundleImage("quay.io/my/bundle-c", newBundle(t, "csv-c", "pkg-0", []string{"c"}, newUnstructuredCSV(t, "csv-c", ""))),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "a",
								CurrentCSVName: "csv-aaa",
							},
							{
								Name:           "b",
								CurrentCSVName: "csv-b",
							},
							{
								Name:           "c",
								CurrentCSVName: "csv-cc",
							},
						},
						DefaultChannelName: "b",
					},
				},
				deprecatedPaths: []string{
					"quay.io/my/bundle-aaa",
				},
			},
			expected: expected{
				err: nil,
				deprecated: map[string]struct{}{
					"csv-aaa": {},
				},
				nontruncated: map[string]struct{}{
					"csv-a:b":  {},
					"csv-aaa:": {},
					"csv-b:b":  {},
					"csv-c:c":  {},
					"csv-cc:c": {},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			db, cleanup := CreateTestDb(t)
			defer cleanup()
			store, err := NewDeprecationAwareLoader(db)
			require.NoError(t, err)
			err = store.Migrate(context.TODO())
			require.NoError(t, err)

			for _, bundle := range tt.fields.bundles {
				require.NoError(t, store.AddOperatorBundle(bundle))
			}

			for _, pkg := range tt.fields.pkgs {
				require.NoError(t, store.AddPackageChannels(pkg))
			}
			for _, deprecatedPath := range tt.fields.deprecatedPaths {
				require.NoError(t, store.DeprecateBundle(deprecatedPath))
			}

			if tt.args.pkg != "" {
				err = store.RemovePackage(tt.args.pkg)
				if tt.expected.err != nil {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			}

			tx, err := db.Begin()
			require.NoError(t, err)

			checkForBundles := func(query, table string, bundleMap map[string]struct{}) {
				rows, err := tx.Query(query)
				require.NoError(t, err)
				require.NotNil(t, rows)

				var bundleName string
				for rows.Next() {
					require.NoError(t, rows.Scan(&bundleName))
					_, ok := bundleMap[bundleName]
					require.True(t, ok, "bundle shouldn't be in the %s table: %s", table, bundleName)
					delete(bundleMap, bundleName)
				}

				require.Len(t, bundleMap, 0, "not all expected bundles exist in %s table: %v", table, bundleMap)
			}
			checkForBundles(`SELECT operatorbundle_name FROM deprecated`, "deprecated", tt.expected.deprecated)
			// operatorbundle_name:<channel list>
			checkForBundles(`SELECT name||":"|| coalesce(group_concat(distinct channel_name), "") FROM (SELECT name, channel_name from operatorbundle left outer join channel_entry on name=operatorbundle_name order by channel_name) group by name`, "operatorbundle", tt.expected.nontruncated)

		})
	}
}

func TestGetTailFromBundle(t *testing.T) {
	type fields struct {
		bundles []*registry.Bundle
		pkgs    []registry.PackageManifest
	}
	type args struct {
		bundle string
	}
	type expected struct {
		err  error
		tail map[string]tailBundle
	}
	tests := []struct {
		description string
		fields      fields
		args        args
		expected    expected
	}{
		{
			description: "ContainsDefaultChannel",
			fields: fields{
				bundles: []*registry.Bundle{
					newBundle(t, "csv-a", "pkg-0", []string{"alpha"}, newUnstructuredCSV(t, "csv-a", "csv-b")),
					newBundle(t, "csv-b", "pkg-0", []string{"alpha", "stable"}, newUnstructuredCSV(t, "csv-b", "csv-c")),
					newBundle(t, "csv-c", "pkg-0", []string{"alpha", "stable"}, newUnstructuredCSV(t, "csv-c", "")),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "alpha",
								CurrentCSVName: "csv-a",
							},
							{
								Name:           "stable",
								CurrentCSVName: "csv-b",
							},
						},
						DefaultChannelName: "stable",
					},
				},
			},
			args: args{
				bundle: "csv-a",
			},
			expected: expected{
				err:  registry.ErrRemovingDefaultChannelDuringDeprecation,
				tail: nil,
			},
		},
		{
			description: "ContainsNoDefaultChannel",
			fields: fields{
				bundles: []*registry.Bundle{
					newBundle(t, "csv-a", "pkg-0", []string{"alpha"}, newUnstructuredCSV(t, "csv-a", "csv-b")),
					newBundle(t, "csv-b", "pkg-0", []string{"alpha", "stable"}, newUnstructuredCSV(t, "csv-b", "csv-c")),
					newBundle(t, "csv-c", "pkg-0", []string{"alpha", "stable"}, newUnstructuredCSV(t, "csv-c", "")),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "alpha",
								CurrentCSVName: "csv-a",
							},
							{
								Name:           "stable",
								CurrentCSVName: "csv-b",
							},
						},
						DefaultChannelName: "alpha",
					},
				},
			},
			args: args{
				bundle: "csv-b",
			},
			expected: expected{
				err: nil,
				tail: map[string]tailBundle{
					"csv-b": {name: "csv-b", channels: []string{"alpha", "stable"}, replaces: []string{"csv-c"}, replacedBy: []string{"csv-a"}},
					"csv-c": {name: "csv-c", channels: []string{"alpha", "stable"}, replacedBy: []string{"csv-b"}},
				},
			},
		},
		{
			description: "ContainsSkips",
			fields: fields{
				bundles: []*registry.Bundle{
					newBundle(t, "csv-a", "pkg-0", []string{"alpha"}, newUnstructuredCSV(t, "csv-a", "csv-b")),
					newBundle(t, "csv-b", "pkg-0", []string{"alpha"}, newUnstructuredCSVWithSkips(t, "csv-b", "csv-c", "csv-d", "csv-e", "csv-f")),
					newBundle(t, "csv-c", "pkg-0", []string{"alpha", "stable"}, newUnstructuredCSV(t, "csv-c", "csv-d")),
					newBundle(t, "csv-d", "pkg-0", []string{"alpha", "stable"}, newUnstructuredCSV(t, "csv-d", "")),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "alpha",
								CurrentCSVName: "csv-a",
							},
							{
								Name:           "stable",
								CurrentCSVName: "csv-b",
							},
						},
						DefaultChannelName: "alpha",
					},
				},
			},
			args: args{
				bundle: "csv-b",
			},
			expected: expected{
				err: nil,
				tail: map[string]tailBundle{
					"csv-b": {name: "csv-b", channels: []string{"alpha", "stable"}, replaces: []string{"csv-c", "csv-d", "csv-e", "csv-f"}, replacedBy: []string{"csv-a"}},
					"csv-c": {name: "csv-c", channels: []string{"alpha", "stable"}, replaces: []string{"csv-d"}, replacedBy: []string{"csv-b"}},
					"csv-d": {name: "csv-d", channels: []string{"alpha", "stable"}, replacedBy: []string{"csv-b", "csv-c"}},
					"csv-e": {name: "csv-e", channels: []string{"alpha", "stable"}, replacedBy: []string{"csv-b"}},
					"csv-f": {name: "csv-f", channels: []string{"alpha", "stable"}, replacedBy: []string{"csv-b"}},
				},
			},
		},
		{
			description: "ContainsDefaultChannelFromSkips",
			fields: fields{
				bundles: []*registry.Bundle{
					newBundle(t, "csv-a", "pkg-0", []string{"alpha"}, newUnstructuredCSV(t, "csv-a", "csv-b")),
					newBundle(t, "csv-b", "pkg-0", []string{"alpha"}, newUnstructuredCSVWithSkips(t, "csv-b", "csv-d", "csv-c")),
					newBundle(t, "csv-c", "pkg-0", []string{"alpha", "stable"}, newUnstructuredCSV(t, "csv-c", "csv-d")),
					newBundle(t, "csv-d", "pkg-0", []string{"alpha", "stable"}, newUnstructuredCSV(t, "csv-d", "")),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "alpha",
								CurrentCSVName: "csv-a",
							},
							{
								Name:           "stable",
								CurrentCSVName: "csv-c",
							},
						},
						DefaultChannelName: "stable",
					},
				},
			},
			args: args{
				bundle: "csv-b",
			},
			expected: expected{
				err:  registry.ErrRemovingDefaultChannelDuringDeprecation,
				tail: nil,
			},
		},
		{
			/*
				0.1.2 <- 0.1.1 <- 0.1.0
						   V (skips)
				1.1.2 <- 1.1.1 <- 1.1.0
			*/
			description: "branchPoint",
			fields: fields{
				bundles: []*registry.Bundle{
					newBundle(t, "csv-0.1.0", "pkg-0", []string{"0.1.x"}, newUnstructuredCSV(t, "csv-0.1.0", "")),
					newBundle(t, "csv-0.1.1", "pkg-0", []string{"0.1.x", "lts"}, newUnstructuredCSV(t, "csv-0.1.1", "csv-0.1.0")),
					newBundle(t, "csv-0.1.2", "pkg-0", []string{"0.1.x"}, newUnstructuredCSV(t, "csv-0.1.2", "csv-0.1.1")),
					newBundle(t, "csv-1.1.0", "pkg-0", []string{"1.1.x"}, newUnstructuredCSV(t, "csv-1.1.0", "")),
					newBundle(t, "csv-1.1.1", "pkg-0", []string{"1.1.x", "lts"}, newUnstructuredCSVWithSkips(t, "csv-1.1.1", "csv-1.1.0", "csv-0.1.1")),
					newBundle(t, "csv-1.1.2", "pkg-0", []string{"1.1.x", "lts"}, newUnstructuredCSV(t, "csv-1.1.2", "csv-1.1.1")),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "0.1.x",
								CurrentCSVName: "csv-0.1.2",
							},
							{
								Name:           "1.1.x",
								CurrentCSVName: "csv-1.1.2",
							},
							{
								Name:           "lts",
								CurrentCSVName: "csv-1.1.2",
							},
						},
						DefaultChannelName: "0.1.x",
					},
				},
			},
			args: args{
				bundle: "csv-1.1.2",
			},
			expected: expected{
				err: nil,
				tail: map[string]tailBundle{
					"csv-1.1.2": {name: "csv-1.1.2", channels: []string{"1.1.x", "lts"}, replaces: []string{"csv-1.1.1"}},
					"csv-1.1.1": {name: "csv-1.1.1", channels: []string{"1.1.x", "lts"}, replaces: []string{"csv-0.1.1", "csv-1.1.0"}, replacedBy: []string{"csv-1.1.2"}},
					"csv-1.1.0": {name: "csv-1.1.0", channels: []string{"1.1.x", "lts"}, replacedBy: []string{"csv-1.1.1"}},
					"csv-0.1.1": {name: "csv-0.1.1", channels: []string{"0.1.x", "1.1.x", "lts"}, replaces: []string{"csv-0.1.0"}, replacedBy: []string{"csv-0.1.2", "csv-1.1.1"}}, // 0.1.2 present in replacedBy but not in tail
					"csv-0.1.0": {name: "csv-0.1.0", channels: []string{"0.1.x"}, replacedBy: []string{"csv-0.1.1"}},
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
				require.NoError(t, store.AddOperatorBundle(bundle))
			}

			for _, pkg := range tt.fields.pkgs {
				require.NoError(t, store.AddPackageChannels(pkg))
			}
			tx, err := db.Begin()
			require.NoError(t, err)
			tail, err := getTailFromBundle(tx, tt.args.bundle)

			require.Equal(t, tt.expected.err, err)
			require.EqualValues(t, tt.expected.tail, tail)
		})
	}
}

func TestAddBundlePropertiesFromAnnotations(t *testing.T) {
	mustMarshal := func(u interface{}) string {
		v, err := json.Marshal(u)
		require.NoError(t, err)
		return string(v)
	}

	type in struct {
		annotations map[string]string
	}
	type expect struct {
		err bool
	}
	for _, tt := range []struct {
		description string
		in          in
		expect      expect
	}{
		{
			description: "Invalid/Properties",
			in: in{
				annotations: map[string]string{
					registry.PropertyKey: "bad_properties",
				},
			},
			expect: expect{
				err: true,
			},
		},
		{
			description: "Invalid/KnownType/Label",
			in: in{
				annotations: map[string]string{
					registry.PropertyKey: fmt.Sprintf(`[{"type": "%s", "value": "bad_value"}]`, registry.LabelType),
				},
			},
			expect: expect{
				err: true,
			},
		},
		{
			description: "Invalid/KnownType/Package",
			in: in{
				annotations: map[string]string{
					registry.PropertyKey: fmt.Sprintf(`[{"type": "%s", "value": "bad_value"}]`, registry.PackageType),
				},
			},
			expect: expect{
				err: true,
			},
		},
		{
			description: "Invalid/KnownType/GVK",
			in: in{
				annotations: map[string]string{
					registry.PropertyKey: fmt.Sprintf(`[{"type": "%s", "value": "bad_value"}]`, registry.GVKType),
				},
			},
			expect: expect{
				err: true,
			},
		},
		{
			description: "Valid/KnownTypes",
			in: in{
				annotations: map[string]string{
					registry.PropertyKey: mustMarshal([]interface{}{
						registry.LabelProperty{
							Label: "sulaco",
						},
						registry.PackageProperty{
							PackageName: "lv-426",
							Version:     "1.0.0",
						},
						registry.GVKProperty{
							Group:   "weyland.io",
							Kind:    "Dropship",
							Version: "v1",
						},
						registry.DeprecatedProperty{},
					}),
				},
			},
			expect: expect{
				err: false,
			},
		},
		{
			description: "Valid/UnknownType", // Unknown types are handled as opaque blobs
			in: in{
				annotations: map[string]string{
					registry.PropertyKey: fmt.Sprintf(`[{"type": "%s", "value": "anything_value"}]`, "anything"),
				},
			},
			expect: expect{
				err: false,
			},
		},
	} {
		t.Run(tt.description, func(t *testing.T) {
			db, cleanup := CreateTestDb(t)
			defer cleanup()

			s, err := NewSQLLiteLoader(db)
			store := s.(*sqlLoader)
			require.NoError(t, err)
			require.NoError(t, store.Migrate(context.TODO()))

			tx, err := db.Begin()
			require.NoError(t, err)

			csv := newUnstructuredCSV(t, "ripley", "")
			csv.SetAnnotations(tt.in.annotations)
			err = store.addBundleProperties(tx, newBundle(t, csv.GetName(), "lv-426", nil, csv))
			if tt.expect.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRemoveOverwrittenChannelHead(t *testing.T) {
	type fields struct {
		bundles []*registry.Bundle
		pkgs    []registry.PackageManifest
	}
	type args struct {
		bundle string
		pkg    string
	}
	type expected struct {
		err     error
		bundles map[string]struct{}
	}
	tests := []struct {
		description string
		fields      fields
		args        args
		expected    expected
	}{
		{
			description: "ChannelHead/SingleBundlePackage",
			fields: fields{
				bundles: []*registry.Bundle{
					newBundle(t, "csv-a", "pkg-0", []string{"a", "b"}, newUnstructuredCSV(t, "csv-a", "")),
					newBundle(t, "csv-b", "pkg-1", []string{"a", "b"}, newUnstructuredCSV(t, "csv-b", "")),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "a",
								CurrentCSVName: "csv-a",
							},
							{
								Name:           "b",
								CurrentCSVName: "csv-a",
							},
						},
						DefaultChannelName: "a",
					},
					{
						PackageName: "pkg-1",
						Channels: []registry.PackageChannel{
							{
								Name:           "a",
								CurrentCSVName: "csv-b",
							},
							{
								Name:           "b",
								CurrentCSVName: "csv-b",
							},
						},
						DefaultChannelName: "a",
					},
				},
			},
			args: args{
				bundle: "csv-a",
				pkg:    "pkg-0",
			},
			expected: expected{
				bundles: map[string]struct{}{
					"pkg-1/a/csv-b": {},
					"pkg-1/b/csv-b": {},
				},
			},
		},
		{
			description: "ChannelHead/WithReplacement",
			fields: fields{
				bundles: []*registry.Bundle{
					newBundle(t, "csv-a", "pkg-0", []string{"a", "b"}, newUnstructuredCSV(t, "csv-a", "")),
					newBundle(t, "csv-aa", "pkg-0", []string{"b"}, newUnstructuredCSV(t, "csv-aa", "csv-a")),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "a",
								CurrentCSVName: "csv-a",
							},
							{
								Name:           "b",
								CurrentCSVName: "csv-aa",
							},
						},
						DefaultChannelName: "b",
					},
				},
			},
			args: args{
				bundle: "csv-a",
				pkg:    "pkg-0",
			},
			expected: expected{
				err: fmt.Errorf("cannot overwrite bundle csv-a from package pkg-0: replaced by csv-aa on channel b"),
				bundles: map[string]struct{}{
					"pkg-0/a/csv-a":  {},
					"pkg-0/b/csv-a":  {},
					"pkg-0/b/csv-aa": {},
				},
			},
		},
		{
			description: "ChannelHead",
			fields: fields{
				bundles: []*registry.Bundle{
					newBundle(t, "csv-a", "pkg-0", []string{"a", "b"}, newUnstructuredCSVWithSkips(t, "csv-a", "csv-b", "csv-c")),
					newBundle(t, "csv-b", "pkg-0", []string{"b", "d"}, newUnstructuredCSV(t, "csv-b", "")),
					newBundle(t, "csv-d", "pkg-0", []string{"d"}, newUnstructuredCSV(t, "csv-d", "csv-b")),
					newBundle(t, "csv-c", "pkg-0", []string{"c"}, newUnstructuredCSV(t, "csv-c", "")),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "a",
								CurrentCSVName: "csv-a",
							},
							{
								Name:           "b",
								CurrentCSVName: "csv-a",
							},
							{
								Name:           "c",
								CurrentCSVName: "csv-c",
							},
							{
								Name:           "d",
								CurrentCSVName: "csv-d",
							},
						},
						DefaultChannelName: "a",
					},
				},
			},
			args: args{
				bundle: "csv-a",
				pkg:    "pkg-0",
			},
			expected: expected{
				err: nil,
				bundles: map[string]struct{}{
					"pkg-0/a/csv-b": {},
					"pkg-0/b/csv-b": {},
					"pkg-0/a/csv-c": {},
					"pkg-0/b/csv-c": {},
					"pkg-0/c/csv-c": {},
					"pkg-0/d/csv-d": {},
					"pkg-0/d/csv-b": {},
				},
			},
		},

		{
			description: "PersistDefaultChannel",
			fields: fields{
				bundles: []*registry.Bundle{
					newBundle(t, "csv-a", "pkg-0", []string{"a"}, newUnstructuredCSV(t, "csv-a", "")),
					newBundle(t, "csv-b", "pkg-0", []string{"b"}, newUnstructuredCSV(t, "csv-b", "")),
				},
				pkgs: []registry.PackageManifest{
					{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "a",
								CurrentCSVName: "csv-a",
							},
							{
								Name:           "b",
								CurrentCSVName: "csv-b",
							},
						},
						DefaultChannelName: "a",
					},
				},
			},
			args: args{
				bundle: "csv-a",
				pkg:    "pkg-0",
			},
			expected: expected{
				err: nil,
				bundles: map[string]struct{}{
					"pkg-0/b/csv-b": {},
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
			err = store.Migrate(context.Background())
			require.NoError(t, err)

			for _, bundle := range tt.fields.bundles {
				// Throw away any errors loading bundles (not testing this)
				store.AddOperatorBundle(bundle)
			}

			for _, pkg := range tt.fields.pkgs {
				// Throw away any errors loading packages (not testing this)
				store.AddPackageChannels(pkg)
			}

			getDefaultChannel := func(pkg string) sql.NullString {
				// get defaultChannel before delete
				rows, err := db.QueryContext(context.Background(), `SELECT default_channel FROM package WHERE name = ?`, pkg)
				require.NoError(t, err)
				defer rows.Close()
				var defaultChannel sql.NullString
				for rows.Next() {
					require.NoError(t, rows.Scan(&defaultChannel))
					break
				}
				return defaultChannel
			}
			oldDefaultChannel := getDefaultChannel(tt.args.pkg)

			err = store.(registry.HeadOverwriter).RemoveOverwrittenChannelHead(tt.args.pkg, tt.args.bundle)
			if tt.expected.err != nil {
				require.EqualError(t, err, tt.expected.err.Error())
			} else {
				require.NoError(t, err)
			}

			querier := NewSQLLiteQuerierFromDb(db)

			bundles, err := querier.ListBundles(context.Background())
			require.NoError(t, err)

			var extra []string
			for _, b := range bundles {
				key := fmt.Sprintf("%s/%s/%s", b.PackageName, b.ChannelName, b.CsvName)
				if _, ok := tt.expected.bundles[key]; ok {
					delete(tt.expected.bundles, key)
				} else {
					extra = append(extra, key)
				}
			}

			if len(tt.expected.bundles) > 0 {
				t.Errorf("not all expected bundles were found: missing %v", tt.expected.bundles)
			}
			if len(extra) > 0 {
				t.Errorf("unexpected bundles found: %v", extra)
			}

			// should preserve defaultChannel entry in package table
			currentDefaultChannel := getDefaultChannel(tt.args.pkg)
			require.Equal(t, oldDefaultChannel, currentDefaultChannel)
		})
	}
}
