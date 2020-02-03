package boltdb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/asdine/storm/v3"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func TestNewStormLoader(t *testing.T) {
	type args struct {
		db *storm.DB
	}
	tests := []struct {
		name string
		args args
		want *StormLoader
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewStormLoader(tt.args.db); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewStormLoader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStormLoader_AddBundlePackageChannels(t *testing.T) {
	type fields struct {
		db *storm.DB
	}
	type args struct {
		manifest registry.PackageManifest
		bundle   registry.Bundle
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &StormLoader{
				db: tt.fields.db,
			}
			if err := s.AddBundlePackageChannels(tt.args.manifest, tt.args.bundle); (err != nil) != tt.wantErr {
				t.Errorf("AddBundlePackageChannels() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStormLoader_AddOperatorBundle(t *testing.T) {
	type fields struct {
		db *storm.DB
	}
	type args struct {
		bundle *registry.Bundle
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "GoodBundle",
			args: args{
				bundle: newBundle(t, "csv-a", "pkg-0", "stable", newUnstructuredCSV(t, "csv-a", "non-existant")),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := storm.Open("my.db")
			require.NoError(t, err)
			defer os.Remove("my.db")
			defer db.Close()

			s := &StormLoader{
				db: db,
			}
			if err := s.AddOperatorBundle(tt.args.bundle); (err != nil) != tt.wantErr {
				t.Errorf("AddOperatorBundle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// func TestStormLoader_AddPackageChannels(t *testing.T) {
// 	type fields struct {
// 		db *storm.DB
// 	}
// 	type args struct {
// 		manifest registry.PackageManifest
// 	}
// 	tests := []struct {
// 		name    string
// 		fields  fields
// 		args    args
// 		wantErr bool
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			s := &StormLoader{
// 				db: tt.fields.db,
// 			}
// 			if err := s.AddPackageChannels(tt.args.manifest); (err != nil) != tt.wantErr {
// 				t.Errorf("AddPackageChannels() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

func TestStormLoader_TestAddPackageChannels(t *testing.T) {
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
			db, err := storm.Open("my.db")
			require.NoError(t, err)
			// defer os.Remove("my.db")
			defer db.Close()

			s := &StormLoader{
				db: db,
			}

			for _, bundle := range tt.fields.bundles {
				// Throw away any errors loading bundles (not testing this)
				s.AddOperatorBundle(bundle)
			}

			for i, pkg := range tt.args.pkgs {
				err := s.AddPackageChannels(pkg)
				require.Equal(t, tt.expected.errs[i], err, "expected %v, got %v", tt.expected.errs[i], err)
			}

			// Ensure expected packages were loaded
			q := &StormQuerier{
				db: db,
			}
			pkgs, err := q.ListPackages(context.Background())
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expected.pkgs, pkgs)
		})
	}
}

func TestStormLoader_ClearNonDefaultBundles(t *testing.T) {
	type fields struct {
		db *storm.DB
	}
	type args struct {
		packageName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &StormLoader{
				db: tt.fields.db,
			}
			if err := s.ClearNonDefaultBundles(tt.args.packageName); (err != nil) != tt.wantErr {
				t.Errorf("ClearNonDefaultBundles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStormLoader_RmPackageName(t *testing.T) {
	type fields struct {
		db *storm.DB
	}
	type args struct {
		packageName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &StormLoader{
				db: tt.fields.db,
			}
			if err := s.RmPackageName(tt.args.packageName); (err != nil) != tt.wantErr {
				t.Errorf("RmPackageName() error = %v, wantErr %v", err, tt.wantErr)
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

func newBundle(t *testing.T, name, pkgName, channelName string, objs ...*unstructured.Unstructured) *registry.Bundle {
	bundle := registry.NewBundle(name, pkgName, channelName, objs...)

	// Bust the bundle cache to set the CSV and CRDs
	_, err := bundle.ClusterServiceVersion()
	require.NoError(t, err)

	return bundle
}
