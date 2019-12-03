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

	etcdFirstVersion := NewSQLLoaderForImage(nil, store, image+"etcd.0.9.0", "")
	etcdFirstVersion.directory = "../../bundles/etcd.0.9.0"
	require.NoError(t, etcdFirstVersion.LoadBundleFunc())

	etcdNextVersion := NewSQLLoaderForImage(nil, store, image+"etcd.0.9.2", "")
	etcdNextVersion.directory = "../../bundles/etcd.0.9.2"
	require.NoError(t, etcdNextVersion.LoadBundleFunc())

	prometheusFirstVersion := NewSQLLoaderForImage(nil, store, image+"prometheus.0.14.0", "")
	prometheusFirstVersion.directory = "../../bundles/prometheus.0.14.0"
	require.NoError(t, prometheusFirstVersion.LoadBundleFunc())

	prometheusSecondVersion := NewSQLLoaderForImage(nil, store, image+"prometheus.0.15.0", "")
	prometheusSecondVersion.directory = "../../bundles/prometheus.0.15.0"
	require.NoError(t, prometheusSecondVersion.LoadBundleFunc())

	prometheusThirdVersion := NewSQLLoaderForImage(nil, store, image+"prometheus.0.22.2", "")
	prometheusThirdVersion.directory = "../../bundles/prometheus.0.22.2"
	require.NoError(t, prometheusThirdVersion.LoadBundleFunc())

	// delete everything
	require.NoError(t, store.RmPackageName("etcd"))
	require.NoError(t, store.RmPackageName("prometheus"))

	// and insert again
	require.NoError(t, etcdFirstVersion.LoadBundleFunc())
	require.NoError(t, etcdNextVersion.LoadBundleFunc())
	require.NoError(t, prometheusFirstVersion.LoadBundleFunc())
	require.NoError(t, prometheusSecondVersion.LoadBundleFunc())
	require.NoError(t, prometheusThirdVersion.LoadBundleFunc())
}
