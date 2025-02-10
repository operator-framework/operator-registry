package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestToModel(t *testing.T) {
	tmpDir := t.TempDir()
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
	require.Len(t, m, 3)

	require.Equal(t, "etcd", m["etcd"].Name)
	require.NotNil(t, m["etcd"].Icon)
	require.Equal(t, "alpha", m["etcd"].DefaultChannel.Name)
	require.Len(t, m["etcd"].Channels, 3)
	require.Len(t, m["etcd"].Channels["alpha"].Bundles, 3)
	require.Len(t, m["etcd"].Channels["beta"].Bundles, 2)
	require.Len(t, m["etcd"].Channels["stable"].Bundles, 3)

	require.Equal(t, "prometheus", m["prometheus"].Name)
	require.NotNil(t, m["prometheus"].Icon)
	require.Equal(t, "preview", m["prometheus"].DefaultChannel.Name)
	require.Len(t, m["prometheus"].Channels, 1)
	require.Len(t, m["prometheus"].Channels["preview"].Bundles, 3)

	require.Equal(t, "strimzi-kafka-operator", m["strimzi-kafka-operator"].Name)
	require.NotNil(t, m["strimzi-kafka-operator"].Icon)
	require.Equal(t, "stable", m["strimzi-kafka-operator"].DefaultChannel.Name)
	require.Len(t, m["strimzi-kafka-operator"].Channels, 3)
	require.Len(t, m["strimzi-kafka-operator"].Channels["alpha"].Bundles, 4)
	require.Len(t, m["strimzi-kafka-operator"].Channels["beta"].Bundles, 3)
	require.Len(t, m["strimzi-kafka-operator"].Channels["stable"].Bundles, 2)
}
