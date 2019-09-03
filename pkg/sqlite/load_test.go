package sqlite

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

var testScheme = runtime.NewScheme()

func init() {
	v1alpha1.AddToScheme(testScheme)
}

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
			db := fmt.Sprintf("%d.db", rand.Int())
			store, err := NewSQLLiteLoader(db)
			require.NoError(t, err)
			defer func() {
				if err := os.Remove(db); err != nil {
					t.Fatal(err)
				}
			}()

			for _, bundle := range tt.fields.bundles {
				// Throw away any errors loading bundles (not testing this)
				store.AddOperatorBundle(bundle)
			}

			for i, pkg := range tt.args.pkgs {
				require.Equal(t, tt.expected.errs[i], store.AddPackageChannels(pkg))
			}

			// Ensure expected packages were loaded
			querier, err := NewSQLLiteQuerier(db)
			require.NoError(t, err)

			pkgs, err := querier.ListPackages(context.Background())
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expected.pkgs, pkgs)
		})
	}
}

func newUnstructuredCSV(t *testing.T, name, replaces string) *unstructured.Unstructured {
	csv := &v1alpha1.ClusterServiceVersion{}
	csv.SetName(name)
	csv.Spec.Replaces = replaces

	out := &unstructured.Unstructured{}
	require.NoError(t, testScheme.Convert(csv, out, nil))

	return out
}

func newBundle(t *testing.T, name, pkgName, channelName string, objs ...*unstructured.Unstructured) *registry.Bundle {
	bundle := registry.NewBundle(name, pkgName, channelName, objs...)

	// Bust the bundle cache to set the CSV and CRDs
	_, err := bundle.ClusterServiceVersion()
	require.NoError(t, err)

	return bundle
}
