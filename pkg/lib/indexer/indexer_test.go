package indexer

import (
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/ghodss/yaml"

	pregistry "github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

func TestGetBundlesToExport(t *testing.T) {
	expected := []string{"quay.io/olmtest/example-bundle:etcdoperator.v0.9.2", "quay.io/olmtest/example-bundle:etcdoperator.v0.9.0",
		"quay.io/olmtest/example-bundle:etcdoperator.v0.6.1"}
	sort.Strings(expected)

	db, err := sqlite.Open("./testdata/bundles.db")
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
	db, err := sqlite.Open("./testdata/bundles.db")
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
	expectedBytes, _ := os.ReadFile("./testdata/package.yaml")
	err = yaml.Unmarshal(expectedBytes, &expected)
	if err != nil {
		t.Fatalf("unmarshaling: %s", err)
	}

	var actual pregistry.PackageManifest
	actualBytes, _ := os.ReadFile("./package.yaml")
	err = yaml.Unmarshal(actualBytes, &actual)
	if err != nil {
		t.Fatalf("unmarshaling: %s", err)
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("comparing package.yaml: expected #%v actual #%v", expected, actual)
	}

	_ = os.RemoveAll("./package.yaml")
}

func TestBuildContext(t *testing.T) {
	// TODO(): Test does not currently have a clean way
	// of testing the generated returned values such as
	// outDockerfile and buildDir.

	defaultBuildDirOnGenerate := "./"
	fooDockerfile := "foo.Dockerfile"
	defaultDockerfile := defaultDockerfileName

	cases := []struct {
		generate              bool
		requestedDockerfile   string
		expectedBuildDir      *string // return values not checked if nil
		expectedOutDockerfile *string // return values not checked if nil
	}{
		{
			generate:              true,
			requestedDockerfile:   "",
			expectedOutDockerfile: &defaultDockerfile,
			expectedBuildDir:      &defaultBuildDirOnGenerate,
		},
		{
			generate:              false,
			requestedDockerfile:   "foo.Dockerfile",
			expectedOutDockerfile: &fooDockerfile,
			expectedBuildDir:      nil,
		},
		{
			generate:              false,
			requestedDockerfile:   "",
			expectedOutDockerfile: nil,
			expectedBuildDir:      nil,
		},
	}

	for _, testCase := range cases {
		actualBuildDir, actualOutDockerfile, actualCleanup, _ := buildContext(
			testCase.generate, testCase.requestedDockerfile)

		if actualCleanup == nil {
			// prevent regression - cleanup should never be nil
			t.Fatal("buildContext returned nil cleanup function")
		}
		defer actualCleanup()

		if testCase.expectedOutDockerfile != nil && actualOutDockerfile != *testCase.expectedOutDockerfile {
			t.Fatalf("comparing outDockerfile: expected %v actual %v",
				*testCase.expectedOutDockerfile,
				actualOutDockerfile)
		}

		if testCase.expectedBuildDir != nil && actualBuildDir != *testCase.expectedBuildDir {
			t.Fatalf("comparing buildDir: expected %v actual %v",
				*testCase.expectedBuildDir,
				actualBuildDir)
		}
	}
}
