package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

func createLoadedTestDB(t *testing.T) (*sql.DB, func()) {
	db, cleanup := CreateTestDB(t)
	store, err := NewSQLLiteLoader(db)
	require.NoError(t, err)
	require.NoError(t, store.Migrate(context.TODO()))

	loader := NewSQLLoaderForDirectory(store, "./testdata/loader_data")
	require.NoError(t, loader.Populate())

	return db, cleanup
}

func TestLoadPackageGraph_Etcd(t *testing.T) {
	expectedGraph := &registry.Package{
		Name:           "etcd",
		DefaultChannel: "alpha",
		Channels: map[string]registry.Channel{
			"alpha": {
				Head: registry.BundleKey{BundlePath: "", Version: "0.9.2", CsvName: "etcdoperator.v0.9.2"},
				Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
					{BundlePath: "", Version: "0.6.1", CsvName: "etcdoperator.v0.6.1"}: {},
					{BundlePath: "", Version: "0.9.0", CsvName: "etcdoperator.v0.9.0"}: {
						registry.BundleKey{BundlePath: "", Version: "0.6.1", CsvName: "etcdoperator.v0.6.1"}: struct{}{},
					},
					{BundlePath: "", Version: "0.9.2", CsvName: "etcdoperator.v0.9.2"}: {
						registry.BundleKey{BundlePath: "", Version: "", CsvName: "etcdoperator.v0.9.1"}:      struct{}{},
						registry.BundleKey{BundlePath: "", Version: "0.9.0", CsvName: "etcdoperator.v0.9.0"}: struct{}{},
					},
				},
			},
			"beta": {
				Head: registry.BundleKey{BundlePath: "", Version: "0.9.0", CsvName: "etcdoperator.v0.9.0"},
				Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
					{BundlePath: "", Version: "0.6.1", CsvName: "etcdoperator.v0.6.1"}: {},
					{BundlePath: "", Version: "0.9.0", CsvName: "etcdoperator.v0.9.0"}: {
						registry.BundleKey{BundlePath: "", Version: "0.6.1", CsvName: "etcdoperator.v0.6.1"}: struct{}{},
					},
				},
			},
			"stable": {
				Head: registry.BundleKey{BundlePath: "", Version: "0.9.2", CsvName: "etcdoperator.v0.9.2"},
				Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
					{BundlePath: "", Version: "0.6.1", CsvName: "etcdoperator.v0.6.1"}: {},
					{BundlePath: "", Version: "0.9.0", CsvName: "etcdoperator.v0.9.0"}: {
						registry.BundleKey{BundlePath: "", Version: "0.6.1", CsvName: "etcdoperator.v0.6.1"}: struct{}{},
					},
					{BundlePath: "", Version: "0.9.2", CsvName: "etcdoperator.v0.9.2"}: {
						registry.BundleKey{BundlePath: "", Version: "", CsvName: "etcdoperator.v0.9.1"}:      struct{}{},
						registry.BundleKey{BundlePath: "", Version: "0.9.0", CsvName: "etcdoperator.v0.9.0"}: struct{}{},
					},
				},
			},
		},
	}

	db, cleanup := createLoadedTestDB(t)
	defer cleanup()

	graphLoader, err := NewSQLGraphLoaderFromDB(db)
	require.NoError(t, err)

	result, err := graphLoader.Generate("etcd")
	require.NoError(t, err)

	require.Equal(t, "etcd", result.Name)
	require.Len(t, result.Channels, 3)

	for channelName, channel := range result.Channels {
		expectedChannel := expectedGraph.Channels[channelName]
		require.Equal(t, expectedChannel.Head, channel.Head)
		require.EqualValues(t, expectedChannel.Nodes, channel.Nodes)
	}
}

func TestLoadPackageGraph_Etcd_NotFound(t *testing.T) {
	db, cleanup := createLoadedTestDB(t)
	defer cleanup()

	graphLoader, err := NewSQLGraphLoaderFromDB(db)
	require.NoError(t, err)

	_, err = graphLoader.Generate("not-a-real-package")
	require.Error(t, err)
	require.Equal(t, registry.ErrPackageNotInDatabase, err)
}
