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
					newBundle(t, "csv-a", "pkg-0", "stable", newUnstructuredCSV(t, "csv-a", "")),
					newBundle(t, "csv-a", "pkg-0", "stable", newUnstructuredCSV(t, "csv-a", "")),
					newBundle(t, "csv-b", "pkg-0", "alpha", newUnstructuredCSV(t, "csv-b", "")),
					newBundle(t, "csv-c", "pkg-1", "stable", newUnstructuredCSV(t, "csv-c", "")),
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
					newBundle(t, "csv-a", "pkg-0", "stable", newUnstructuredCSV(t, "csv-a", "non-existant")),
					newBundle(t, "csv-b", "pkg-0", "alpha", newUnstructuredCSV(t, "csv-b", "")),
					newBundle(t, "csv-c", "pkg-1", "stable", newUnstructuredCSV(t, "csv-c", "")),
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
					"pkg-0",
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
				errs := store.AddPackageChannels(pkg, registry.ReplacesMode)
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

func TestAddPackageChannels_SemVer(t *testing.T) {
	type bundleImageBlob struct {
		bundle *registry.Bundle
		pkg    registry.PackageManifest
	}
	type replace struct {
		from    string
		to      string
		channel string
		pkg     string
	}
	type expected struct {
		errs     []error
		pkgs     []string
		replaces []replace
	}
	tests := []struct {
		description      string
		bundleImageBlobs []bundleImageBlob
		expected         expected
	}{
		{
			description: "AddOneBundleWithReplacesSet",
			bundleImageBlobs: []bundleImageBlob{
				{
					bundle: newBundle(t, "csv-a", "pkg-0", "stable", newUnstructuredCSV(t, "csv-a", "csv-b")),
					pkg: registry.PackageManifest{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-a",
							},
							{
								Name:           "alpha",
								CurrentCSVName: "csv-a",
							},
						},
						DefaultChannelName: "stable",
					},
				},
			},
			expected: expected{
				errs: make([]error, 2),
				pkgs: []string{
					"pkg-0",
				},
			},
		},
		{
			description: "AddMultipleBundlesInOrder",
			bundleImageBlobs: []bundleImageBlob{
				{
					bundle: newBundle(t, "csv-a", "pkg-0", "stable", newUnstructuredCSVWithVersion(t, "csv-a", "0.6.0")),
					pkg: registry.PackageManifest{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-a",
							},
							{
								Name:           "alpha",
								CurrentCSVName: "csv-a",
							},
						},
						DefaultChannelName: "stable",
					},
				}, {
					bundle: newBundle(t, "csv-b", "pkg-0", "stable", newUnstructuredCSVWithVersion(t, "csv-b", "0.6.1")),
					pkg: registry.PackageManifest{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-b",
							},
							{
								Name:           "alpha",
								CurrentCSVName: "csv-a",
							},
						},
						DefaultChannelName: "stable",
					},
				},
			},
			expected: expected{
				errs: make([]error, 2),
				pkgs: []string{
					"pkg-0",
				},
				replaces: []replace{
					{
						from:    "csv-a",
						to:      "csv-b",
						pkg:     "pkg-0",
						channel: "stable",
					},
				},
			},
		},
		{
			description: "AddMultipleBundlesOutOfOrder",
			bundleImageBlobs: []bundleImageBlob{
				{
					bundle: newBundle(t, "csv-b", "pkg-0", "stable", newUnstructuredCSVWithVersion(t, "csv-b", "0.6.1")),
					pkg: registry.PackageManifest{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-b",
							},
							{
								Name:           "alpha",
								CurrentCSVName: "csv-b",
							},
						},
						DefaultChannelName: "stable",
					},
				}, {
					bundle: newBundle(t, "csv-a", "pkg-0", "stable", newUnstructuredCSVWithVersion(t, "csv-a", "0.6.0")),
					pkg: registry.PackageManifest{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-a",
							},
							{
								Name:           "alpha",
								CurrentCSVName: "csv-a",
							},
						},
						DefaultChannelName: "stable",
					},
				}, {
					bundle: newBundle(t, "csv-c", "pkg-0", "stable", newUnstructuredCSVWithVersion(t, "csv-c", "0.6.2")),
					pkg: registry.PackageManifest{
						PackageName: "pkg-0",
						Channels: []registry.PackageChannel{
							{
								Name:           "stable",
								CurrentCSVName: "csv-c",
							},
							{
								Name:           "alpha",
								CurrentCSVName: "csv-a",
							},
						},
						DefaultChannelName: "stable",
					},
				},
			},
			expected: expected{
				errs: make([]error, 2),
				pkgs: []string{
					"pkg-0",
				},
				replaces: []replace{
					{
						from:    "csv-a",
						to:      "csv-b",
						pkg:     "pkg-0",
						channel: "stable",
					},
					{
						from:    "csv-b",
						to:      "csv-c",
						pkg:     "pkg-0",
						channel: "stable",
					},
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

			for _, blob := range tt.bundleImageBlobs {
				err := store.AddBundlePackageChannels(blob.pkg, *blob.bundle, registry.SemVerMode)
				require.NoError(t, err)
			}

			// Ensure expected packages were loaded
			querier := NewSQLLiteQuerierFromDb(db)
			pkgs, err := querier.ListPackages(context.Background())
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expected.pkgs, pkgs)

			for _, replace := range tt.expected.replaces {
				r, err := querier.GetBundleThatReplaces(context.Background(), replace.from, replace.pkg, replace.channel)
				require.NoError(t, err)
				require.Equal(t, replace.to, r.CsvName)
			}
		})
	}
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

func newUnstructuredCSVWithVersion(t *testing.T, name, version string) *unstructured.Unstructured {
	csv := &registry.ClusterServiceVersion{}
	csv.TypeMeta.Kind = "ClusterServiceVersion"
	csv.SetName(name)
	csv.Spec = json.RawMessage(fmt.Sprintf(`{"version": "%s"}`, version))

	out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(csv)
	require.NoError(t, err)
	return &unstructured.Unstructured{Object: out}
}

func newBundle(t *testing.T, name, pkgName, channelName string, objs ...*unstructured.Unstructured) *registry.Bundle {
	bundle := registry.NewBundle(name, pkgName, channelName, objs...)

	// Bust the bundle cache to set the CSV and CRDs
	_, err := bundle.ClusterServiceVersion()
	require.NoError(t, err)

	return bundle
}
