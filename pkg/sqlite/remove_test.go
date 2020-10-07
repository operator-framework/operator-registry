package sqlite

import (
	"context"
	"testing"

	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/registry"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestRemover(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	db, cleanup := CreateTestDb(t)
	defer cleanup()
	store, err := NewSQLLiteLoader(db)
	require.NoError(t, err)
	require.NoError(t, store.Migrate(context.TODO()))

	query := NewSQLLiteQuerierFromDb(db)

	graphLoader, err := NewSQLGraphLoaderFromDB(db)
	require.NoError(t, err)

	populate := func(name string) error {
		return registry.NewDirectoryPopulator(
			store,
			graphLoader,
			query,
			map[image.Reference]string{
				image.SimpleReference("quay.io/test/" + name): "../../bundles/" + name,
			},
			make(map[string]map[image.Reference]string, 0), false).Populate(registry.ReplacesMode)
	}
	for _, name := range []string{"etcd.0.9.0", "etcd.0.9.2", "prometheus.0.14.0", "prometheus.0.15.0", "prometheus.0.22.2"} {
		require.NoError(t, populate(name))
	}

	// delete etcd
	require.NoError(t, store.RemovePackage("etcd"))

	querier := NewSQLLiteQuerierFromDb(db)
	_, err = querier.GetPackage(context.TODO(), "etcd")
	require.EqualError(t, err, "package etcd not found")

	// prometheus apis still around
	rows, err := db.QueryContext(context.TODO(), "select * from api")
	require.NoError(t, err)
	require.True(t, rows.Next())
	require.NoError(t, rows.Close())

	// delete prometheus
	require.NoError(t, store.RemovePackage("prometheus"))

	_, err = querier.GetPackage(context.TODO(), "prometheus")
	require.EqualError(t, err, "package prometheus not found")

	// no apis after all packages are removed
	rows, err = db.QueryContext(context.TODO(), "select * from api")
	require.NoError(t, err)
	require.False(t, rows.Next())
	require.NoError(t, rows.Close())

	// and insert again
	for _, name := range []string{"etcd.0.9.0", "etcd.0.9.2", "prometheus.0.14.0", "prometheus.0.15.0", "prometheus.0.22.2"} {
		require.NoError(t, populate(name))
	}

	// apis are back
	rows, err = db.QueryContext(context.TODO(), "select * from api")
	require.NoError(t, err)
	require.True(t, rows.Next())
	require.NoError(t, rows.Close())
}
