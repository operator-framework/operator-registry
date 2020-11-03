package indexer

import (
	"database/sql"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/operator-framework/operator-registry/pkg/sqlite"

	pregistry "github.com/operator-framework/operator-registry/pkg/registry"
)

func TestGetBundlesToExport(t *testing.T) {
	expected := []string{"quay.io/olmtest/example-bundle:etcdoperator.v0.9.2", "quay.io/olmtest/example-bundle:etcdoperator.v0.9.0",
		"quay.io/olmtest/example-bundle:etcdoperator.v0.6.1"}
	sort.Strings(expected)

	db, err := sql.Open("sqlite3", "./testdata/bundles.db")
	if err != nil {
		t.Fatalf("opening db: %s", err)
	}
	defer db.Close()

	dbQuerier := sqlite.NewSQLLiteQuerierFromDb(db)
	if err != nil {
		t.Fatalf("creating querier: %s", err)
	}

	bundleMap, err := getBundlesToExport(dbQuerier, []string{"etcd"})
	if err != nil {
		t.Fatalf("exporting bundles from db: %s", err)
	}

	var bundleImages []string
	for bundlePath, _ := range bundleMap {
		bundleImages = append(bundleImages, bundlePath)
	}

	sort.Strings(bundleImages)

	if !reflect.DeepEqual(expected, bundleImages) {
		t.Fatalf("exporting images: expected matching bundlepaths: expected %s got %s", expected, bundleImages)
	}
}

func TestGeneratePackageYaml(t *testing.T) {
	db, err := sql.Open("sqlite3", "./testdata/bundles.db")
	if err != nil {
		t.Fatalf("opening db: %s", err)
	}
	defer db.Close()

	dbQuerier := sqlite.NewSQLLiteQuerierFromDb(db)
	if err != nil {
		t.Fatalf("creating querier: %s", err)
	}

	err = generatePackageYaml(dbQuerier, "etcd", ".")
	if err != nil {
		t.Fatalf("writing package.yaml: %s", err)
	}

	var expected pregistry.PackageManifest
	expectedBytes, _ := ioutil.ReadFile("./testdata/package.yaml")
	err = yaml.Unmarshal(expectedBytes, &expected)
	if err != nil {
		t.Fatalf("unmarshaling: %s", err)
	}

	var actual pregistry.PackageManifest
	actualBytes, _ := ioutil.ReadFile("./package.yaml")
	err = yaml.Unmarshal(actualBytes, &actual)
	if err != nil {
		t.Fatalf("unmarshaling: %s", err)
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("comparing package.yaml: expected #%v actual #%v", expected, actual)
	}

	_ = os.RemoveAll("./package.yaml")
}

func TestEnsureCSVFields(t *testing.T) {
	tests := []struct {
		name   string
		values bundleDirPrefix
		csv    string
		want   string
	}{
		{
			name: "don't add replaces when it is already an existing skips entry",
			values: bundleDirPrefix{
				replaces: "v0.0.1",
			},
			csv:  `{"kind":"ClusterServiceVersion","metadata":{"name":"testoperator.v1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"skips":["v0.0.0","v0.0.1","v0.0.1-1"],"version":"v1.0.0"}}`,
			want: `{"kind":"ClusterServiceVersion","metadata":{"name":"testoperator.v1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"skips":["v0.0.0","v0.0.1","v0.0.1-1"],"version":"v1.0.0"}}`,
		},
		{
			name: "add new replaces entry",
			values: bundleDirPrefix{
				replaces: "v0.0.2",
			},
			csv:  `{"kind":"ClusterServiceVersion","metadata":{"name":"testoperator.v1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"skips":["v0.0.0","v0.0.1","v0.0.1-1"],"version":"v1.0.0"}}`,
			want: `{"kind":"ClusterServiceVersion","metadata":{"name":"testoperator.v1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"replaces":"v0.0.2","skips":["v0.0.0","v0.0.1","v0.0.1-1"],"version":"v1.0.0"}}`,
		},
		{
			name: "update replaces entry",
			values: bundleDirPrefix{
				replaces: "v0.0.3",
			},
			csv:  `{"kind":"ClusterServiceVersion","metadata":{"name":"testoperator.v1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"replaces":"v0.0.2","skips":["v0.0.0","v0.0.1","v0.0.1-1"],"version":"v1.0.0"}}`,
			want: `{"kind":"ClusterServiceVersion","metadata":{"name":"testoperator.v1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"replaces":"v0.0.3","skips":["v0.0.0","v0.0.1","v0.0.1-1","v0.0.2"],"version":"v1.0.0"}}`,
		},
		{
			name: "update skips list",
			values: bundleDirPrefix{
				skips: map[string]struct{}{
					"v0.0.1": {},
					"v0.0.2": {},
					"v0.0.3": {},
				},
			},
			csv:  `{"kind":"ClusterServiceVersion","metadata":{"name":"testoperator.v0.0.1"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"replaces":"v0.0.3","skips":["v0.0.0","v0.0.2"],"version":"1.0.0"}}`,
			want: `{"kind":"ClusterServiceVersion","metadata":{"name":"testoperator.v0.0.1"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"replaces":"v0.0.3","skips":["v0.0.0","v0.0.1","v0.0.2"],"version":"1.0.0"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.OpenFile("testdata/testoperator/testoperator.csv.yaml", os.O_WRONLY|os.O_TRUNC, 0)
			require.NoError(t, err)
			_, err = f.Write([]byte(tt.csv))
			require.NoError(t, err)
			f.Close()

			err = ensureCSVFields("testdata/testoperator", tt.values)
			require.NoError(t, err)

			f, err = os.Open("testdata/testoperator/testoperator.csv.yaml")
			defer f.Close()
			require.NoError(t, err)

			actual, err := ioutil.ReadAll(f)
			require.NoError(t, err)

			require.EqualValues(t, tt.want, string(actual))
		})
	}
}
