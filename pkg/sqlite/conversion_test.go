package sqlite

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestToModel(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "server_test-")
	if err != nil {
		logrus.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		logrus.Fatal(err)
	}
	load, err := NewSQLLiteLoader(db)
	if err != nil {
		logrus.Fatal(err)
	}
	if err := load.Migrate(context.TODO()); err != nil {
		logrus.Fatal(err)
	}

	loader := NewSQLLoaderForDirectory(load, "../../manifests")
	if err := loader.Populate(); err != nil {
		logrus.Fatal(err)
	}
	if err := db.Close(); err != nil {
		logrus.Fatal(err)
	}
	store, err := NewSQLLiteQuerier(dbPath)
	if err != nil {
		logrus.Fatal(err)
	}

	m, err := ToModel(context.TODO(), store)
	require.NoError(t, err)
	require.NotNil(t, m)
	require.NoError(t, m.Validate())
	require.Equal(t, 3, len(m))

	require.Equal(t, "etcd", m["etcd"].Name)
	require.NotNil(t, m["etcd"].Icon)
	require.Equal(t, "alpha", m["etcd"].DefaultChannel.Name)
	require.Equal(t, 3, len(m["etcd"].Channels))
	require.Equal(t, 3, len(m["etcd"].Channels["alpha"].Bundles))
	require.Equal(t, 2, len(m["etcd"].Channels["beta"].Bundles))
	require.Equal(t, 3, len(m["etcd"].Channels["stable"].Bundles))

	require.Equal(t, "prometheus", m["prometheus"].Name)
	require.NotNil(t, m["prometheus"].Icon)
	require.Equal(t, "preview", m["prometheus"].DefaultChannel.Name)
	require.Equal(t, 1, len(m["prometheus"].Channels))
	require.Equal(t, 3, len(m["prometheus"].Channels["preview"].Bundles))

	require.Equal(t, "strimzi-kafka-operator", m["strimzi-kafka-operator"].Name)
	require.NotNil(t, m["strimzi-kafka-operator"].Icon)
	require.Equal(t, "stable", m["strimzi-kafka-operator"].DefaultChannel.Name)
	require.Equal(t, 3, len(m["strimzi-kafka-operator"].Channels))
	require.Equal(t, 4, len(m["strimzi-kafka-operator"].Channels["alpha"].Bundles))
	require.Equal(t, 3, len(m["strimzi-kafka-operator"].Channels["beta"].Bundles))
	require.Equal(t, 2, len(m["strimzi-kafka-operator"].Channels["stable"].Bundles))
}
