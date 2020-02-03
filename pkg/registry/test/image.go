package test

import (
	"testing"

	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func RunImageLoadSuite(t *testing.T, setup Setup) {
	logrus.SetLevel(logrus.DebugLevel)

	tests := []struct {
		description  string
		registryTest registryTest
	}{
		{"LoadsImage", loadsImage},
	}

	for _, tt := range tests {
		t.Run(tt.description, curryRegistryTest(tt.registryTest, setup))
	}
}

func loadsImage(t *testing.T, loader registry.Load, _ registry.Query) {
	image := "quay.io/test/"
	etcdFirstVersion := registry.NewImagePopulator(loader, image+"etcd.0.9.0", "../../../bundles/etcd.0.9.0", "")
	require.NoError(t, etcdFirstVersion.LoadBundleFunc())

	etcdNextVersion := registry.NewImagePopulator(loader, image+"etcd.0.9.2", "../../../bundles/etcd.0.9.2", "")
	require.NoError(t, etcdNextVersion.LoadBundleFunc())

	prometheusFirstVersion := registry.NewImagePopulator(loader, image+"prometheus.0.14.0", "../../../bundles/prometheus.0.14.0", "")
	require.NoError(t, prometheusFirstVersion.LoadBundleFunc())

	prometheusSecondVersion := registry.NewImagePopulator(loader, image+"prometheus.0.15.0", "../../../bundles/prometheus.0.15.0", "")
	require.NoError(t, prometheusSecondVersion.LoadBundleFunc())

	prometheusThirdVersion := registry.NewImagePopulator(loader, image+"prometheus.0.22.2", "../../../bundles/prometheus.0.22.2", "")
	require.NoError(t, prometheusThirdVersion.LoadBundleFunc())
}
