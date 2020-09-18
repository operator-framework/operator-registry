package indexer

import (
	"database/sql"
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

	bundleMap, err := getBundlesToExport(dbQuerier, []string {"etcd"})
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
