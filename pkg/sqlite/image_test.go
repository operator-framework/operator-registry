package sqlite

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func loadWithTwoEmptyDoc(t *testing.T, store *SQLLoader, image string) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer func() {
		logrus.SetOutput(os.Stderr)
	}()

	prometheusWhitespace := &ImageLoader{
		store:     store,
		image:     image + "prometheus.0.23.1",
		directory: "../../bundles/prometheus.0.23.1_doublewhitespace",
	}
	err := prometheusWhitespace.LoadBundleFunc()
	require.Error(t, err, "couldn't find monitoring.coreos.com/v1/Alertmanager")
	require.Contains(t, buf.String(), "skipping empty yaml doc")
}

func loadWithEmptyDoc(t *testing.T, store *SQLLoader, image string) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer func() {
		logrus.SetOutput(os.Stderr)
	}()

	prometheusWhitespace := &ImageLoader{
		store:     store,
		image:     image + "prometheus.0.23.0",
		directory: "../../bundles/prometheus.0.23.0_whitespace",
	}
	require.NoError(t, prometheusWhitespace.LoadBundleFunc())
	require.Contains(t, buf.String(), "skipping empty yaml doc")
}

func TestImageLoader(t *testing.T) {
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

	loadWithEmptyDoc(t, store, image)
	loadWithTwoEmptyDoc(t, store, image)
}
