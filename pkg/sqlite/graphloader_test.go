package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func createLoadedTestDb(t *testing.T) (*sql.DB, func()) {
	dbName := fmt.Sprintf("test-%d.db", rand.Int())

	db, err := sql.Open("sqlite3", dbName)
	require.NoError(t, err)

	dbLoader, err := NewSQLLiteLoader(db)
	require.NoError(t, err)

	err = dbLoader.Migrate(context.TODO())
	require.NoError(t, err)

	loader := NewSQLLoaderForDirectory(dbLoader, "./testdata/loader_data")
	err = loader.Populate()
	require.NoError(t, err)

	return db, func() {
		defer func() {
			if err := os.Remove(dbName); err != nil {
				t.Fatal(err)
			}
		}()
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadPackageGraph_Etcd(t *testing.T) {
	expectedGraph := &registry.Package{
		Name:           "etcd",
		DefaultChannel: "alpha",
		Channels: map[string]registry.Channel{
			"alpha": {
				Head: registry.BundleKey{BundlePath: "", Version: "0.9.2", CsvName: "etcdoperator.v0.9.2"},
				Replaces: map[registry.BundleKey]map[registry.BundleKey]struct{}{
					registry.BundleKey{BundlePath: "", Version: "", CsvName: "etcdoperator.v0.9.1"}:      {},
					registry.BundleKey{BundlePath: "", Version: "0.6.1", CsvName: "etcdoperator.v0.6.1"}: {},
					registry.BundleKey{BundlePath: "", Version: "0.9.0", CsvName: "etcdoperator.v0.9.0"}: {
						registry.BundleKey{BundlePath: "", Version: "0.6.1", CsvName: "etcdoperator.v0.6.1"}: struct{}{},
					},
					registry.BundleKey{BundlePath: "", Version: "0.9.2", CsvName: "etcdoperator.v0.9.2"}: {
						registry.BundleKey{BundlePath: "", Version: "", CsvName: "etcdoperator.v0.9.1"}:      struct{}{},
						registry.BundleKey{BundlePath: "", Version: "0.9.0", CsvName: "etcdoperator.v0.9.0"}: struct{}{},
					},
				},
			},
			"beta": {
				Head: registry.BundleKey{BundlePath: "", Version: "0.9.0", CsvName: "etcdoperator.v0.9.0"},
				Replaces: map[registry.BundleKey]map[registry.BundleKey]struct{}{
					registry.BundleKey{BundlePath: "", Version: "0.6.1", CsvName: "etcdoperator.v0.6.1"}: {},
					registry.BundleKey{BundlePath: "", Version: "0.9.0", CsvName: "etcdoperator.v0.9.0"}: {
						registry.BundleKey{BundlePath: "", Version: "0.6.1", CsvName: "etcdoperator.v0.6.1"}: struct{}{},
					},
				},
			},
			"stable": {
				Head: registry.BundleKey{BundlePath: "", Version: "0.9.2", CsvName: "etcdoperator.v0.9.2"},
				Replaces: map[registry.BundleKey]map[registry.BundleKey]struct{}{
					registry.BundleKey{BundlePath: "", Version: "", CsvName: "etcdoperator.v0.9.1"}:      {},
					registry.BundleKey{BundlePath: "", Version: "0.6.1", CsvName: "etcdoperator.v0.6.1"}: {},
					registry.BundleKey{BundlePath: "", Version: "0.9.0", CsvName: "etcdoperator.v0.9.0"}: {
						registry.BundleKey{BundlePath: "", Version: "0.6.1", CsvName: "etcdoperator.v0.6.1"}: struct{}{},
					},
					registry.BundleKey{BundlePath: "", Version: "0.9.2", CsvName: "etcdoperator.v0.9.2"}: {
						registry.BundleKey{BundlePath: "", Version: "", CsvName: "etcdoperator.v0.9.1"}:      struct{}{},
						registry.BundleKey{BundlePath: "", Version: "0.9.0", CsvName: "etcdoperator.v0.9.0"}: struct{}{},
					},
				},
			},
		},
	}

	db, cleanup := createLoadedTestDb(t)
	defer cleanup()

	graphLoader, err := NewSQLGraphLoaderFromDB(db, "etcd")
	require.NoError(t, err)

	result, err := graphLoader.Generate()
	require.NoError(t, err)

	require.Equal(t, "etcd", result.Name)
	require.Equal(t, 3, len(result.Channels))

	for channelName, channel := range result.Channels {
		expectedChannel := expectedGraph.Channels[channelName]
		require.Equal(t, expectedChannel.Head, channel.Head)
		require.EqualValues(t, expectedChannel.Replaces, channel.Replaces)
	}
}
