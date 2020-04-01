package sqlite

import (
	"context"
	"testing"

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

	image := "quay.io/test/"
	etcdFirstVersion := &ImageLoader{
		store:     store,
		image:     image + "etcd.0.9.0",
		directory: "../../bundles/etcd.0.9.0",
	}
	require.NoError(t, etcdFirstVersion.LoadBundleFunc())

	etcdNextVersion := &ImageLoader{
		store:     store,
		image:     image + "etcd.0.9.2",
		directory: "../../bundles/etcd.0.9.2",
	}
	require.NoError(t, etcdNextVersion.LoadBundleFunc())

	prometheusFirstVersion := &ImageLoader{
		store:     store,
		image:     image + "prometheus.0.14.0",
		directory: "../../bundles/prometheus.0.14.0",
	}
	require.NoError(t, prometheusFirstVersion.LoadBundleFunc())

	prometheusSecondVersion := &ImageLoader{
		store:     store,
		image:     image + "prometheus.0.15.0",
		directory: "../../bundles/prometheus.0.15.0",
	}
	require.NoError(t, prometheusSecondVersion.LoadBundleFunc())

	prometheusThirdVersion := &ImageLoader{
		store:     store,
		image:     image + "prometheus.0.22.2",
		directory: "../../bundles/prometheus.0.22.2",
	}
	require.NoError(t, prometheusThirdVersion.LoadBundleFunc())

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
	require.NoError(t, etcdFirstVersion.LoadBundleFunc())
	require.NoError(t, etcdNextVersion.LoadBundleFunc())
	require.NoError(t, prometheusFirstVersion.LoadBundleFunc())
	require.NoError(t, prometheusSecondVersion.LoadBundleFunc())
	require.NoError(t, prometheusThirdVersion.LoadBundleFunc())

	// apis are back
	rows, err = db.QueryContext(context.TODO(), "select * from api")
	require.NoError(t, err)
	require.True(t, rows.Next())
	require.NoError(t, rows.Close())
}
