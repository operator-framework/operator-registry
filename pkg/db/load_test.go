package db

import (
	"encoding/json"
	"fmt"
	"github.com/asdine/storm/v3"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	"testing"
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

func TestStormLoader_AddPackageChannels(t *testing.T) {
	type fields struct {
		db *storm.DB
	}
	type args struct {
		manifest registry.PackageManifest
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
			if err := s.AddPackageChannels(tt.args.manifest); (err != nil) != tt.wantErr {
				t.Errorf("AddPackageChannels() error = %v, wantErr %v", err, tt.wantErr)
			}
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
