package registry

import (
	"context"
	"testing"
	"testing/fstest"

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
	cfg, err := declcfg.LoadFS(validFS)
	if err != nil {
		panic(err)
	}
	m, err := declcfg.ConvertToModel(*cfg)
	if err != nil {
		panic(err)
	}
	return NewQuerier(m)
}

var validFS = fstest.MapFS{
	"cockroachdb.json": &fstest.MapFile{
		Data: []byte(`{
    "schema": "olm.package",
    "name": "cockroachdb",
    "defaultChannel": "stable-5.x",
    "icon": {
        "base64data": "PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAzMS44MiAzMiIgd2lkdGg9IjI0ODYiIGhlaWdodD0iMjUwMCI+PHRpdGxlPkNMPC90aXRsZT48cGF0aCBkPSJNMTkuNDIgOS4xN2ExNS4zOSAxNS4zOSAwIDAgMS0zLjUxLjQgMTUuNDYgMTUuNDYgMCAwIDEtMy41MS0uNCAxNS42MyAxNS42MyAwIDAgMSAzLjUxLTMuOTEgMTUuNzEgMTUuNzEgMCAwIDEgMy41MSAzLjkxek0zMCAuNTdBMTcuMjIgMTcuMjIgMCAwIDAgMjUuNTkgMGExNy40IDE3LjQgMCAwIDAtOS42OCAyLjkzQTE3LjM4IDE3LjM4IDAgMCAwIDYuMjMgMGExNy4yMiAxNy4yMiAwIDAgMC00LjQ0LjU3QTE2LjIyIDE2LjIyIDAgMCAwIDAgMS4xM2EuMDcuMDcgMCAwIDAgMCAuMDkgMTcuMzIgMTcuMzIgMCAwIDAgLjgzIDEuNTcuMDcuMDcgMCAwIDAgLjA4IDAgMTYuMzkgMTYuMzkgMCAwIDEgMS44MS0uNTQgMTUuNjUgMTUuNjUgMCAwIDEgMTEuNTkgMS44OCAxNy41MiAxNy41MiAwIDAgMC0zLjc4IDQuNDhjLS4yLjMyLS4zNy42NS0uNTUgMXMtLjIyLjQ1LS4zMy42OS0uMzEuNzItLjQ0IDEuMDhhMTcuNDYgMTcuNDYgMCAwIDAgNC4yOSAxOC43Yy4yNi4yNS41My40OS44MS43M3MuNDQuMzcuNjcuNTQuNTkuNDQuODkuNjRhLjA3LjA3IDAgMCAwIC4wOCAwYy4zLS4yMS42LS40Mi44OS0uNjRzLjQ1LS4zNS42Ny0uNTQuNTUtLjQ4LjgxLS43M2ExNy40NSAxNy40NSAwIDAgMCA1LjM4LTEyLjYxIDE3LjM5IDE3LjM5IDAgMCAwLTEuMDktNi4wOWMtLjE0LS4zNy0uMjktLjczLS40NS0xLjA5cy0uMjItLjQ3LS4zMy0uNjktLjM1LS42Ni0uNTUtMWExNy42MSAxNy42MSAwIDAgMC0zLjc4LTQuNDggMTUuNjUgMTUuNjUgMCAwIDEgMTEuNi0xLjg0IDE2LjEzIDE2LjEzIDAgMCAxIDEuODEuNTQuMDcuMDcgMCAwIDAgLjA4IDBxLjQ0LS43Ni44Mi0xLjU2YS4wNy4wNyAwIDAgMCAwLS4wOUExNi44OSAxNi44OSAwIDAgMCAzMCAuNTd6IiBmaWxsPSIjMTUxZjM0Ii8+PHBhdGggZD0iTTIxLjgyIDE3LjQ3YTE1LjUxIDE1LjUxIDAgMCAxLTQuMjUgMTAuNjkgMTUuNjYgMTUuNjYgMCAwIDEtLjcyLTQuNjggMTUuNSAxNS41IDAgMCAxIDQuMjUtMTAuNjkgMTUuNjIgMTUuNjIgMCAwIDEgLjcyIDQuNjgiIGZpbGw9IiMzNDg1NDAiLz48cGF0aCBkPSJNMTUgMjMuNDhhMTUuNTUgMTUuNTUgMCAwIDEtLjcyIDQuNjggMTUuNTQgMTUuNTQgMCAwIDEtMy41My0xNS4zN0ExNS41IDE1LjUgMCAwIDEgMTUgMjMuNDgiIGZpbGw9IiM3ZGJjNDIiLz48L3N2Zz4=",
        "mediatype": "image/svg+xml"
    }
}
{
    "schema": "olm.bundle",
    "name": "cockroachdb.v2.0.9",
    "package": "cockroachdb",
    "image": "quay.io/openshift-community-operators/cockroachdb:v2.0.9",
    "properties": [
        {
            "type": "olm.channel",
            "value": {
                "name": "stable"
            }
        },
        {
            "type": "olm.package",
            "value": {
                "packageName": "cockroachdb",
                "version": "2.0.9"
            }
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "cockroachdb.v2.1.11",
    "package": "cockroachdb",
    "image": "quay.io/openshift-community-operators/cockroachdb:v2.1.11",
    "properties": [
        {
            "type": "olm.channel",
            "value": {
                "name": "stable",
                "replaces": "cockroachdb.v2.1.1"
            }
        },
        {
            "type": "olm.package",
            "value": {
                "packageName": "cockroachdb",
                "version": "2.1.11"
            }
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "cockroachdb.v2.1.1",
    "package": "cockroachdb",
    "image": "quay.io/openshift-community-operators/cockroachdb:v2.1.1",
    "properties": [
        {
            "type": "olm.channel",
            "value": {
                "name": "stable",
                "replaces": "cockroachdb.v2.0.9"
            }
        },
        {
            "type": "olm.package",
            "value": {
                "packageName": "cockroachdb",
                "version": "2.1.1"
            }
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "cockroachdb.v3.0.7",
    "package": "cockroachdb",
    "image": "quay.io/openshift-community-operators/cockroachdb:v3.0.7",
    "properties": [
        {
            "type": "olm.channel",
            "value": {
                "name": "stable-3.x"
            }
        },
        {
            "type": "olm.package",
            "value": {
                "packageName": "cockroachdb",
                "version": "3.0.7"
            }
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "cockroachdb.v5.0.3",
    "package": "cockroachdb",
    "image": "quay.io/openshift-community-operators/cockroachdb:v5.0.3",
    "properties": [
        {
            "type": "olm.channel",
            "value": {
                "name": "stable-5.x"
            }
        },
        {
            "type": "olm.package",
            "value": {
                "packageName": "cockroachdb",
                "version": "5.0.3"
            }
        }
    ]
}`),
	},
	"etcd.json": &fstest.MapFile{
		Data: []byte(`{
    "schema": "olm.package",
    "name": "etcd",
    "defaultChannel": "singlenamespace-alpha",
    "icon": {
        "base64data": "PHN2ZyB3aWR0aD0iMjUwMCIgaGVpZ2h0PSIyNDIyIiB2aWV3Qm94PSIwIDAgMjU2IDI0OCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIiBwcmVzZXJ2ZUFzcGVjdFJhdGlvPSJ4TWlkWU1pZCI+PHBhdGggZD0iTTI1Mi4zODYgMTI4LjA2NGMtMS4yMDIuMS0yLjQxLjE0Ny0zLjY5My4xNDctNy40NDYgMC0xNC42Ny0xLjc0Ni0yMS4xODctNC45NDQgMi4xNy0xMi40NDcgMy4wOTItMjQuOTg3IDIuODUtMzcuNDgxLTcuMDY1LTEwLjIyLTE1LjE0LTE5Ljg2My0yNC4yNTYtMjguNzQ3IDMuOTU1LTcuNDE1IDkuODAxLTEzLjc5NSAxNy4xLTE4LjMxOWwzLjEzMy0xLjkzNy0yLjQ0Mi0yLjc1NGMtMTIuNTgxLTE0LjE2Ny0yNy41OTYtMjUuMTItNDQuNjItMzIuNTUyTDE3NS44NzYgMGwtLjg2MiAzLjU4OGMtMi4wMyA4LjM2My02LjI3NCAxNS45MDgtMTIuMSAyMS45NjJhMTkzLjg0MiAxOTMuODQyIDAgMCAwLTM0Ljk1Ni0xNC40MDVBMTk0LjAxMiAxOTQuMDEyIDAgMCAwIDkzLjA1NiAyNS41MkM4Ny4yNTQgMTkuNDczIDgzLjAyIDExLjk0NyA4MC45OTkgMy42MDhMODAuMTMuMDJsLTMuMzgyIDEuNDdDNTkuOTM5IDguODE1IDQ0LjUxIDIwLjA2NSAzMi4xMzUgMzQuMDJsLTIuNDQ5IDIuNzYgMy4xMyAxLjkzN2M3LjI3NiA0LjUwNiAxMy4xMDYgMTAuODQ5IDE3LjA1NCAxOC4yMjMtOS4wODggOC44NS0xNy4xNTQgMTguNDYyLTI0LjIxNCAyOC42MzUtLjI3NSAxMi40ODkuNiAyNS4xMiAyLjc4IDM3Ljc0LTYuNDg0IDMuMTY3LTEzLjY2OCA0Ljg5NC0yMS4wNjUgNC44OTQtMS4yOTggMC0yLjUxMy0uMDQ3LTMuNjkzLS4xNDVMMCAxMjcuNzg1bC4zNDUgMy42NzFjMS44MDIgMTguNTc4IDcuNTcgMzYuMjQ3IDE3LjE1NCA1Mi41MjNsMS44NyAzLjE3NiAyLjgxLTIuMzg0YTQ4LjA0IDQ4LjA0IDAgMCAxIDIyLjczNy0xMC42NSAxOTQuODYgMTk0Ljg2IDAgMCAwIDE5LjQ2IDMxLjY5NmMxMS44MjggNC4xMzcgMjQuMTUxIDcuMjI1IDM2Ljg3OCA5LjA2MyAxLjIyIDguNDE3LjI0OCAxNy4xMjItMy4wNzIgMjUuMTcxbC0xLjQgMy40MTEgMy42Ljc5M2M5LjIyIDIuMDI3IDE4LjUyMyAzLjA2IDI3LjYzMSAzLjA2bDI3LjYyMy0zLjA2IDMuNjA0LS43OTMtMS40MDMtMy40MTdjLTMuMzEyLTguMDUtNC4yODQtMTYuNzY1LTMuMDYzLTI1LjE4MyAxMi42NzYtMS44NCAyNC45NTQtNC45MiAzNi43MzgtOS4wNDVhMTk1LjEwOCAxOTUuMTA4IDAgMCAwIDE5LjQ4Mi0zMS43MjYgNDguMjU0IDQ4LjI1NCAwIDAgMSAyMi44NDggMTAuNjZsMi44MDkgMi4zOCAxLjg2Mi0zLjE2OGM5LjYtMTYuMjk3IDE1LjM2OC0zMy45NjUgMTcuMTQyLTUyLjUxM2wuMzQ1LTMuNjY1LTMuNjE0LjI3OXpNMTY3LjQ5IDE3Mi45NmMtMTMuMDY4IDMuNTU0LTI2LjM0IDUuMzQ4LTM5LjUzMiA1LjM0OC0xMy4yMjggMC0yNi40ODMtMS43OTMtMzkuNTYzLTUuMzQ4YTE1My4yNTUgMTUzLjI1NSAwIDAgMS0xNi45MzItMzUuNjdjLTQuMDY2LTEyLjUxNy02LjQ0NS0yNS42My03LjEzNS0zOS4xMzQgOC40NDYtMTAuNDQzIDE4LjA1Mi0xOS41OTEgMjguNjY1LTI3LjI5M2ExNTIuNjIgMTUyLjYyIDAgMCAxIDM0Ljk2NS0xOS4wMTEgMTUzLjI0MiAxNTMuMjQyIDAgMCAxIDM0Ljg5OCAxOC45N2MxMC42NTQgNy43NDMgMjAuMzAyIDE2Ljk2MiAyOC43OSAyNy40Ny0uNzI0IDEzLjQyNy0zLjEzMiAyNi40NjUtNy4yMDQgMzguOTYxYTE1Mi43NjcgMTUyLjc2NyAwIDAgMS0xNi45NTIgMzUuNzA3em0tMjguNzQtNjIuOTk4YzAgOS4yMzIgNy40ODIgMTYuNyAxNi43MDIgMTYuNyA5LjIxNyAwIDE2LjY5LTcuNDY2IDE2LjY5LTE2LjcgMC05LjE5Ni03LjQ3My0xNi42OTItMTYuNjktMTYuNjkyLTkuMjIgMC0xNi43MDEgNy40OTYtMTYuNzAxIDE2LjY5MnptLTIxLjU3OCAwYzAgOS4yMzItNy40OCAxNi43LTE2LjcgMTYuNy05LjIyNiAwLTE2LjY4NS03LjQ2Ni0xNi42ODUtMTYuNyAwLTkuMTkzIDcuNDYtMTYuNjg5IDE2LjY4Ni0xNi42ODkgOS4yMiAwIDE2LjcgNy40OTYgMTYuNyAxNi42OXoiIGZpbGw9IiM0MTlFREEiLz48L3N2Zz4K",
        "mediatype": "image/svg+xml"
    },
    "description": "A message about etcd operator, a description of channels"
}
{
    "schema": "olm.bundle",
    "name": "etcdoperator-community.v0.6.1",
    "package": "etcd",
    "image": "quay.io/operatorhubio/etcd:v0.6.1",
    "properties":[
        {
            "type": "olm.package",
            "value": {
                "packageName": "etcd",
                "version": "0.6.1"
            }
        },
        {
            "type":"olm.gvk",
            "value": {
                "group": "etcd.database.coreos.com",
                "kind": "EtcdCluster",
                "version": "v1beta2"
            }
        },
        {
            "type": "olm.channel",
            "value": {
                "name": "alpha"
            }
        }
    ],
    "relatedImages": [
        {
            "name": "etcdv0.6.1",
            "image": "quay.io/coreos/etcd-operator@sha256:bd944a211eaf8f31da5e6d69e8541e7cada8f16a9f7a5a570b22478997819943"
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "etcdoperator.v0.9.0",
    "package": "etcd",
    "image": "quay.io/operatorhubio/etcd:v0.9.0",
    "properties":[
        {
            "type": "olm.package",
            "value": {
                "packageName": "etcd",
                "version": "0.9.0"
            }
        },
        {
            "type":"olm.gvk",
            "value":{
                "group": "etcd.database.coreos.com",
                "kind": "EtcdBackup",
                "version": "v1beta2"
            }
        },
        {
            "type": "olm.channel",
            "value": {
                "name": "singlenamespace-alpha"
            }
        },
        {
            "type": "olm.channel",
            "value": {
                "name": "clusterwide-alpha"
            }
        }
    ],
    "relatedImages" : [
        {
            "name": "etcdv0.9.0",
            "image": "quay.io/coreos/etcd-operator@sha256:db563baa8194fcfe39d1df744ed70024b0f1f9e9b55b5923c2f3a413c44dc6b8"
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "etcdoperator.v0.9.2",
    "package": "etcd",
    "image": "quay.io/operatorhubio/etcd:v0.9.2",
    "properties":[
        {
            "type": "olm.package",
            "value": {
                "packageName": "etcd",
                "version": "0.9.2"
            }
        },
        {
            "type":"olm.gvk",
            "value":{
                "group": "etcd.database.coreos.com",
                "kind": "EtcdRestore",
                "version": "v1beta2"
            }
        },
        {
            "type": "olm.channel",
            "value": {
                "name": "singlenamespace-alpha",
                "replaces": "etcdoperator.v0.9.0"
            }
        }
    ],
    "relatedImages":[
        {
            "name":"etcdv0.9.2",
            "image": "quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2"
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "etcdoperator.v0.9.2-clusterwide",
    "package": "etcd",
    "image": "quay.io/operatorhubio/etcd:v0.9.2-clusterwide",
    "properties":[
        {
            "type": "olm.package",
            "value": {
                "packageName": "etcd",
                "version": "0.9.2-clusterwide"
            }
        },
        {
            "type": "olm.gvk",
            "value": {
                "group": "etcd.database.coreos.com",
                "kind": "EtcdBackup",
                "version": "v1beta2"
            }
        },
        {
            "type": "olm.skipRange",
            "value": ">=0.9.0 <=0.9.1"
        },
        {
            "type": "olm.skips",
            "value" : "etcdoperator.v0.6.1"
        },
        {
            "type": "olm.skips",
            "value" : "etcdoperator.v0.9.0"
        },
        {
            "type": "olm.channel",
            "value": {
                "name": "clusterwide-alpha",
                "replaces": "etcdoperator.v0.9.0"
            }
        }
    ],
    "relatedImages":[
        {
            "name":"etcdv0.9.2",
            "image":"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2"
        }
    ]
}
{
    "schema": "olm.bundle",
    "name" : "etcdoperator.v0.9.4",
    "package": "etcd",
    "image": "quay.io/operatorhubio/etcd:v0.9.4",
    "properties":[
        {
            "type": "olm.package",
            "value": {
                "packageName": "etcd",
                "version": "0.9.4"
            }
        },
        {
            "type": "olm.package.required",
            "value": {
                "packageName": "test",
                "versionRange": ">=1.2.3 <2.0.0-0"
            }
        },
        {
            "type": "olm.gvk",
            "value": {
                "group": "etcd.database.coreos.com",
                "kind": "EtcdBackup",
                "version": "v1beta2"
            }
        },
        {
            "type": "olm.gvk.required",
            "value": {
                "group": "testapi.coreos.com",
                "kind": "Testapi",
                "version": "v1"
            }
        },
        {
            "type": "olm.channel",
            "value": {
                "name": "singlenamespace-alpha",
                "replaces": "etcdoperator.v0.9.2"
            }
        }
    ],
    "relatedImages":[
        {
            "name":"etcdv0.9.2",
            "image": "quay.io/coreos/etcd-operator@sha256:66a37fd61a06a43969854ee6d3e21087a98b93838e284a6086b13917f96b0d9b"
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "etcdoperator.v0.9.4-clusterwide",
    "package": "etcd",
    "image": "quay.io/operatorhubio/etcd:v0.9.4-clusterwide",
    "properties":[
        {
            "type": "olm.package",
            "value": {
                "packageName": "etcd",
                "version": "0.9.4-clusterwide"
            }
        },
        {
            "type": "olm.gvk",
            "value": {
                "group": "etcd.database.coreos.com",
                "kind": "EtcdBackup",
                "version": "v1beta2"
            }
        },
        {
            "type": "olm.channel",
            "value": {
                "name": "clusterwide-alpha",
                "replaces": "etcdoperator.v0.9.2-clusterwide"
            }
        }
    ],
    "relatedImages":[
        {
            "name":"etcdv0.9.2",
            "image": "quay.io/coreos/etcd-operator@sha256:66a37fd61a06a43969854ee6d3e21087a98b93838e284a6086b13917f96b0d9b"
        }
    ]
}`),
	},
}
