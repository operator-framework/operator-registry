package api

import (
	"encoding/json"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"

	"github.com/operator-framework/operator-registry/alpha/model"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func TestConvertAPIBundleToModelBundle(t *testing.T) {
	apiBundle := testAPIBundle()
	expected := testModelBundle(t)

	actual, err := ConvertAPIBundleToModelBundle(&apiBundle)
	require.NoError(t, err)
	assertEqualsModelBundle(t, expected, *actual)
}

func TestConvertModelBundleToAPIBundle(t *testing.T) {
	modelBundle := testModelBundle(t)
	modelBundle.Package = &model.Package{Name: "etcd"}
	modelBundle.Channel = &model.Channel{Name: "singlenamespace-alpha"}
	expected := testAPIBundle()
	expected.Properties = append(expected.Properties,
		&Property{Type: "olm.package.required", Value: "{\"packageName\":\"test\",\"versionRange\":\">=1.2.3 <2.0.0-0\"}"},
		&Property{Type: "olm.gvk.required", Value: "{\"group\":\"testapi.coreos.com\",\"kind\":\"Testapi\",\"version\":\"v1\"}"},
	)

	actual, err := ConvertModelBundleToAPIBundle(modelBundle)
	require.NoError(t, err)
	assertEqualsAPIBundle(t, expected, *actual)
}

const (
	csvJSON     = "{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"annotations\":{\"alm-examples\":\"[\\n  {\\n    \\\"apiVersion\\\": \\\"etcd.database.coreos.com/v1beta2\\\",\\n    \\\"kind\\\": \\\"EtcdCluster\\\",\\n    \\\"metadata\\\": {\\n      \\\"name\\\": \\\"example\\\"\\n    },\\n    \\\"spec\\\": {\\n      \\\"size\\\": 3,\\n      \\\"version\\\": \\\"3.2.13\\\"\\n    }\\n  },\\n  {\\n    \\\"apiVersion\\\": \\\"etcd.database.coreos.com/v1beta2\\\",\\n    \\\"kind\\\": \\\"EtcdRestore\\\",\\n    \\\"metadata\\\": {\\n      \\\"name\\\": \\\"example-etcd-cluster-restore\\\"\\n    },\\n    \\\"spec\\\": {\\n      \\\"etcdCluster\\\": {\\n        \\\"name\\\": \\\"example-etcd-cluster\\\"\\n      },\\n      \\\"backupStorageType\\\": \\\"S3\\\",\\n      \\\"s3\\\": {\\n        \\\"path\\\": \\\"\\u003cfull-s3-path\\u003e\\\",\\n        \\\"awsSecret\\\": \\\"\\u003caws-secret\\u003e\\\"\\n      }\\n    }\\n  },\\n  {\\n    \\\"apiVersion\\\": \\\"etcd.database.coreos.com/v1beta2\\\",\\n    \\\"kind\\\": \\\"EtcdBackup\\\",\\n    \\\"metadata\\\": {\\n      \\\"name\\\": \\\"example-etcd-cluster-backup\\\"\\n    },\\n    \\\"spec\\\": {\\n      \\\"etcdEndpoints\\\": [\\\"\\u003cetcd-cluster-endpoints\\u003e\\\"],\\n      \\\"storageType\\\":\\\"S3\\\",\\n      \\\"s3\\\": {\\n        \\\"path\\\": \\\"\\u003cfull-s3-path\\u003e\\\",\\n        \\\"awsSecret\\\": \\\"\\u003caws-secret\\u003e\\\"\\n      }\\n    }\\n  }\\n]\\n\",\"capabilities\":\"Full Lifecycle\",\"categories\":\"Database\",\"containerImage\":\"quay.io/coreos/etcd-operator@sha256:66a37fd61a06a43969854ee6d3e21087a98b93838e284a6086b13917f96b0d9b\",\"createdAt\":\"2019-02-28 01:03:00\",\"description\":\"Create and maintain highly-available etcd clusters on Kubernetes\",\"repository\":\"https://github.com/coreos/etcd-operator\",\"tectonic-visibility\":\"ocs\"},\"name\":\"etcdoperator.v0.9.4\",\"namespace\":\"placeholder\"},\"spec\":{\"relatedImages\":[{\"name\":\"etcdv0.9.4\",\"image\":\"quay.io/coreos/etcd-operator@sha256:66a37fd61a06a43969854ee6d3e21087a98b93838e284a6086b13917f96b0d9b\"}],\"customresourcedefinitions\":{\"owned\":[{\"description\":\"Represents a cluster of etcd nodes.\",\"displayName\":\"etcd Cluster\",\"kind\":\"EtcdCluster\",\"name\":\"etcdclusters.etcd.database.coreos.com\",\"resources\":[{\"kind\":\"Service\",\"version\":\"v1\"},{\"kind\":\"Pod\",\"version\":\"v1\"}],\"specDescriptors\":[{\"description\":\"The desired number of member Pods for the etcd cluster.\",\"displayName\":\"Size\",\"path\":\"size\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:podCount\"]},{\"description\":\"Limits describes the minimum/maximum amount of compute resources required/allowed\",\"displayName\":\"Resource Requirements\",\"path\":\"pod.resources\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:resourceRequirements\"]}],\"statusDescriptors\":[{\"description\":\"The status of each of the member Pods for the etcd cluster.\",\"displayName\":\"Member Status\",\"path\":\"members\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:podStatuses\"]},{\"description\":\"The service at which the running etcd cluster can be accessed.\",\"displayName\":\"Service\",\"path\":\"serviceName\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Service\"]},{\"description\":\"The current size of the etcd cluster.\",\"displayName\":\"Cluster Size\",\"path\":\"size\"},{\"description\":\"The current version of the etcd cluster.\",\"displayName\":\"Current Version\",\"path\":\"currentVersion\"},{\"description\":\"The target version of the etcd cluster, after upgrading.\",\"displayName\":\"Target Version\",\"path\":\"targetVersion\"},{\"description\":\"The current status of the etcd cluster.\",\"displayName\":\"Status\",\"path\":\"phase\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase\"]},{\"description\":\"Explanation for the current status of the cluster.\",\"displayName\":\"Status Details\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"},{\"description\":\"Represents the intent to backup an etcd cluster.\",\"displayName\":\"etcd Backup\",\"kind\":\"EtcdBackup\",\"name\":\"etcdbackups.etcd.database.coreos.com\",\"specDescriptors\":[{\"description\":\"Specifies the endpoints of an etcd cluster.\",\"displayName\":\"etcd Endpoint(s)\",\"path\":\"etcdEndpoints\",\"x-descriptors\":[\"urn:alm:descriptor:etcd:endpoint\"]},{\"description\":\"The full AWS S3 path where the backup is saved.\",\"displayName\":\"S3 Path\",\"path\":\"s3.path\",\"x-descriptors\":[\"urn:alm:descriptor:aws:s3:path\"]},{\"description\":\"The name of the secret object that stores the AWS credential and config files.\",\"displayName\":\"AWS Secret\",\"path\":\"s3.awsSecret\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Secret\"]}],\"statusDescriptors\":[{\"description\":\"Indicates if the backup was successful.\",\"displayName\":\"Succeeded\",\"path\":\"succeeded\",\"x-descriptors\":[\"urn:alm:descriptor:text\"]},{\"description\":\"Indicates the reason for any backup related failures.\",\"displayName\":\"Reason\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"},{\"description\":\"Represents the intent to restore an etcd cluster from a backup.\",\"displayName\":\"etcd Restore\",\"kind\":\"EtcdRestore\",\"name\":\"etcdrestores.etcd.database.coreos.com\",\"specDescriptors\":[{\"description\":\"References the EtcdCluster which should be restored,\",\"displayName\":\"etcd Cluster\",\"path\":\"etcdCluster.name\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:EtcdCluster\",\"urn:alm:descriptor:text\"]},{\"description\":\"The full AWS S3 path where the backup is saved.\",\"displayName\":\"S3 Path\",\"path\":\"s3.path\",\"x-descriptors\":[\"urn:alm:descriptor:aws:s3:path\"]},{\"description\":\"The name of the secret object that stores the AWS credential and config files.\",\"displayName\":\"AWS Secret\",\"path\":\"s3.awsSecret\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Secret\"]}],\"statusDescriptors\":[{\"description\":\"Indicates if the restore was successful.\",\"displayName\":\"Succeeded\",\"path\":\"succeeded\",\"x-descriptors\":[\"urn:alm:descriptor:text\"]},{\"description\":\"Indicates the reason for any restore related failures.\",\"displayName\":\"Reason\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"}]},\"description\":\"The etcd Operater creates and maintains highly-available etcd clusters on Kubernetes, allowing engineers to easily deploy and manage etcd clusters for their applications.\\n\\netcd is a distributed key value store that provides a reliable way to store data across a cluster of machines. Itâ€™s open-source and available on GitHub. etcd gracefully handles leader elections during network partitions and will tolerate machine failure, including the leader.\\n\\n\\n### Reading and writing to etcd\\n\\nCommunicate with etcd though its command line utility `etcdctl` via port forwarding:\\n\\n    $ kubectl --namespace default port-forward service/example-client 2379:2379\\n    $ etcdctl --endpoints http://127.0.0.1:2379 get /\\n\\nOr directly to the API using the Kubernetes Service:\\n\\n    $ etcdctl --endpoints http://example-client.default.svc:2379 get /\\n\\nBe sure to secure your etcd cluster (see Common Configurations) before exposing it outside of the namespace or cluster.\\n\\n\\n### Supported Features\\n\\n* **High availability** - Multiple instances of etcd are networked together and secured. Individual failures or networking issues are transparently handled to keep your cluster up and running.\\n\\n* **Automated updates** - Rolling out a new etcd version works like all Kubernetes rolling updates. Simply declare the desired version, and the etcd service starts a safe rolling update to the new version automatically.\\n\\n* **Backups included** - Create etcd backups and restore them through the etcd Operator.\\n\\n### Common Configurations\\n\\n* **Configure TLS** - Specify [static TLS certs](https://github.com/coreos/etcd-operator/blob/master/doc/user/cluster_tls.md) as Kubernetes secrets.\\n\\n* **Set Node Selector and Affinity** - [Spread your etcd Pods](https://github.com/coreos/etcd-operator/blob/master/doc/user/spec_examples.md#three-member-cluster-with-node-selector-and-anti-affinity-across-nodes) across Nodes and availability zones.\\n\\n* **Set Resource Limits** - [Set the Kubernetes limit and request](https://github.com/coreos/etcd-operator/blob/master/doc/user/spec_examples.md#three-member-cluster-with-resource-requirement) values for your etcd Pods.\\n\\n* **Customize Storage** - [Set a custom StorageClass](https://github.com/coreos/etcd-operator/blob/master/doc/user/spec_examples.md#custom-persistentvolumeclaim-definition) that you would like to use.\\n\",\"displayName\":\"etcd\",\"icon\":[{\"base64data\":\"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\",\"mediatype\":\"image/png\"}],\"install\":{\"spec\":{\"deployments\":[{\"name\":\"etcd-operator\",\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"name\":\"etcd-operator-alm-owned\"}},\"template\":{\"metadata\":{\"labels\":{\"name\":\"etcd-operator-alm-owned\"},\"name\":\"etcd-operator-alm-owned\"},\"spec\":{\"containers\":[{\"command\":[\"etcd-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:66a37fd61a06a43969854ee6d3e21087a98b93838e284a6086b13917f96b0d9b\",\"name\":\"etcd-operator\"},{\"command\":[\"etcd-backup-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:66a37fd61a06a43969854ee6d3e21087a98b93838e284a6086b13917f96b0d9b\",\"name\":\"etcd-backup-operator\"},{\"command\":[\"etcd-restore-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:66a37fd61a06a43969854ee6d3e21087a98b93838e284a6086b13917f96b0d9b\",\"name\":\"etcd-restore-operator\"}],\"serviceAccountName\":\"etcd-operator\"}}}}],\"permissions\":[{\"rules\":[{\"apiGroups\":[\"etcd.database.coreos.com\"],\"resources\":[\"etcdclusters\",\"etcdbackups\",\"etcdrestores\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"\"],\"resources\":[\"pods\",\"services\",\"endpoints\",\"persistentvolumeclaims\",\"events\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"apps\"],\"resources\":[\"deployments\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"\"],\"resources\":[\"secrets\"],\"verbs\":[\"get\"]}],\"serviceAccountName\":\"etcd-operator\"}]},\"strategy\":\"deployment\"},\"installModes\":[{\"supported\":true,\"type\":\"OwnNamespace\"},{\"supported\":true,\"type\":\"SingleNamespace\"},{\"supported\":false,\"type\":\"MultiNamespace\"},{\"supported\":false,\"type\":\"AllNamespaces\"}],\"keywords\":[\"etcd\",\"key value\",\"database\",\"coreos\",\"open source\"],\"labels\":{\"alm-owner-etcd\":\"etcdoperator\",\"operated-by\":\"etcdoperator\"},\"links\":[{\"name\":\"Blog\",\"url\":\"https://coreos.com/etcd\"},{\"name\":\"Documentation\",\"url\":\"https://coreos.com/operators/etcd/docs/latest/\"},{\"name\":\"etcd Operator Source Code\",\"url\":\"https://github.com/coreos/etcd-operator\"}],\"maintainers\":[{\"email\":\"etcd-dev@googlegroups.com\",\"name\":\"etcd Community\"}],\"maturity\":\"alpha\",\"provider\":{\"name\":\"CNCF\"},\"replaces\":\"etcdoperator.v0.9.2\",\"selector\":{\"matchLabels\":{\"alm-owner-etcd\":\"etcdoperator\",\"operated-by\":\"etcdoperator\"}},\"version\":\"0.9.4\"}}"
	crdbackups  = `{"apiVersion":"apiextensions.k8s.io/v1beta1","kind":"CustomResourceDefinition","metadata":{"name":"etcdbackups.etcd.database.coreos.com"},"spec":{"group":"etcd.database.coreos.com","names":{"kind":"EtcdBackup","listKind":"EtcdBackupList","plural":"etcdbackups","singular":"etcdbackup"},"scope":"Namespaced","version":"v1beta2"}}`
	crdclusters = `{"apiVersion":"apiextensions.k8s.io/v1beta1","kind":"CustomResourceDefinition","metadata":{"name":"etcdclusters.etcd.database.coreos.com"},"spec":{"group":"etcd.database.coreos.com","names":{"kind":"EtcdCluster","listKind":"EtcdClusterList","plural":"etcdclusters","shortNames":["etcdclus","etcd"],"singular":"etcdcluster"},"scope":"Namespaced","version":"v1beta2"}}`
	crdrestores = `{"apiVersion":"apiextensions.k8s.io/v1beta1","kind":"CustomResourceDefinition","metadata":{"name":"etcdrestores.etcd.database.coreos.com"},"spec":{"group":"etcd.database.coreos.com","names":{"kind":"EtcdRestore","listKind":"EtcdRestoreList","plural":"etcdrestores","singular":"etcdrestore"},"scope":"Namespaced","version":"v1beta2"}}`
)

func testModelBundle(t *testing.T) model.Bundle {
	t.Helper()
	var csv v1alpha1.ClusterServiceVersion
	if err := json.Unmarshal([]byte(csvJSON), &csv); err != nil {
		t.Fatalf("failed to unmarshal csv json: %v", err)
	}
	b := model.Bundle{
		Name:     "etcdoperator.v0.9.4",
		Image:    "quay.io/operatorhubio/etcd:v0.9.4",
		Replaces: "etcdoperator.v0.9.2",
		Skips:    nil,
		Properties: []property.Property{
			property.MustBuildPackage("etcd", "0.9.4"),
			property.MustBuildPackageRequired("test", ">=1.2.3 <2.0.0-0"),
			property.MustBuildGVKRequired("testapi.coreos.com", "v1", "Testapi"),
			property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
			property.MustBuildBundleObject([]byte(crdbackups)),
			property.MustBuildBundleObject([]byte(crdclusters)),
			property.MustBuildBundleObject([]byte(csvJSON)),
			property.MustBuildBundleObject([]byte(crdrestores)),
		},
		CsvJSON: csvJSON,
		Objects: []string{
			crdbackups,
			crdclusters,
			csvJSON,
			crdrestores,
		},
		RelatedImages: []model.RelatedImage{
			{
				Name:  "etcdv0.9.4",
				Image: "quay.io/coreos/etcd-operator@sha256:66a37fd61a06a43969854ee6d3e21087a98b93838e284a6086b13917f96b0d9b",
			},
		},
		Version: semver.MustParse("0.9.4"),
	}
	return b
}

func testAPIBundle() Bundle {
	return Bundle{
		CsvName:     "etcdoperator.v0.9.4",
		PackageName: "etcd",
		ChannelName: "singlenamespace-alpha",
		BundlePath:  "quay.io/operatorhubio/etcd:v0.9.4",
		ProvidedApis: []*GroupVersionKind{
			{Group: "etcd.database.coreos.com", Version: "v1beta2", Kind: "EtcdBackup"},
		},
		RequiredApis: []*GroupVersionKind{
			{Group: "testapi.coreos.com", Version: "v1", Kind: "Testapi"},
		},
		Version: "0.9.4",
		Dependencies: []*Dependency{
			{Type: "olm.package", Value: `{"packageName":"test","version":">=1.2.3 <2.0.0-0"}`},
			{Type: "olm.gvk", Value: `{"group":"testapi.coreos.com","kind":"Testapi","version":"v1"}`},
		},
		Properties: []*Property{
			{Type: "olm.package", Value: `{"packageName":"etcd","version":"0.9.4"}`},
			{Type: "olm.gvk", Value: `{"group":"etcd.database.coreos.com","kind":"EtcdBackup","version":"v1beta2"}`},
		},
		Replaces: "etcdoperator.v0.9.2",
		CsvJson:  csvJSON,
		Object: []string{
			crdbackups,
			crdclusters,
			csvJSON,
			crdrestores},
	}
}

func assertEqualsModelBundle(t *testing.T, a, b model.Bundle) bool {
	assert.ElementsMatch(t, a.Properties, b.Properties)
	assert.ElementsMatch(t, a.Objects, b.Objects)
	assert.ElementsMatch(t, a.Skips, b.Skips)
	assert.ElementsMatch(t, a.RelatedImages, b.RelatedImages)

	a.Properties, b.Properties = nil, nil
	a.Objects, b.Objects = nil, nil
	a.Skips, b.Skips = nil, nil
	a.RelatedImages, b.RelatedImages = nil, nil

	return assert.Equal(t, a, b)
}

func assertEqualsAPIBundle(t *testing.T, a, b Bundle) bool {
	assert.ElementsMatch(t, a.Properties, b.Properties)
	assert.ElementsMatch(t, a.Dependencies, b.Dependencies)
	assert.ElementsMatch(t, a.Object, b.Object)
	assert.ElementsMatch(t, a.Skips, b.Skips)
	assert.ElementsMatch(t, a.ProvidedApis, b.ProvidedApis)
	assert.ElementsMatch(t, a.RequiredApis, b.RequiredApis)

	a.Properties, b.Properties = nil, nil
	a.Dependencies, b.Dependencies = nil, nil
	a.Object, b.Object = nil, nil
	a.Skips, b.Skips = nil, nil
	a.ProvidedApis, b.ProvidedApis = nil, nil
	a.RequiredApis, b.RequiredApis = nil, nil

	return assert.Equal(t, a, b)
}
