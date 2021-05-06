package sqlite

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/operator-framework/api/pkg/lib/version"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type testBundle struct {
	version        string
	pkg            string
	channels       []string
	defaultChannel string
	skips          []string
	replaces       string
}

func newRegistryBundle(t *testing.T, b testBundle) *registry.Bundle {
	name := b.version
	csvYAML, err := yaml.Marshal(&v1alpha1.ClusterServiceVersion{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.ClusterServiceVersionKind,
			APIVersion: v1alpha1.ClusterServiceVersionAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: map[string]string{}},
		Spec: v1alpha1.ClusterServiceVersionSpec{
			Version: version.OperatorVersion{
				Version: semver.MustParse(b.version),
			},
			Replaces: b.replaces,
			Skips:    b.skips,
		},
	})
	require.NoError(t, err)

	tmpdir, err := ioutil.TempDir(".", "depr-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	annotationsYAML, err := yaml.Marshal(&registry.AnnotationsFile{
		Annotations: registry.Annotations{
			PackageName:        b.pkg,
			Channels:           strings.Join(b.channels, ","),
			DefaultChannelName: b.defaultChannel,
		},
	})
	require.NoError(t, err)

	require.NoError(t, os.Mkdir(filepath.Join(tmpdir, "manifests"), 0755))
	require.NoError(t, os.Mkdir(filepath.Join(tmpdir, "metadata"), 0755))
	require.NoError(t, ioutil.WriteFile(filepath.Join(tmpdir, "manifests", "csv.yaml"), csvYAML, 0644))
	require.NoError(t, ioutil.WriteFile(filepath.Join(tmpdir, "metadata", "annotations.yaml"), annotationsYAML, 0644))

	img, err := registry.NewImageInput(image.SimpleReference(fmt.Sprintf("bundle-%s", b.version)), tmpdir)
	require.NoError(t, err)

	return img.Bundle
}

func TestDeprecate(t *testing.T) {
	tests := []struct {
		description   string
		initial       []testBundle
		deprecate     []string
		expected      []*api.Bundle
		expectedError error
	}{
		{
			description: "Missing Image",
			initial: []testBundle{
				{
					version:        "0.0.1",
					pkg:            "testpkg",
					channels:       []string{"stable"},
					defaultChannel: "stable",
					skips:          nil,
					replaces:       "",
				},
			},
			deprecate: []string{
				"bundle-nonexistent",
			},
			expectedError: fmt.Errorf("error deprecating bundle bundle-nonexistent: %s", registry.ErrBundleImageNotInDatabase),
		},
		{
			description: "Unordered multi-bundle deprecation",
			initial: []testBundle{
				{
					version:        "0.0.1",
					pkg:            "testpkg",
					channels:       []string{"stable", "0.x"},
					defaultChannel: "stable",
					skips:          []string{"0.0.1-rc0"},
				},
				{
					version:        "0.0.2",
					pkg:            "testpkg",
					channels:       []string{"stable", "0.x"},
					defaultChannel: "stable",
					replaces:       "0.0.1",
				},
				{
					version:        "0.0.3-rc0",
					pkg:            "testpkg",
					channels:       []string{"0.x"},
					defaultChannel: "stable",
					skips:          nil,
					replaces:       "0.0.2",
				},
				{
					version:        "1.0.1",
					pkg:            "testpkg",
					channels:       []string{"stable", "1.x"},
					defaultChannel: "stable",
					skips:          []string{"0.0.3-rc0", "0.0.1"},
					replaces:       "0.0.2",
				},
				{
					version:        "1.0.2-rc0",
					pkg:            "testpkg",
					channels:       []string{"1.x"},
					defaultChannel: "stable",
					skips:          nil,
					replaces:       "1.0.1",
				},
				{
					version:        "1.0.2",
					pkg:            "testpkg",
					channels:       []string{"stable", "1.x"},
					defaultChannel: "stable",
					skips:          []string{"1.0.2-rc0"},
					replaces:       "1.0.1",
				},
				{
					version:        "2.0.1",
					pkg:            "testpkg",
					channels:       []string{"stable", "2.x"},
					defaultChannel: "stable",
					skips:          nil,
					replaces:       "1.0.2",
				},
				{
					version:        "2.0.2",
					pkg:            "testpkg",
					channels:       []string{"stable", "2.x"},
					defaultChannel: "stable",
					skips:          nil,
					replaces:       "2.0.1",
				},
			},
			deprecate: []string{
				"bundle-1.0.2",
				"bundle-0.0.2",
				"bundle-2.0.2",
			},
			expected: []*api.Bundle{
				{
					CsvName:     "2.0.2",
					PackageName: "testpkg",
					ChannelName: "2.x",
					CsvJson:     "{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"creationTimestamp\":null,\"name\":\"2.0.2\"},\"spec\":{\"apiservicedefinitions\":{},\"cleanup\":{\"enabled\":false},\"customresourcedefinitions\":{},\"displayName\":\"\",\"install\":{\"spec\":{\"deployments\":null},\"strategy\":\"\"},\"provider\":{},\"replaces\":\"2.0.1\",\"version\":\"2.0.2\"},\"status\":{\"cleanup\":{}}}",
					Object: []string{
						"{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"creationTimestamp\":null,\"name\":\"2.0.2\"},\"spec\":{\"apiservicedefinitions\":{},\"cleanup\":{\"enabled\":false},\"customresourcedefinitions\":{},\"displayName\":\"\",\"install\":{\"spec\":{\"deployments\":null},\"strategy\":\"\"},\"provider\":{},\"replaces\":\"2.0.1\",\"version\":\"2.0.2\"},\"status\":{\"cleanup\":{}}}",
					},
					BundlePath: "bundle-2.0.2",
					Version:    "2.0.2",
					Replaces:   "",
					Skips:      nil,
					Properties: []*api.Property{{
						Type:  registry.DeprecatedType,
						Value: "{}",
					}},
				},
				{
					CsvName:     "2.0.2",
					PackageName: "testpkg",
					ChannelName: "stable",
					CsvJson:     "{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"creationTimestamp\":null,\"name\":\"2.0.2\"},\"spec\":{\"apiservicedefinitions\":{},\"cleanup\":{\"enabled\":false},\"customresourcedefinitions\":{},\"displayName\":\"\",\"install\":{\"spec\":{\"deployments\":null},\"strategy\":\"\"},\"provider\":{},\"replaces\":\"2.0.1\",\"version\":\"2.0.2\"},\"status\":{\"cleanup\":{}}}",
					Object: []string{
						"{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"creationTimestamp\":null,\"name\":\"2.0.2\"},\"spec\":{\"apiservicedefinitions\":{},\"cleanup\":{\"enabled\":false},\"customresourcedefinitions\":{},\"displayName\":\"\",\"install\":{\"spec\":{\"deployments\":null},\"strategy\":\"\"},\"provider\":{},\"replaces\":\"2.0.1\",\"version\":\"2.0.2\"},\"status\":{\"cleanup\":{}}}",
					},
					BundlePath: "bundle-2.0.2",
					Version:    "2.0.2",
					Replaces:   "",
					Skips:      nil,
					Properties: []*api.Property{{
						Type:  registry.DeprecatedType,
						Value: "{}",
					}},
				},
			},
		},
		{
			description: "deprecate semver branch",
			initial: []testBundle{
				{
					version:        "1.0.1",
					pkg:            "testpkg",
					channels:       []string{"stable", "1.x"},
					defaultChannel: "stable",
					skips:          nil,
					replaces:       "",
				},
				{
					version:        "1.0.2-rc0",
					pkg:            "testpkg",
					channels:       []string{"1.x"},
					defaultChannel: "stable",
					skips:          nil,
					replaces:       "1.0.1",
				},
				{
					version:        "1.0.2-rc1",
					pkg:            "testpkg",
					channels:       []string{"1.x"},
					defaultChannel: "stable",
					skips:          []string{"1.0.2-rc0"},
					replaces:       "1.0.1",
				},
				{
					version:        "1.0.2",
					pkg:            "testpkg",
					channels:       []string{"stable", "1.x"},
					defaultChannel: "stable",
					skips:          []string{"1.0.2-rc0"},
					replaces:       "1.0.1",
				},
			},
			deprecate: []string{
				"bundle-1.0.2-rc1",
				"bundle-1.0.2-rc0",
			},
			expected: []*api.Bundle{

				{
					CsvName:     "1.0.2",
					PackageName: "testpkg",
					ChannelName: "1.x",
					CsvJson:     "{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"creationTimestamp\":null,\"name\":\"1.0.2\"},\"spec\":{\"apiservicedefinitions\":{},\"cleanup\":{\"enabled\":false},\"customresourcedefinitions\":{},\"displayName\":\"\",\"install\":{\"spec\":{\"deployments\":null},\"strategy\":\"\"},\"provider\":{},\"replaces\":\"1.0.1\",\"skips\":[\"1.0.2-rc0\"],\"version\":\"1.0.2\"},\"status\":{\"cleanup\":{}}}",
					Object:      []string{"{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"creationTimestamp\":null,\"name\":\"1.0.2\"},\"spec\":{\"apiservicedefinitions\":{},\"cleanup\":{\"enabled\":false},\"customresourcedefinitions\":{},\"displayName\":\"\",\"install\":{\"spec\":{\"deployments\":null},\"strategy\":\"\"},\"provider\":{},\"replaces\":\"1.0.1\",\"skips\":[\"1.0.2-rc0\"],\"version\":\"1.0.2\"},\"status\":{\"cleanup\":{}}}"},
					BundlePath:  "bundle-1.0.2",
					Version:     "1.0.2",
					Replaces:    "1.0.2-rc1",
				},
				{
					CsvName:     "1.0.2",
					PackageName: "testpkg",
					ChannelName: "stable",
					CsvJson:     "{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"creationTimestamp\":null,\"name\":\"1.0.2\"},\"spec\":{\"apiservicedefinitions\":{},\"cleanup\":{\"enabled\":false},\"customresourcedefinitions\":{},\"displayName\":\"\",\"install\":{\"spec\":{\"deployments\":null},\"strategy\":\"\"},\"provider\":{},\"replaces\":\"1.0.1\",\"skips\":[\"1.0.2-rc0\"],\"version\":\"1.0.2\"},\"status\":{\"cleanup\":{}}}",
					Object:      []string{"{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"creationTimestamp\":null,\"name\":\"1.0.2\"},\"spec\":{\"apiservicedefinitions\":{},\"cleanup\":{\"enabled\":false},\"customresourcedefinitions\":{},\"displayName\":\"\",\"install\":{\"spec\":{\"deployments\":null},\"strategy\":\"\"},\"provider\":{},\"replaces\":\"1.0.1\",\"skips\":[\"1.0.2-rc0\"],\"version\":\"1.0.2\"},\"status\":{\"cleanup\":{}}}"},
					BundlePath:  "bundle-1.0.2",
					Version:     "1.0.2",
				},
				{
					CsvName:     "1.0.2-rc1",
					PackageName: "testpkg",
					ChannelName: "1.x",
					CsvJson:     "{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"creationTimestamp\":null,\"name\":\"1.0.2-rc1\"},\"spec\":{\"apiservicedefinitions\":{},\"cleanup\":{\"enabled\":false},\"customresourcedefinitions\":{},\"displayName\":\"\",\"install\":{\"spec\":{\"deployments\":null},\"strategy\":\"\"},\"provider\":{},\"replaces\":\"1.0.1\",\"skips\":[\"1.0.2-rc0\"],\"version\":\"1.0.2-rc1\"},\"status\":{\"cleanup\":{}}}",
					Object:      []string{"{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"creationTimestamp\":null,\"name\":\"1.0.2-rc1\"},\"spec\":{\"apiservicedefinitions\":{},\"cleanup\":{\"enabled\":false},\"customresourcedefinitions\":{},\"displayName\":\"\",\"install\":{\"spec\":{\"deployments\":null},\"strategy\":\"\"},\"provider\":{},\"replaces\":\"1.0.1\",\"skips\":[\"1.0.2-rc0\"],\"version\":\"1.0.2-rc1\"},\"status\":{\"cleanup\":{}}}"},
					BundlePath:  "bundle-1.0.2-rc1",
					Version:     "1.0.2-rc1",
					Properties: []*api.Property{{
						Type:  registry.DeprecatedType,
						Value: "{}",
					}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			logrus.SetLevel(logrus.DebugLevel)
			db, cleanup := CreateTestDb(t)
			defer cleanup()
			store, err := NewSQLLiteLoader(db)
			require.NoError(t, err)
			err = store.Migrate(context.TODO())
			require.NoError(t, err)

			graphLoader, err := NewSQLGraphLoaderFromDB(db)
			require.NoError(t, err)

			querier := NewSQLLiteQuerierFromDb(db)

			bundleLoader := registry.BundleGraphLoader{}
			for _, b := range tt.initial {
				graph, err := graphLoader.Generate(b.pkg)
				if err != nil {
					require.EqualError(t, err, registry.ErrPackageNotInDatabase.Error(), "Expected empty error or package not in database")
				}
				bndl := newRegistryBundle(t, b)
				graph, err = bundleLoader.AddBundleToGraph(bndl, graph, &registry.AnnotationsFile{Annotations: *bndl.Annotations}, false)
				require.NoError(t, err)

				require.NoError(t, store.AddBundleSemver(graph, bndl))
			}

			err = NewSQLDeprecatorForBundles(store, querier, tt.deprecate).Deprecate()
			if tt.expectedError != nil && err != nil {
				require.EqualError(t, err, tt.expectedError.Error())
			}
			var result []*api.Bundle
			if err == nil {
				result, err = querier.ListBundles(context.TODO())
			}
			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
				return
			}
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expected, result)
		})
	}
}
