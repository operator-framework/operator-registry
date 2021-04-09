package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/internal/declcfg"
)

var testModelQuerier = genTestModelQuerier()

func TestQuerier_GetBundle(t *testing.T) {
	b, err := testModelQuerier.GetBundle(context.TODO(), "etcd", "singlenamespace-alpha", "etcdoperator.v0.9.4")
	require.NoError(t, err)
	require.Equal(t, b.PackageName, "etcd")
	require.Equal(t, b.ChannelName, "singlenamespace-alpha")
	require.Equal(t, b.CsvName, "etcdoperator.v0.9.4")
}

func TestQuerier_GetBundleForChannel(t *testing.T) {
	b, err := testModelQuerier.GetBundleForChannel(context.TODO(), "etcd", "singlenamespace-alpha")
	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, b.PackageName, "etcd")
	require.Equal(t, b.ChannelName, "singlenamespace-alpha")
	require.Equal(t, b.CsvName, "etcdoperator.v0.9.4")
}

func TestQuerier_GetBundleThatProvides(t *testing.T) {
	b, err := testModelQuerier.GetBundleThatProvides(context.TODO(), "etcd.database.coreos.com", "v1beta2", "EtcdBackup")
	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, b.PackageName, "etcd")
	require.Equal(t, b.ChannelName, "singlenamespace-alpha")
	require.Equal(t, b.CsvName, "etcdoperator.v0.9.4")
}

func TestQuerier_GetBundleThatReplaces(t *testing.T) {
	b, err := testModelQuerier.GetBundleThatReplaces(context.TODO(), "etcdoperator.v0.9.0", "etcd", "singlenamespace-alpha")
	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, b.PackageName, "etcd")
	require.Equal(t, b.ChannelName, "singlenamespace-alpha")
	require.Equal(t, b.CsvName, "etcdoperator.v0.9.2")
}

func TestQuerier_GetChannelEntriesThatProvide(t *testing.T) {
	entries, err := testModelQuerier.GetChannelEntriesThatProvide(context.TODO(), "etcd.database.coreos.com", "v1beta2", "EtcdBackup")
	require.NoError(t, err)
	require.NotNil(t, entries)
	require.ElementsMatch(t, []*ChannelEntry{
		{
			PackageName: "etcd",
			ChannelName: "singlenamespace-alpha",
			BundleName:  "etcdoperator.v0.9.0",
			Replaces:    "",
		},
		{
			PackageName: "etcd",
			ChannelName: "singlenamespace-alpha",
			BundleName:  "etcdoperator.v0.9.4",
			Replaces:    "etcdoperator.v0.9.2",
		},
		{
			PackageName: "etcd",
			ChannelName: "clusterwide-alpha",
			BundleName:  "etcdoperator.v0.9.0",
			Replaces:    "",
		},
		{
			PackageName: "etcd",
			ChannelName: "clusterwide-alpha",
			BundleName:  "etcdoperator.v0.9.2-clusterwide",
			Replaces:    "etcdoperator.v0.9.0",
		},
		{
			PackageName: "etcd",
			ChannelName: "clusterwide-alpha",
			BundleName:  "etcdoperator.v0.9.2-clusterwide",
			Replaces:    "etcdoperator.v0.6.1",
		},
		{
			PackageName: "etcd",
			ChannelName: "clusterwide-alpha",
			BundleName:  "etcdoperator.v0.9.4-clusterwide",
			Replaces:    "etcdoperator.v0.9.2-clusterwide",
		},
	}, entries)
}

func TestQuerier_GetChannelEntriesThatReplace(t *testing.T) {
	entries, err := testModelQuerier.GetChannelEntriesThatReplace(context.TODO(), "etcdoperator.v0.9.0")
	require.NoError(t, err)
	require.NotNil(t, entries)
	require.ElementsMatch(t, []*ChannelEntry{
		{
			PackageName: "etcd",
			ChannelName: "singlenamespace-alpha",
			BundleName:  "etcdoperator.v0.9.2",
			Replaces:    "etcdoperator.v0.9.0",
		},
		{
			PackageName: "etcd",
			ChannelName: "clusterwide-alpha",
			BundleName:  "etcdoperator.v0.9.2-clusterwide",
			Replaces:    "etcdoperator.v0.9.0",
		},
	}, entries)
}

func TestQuerier_GetLatestChannelEntriesThatProvide(t *testing.T) {
	entries, err := testModelQuerier.GetLatestChannelEntriesThatProvide(context.TODO(), "etcd.database.coreos.com", "v1beta2", "EtcdBackup")
	require.NoError(t, err)
	require.NotNil(t, entries)
	require.ElementsMatch(t, []*ChannelEntry{
		{
			PackageName: "etcd",
			ChannelName: "singlenamespace-alpha",
			BundleName:  "etcdoperator.v0.9.4",
			Replaces:    "etcdoperator.v0.9.2",
		},
		{
			PackageName: "etcd",
			ChannelName: "clusterwide-alpha",
			BundleName:  "etcdoperator.v0.9.4-clusterwide",
			Replaces:    "etcdoperator.v0.9.2-clusterwide",
		},
	}, entries)
}

func TestQuerier_GetPackage(t *testing.T) {
	p, err := testModelQuerier.GetPackage(context.TODO(), "etcd")
	require.NoError(t, err)
	require.NotNil(t, p)

	expected := &PackageManifest{
		PackageName:        "etcd",
		DefaultChannelName: "singlenamespace-alpha",
		Channels: []PackageChannel{
			{
				Name:           "singlenamespace-alpha",
				CurrentCSVName: "etcdoperator.v0.9.4",
			},
			{
				Name:           "clusterwide-alpha",
				CurrentCSVName: "etcdoperator.v0.9.4-clusterwide",
			},
			{
				Name:           "alpha",
				CurrentCSVName: "etcdoperator-community.v0.6.1",
			},
		},
	}

	require.ElementsMatch(t, expected.Channels, p.Channels)
	expected.Channels, p.Channels = nil, nil
	require.Equal(t, expected, p)
}

func TestQuerier_ListBundles(t *testing.T) {
	bundles, err := testModelQuerier.ListBundles(context.TODO())
	require.NoError(t, err)
	require.NotNil(t, bundles)
	require.Equal(t, 12, len(bundles))
}

func TestQuerier_ListPackages(t *testing.T) {
	packages, err := testModelQuerier.ListPackages(context.TODO())
	require.NoError(t, err)
	require.NotNil(t, packages)
	require.Equal(t, 2, len(packages))
}

func genTestModelQuerier() *Querier {
	cfg, err := declcfg.LoadDir("testdata/validDeclCfg")
	if err != nil {
		panic(err)
	}
	m, err := declcfg.ConvertToModel(*cfg)
	if err != nil {
		panic(err)
	}
	return NewQuerier(m)
}
