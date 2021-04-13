package sqlite

import (
	"context"
	"encoding/json"
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
			t.Logf("%#v", tt.expected.pkgs)
			t.Logf("%#v", pkgs)
			require.ElementsMatch(t, tt.expected.pkgs, pkgs)
		})
	}
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

func newUnstructuredCSVwithSkips(t *testing.T, name, replaces string, skips ...string) *unstructured.Unstructured {
	csv := &registry.ClusterServiceVersion{}
	csv.TypeMeta.Kind = "ClusterServiceVersion"
	csv.SetName(name)
	allSkips, err := json.Marshal(skips)
	require.NoError(t, err)
	replacesSkips := fmt.Sprintf(`{"replaces": "%s", "skips": %s}`, replaces, string(allSkips))
	t.Logf("%v", replacesSkips)
	csv.Spec = json.RawMessage(replacesSkips)

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
		tail []string
	}
	tests := []struct {
		description string
		fields      fields
		args        args
		expected    expected
	}{
		{
			description: "GetTailFromBundle/RemoveDefaultChannelForbidden",
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
			description: "GetTailFromBundle/RemovingNonDefaultChannel",
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
				bundle: "csv-a",
			},
			expected: expected{
				err: nil,
				tail: []string{
					"csv-b",
					"csv-c",
				},
			},
		},
		{
			description: "GetTailFromBundle/HandlesSkips",
			fields: fields{
				bundles: []*registry.Bundle{
					newBundle(t, "csv-a", "pkg-0", []string{"alpha"}, newUnstructuredCSVwithSkips(t, "csv-a", "csv-b", "csv-d", "csv-e", "csv-f")),
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
				bundle: "csv-a",
			},
			expected: expected{
				err: nil,
				tail: []string{
					"csv-b",
					"csv-c",
					"csv-d",
					"csv-e",
					"csv-f",
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

			for _, pkg := range tt.fields.pkgs {
				// Throw away any errors loading packages (not testing this)
				store.AddPackageChannels(pkg)
			}
			tx, err := db.Begin()
			require.NoError(t, err)
			tail, err := getTailFromBundle(tx, tt.args.bundle)

			require.Equal(t, tt.expected.err, err)
			t.Logf("tt.expected.tail %#v", tt.expected.tail)
			t.Logf("tail %#v", tail)
			require.ElementsMatch(t, tt.expected.tail, tail)
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
