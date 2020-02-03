package test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

func RunGeneralLoadSuite(t *testing.T, setup Setup) {
	logrus.SetLevel(logrus.DebugLevel)

	TestAddPackageChannels(t, setup)
}

func TestAddPackageChannels(t *testing.T, setup Setup) {
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
			loader, querier, teardown := setup(t)
			defer teardown(t)

			for _, bundle := range tt.fields.bundles {
				// Throw away any errors loading bundles (not testing this)
				loader.AddOperatorBundle(bundle)
			}

			for i, pkg := range tt.args.pkgs {
				errs := loader.AddPackageChannels(pkg)
				require.Equal(t, tt.expected.errs[i], errs, "expected %v, got %v", tt.expected.errs[i], errs)
			}

			// Ensure expected packages were loaded
			pkgs, err := querier.ListPackages(context.Background())
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expected.pkgs, pkgs)
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
