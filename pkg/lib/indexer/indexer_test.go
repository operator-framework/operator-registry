package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
	"github.com/stretchr/testify/require"

	pregistry "github.com/operator-framework/operator-registry/pkg/registry"
)

func CreateTestDb(t *testing.T) (*sql.DB, func(), error) {
	dir, err := ioutil.TempDir(".", "tempdb-*")
	if err != nil {
		return nil, nil, err
	}
	dbName := dir + "/test.db"

	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		os.RemoveAll(dir)
		return nil, nil, err
	}

	return db, func() {
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Fatal(err)
			}
		}()
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}, nil
}

func testGetBundlesToExportAcrossChannels(t *testing.T) {
	dbContents := []struct {
		pkg            string
		mode           pregistry.Mode
		img            string
		name           string
		version        string
		channels       []string
		csv            string
		defaultChannel string
	}{
		{
			pkg:            "depth",
			mode:           pregistry.SemVerMode,
			name:           "depth.1.0.0",
			channels:       []string{"alpha", "stable"},
			defaultChannel: "stable",
			img:            "depth:1.0.0",
			csv:            `{"kind":"ClusterServiceVersion","metadata":{"name":"depth.1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"version":"1.0.0"}}`,
		},
		{
			pkg:            "depth",
			mode:           pregistry.ReplacesMode,
			name:           "depth.1.1.0",
			channels:       []string{"alpha", "stable"},
			defaultChannel: "stable",
			img:            "depth:1.1.0",
			csv:            `{"kind":"ClusterServiceVersion","metadata":{"name":"depth.1.1.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"replaces":"depth.1.0.0","skips":["depth.1.0.1"],"version":"1.1.0"}}`,
		},
		{
			pkg:            "depth",
			mode:           pregistry.SemVerMode,
			name:           "depth.1.0.2",
			channels:       []string{"alpha"},
			defaultChannel: "alpha",
			img:            "depth:1.0.2",
			csv:            `{"kind":"ClusterServiceVersion","metadata":{"name":"depth.1.0.2"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"skips":["depth.1.0.1"],"version":"1.0.2"}}`,
		},
		{
			pkg:            "semver",
			mode:           pregistry.SemVerMode,
			name:           "semver.1.0.0",
			channels:       []string{"stable"},
			defaultChannel: "stable",
			img:            "semver:1.0.0",
			csv:            `{"kind":"ClusterServiceVersion","metadata":{"name":"semver.1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"version":"1.0.0"}}`,
		},
		{
			pkg:            "semver",
			mode:           pregistry.ReplacesMode,
			name:           "semver.1.1.0",
			channels:       []string{"alpha", "stable"},
			defaultChannel: "stable",
			img:            "semver:1.1.0",
			csv:            `{"kind":"ClusterServiceVersion","metadata":{"name":"semver.1.1.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"replaces":"semver.1.0.0","skips":["semver.1.0.1"],"version":"1.1.0"}}`,
		},
		{
			pkg:            "semver",
			mode:           pregistry.SemVerMode,
			name:           "semver.1.0.2",
			channels:       []string{"alpha"},
			defaultChannel: "alpha",
			img:            "semver:1.0.2",
			csv:            `{"kind":"ClusterServiceVersion","metadata":{"name":"semver.1.0.2"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"version":"1.0.2"}}`,
		},
		{
			pkg:            "multiplereplaces",
			mode:           pregistry.SemVerMode,
			name:           "multiplereplaces.1.0.0",
			channels:       []string{"stable"},
			defaultChannel: "stable",
			img:            "multiplereplaces:1.0.0",
			csv:            `{"kind":"ClusterServiceVersion","metadata":{"name":"multiplereplaces.1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"version":"1.0.0"}}`,
		},
		{
			pkg:            "multiplereplaces",
			mode:           pregistry.SemVerMode,
			name:           "multiplereplaces.1.0.1",
			channels:       []string{"alpha", "stable"},
			defaultChannel: "stable",
			img:            "multiplereplaces:1.0.1",
			csv:            `{"kind":"ClusterServiceVersion","metadata":{"name":"multiplereplaces.1.0.1"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"version":"1.0.1"}}`,
		},
		{
			pkg:            "multiplereplaces",
			mode:           pregistry.SemVerMode,
			name:           "badbundle.1.0.0+alpha",
			channels:       []string{"alpha"},
			defaultChannel: "stable",
			img:            "badbundle:1.0.0.alpha",
			csv:            `{"kind":"ClusterServiceVersion","metadata":{"name":"badbundle.1.0.0+alpha"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"version":"1.0.0+alpha"}}`,
		},
		{
			pkg:            "duplicatecsv",
			mode:           pregistry.ReplacesMode,
			name:           "duplicatecsv.1.0.0",
			channels:       []string{"alpha"},
			defaultChannel: "alpha",
			img:            "duplicatecsv:1.0.0+alpha",
			csv:            `{"kind":"ClusterServiceVersion","metadata":{"name":"duplicatecsv.1.0.0+alpha"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"version":"1.0.0"}}`,
		},
		{
			pkg:            "duplicatecsv",
			mode:           pregistry.ReplacesMode,
			name:           "duplicatecsv.1.0.0",
			channels:       []string{"stable"},
			defaultChannel: "stable",
			img:            "duplicatecsv:1.0.0",
			csv:            `{"kind":"ClusterServiceVersion","metadata":{"name":"duplicatecsv.1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"version":"1.0.0"}}`,
		},
	}
	db, cleanup, err := CreateTestDb(t)
	store, err := sqlite.NewSQLLiteLoader(db)
	require.NoError(t, err)
	err = store.Migrate(context.TODO())
	require.NoError(t, err)
	graphLoader, err := sqlite.NewSQLGraphLoaderFromDB(db)
	require.NoError(t, err)
	bundleLoader := pregistry.BundleGraphLoader{}

	currentCSVs := make(map[string]map[string]string)
	for _, b := range dbContents {
		sort.Strings(b.channels)
		channels := strings.Join(b.channels, ",")
		bundle, err := pregistry.NewBundleFromStrings(b.name, b.version, b.pkg, b.defaultChannel, channels, b.csv)
		require.NoError(t, err)
		bundle.BundleImage = b.img
		var skippatch bool
		switch b.mode {
		case pregistry.ReplacesMode:
			bcsv, err := bundle.ClusterServiceVersion()
			require.NoError(t, err)
			if _, ok := currentCSVs[b.pkg]; !ok {
				currentCSVs[b.pkg] = make(map[string]string)
			}
			for _, c := range b.channels {
				currentCSVs[b.pkg][c] = bcsv.Name
			}
			channels := make([]pregistry.PackageChannel, 0)
			for c, csv := range currentCSVs[b.pkg] {
				channels = append(channels, pregistry.PackageChannel{
					Name:           c,
					CurrentCSVName: csv,
				})
			}
			manifest := pregistry.PackageManifest{
				PackageName:        b.pkg,
				Channels:           channels,
				DefaultChannelName: b.defaultChannel,
			}
			require.NoError(t, store.AddBundlePackageChannels(manifest, bundle))
		case pregistry.SkipPatchMode:
			skippatch = true
			fallthrough
		case pregistry.SemVerMode:
			graph, err := graphLoader.Generate(b.pkg)
			if err != nil && !errors.Is(err, pregistry.ErrPackageNotInDatabase) {
				require.Error(t, err)
			}
			annotations := &pregistry.AnnotationsFile{
				Annotations: pregistry.Annotations{
					PackageName:        b.pkg,
					Channels:           strings.Join(b.channels, ","),
					DefaultChannelName: b.defaultChannel,
				},
			}
			updatedGraph, err := bundleLoader.AddBundleToGraph(bundle, graph, annotations, skippatch)
			require.NoError(t, err)

			require.NoError(t, store.AddBundleSemver(updatedGraph, bundle))
		}
	}
	require.NoError(t, err)
	defer cleanup()
	tests := []struct {
		name    string
		pkg     []string
		want    map[bundleDirPrefix]bundleExportInfo
		wantErr string
	}{
		{
			name: "choose replaces highest in replaces chain",
			pkg:  []string{"depth"},
			want: map[bundleDirPrefix]bundleExportInfo{
				{"depth", "1.0.0"}: {
					name:       "depth.1.0.0",
					bundlePath: "depth:1.0.0",
				},
				{"depth", "1.0.2"}: {
					name:       "depth.1.0.2",
					bundlePath: "depth:1.0.2",
					replaces:   "depth.1.0.0",
					skips:      map[string]struct{}{},
				},
				{"depth", "1.1.0"}: {
					name:       "depth.1.1.0",
					bundlePath: "depth:1.1.0",
					replaces:   "depth.1.0.2",
					skips:      map[string]struct{}{"depth.1.0.1": {}, "depth.1.0.0": {}},
				},
			},
		},
		{
			name: "choose replaces with greater semver",
			pkg:  []string{"semver"},
			want: map[bundleDirPrefix]bundleExportInfo{
				{"semver", "1.0.0"}: {
					name:       "semver.1.0.0",
					bundlePath: "semver:1.0.0",
				},
				{"semver", "1.0.2"}: {
					name:       "semver.1.0.2",
					bundlePath: "semver:1.0.2",
					replaces:   "semver.1.0.0",
					skips:      map[string]struct{}{},
				},
				{"semver", "1.1.0"}: {
					name:       "semver.1.1.0",
					bundlePath: "semver:1.1.0",
					replaces:   "semver.1.0.2",
					skips:      map[string]struct{}{"semver.1.0.1": {}, "semver.1.0.0": {}},
				},
			},
		},
		{
			name:    "reject bundle with multiple replaces",
			pkg:     []string{"multiplereplaces"},
			wantErr: "multiple replaces on package multiplereplaces, bundle version multiplereplaces.1.0.1",
		},
		{
			name:    "reject bundle with multiple CSVs",
			pkg:     []string{"duplicatecsv"},
			wantErr: "cannot export bundle version 1.0.0 for package duplicatecsv: multiple CSVs found",
		},
	}
	dbQuerier := sqlite.NewSQLLiteQuerierFromDb(db)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundleMap, err := getBundlesToExport(dbQuerier, tt.pkg)
			if len(tt.wantErr) != 0 {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			require.EqualValues(t, fmt.Sprintf("%+v", tt.want), fmt.Sprintf("%+v", bundleMap))
		})
	}
}

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
	for _, b := range bundleMap {
		bundleImages = append(bundleImages, b.bundlePath)
	}

	sort.Strings(bundleImages)

	if !reflect.DeepEqual(expected, bundleImages) {
		t.Fatalf("exporting images: expected matching bundlepaths: expected %s got %s", expected, bundleImages)
	}

	testGetBundlesToExportAcrossChannels(t)
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

func TestGetSkipsReplaces(t *testing.T) {
	tests := []struct {
		name         string
		csv          bundleExportInfo
		idx          bundleExportInfo
		wantReplaces string
		wantSkips    []string
	}{
		{
			name: "merge skips list",
			csv: bundleExportInfo{
				skips: map[string]struct{}{
					"0.0.1": {},
					"0.0.2": {},
				},
				replaces: "1.0.0",
			},
			idx: bundleExportInfo{
				skips: map[string]struct{}{
					"0.0.2": {},
					"0.0.3": {},
				},
				replaces: "",
			},
			wantReplaces: "1.0.0",
			wantSkips:    []string{"0.0.1", "0.0.2", "0.0.3"},
		},
		{
			name: "do not update replaces if present on csv skips",
			csv: bundleExportInfo{
				skips: map[string]struct{}{
					"0.0.1": {},
					"0.0.2": {},
					"1.0.1": {},
				},
				replaces: "",
			},
			idx: bundleExportInfo{
				skips: map[string]struct{}{
					"0.0.2": {},
					"0.0.3": {},
				},
				replaces: "1.0.1",
			},
			wantReplaces: "",
			wantSkips:    []string{"0.0.1", "0.0.2", "0.0.3", "1.0.1"},
		},
		{
			name: "do not add csv replaces to skips if alternate replaces not specified",
			csv: bundleExportInfo{
				skips: map[string]struct{}{
					"0.0.1": {},
					"0.0.2": {},
				},
				replaces: "1.0.0",
			},
			idx: bundleExportInfo{
				skips: map[string]struct{}{
					"0.0.2": {},
					"0.0.3": {},
					"1.0.0": {},
				},
				replaces: "",
			},
			wantReplaces: "1.0.0",
			wantSkips:    []string{"0.0.1", "0.0.2", "0.0.3"},
		},
		{
			name: "add csv replaces to skips if alternate replaces specified",
			csv: bundleExportInfo{
				skips: map[string]struct{}{
					"0.0.1": {},
					"0.0.2": {},
				},
				replaces: "1.0.0",
			},
			idx: bundleExportInfo{
				skips: map[string]struct{}{
					"0.0.2": {},
					"0.0.3": {},
				},
				replaces: "1.0.1",
			},
			wantReplaces: "1.0.1",
			wantSkips:    []string{"0.0.1", "0.0.2", "0.0.3", "1.0.0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skips, replaces := getSkipsReplaces(tt.csv, tt.idx)
			require.EqualValues(t, tt.wantReplaces, replaces)
			require.EqualValues(t, tt.wantSkips, skips)
		})
	}
}

func TestEnsureCSVFields(t *testing.T) {
	tests := []struct {
		name   string
		values bundleExportInfo
		csv    string
		want   string
	}{
		{
			name: "don't add replaces when it is already an existing skips entry",
			values: bundleExportInfo{
				replaces: "v0.0.1",
			},
			csv:  `{"kind":"ClusterServiceVersion","metadata":{"name":"testoperator.v1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"skips":["v0.0.0","v0.0.1","v0.0.1-1"],"version":"v1.0.0"}}`,
			want: `{"kind":"ClusterServiceVersion","metadata":{"name":"testoperator.v1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"skips":["v0.0.0","v0.0.1","v0.0.1-1"],"version":"v1.0.0"}}`,
		},
		{
			name: "add new replaces entry",
			values: bundleExportInfo{
				replaces: "v0.0.2",
			},
			csv:  `{"kind":"ClusterServiceVersion","metadata":{"name":"testoperator.v1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"skips":["v0.0.0","v0.0.1","v0.0.1-1"],"version":"v1.0.0"}}`,
			want: `{"kind":"ClusterServiceVersion","metadata":{"name":"testoperator.v1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"replaces":"v0.0.2","skips":["v0.0.0","v0.0.1","v0.0.1-1"],"version":"v1.0.0"}}`,
		},
		{
			name: "update replaces entry",
			values: bundleExportInfo{
				replaces: "v0.0.3",
			},
			csv:  `{"kind":"ClusterServiceVersion","metadata":{"name":"testoperator.v1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"replaces":"v0.0.2","skips":["v0.0.0","v0.0.1","v0.0.1-1"],"version":"v1.0.0"}}`,
			want: `{"kind":"ClusterServiceVersion","metadata":{"name":"testoperator.v1.0.0"},"spec":{"installModes":[{"supported":true,"type":"OwnNamespace"}],"replaces":"v0.0.3","skips":["v0.0.0","v0.0.1","v0.0.1-1","v0.0.2"],"version":"v1.0.0"}}`,
		},
		{
			name: "update skips list",
			values: bundleExportInfo{
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
			dir, err := ioutil.TempDir(".", "testoperator-")
			defer os.RemoveAll(dir)
			require.NoError(t, err)

			csvFile := path.Join(dir, "testoperator.csv")
			w, err := os.OpenFile(csvFile, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
			defer w.Close()
			require.NoError(t, err)
			_, err = w.Write([]byte(tt.csv))
			require.NoError(t, err)

			err = ensureCSVFields(dir, tt.values)
			require.NoError(t, err)

			r, err := os.Open(csvFile)
			defer r.Close()
			require.NoError(t, err)

			actual, err := ioutil.ReadAll(r)
			require.NoError(t, err)

			require.EqualValues(t, tt.want, string(actual))
		})
	}
}

func TestUpdateJSONOrYAMLFile(t *testing.T) {
	type testData struct {
		Foo *time.Time `json:"foo,omitempty"`
		Bar time.Time  `json:"bar,omitempty"`
	}
	t1 := time.Date(2009, time.February, 13, 18, 31, 30, 0, time.UTC)
	tests := []struct {
		name string
		old  string
		want string
		data interface{}
	}{
		{
			name: "update json file",
			old:  `{"foo":"2020-12-31T23:59:59Z","bar":"2020-12-31T23:59:59Z"}`,
			want: `{"foo":"2009-02-13T18:31:30Z","bar":"0001-01-01T00:00:00Z"}`,
			data: testData{
				Foo: &t1,
			},
		},
		{
			name: "update yaml file",
			old:  "foo: \"2020-12-31T23:59:59Z\"\nbar:\"2020-12-31T23:59:59Z\"",
			want: "bar: \"2009-02-13T18:31:30Z\"\n",
			data: testData{
				Bar: t1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := ioutil.TempFile(".", "test")
			file := f.Name()
			defer func() {
				f.Close()
				os.RemoveAll(file)
			}()
			require.NoError(t, err)
			_, err = f.Write([]byte(tt.old))
			require.NoError(t, err)
			f.Close()

			err = updateJSONOrYAMLFile(file, tt.data)
			require.NoError(t, err)

			f2, err := os.Open(file)
			defer f2.Close()
			require.NoError(t, err)
			actual, err := ioutil.ReadAll(f2)
			require.NoError(t, err)
			require.EqualValues(t, tt.want, string(actual))
		})
	}
}
