package registry_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func CreateTestDb(t *testing.T) (*sql.DB, func()) {
	dbName := fmt.Sprintf("test-%d.db", rand.Int())

	db, err := sqlite.Open(dbName)
	require.NoError(t, err)

	return db, func() {
		defer func() {
			if err := os.Remove(dbName); err != nil {
				t.Fatal(err)
			}
		}()
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func createAndPopulateDB(db *sql.DB) (*sqlite.SQLQuerier, error) {
	load, err := sqlite.NewSQLLiteLoader(db)
	if err != nil {
		return nil, err
	}
	err = load.Migrate(context.TODO())
	if err != nil {
		return nil, err
	}
	query := sqlite.NewSQLLiteQuerierFromDb(db)

	graphLoader, err := sqlite.NewSQLGraphLoaderFromDB(db)
	if err != nil {
		return nil, err
	}

	populate := func(names []string) error {
		refMap := make(map[image.Reference]string, 0)
		for _, name := range names {
			refMap[image.SimpleReference("quay.io/test/"+name)] = "../../bundles/" + name
		}
		return registry.NewDirectoryPopulator(
			load,
			graphLoader,
			query,
			refMap,
			make(map[string]map[image.Reference]string, 0), false).Populate(registry.ReplacesMode)
	}
	names := []string{"etcd.0.9.0", "etcd.0.9.2", "prometheus.0.22.2", "prometheus.0.14.0", "prometheus.0.15.0"}
	if err := populate(names); err != nil {
		return nil, err
	}

	return query, nil
}

func TestImageLoader(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	db, cleanup := CreateTestDb(t)
	defer cleanup()

	_, err := createAndPopulateDB(db)
	require.NoError(t, err)
}

func TestQuerierForImage(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	db, cleanup := CreateTestDb(t)
	defer cleanup()

	store, err := createAndPopulateDB(db)
	require.NoError(t, err)

	foundPackages, err := store.ListPackages(context.TODO())
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"etcd", "prometheus"}, foundPackages)

	etcdPackage, err := store.GetPackage(context.TODO(), "etcd")
	require.NoError(t, err)
	require.EqualValues(t, &registry.PackageManifest{
		PackageName:        "etcd",
		DefaultChannelName: "alpha",
		Channels: []registry.PackageChannel{
			{
				Name:           "alpha",
				CurrentCSVName: "etcdoperator.v0.9.2",
			},
			{
				Name:           "beta",
				CurrentCSVName: "etcdoperator.v0.9.0",
			},
			{
				Name:           "stable",
				CurrentCSVName: "etcdoperator.v0.9.2",
			},
		},
	}, etcdPackage)

	etcdBundleByChannel, err := store.GetBundleForChannel(context.TODO(), "etcd", "alpha")
	require.NoError(t, err)
	expectedBundle := &api.Bundle{
		CsvName:     "etcdoperator.v0.9.2",
		PackageName: "etcd",
		ChannelName: "alpha",
		CsvJson:     "{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"annotations\":{\"alm-examples\":\"[{\\\"apiVersion\\\":\\\"etcd.database.coreos.com/v1beta2\\\",\\\"kind\\\":\\\"EtcdCluster\\\",\\\"metadata\\\":{\\\"name\\\":\\\"example\\\",\\\"namespace\\\":\\\"default\\\"},\\\"spec\\\":{\\\"size\\\":3,\\\"version\\\":\\\"3.2.13\\\"}},{\\\"apiVersion\\\":\\\"etcd.database.coreos.com/v1beta2\\\",\\\"kind\\\":\\\"EtcdRestore\\\",\\\"metadata\\\":{\\\"name\\\":\\\"example-etcd-cluster\\\"},\\\"spec\\\":{\\\"etcdCluster\\\":{\\\"name\\\":\\\"example-etcd-cluster\\\"},\\\"backupStorageType\\\":\\\"S3\\\",\\\"s3\\\":{\\\"path\\\":\\\"\\u003cfull-s3-path\\u003e\\\",\\\"awsSecret\\\":\\\"\\u003caws-secret\\u003e\\\"}}},{\\\"apiVersion\\\":\\\"etcd.database.coreos.com/v1beta2\\\",\\\"kind\\\":\\\"EtcdBackup\\\",\\\"metadata\\\":{\\\"name\\\":\\\"example-etcd-cluster-backup\\\"},\\\"spec\\\":{\\\"etcdEndpoints\\\":[\\\"\\u003cetcd-cluster-endpoints\\u003e\\\"],\\\"storageType\\\":\\\"S3\\\",\\\"s3\\\":{\\\"path\\\":\\\"\\u003cfull-s3-path\\u003e\\\",\\\"awsSecret\\\":\\\"\\u003caws-secret\\u003e\\\"}}}]\",\"tectonic-visibility\":\"ocs\"},\"name\":\"etcdoperator.v0.9.2\",\"namespace\":\"placeholder\"},\"spec\":{\"customresourcedefinitions\":{\"owned\":[{\"description\":\"Represents a cluster of etcd nodes.\",\"displayName\":\"etcd Cluster\",\"kind\":\"EtcdCluster\",\"name\":\"etcdclusters.etcd.database.coreos.com\",\"resources\":[{\"kind\":\"Service\",\"version\":\"v1\"},{\"kind\":\"Pod\",\"version\":\"v1\"}],\"specDescriptors\":[{\"description\":\"The desired number of member Pods for the etcd cluster.\",\"displayName\":\"Size\",\"path\":\"size\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:podCount\"]},{\"description\":\"Limits describes the minimum/maximum amount of compute resources required/allowed\",\"displayName\":\"Resource Requirements\",\"path\":\"pod.resources\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:resourceRequirements\"]}],\"statusDescriptors\":[{\"description\":\"The status of each of the member Pods for the etcd cluster.\",\"displayName\":\"Member Status\",\"path\":\"members\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:podStatuses\"]},{\"description\":\"The service at which the running etcd cluster can be accessed.\",\"displayName\":\"Service\",\"path\":\"serviceName\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Service\"]},{\"description\":\"The current size of the etcd cluster.\",\"displayName\":\"Cluster Size\",\"path\":\"size\"},{\"description\":\"The current version of the etcd cluster.\",\"displayName\":\"Current Version\",\"path\":\"currentVersion\"},{\"description\":\"The target version of the etcd cluster, after upgrading.\",\"displayName\":\"Target Version\",\"path\":\"targetVersion\"},{\"description\":\"The current status of the etcd cluster.\",\"displayName\":\"Status\",\"path\":\"phase\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase\"]},{\"description\":\"Explanation for the current status of the cluster.\",\"displayName\":\"Status Details\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"},{\"description\":\"Represents the intent to backup an etcd cluster.\",\"displayName\":\"etcd Backup\",\"kind\":\"EtcdBackup\",\"name\":\"etcdbackups.etcd.database.coreos.com\",\"specDescriptors\":[{\"description\":\"Specifies the endpoints of an etcd cluster.\",\"displayName\":\"etcd Endpoint(s)\",\"path\":\"etcdEndpoints\",\"x-descriptors\":[\"urn:alm:descriptor:etcd:endpoint\"]},{\"description\":\"The full AWS S3 path where the backup is saved.\",\"displayName\":\"S3 Path\",\"path\":\"s3.path\",\"x-descriptors\":[\"urn:alm:descriptor:aws:s3:path\"]},{\"description\":\"The name of the secret object that stores the AWS credential and config files.\",\"displayName\":\"AWS Secret\",\"path\":\"s3.awsSecret\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Secret\"]}],\"statusDescriptors\":[{\"description\":\"Indicates if the backup was successful.\",\"displayName\":\"Succeeded\",\"path\":\"succeeded\",\"x-descriptors\":[\"urn:alm:descriptor:text\"]},{\"description\":\"Indicates the reason for any backup related failures.\",\"displayName\":\"Reason\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"},{\"description\":\"Represents the intent to restore an etcd cluster from a backup.\",\"displayName\":\"etcd Restore\",\"kind\":\"EtcdRestore\",\"name\":\"etcdrestores.etcd.database.coreos.com\",\"specDescriptors\":[{\"description\":\"References the EtcdCluster which should be restored,\",\"displayName\":\"etcd Cluster\",\"path\":\"etcdCluster.name\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:EtcdCluster\",\"urn:alm:descriptor:text\"]},{\"description\":\"The full AWS S3 path where the backup is saved.\",\"displayName\":\"S3 Path\",\"path\":\"s3.path\",\"x-descriptors\":[\"urn:alm:descriptor:aws:s3:path\"]},{\"description\":\"The name of the secret object that stores the AWS credential and config files.\",\"displayName\":\"AWS Secret\",\"path\":\"s3.awsSecret\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Secret\"]}],\"statusDescriptors\":[{\"description\":\"Indicates if the restore was successful.\",\"displayName\":\"Succeeded\",\"path\":\"succeeded\",\"x-descriptors\":[\"urn:alm:descriptor:text\"]},{\"description\":\"Indicates the reason for any restore related failures.\",\"displayName\":\"Reason\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"}],\"required\":[{\"description\":\"Represents a cluster of etcd nodes.\",\"displayName\":\"etcd Cluster\",\"kind\":\"EtcdCluster\",\"name\":\"etcdclusters.etcd.database.coreos.com\",\"resources\":[{\"kind\":\"Service\",\"version\":\"v1\"},{\"kind\":\"Pod\",\"version\":\"v1\"}],\"specDescriptors\":[{\"description\":\"The desired number of member Pods for the etcd cluster.\",\"displayName\":\"Size\",\"path\":\"size\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:podCount\"]}],\"version\":\"v1beta2\"}]},\"description\":\"etcd is a distributed key value store that provides a reliable way to store data across a cluster of machines. It’s open-source and available on GitHub. etcd gracefully handles leader elections during network partitions and will tolerate machine failure, including the leader. Your applications can read and write data into etcd.\\nA simple use-case is to store database connection details or feature flags within etcd as key value pairs. These values can be watched, allowing your app to reconfigure itself when they change. Advanced uses take advantage of the consistency guarantees to implement database leader elections or do distributed locking across a cluster of workers.\\n\\n_The etcd Open Cloud Service is Public Alpha. The goal before Beta is to fully implement backup features._\\n\\n### Reading and writing to etcd\\n\\nCommunicate with etcd though its command line utility `etcdctl` or with the API using the automatically generated Kubernetes Service.\\n\\n[Read the complete guide to using the etcd Open Cloud Service](https://coreos.com/tectonic/docs/latest/alm/etcd-ocs.html)\\n\\n### Supported Features\\n\\n\\n**High availability**\\n\\n\\nMultiple instances of etcd are networked together and secured. Individual failures or networking issues are transparently handled to keep your cluster up and running.\\n\\n\\n**Automated updates**\\n\\n\\nRolling out a new etcd version works like all Kubernetes rolling updates. Simply declare the desired version, and the etcd service starts a safe rolling update to the new version automatically.\\n\\n\\n**Backups included**\\n\\n\\nComing soon, the ability to schedule backups to happen on or off cluster.\\n\",\"displayName\":\"etcd\",\"icon\":[{\"base64data\":\"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\",\"mediatype\":\"image/png\"}],\"install\":{\"spec\":{\"deployments\":[{\"name\":\"etcd-operator\",\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"name\":\"etcd-operator-alm-owned\"}},\"template\":{\"metadata\":{\"labels\":{\"name\":\"etcd-operator-alm-owned\"},\"name\":\"etcd-operator-alm-owned\"},\"spec\":{\"containers\":[{\"command\":[\"etcd-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2\",\"name\":\"etcd-operator\"},{\"command\":[\"etcd-backup-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2\",\"name\":\"etcd-backup-operator\"},{\"command\":[\"etcd-restore-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2\",\"name\":\"etcd-restore-operator\"}],\"serviceAccountName\":\"etcd-operator\"}}}}],\"permissions\":[{\"rules\":[{\"apiGroups\":[\"etcd.database.coreos.com\"],\"resources\":[\"etcdclusters\",\"etcdbackups\",\"etcdrestores\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"\"],\"resources\":[\"pods\",\"services\",\"endpoints\",\"persistentvolumeclaims\",\"events\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"apps\"],\"resources\":[\"deployments\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"\"],\"resources\":[\"secrets\"],\"verbs\":[\"get\"]}],\"serviceAccountName\":\"etcd-operator\"}]},\"strategy\":\"deployment\"},\"keywords\":[\"etcd\",\"key value\",\"database\",\"coreos\",\"open source\"],\"labels\":{\"alm-owner-etcd\":\"etcdoperator\",\"operated-by\":\"etcdoperator\"},\"links\":[{\"name\":\"Blog\",\"url\":\"https://coreos.com/etcd\"},{\"name\":\"Documentation\",\"url\":\"https://coreos.com/operators/etcd/docs/latest/\"},{\"name\":\"etcd Operator Source Code\",\"url\":\"https://github.com/coreos/etcd-operator\"}],\"maintainers\":[{\"email\":\"support@coreos.com\",\"name\":\"CoreOS, Inc\"}],\"maturity\":\"alpha\",\"provider\":{\"name\":\"CoreOS, Inc\"},\"replaces\":\"etcdoperator.v0.9.0\",\"selector\":{\"matchLabels\":{\"alm-owner-etcd\":\"etcdoperator\",\"operated-by\":\"etcdoperator\"}},\"skips\":[\"etcdoperator.v0.9.1\"],\"version\":\"0.9.2\"}}",
		Object:      []string{"{\"apiVersion\":\"apiextensions.k8s.io/v1beta1\",\"kind\":\"CustomResourceDefinition\",\"metadata\":{\"name\":\"etcdbackups.etcd.database.coreos.com\"},\"spec\":{\"group\":\"etcd.database.coreos.com\",\"names\":{\"kind\":\"EtcdBackup\",\"listKind\":\"EtcdBackupList\",\"plural\":\"etcdbackups\",\"singular\":\"etcdbackup\"},\"scope\":\"Namespaced\",\"version\":\"v1beta2\"}}", "{\"apiVersion\":\"apiextensions.k8s.io/v1beta1\",\"kind\":\"CustomResourceDefinition\",\"metadata\":{\"name\":\"etcdclusters.etcd.database.coreos.com\"},\"spec\":{\"group\":\"etcd.database.coreos.com\",\"names\":{\"kind\":\"EtcdCluster\",\"listKind\":\"EtcdClusterList\",\"plural\":\"etcdclusters\",\"shortNames\":[\"etcdclus\",\"etcd\"],\"singular\":\"etcdcluster\"},\"scope\":\"Namespaced\",\"version\":\"v1beta2\"}}", "{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"annotations\":{\"alm-examples\":\"[{\\\"apiVersion\\\":\\\"etcd.database.coreos.com/v1beta2\\\",\\\"kind\\\":\\\"EtcdCluster\\\",\\\"metadata\\\":{\\\"name\\\":\\\"example\\\",\\\"namespace\\\":\\\"default\\\"},\\\"spec\\\":{\\\"size\\\":3,\\\"version\\\":\\\"3.2.13\\\"}},{\\\"apiVersion\\\":\\\"etcd.database.coreos.com/v1beta2\\\",\\\"kind\\\":\\\"EtcdRestore\\\",\\\"metadata\\\":{\\\"name\\\":\\\"example-etcd-cluster\\\"},\\\"spec\\\":{\\\"etcdCluster\\\":{\\\"name\\\":\\\"example-etcd-cluster\\\"},\\\"backupStorageType\\\":\\\"S3\\\",\\\"s3\\\":{\\\"path\\\":\\\"\\u003cfull-s3-path\\u003e\\\",\\\"awsSecret\\\":\\\"\\u003caws-secret\\u003e\\\"}}},{\\\"apiVersion\\\":\\\"etcd.database.coreos.com/v1beta2\\\",\\\"kind\\\":\\\"EtcdBackup\\\",\\\"metadata\\\":{\\\"name\\\":\\\"example-etcd-cluster-backup\\\"},\\\"spec\\\":{\\\"etcdEndpoints\\\":[\\\"\\u003cetcd-cluster-endpoints\\u003e\\\"],\\\"storageType\\\":\\\"S3\\\",\\\"s3\\\":{\\\"path\\\":\\\"\\u003cfull-s3-path\\u003e\\\",\\\"awsSecret\\\":\\\"\\u003caws-secret\\u003e\\\"}}}]\",\"tectonic-visibility\":\"ocs\"},\"name\":\"etcdoperator.v0.9.2\",\"namespace\":\"placeholder\"},\"spec\":{\"customresourcedefinitions\":{\"owned\":[{\"description\":\"Represents a cluster of etcd nodes.\",\"displayName\":\"etcd Cluster\",\"kind\":\"EtcdCluster\",\"name\":\"etcdclusters.etcd.database.coreos.com\",\"resources\":[{\"kind\":\"Service\",\"version\":\"v1\"},{\"kind\":\"Pod\",\"version\":\"v1\"}],\"specDescriptors\":[{\"description\":\"The desired number of member Pods for the etcd cluster.\",\"displayName\":\"Size\",\"path\":\"size\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:podCount\"]},{\"description\":\"Limits describes the minimum/maximum amount of compute resources required/allowed\",\"displayName\":\"Resource Requirements\",\"path\":\"pod.resources\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:resourceRequirements\"]}],\"statusDescriptors\":[{\"description\":\"The status of each of the member Pods for the etcd cluster.\",\"displayName\":\"Member Status\",\"path\":\"members\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:podStatuses\"]},{\"description\":\"The service at which the running etcd cluster can be accessed.\",\"displayName\":\"Service\",\"path\":\"serviceName\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Service\"]},{\"description\":\"The current size of the etcd cluster.\",\"displayName\":\"Cluster Size\",\"path\":\"size\"},{\"description\":\"The current version of the etcd cluster.\",\"displayName\":\"Current Version\",\"path\":\"currentVersion\"},{\"description\":\"The target version of the etcd cluster, after upgrading.\",\"displayName\":\"Target Version\",\"path\":\"targetVersion\"},{\"description\":\"The current status of the etcd cluster.\",\"displayName\":\"Status\",\"path\":\"phase\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase\"]},{\"description\":\"Explanation for the current status of the cluster.\",\"displayName\":\"Status Details\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"},{\"description\":\"Represents the intent to backup an etcd cluster.\",\"displayName\":\"etcd Backup\",\"kind\":\"EtcdBackup\",\"name\":\"etcdbackups.etcd.database.coreos.com\",\"specDescriptors\":[{\"description\":\"Specifies the endpoints of an etcd cluster.\",\"displayName\":\"etcd Endpoint(s)\",\"path\":\"etcdEndpoints\",\"x-descriptors\":[\"urn:alm:descriptor:etcd:endpoint\"]},{\"description\":\"The full AWS S3 path where the backup is saved.\",\"displayName\":\"S3 Path\",\"path\":\"s3.path\",\"x-descriptors\":[\"urn:alm:descriptor:aws:s3:path\"]},{\"description\":\"The name of the secret object that stores the AWS credential and config files.\",\"displayName\":\"AWS Secret\",\"path\":\"s3.awsSecret\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Secret\"]}],\"statusDescriptors\":[{\"description\":\"Indicates if the backup was successful.\",\"displayName\":\"Succeeded\",\"path\":\"succeeded\",\"x-descriptors\":[\"urn:alm:descriptor:text\"]},{\"description\":\"Indicates the reason for any backup related failures.\",\"displayName\":\"Reason\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"},{\"description\":\"Represents the intent to restore an etcd cluster from a backup.\",\"displayName\":\"etcd Restore\",\"kind\":\"EtcdRestore\",\"name\":\"etcdrestores.etcd.database.coreos.com\",\"specDescriptors\":[{\"description\":\"References the EtcdCluster which should be restored,\",\"displayName\":\"etcd Cluster\",\"path\":\"etcdCluster.name\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:EtcdCluster\",\"urn:alm:descriptor:text\"]},{\"description\":\"The full AWS S3 path where the backup is saved.\",\"displayName\":\"S3 Path\",\"path\":\"s3.path\",\"x-descriptors\":[\"urn:alm:descriptor:aws:s3:path\"]},{\"description\":\"The name of the secret object that stores the AWS credential and config files.\",\"displayName\":\"AWS Secret\",\"path\":\"s3.awsSecret\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Secret\"]}],\"statusDescriptors\":[{\"description\":\"Indicates if the restore was successful.\",\"displayName\":\"Succeeded\",\"path\":\"succeeded\",\"x-descriptors\":[\"urn:alm:descriptor:text\"]},{\"description\":\"Indicates the reason for any restore related failures.\",\"displayName\":\"Reason\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"}],\"required\":[{\"description\":\"Represents a cluster of etcd nodes.\",\"displayName\":\"etcd Cluster\",\"kind\":\"EtcdCluster\",\"name\":\"etcdclusters.etcd.database.coreos.com\",\"resources\":[{\"kind\":\"Service\",\"version\":\"v1\"},{\"kind\":\"Pod\",\"version\":\"v1\"}],\"specDescriptors\":[{\"description\":\"The desired number of member Pods for the etcd cluster.\",\"displayName\":\"Size\",\"path\":\"size\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:podCount\"]}],\"version\":\"v1beta2\"}]},\"description\":\"etcd is a distributed key value store that provides a reliable way to store data across a cluster of machines. It’s open-source and available on GitHub. etcd gracefully handles leader elections during network partitions and will tolerate machine failure, including the leader. Your applications can read and write data into etcd.\\nA simple use-case is to store database connection details or feature flags within etcd as key value pairs. These values can be watched, allowing your app to reconfigure itself when they change. Advanced uses take advantage of the consistency guarantees to implement database leader elections or do distributed locking across a cluster of workers.\\n\\n_The etcd Open Cloud Service is Public Alpha. The goal before Beta is to fully implement backup features._\\n\\n### Reading and writing to etcd\\n\\nCommunicate with etcd though its command line utility `etcdctl` or with the API using the automatically generated Kubernetes Service.\\n\\n[Read the complete guide to using the etcd Open Cloud Service](https://coreos.com/tectonic/docs/latest/alm/etcd-ocs.html)\\n\\n### Supported Features\\n\\n\\n**High availability**\\n\\n\\nMultiple instances of etcd are networked together and secured. Individual failures or networking issues are transparently handled to keep your cluster up and running.\\n\\n\\n**Automated updates**\\n\\n\\nRolling out a new etcd version works like all Kubernetes rolling updates. Simply declare the desired version, and the etcd service starts a safe rolling update to the new version automatically.\\n\\n\\n**Backups included**\\n\\n\\nComing soon, the ability to schedule backups to happen on or off cluster.\\n\",\"displayName\":\"etcd\",\"icon\":[{\"base64data\":\"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\",\"mediatype\":\"image/png\"}],\"install\":{\"spec\":{\"deployments\":[{\"name\":\"etcd-operator\",\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"name\":\"etcd-operator-alm-owned\"}},\"template\":{\"metadata\":{\"labels\":{\"name\":\"etcd-operator-alm-owned\"},\"name\":\"etcd-operator-alm-owned\"},\"spec\":{\"containers\":[{\"command\":[\"etcd-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2\",\"name\":\"etcd-operator\"},{\"command\":[\"etcd-backup-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2\",\"name\":\"etcd-backup-operator\"},{\"command\":[\"etcd-restore-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2\",\"name\":\"etcd-restore-operator\"}],\"serviceAccountName\":\"etcd-operator\"}}}}],\"permissions\":[{\"rules\":[{\"apiGroups\":[\"etcd.database.coreos.com\"],\"resources\":[\"etcdclusters\",\"etcdbackups\",\"etcdrestores\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"\"],\"resources\":[\"pods\",\"services\",\"endpoints\",\"persistentvolumeclaims\",\"events\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"apps\"],\"resources\":[\"deployments\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"\"],\"resources\":[\"secrets\"],\"verbs\":[\"get\"]}],\"serviceAccountName\":\"etcd-operator\"}]},\"strategy\":\"deployment\"},\"keywords\":[\"etcd\",\"key value\",\"database\",\"coreos\",\"open source\"],\"labels\":{\"alm-owner-etcd\":\"etcdoperator\",\"operated-by\":\"etcdoperator\"},\"links\":[{\"name\":\"Blog\",\"url\":\"https://coreos.com/etcd\"},{\"name\":\"Documentation\",\"url\":\"https://coreos.com/operators/etcd/docs/latest/\"},{\"name\":\"etcd Operator Source Code\",\"url\":\"https://github.com/coreos/etcd-operator\"}],\"maintainers\":[{\"email\":\"support@coreos.com\",\"name\":\"CoreOS, Inc\"}],\"maturity\":\"alpha\",\"provider\":{\"name\":\"CoreOS, Inc\"},\"replaces\":\"etcdoperator.v0.9.0\",\"selector\":{\"matchLabels\":{\"alm-owner-etcd\":\"etcdoperator\",\"operated-by\":\"etcdoperator\"}},\"skips\":[\"etcdoperator.v0.9.1\"],\"version\":\"0.9.2\"}}", "{\"apiVersion\":\"apiextensions.k8s.io/v1beta1\",\"kind\":\"CustomResourceDefinition\",\"metadata\":{\"name\":\"etcdrestores.etcd.database.coreos.com\"},\"spec\":{\"group\":\"etcd.database.coreos.com\",\"names\":{\"kind\":\"EtcdRestore\",\"listKind\":\"EtcdRestoreList\",\"plural\":\"etcdrestores\",\"singular\":\"etcdrestore\"},\"scope\":\"Namespaced\",\"version\":\"v1beta2\"}}"}, BundlePath: "quay.io/test/etcd.0.9.2",
		Version:   "0.9.2",
		SkipRange: "",
		Dependencies: []*api.Dependency{
			{
				Type:  "olm.gvk",
				Value: `{"group":"testapi.coreos.com","kind":"testapi","version":"v1"}`,
			},
			{
				Type:  "olm.gvk",
				Value: `{"group":"etcd.database.coreos.com","kind":"EtcdCluster","version":"v1beta2"}`,
			},
		},
		Properties: []*api.Property{
			{
				Type:  "olm.package",
				Value: `{"packageName":"etcd","version":"0.9.2"}`,
			},
			{
				Type:  "olm.gvk",
				Value: `{"group":"etcd.database.coreos.com","kind":"EtcdCluster","version":"v1beta2"}`,
			},
			{
				Type:  "olm.gvk",
				Value: `{"group":"etcd.database.coreos.com","kind":"EtcdBackup","version":"v1beta2"}`,
			},
			{
				Type:  "olm.gvk",
				Value: `{"group":"etcd.database.coreos.com","kind":"EtcdRestore","version":"v1beta2"}`,
			},
		},
		ProvidedApis: []*api.GroupVersionKind{
			{Group: "etcd.database.coreos.com", Version: "v1beta2", Kind: "EtcdCluster", Plural: "etcdclusters"},
			{Group: "etcd.database.coreos.com", Version: "v1beta2", Kind: "EtcdBackup", Plural: "etcdbackups"},
			{Group: "etcd.database.coreos.com", Version: "v1beta2", Kind: "EtcdRestore", Plural: "etcdrestores"},
		},
		RequiredApis: []*api.GroupVersionKind{
			{Group: "etcd.database.coreos.com", Version: "v1beta2", Kind: "EtcdCluster", Plural: "etcdclusters"},
			{Group: "testapi.coreos.com", Version: "v1", Kind: "testapi"},
		},
	}
	EqualBundles(t, *expectedBundle, *etcdBundleByChannel)

	etcdBundle, err := store.GetBundle(context.TODO(), "etcd", "alpha", "etcdoperator.v0.9.2")
	require.NoError(t, err)
	EqualBundles(t, *expectedBundle, *etcdBundle)

	etcdChannelEntries, err := store.GetChannelEntriesThatReplace(context.TODO(), "etcdoperator.v0.9.0")
	require.NoError(t, err)
	require.ElementsMatch(t, []*registry.ChannelEntry{{"etcd", "alpha", "etcdoperator.v0.9.2", "etcdoperator.v0.9.0"}, {"etcd", "stable", "etcdoperator.v0.9.2", "etcdoperator.v0.9.0"}}, etcdChannelEntries)

	etcdBundleByReplaces, err := store.GetBundleThatReplaces(context.TODO(), "etcdoperator.v0.9.0", "etcd", "alpha")
	require.NoError(t, err)
	EqualBundles(t, *expectedBundle, *etcdBundleByReplaces)

	etcdChannelEntriesThatProvide, err := store.GetChannelEntriesThatProvide(context.TODO(), "etcd.database.coreos.com", "v1beta2", "EtcdCluster")
	require.ElementsMatch(t, []*registry.ChannelEntry{
		{"etcd", "alpha", "etcdoperator.v0.9.0", ""},
		{"etcd", "alpha", "etcdoperator.v0.9.2", "etcdoperator.v0.9.1"},
		{"etcd", "alpha", "etcdoperator.v0.9.2", "etcdoperator.v0.9.0"},
		{"etcd", "stable", "etcdoperator.v0.9.0", ""},
		{"etcd", "stable", "etcdoperator.v0.9.2", "etcdoperator.v0.9.1"},
		{"etcd", "stable", "etcdoperator.v0.9.2", "etcdoperator.v0.9.0"},
		{"etcd", "beta", "etcdoperator.v0.9.0", ""}}, etcdChannelEntriesThatProvide)

	etcdLatestChannelEntriesThatProvide, err := store.GetLatestChannelEntriesThatProvide(context.TODO(), "etcd.database.coreos.com", "v1beta2", "EtcdCluster")
	require.NoError(t, err)
	require.ElementsMatch(t, []*registry.ChannelEntry{{"etcd", "alpha", "etcdoperator.v0.9.2", "etcdoperator.v0.9.0"},
		{"etcd", "stable", "etcdoperator.v0.9.2", "etcdoperator.v0.9.0"},
		{"etcd", "beta", "etcdoperator.v0.9.0", ""}}, etcdLatestChannelEntriesThatProvide)

	etcdBundleByProvides, err := store.GetBundleThatProvides(context.TODO(), "etcd.database.coreos.com", "v1beta2", "EtcdCluster")
	require.NoError(t, err)
	EqualBundles(t, *expectedBundle, *etcdBundleByProvides)

	expectedEtcdImages := []string{
		"quay.io/test/etcd.0.9.2",
		"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2",
	}
	etcdImages, err := store.GetImagesForBundle(context.TODO(), "etcdoperator.v0.9.2")
	require.NoError(t, err)
	require.ElementsMatch(t, expectedEtcdImages, etcdImages)

	expectedDatabaseImages := []string{
		"quay.io/test/etcd.0.9.0",
		"quay.io/coreos/etcd-operator@sha256:db563baa8194fcfe39d1df744ed70024b0f1f9e9b55b5923c2f3a413c44dc6b8",
		"quay.io/test/etcd.0.9.2",
		"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2",
		"quay.io/test/prometheus.0.14.0",
		"quay.io/coreos/prometheus-operator@sha256:5037b4e90dbb03ebdefaa547ddf6a1f748c8eeebeedf6b9d9f0913ad662b5731",
		"quay.io/test/prometheus.0.15.0",
		"quay.io/coreos/prometheus-operator@sha256:0e92dd9b5789c4b13d53e1319d0a6375bcca4caaf0d698af61198061222a576d",
		"quay.io/test/prometheus.0.22.2",
		"quay.io/coreos/prometheus-operator@sha256:3daa69a8c6c2f1d35dcf1fe48a7cd8b230e55f5229a1ded438f687debade5bcf",
	}
	dbImages, err := store.ListImages(context.TODO())
	require.NoError(t, err)
	require.ElementsMatch(t, expectedDatabaseImages, dbImages)

	version, err := store.GetBundleVersion(context.TODO(), "quay.io/test/etcd.0.9.2")
	require.NoError(t, err)
	require.Equal(t, "0.9.2", version)

	bundlePaths, err := store.GetBundlePathsForPackage(context.TODO(), "etcd")
	require.NoError(t, err)
	expectedBundlePaths := []string{"quay.io/test/etcd.0.9.0", "quay.io/test/etcd.0.9.2"}
	require.ElementsMatch(t, expectedBundlePaths, bundlePaths)

	defaultChannel, err := store.GetDefaultChannelForPackage(context.TODO(), "etcd")
	require.NoError(t, err)
	require.Equal(t, "alpha", defaultChannel)

	listChannels, err := store.ListChannels(context.TODO(), "etcd")
	require.NoError(t, err)
	expectedListChannels := []string{"alpha", "stable", "beta"}
	require.ElementsMatch(t, expectedListChannels, listChannels)

	currentCSVName, err := store.GetCurrentCSVNameForChannel(context.TODO(), "etcd", "alpha")
	require.NoError(t, err)
	require.Equal(t, "etcdoperator.v0.9.2", currentCSVName)
}

func TestImageLoading(t *testing.T) {
	// TODO: remove requirement to have real files
	type img struct {
		ref image.SimpleReference
		dir string
	}
	tests := []struct {
		name         string
		initImages   []img
		addImage     img
		wantPackages []*registry.Package
		wantErr      bool
		err          error
	}{
		{
			name: "OneChannel/AddBundleToTwoChannels",
			initImages: []img{
				{
					// this is in the "preview" channel
					ref: image.SimpleReference("quay.io/prometheus/operator:0.14.0"),
					dir: "../../bundles/prometheus.0.14.0",
				},
			},
			addImage: img{
				// this is in the "preview" and "stable" channels and replaces 0.14.0
				ref: image.SimpleReference("quay.io/prometheus/operator:0.15.0"),
				dir: "../../bundles/prometheus.0.15.0",
			},
			wantPackages: []*registry.Package{
				{
					Name:           "prometheus",
					DefaultChannel: "preview",
					Channels: map[string]registry.Channel{
						"preview": {
							Head: registry.BundleKey{
								BundlePath: "quay.io/prometheus/operator:0.15.0",
								Version:    "0.15.0",
								CsvName:    "prometheusoperator.0.15.0",
							},
							Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
								{BundlePath: "quay.io/prometheus/operator:0.15.0", Version: "0.15.0", CsvName: "prometheusoperator.0.15.0"}: {
									{BundlePath: "quay.io/prometheus/operator:0.14.0", Version: "0.14.0", CsvName: "prometheusoperator.0.14.0"}: struct{}{},
								},
								{BundlePath: "quay.io/prometheus/operator:0.14.0", Version: "0.14.0", CsvName: "prometheusoperator.0.14.0"}: {},
							},
						},
						"stable": {
							Head: registry.BundleKey{
								BundlePath: "quay.io/prometheus/operator:0.15.0",
								Version:    "0.15.0",
								CsvName:    "prometheusoperator.0.15.0",
							},
							Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
								{BundlePath: "quay.io/prometheus/operator:0.15.0", Version: "0.15.0", CsvName: "prometheusoperator.0.15.0"}: {
									{BundlePath: "quay.io/prometheus/operator:0.14.0", Version: "0.14.0", CsvName: "prometheusoperator.0.14.0"}: struct{}{},
								},
								{BundlePath: "quay.io/prometheus/operator:0.14.0", Version: "0.14.0", CsvName: "prometheusoperator.0.14.0"}: {}},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "OneChannel/AddBundleToNewChannel",
			initImages: []img{
				{
					// this is in the "preview" channel
					ref: image.SimpleReference("quay.io/prometheus/operator:0.14.0"),
					dir: "../../bundles/prometheus.0.14.0",
				},
			},
			addImage: img{
				// this is in the "beta" channel
				ref: image.SimpleReference("quay.io/prometheus/operator:0.14.0-beta"),
				dir: "../../bundles/prometheus.0.14.0-beta",
			},
			wantPackages: []*registry.Package{
				{
					Name:           "prometheus",
					DefaultChannel: "preview",
					Channels: map[string]registry.Channel{
						"preview": {
							Head: registry.BundleKey{
								BundlePath: "quay.io/prometheus/operator:0.14.0",
								Version:    "0.14.0",
								CsvName:    "prometheusoperator.0.14.0",
							},
							Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
								{BundlePath: "quay.io/prometheus/operator:0.14.0", Version: "0.14.0", CsvName: "prometheusoperator.0.14.0"}: {},
							},
						},
						"beta": {
							Head: registry.BundleKey{
								BundlePath: "quay.io/prometheus/operator:0.14.0-beta",
								Version:    "0.14.0-beta",
								CsvName:    "prometheusoperator.0.14.0-beta",
							},
							Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
								{BundlePath: "quay.io/prometheus/operator:0.14.0-beta", Version: "0.14.0-beta", CsvName: "prometheusoperator.0.14.0-beta"}: {},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "TwoChannel/OneChannelIsASubset",
			initImages: []img{
				{
					// this is in the "preview" channel
					ref: image.SimpleReference("quay.io/prometheus/operator:0.14.0"),
					dir: "../../bundles/prometheus.0.14.0",
				},
			},
			addImage: img{
				// this is in the "stable" channel and replaces v0.14.0
				ref: image.SimpleReference("quay.io/prometheus/operator:0.15.0-stable"),
				dir: "../../bundles/prometheus.0.15.0-stable",
			},
			wantPackages: []*registry.Package{
				{
					Name:           "prometheus",
					DefaultChannel: "stable",
					Channels: map[string]registry.Channel{
						"preview": {
							Head: registry.BundleKey{
								BundlePath: "quay.io/prometheus/operator:0.14.0",
								Version:    "0.14.0",
								CsvName:    "prometheusoperator.0.14.0",
							},
							Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
								{BundlePath: "quay.io/prometheus/operator:0.14.0", Version: "0.14.0", CsvName: "prometheusoperator.0.14.0"}: {},
							},
						},
						"stable": {
							Head: registry.BundleKey{
								BundlePath: "quay.io/prometheus/operator:0.15.0-stable",
								Version:    "0.15.0",
								CsvName:    "prometheusoperator.0.15.0-stable",
							},
							Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
								{BundlePath: "quay.io/prometheus/operator:0.15.0-stable", Version: "0.15.0", CsvName: "prometheusoperator.0.15.0-stable"}: {
									{BundlePath: "quay.io/prometheus/operator:0.14.0", Version: "0.14.0", CsvName: "prometheusoperator.0.14.0"}: struct{}{},
								},
								{BundlePath: "quay.io/prometheus/operator:0.14.0", Version: "0.14.0", CsvName: "prometheusoperator.0.14.0"}: {}},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AddBundleAlreadyExists",
			initImages: []img{
				{
					// this is in the "preview" channel
					ref: image.SimpleReference("quay.io/prometheus/operator:0.14.0"),
					dir: "../../bundles/prometheus.0.14.0",
				},
			},
			addImage: img{
				//Adding same bundle different bundle
				ref: image.SimpleReference("quay.io/prometheus/operator-test:testing"),
				dir: "../../bundles/prometheus.0.14.0",
			},
			wantPackages: []*registry.Package{
				{
					Name:           "prometheus",
					DefaultChannel: "stable",
					Channels: map[string]registry.Channel{
						"preview": {
							Head: registry.BundleKey{
								BundlePath: "quay.io/prometheus/operator:0.14.0",
								Version:    "0.14.0",
								CsvName:    "prometheusoperator.0.14.0",
							},
							Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
								{BundlePath: "quay.io/prometheus/operator:0.14.0", Version: "0.14.0", CsvName: "prometheusoperator.0.14.0"}: {},
							},
						},
					},
				},
			},
			wantErr: true,
			err:     registry.PackageVersionAlreadyAddedErr{},
		},
		{
			name: "AddExactBundleAlreadyExists",
			initImages: []img{
				{
					// this is in the "preview" channel
					ref: image.SimpleReference("quay.io/prometheus/operator:0.14.0"),
					dir: "../../bundles/prometheus.0.14.0",
				},
			},
			addImage: img{
				// Add the same package
				ref: image.SimpleReference("quay.io/prometheus/operator:0.14.0"),
				dir: "../../bundles/prometheus.0.14.0",
			},
			wantPackages: []*registry.Package{
				{
					Name:           "prometheus",
					DefaultChannel: "stable",
					Channels: map[string]registry.Channel{
						"preview": {
							Head: registry.BundleKey{
								BundlePath: "quay.io/prometheus/operator:0.14.0",
								Version:    "0.14.0",
								CsvName:    "prometheusoperator.0.14.0",
							},
							Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{
								{BundlePath: "quay.io/prometheus/operator:0.14.0", Version: "0.14.0", CsvName: "prometheusoperator.0.14.0"}: {},
							},
						},
					},
				},
			},
			wantErr: true,
			err:     registry.BundleImageAlreadyAddedErr{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logrus.SetLevel(logrus.DebugLevel)
			db, cleanup := CreateTestDb(t)
			defer cleanup()
			load, err := sqlite.NewSQLLiteLoader(db)
			require.NoError(t, err)
			require.NoError(t, load.Migrate(context.TODO()))
			query := sqlite.NewSQLLiteQuerierFromDb(db)
			graphLoader, err := sqlite.NewSQLGraphLoaderFromDB(db)
			require.NoError(t, err)
			for _, i := range tt.initImages {
				p := registry.NewDirectoryPopulator(
					load,
					graphLoader,
					query,
					map[image.Reference]string{i.ref: i.dir},
					make(map[string]map[image.Reference]string, 0), false)
				require.NoError(t, p.Populate(registry.ReplacesMode))
			}
			add := registry.NewDirectoryPopulator(
				load,
				graphLoader,
				query,
				map[image.Reference]string{tt.addImage.ref: tt.addImage.dir},
				make(map[string]map[image.Reference]string, 0), false)
			err = add.Populate(registry.ReplacesMode)
			if tt.wantErr {
				require.True(t, checkAggErr(err, tt.err))
				return
			}
			require.NoError(t, err)

			for _, p := range tt.wantPackages {
				graphLoader, err := sqlite.NewSQLGraphLoaderFromDB(db)
				require.NoError(t, err)

				result, err := graphLoader.Generate(p.Name)
				require.NoError(t, err)
				require.Equal(t, p, result)
			}
			CheckInvariants(t, db)
		})
	}
}

func checkAggErr(aggErr, wantErr error) bool {
	if a, ok := aggErr.(utilerrors.Aggregate); ok {
		for _, e := range a.Errors() {
			if reflect.TypeOf(e).String() == reflect.TypeOf(wantErr).String() {
				return true
			}
		}
		return false
	}
	return reflect.TypeOf(aggErr).String() == reflect.TypeOf(wantErr).String()
}

func TestQuerierForDependencies(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	db, cleanup := CreateTestDb(t)
	defer cleanup()

	store, err := createAndPopulateDB(db)
	require.NoError(t, err)

	expectedDependencies := []*api.Dependency{
		{
			Type:  "olm.package",
			Value: `{"packageName":"testoperator","version":"\u003e 0.2.0"}`,
		},
		{
			Type:  "olm.gvk",
			Value: `{"group":"testapi.coreos.com","kind":"testapi","version":"v1"}`,
		},
		{
			Type:  "olm.gvk",
			Value: `{"group":"etcd.database.coreos.com","kind":"EtcdCluster","version":"v1beta2"}`,
		},
		{
			Type:  "olm.gvk",
			Value: `{"group":"testprometheus.coreos.com","kind":"testtestprometheus","version":"v1"}`,
		},
	}

	type operatorbundle struct {
		name    string
		version string
		path    string
	}

	bundlesList := []operatorbundle{
		{
			name:    "etcdoperator.v0.9.2",
			version: "0.9.2",
			path:    "quay.io/test/etcd.0.9.2",
		},
		{
			name:    "prometheusoperator.0.22.2",
			version: "0.22.2",
			path:    "quay.io/test/prometheus.0.22.2",
		},
	}

	dependencies := []*api.Dependency{}
	for _, b := range bundlesList {
		dep, err := store.GetDependenciesForBundle(context.TODO(), b.name, b.version, b.path)
		require.NoError(t, err)
		dependencies = append(dependencies, dep...)
	}
	require.ElementsMatch(t, expectedDependencies, dependencies)
}

func TestListBundles(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	db, cleanup := CreateTestDb(t)
	defer cleanup()

	store, err := createAndPopulateDB(db)
	require.NoError(t, err)

	expectedDependencies := []*api.Dependency{
		{
			Type:  "olm.package",
			Value: `{"packageName":"testoperator","version":"\u003e 0.2.0"}`,
		},
		{
			Type:  "olm.gvk",
			Value: `{"group":"testapi.coreos.com","kind":"testapi","version":"v1"}`,
		},
		{
			Type:  "olm.gvk",
			Value: `{"group":"etcd.database.coreos.com","kind":"EtcdCluster","version":"v1beta2"}`,
		},
		{
			Type:  "olm.gvk",
			Value: `{"group":"testprometheus.coreos.com","kind":"testtestprometheus","version":"v1"}`,
		},
		{
			Type:  "olm.gvk",
			Value: `{"group":"testapi.coreos.com","kind":"testapi","version":"v1"}`,
		},
		{
			Type:  "olm.gvk",
			Value: `{"group":"etcd.database.coreos.com","kind":"EtcdCluster","version":"v1beta2"}`,
		},
	}

	dependencies := []*api.Dependency{}
	bundles, err := store.ListBundles(context.TODO())
	require.NoError(t, err)
	for _, b := range bundles {
		for _, d := range b.Dependencies {
			if d.GetType() != "" {
				dependencies = append(dependencies, d)
			}
		}
	}
	require.Equal(t, 10, len(bundles))
	require.ElementsMatch(t, expectedDependencies, dependencies)
}

func EqualBundles(t *testing.T, expected, actual api.Bundle) {
	require.ElementsMatch(t, expected.ProvidedApis, actual.ProvidedApis, "provided apis don't match: %#v\n%#v", expected.ProvidedApis, actual.ProvidedApis)
	require.ElementsMatch(t, expected.RequiredApis, actual.RequiredApis, "required apis don't match: %#v\n%#v", expected.RequiredApis, actual.RequiredApis)
	require.ElementsMatch(t, expected.Dependencies, actual.Dependencies, "dependencies don't match: %#v\n%#v", expected.Dependencies, actual.Dependencies)
	require.ElementsMatch(t, expected.Properties, actual.Properties, "properties don't match %#v\n%#v", expected.Properties, actual.Properties)
	expected.RequiredApis, expected.ProvidedApis, actual.RequiredApis, actual.ProvidedApis = nil, nil, nil, nil
	expected.Dependencies, expected.Properties, actual.Dependencies, actual.Properties = nil, nil, nil, nil
	require.EqualValues(t, expected, actual)
}

func CheckInvariants(t *testing.T, db *sql.DB) {
	CheckChannelHeadsHaveDescriptions(t, db)
	CheckBundlesHaveContentsIfNoPath(t, db)
}

func CheckChannelHeadsHaveDescriptions(t *testing.T, db *sql.DB) {
	// check channel heads have csv / bundle
	rows, err := db.Query(`
		select operatorbundle.name,length(operatorbundle.csv),length(operatorbundle.bundle) from operatorbundle
		join channel on channel.head_operatorbundle_name = operatorbundle.name`)
	require.NoError(t, err)

	for rows.Next() {
		var name sql.NullString
		var csvlen sql.NullInt64
		var bundlelen sql.NullInt64
		err := rows.Scan(&name, &csvlen, &bundlelen)
		require.NoError(t, err)
		t.Logf("channel head %s has csvlen %d and bundlelen %d", name.String, csvlen.Int64, bundlelen.Int64)
		require.NotZero(t, csvlen.Int64, "length of csv for %s should not be zero, it is a channel head", name.String)
		require.NotZero(t, bundlelen.Int64, "length of bundle for %s should not be zero, it is a channel head", name.String)
	}
}

func CheckBundlesHaveContentsIfNoPath(t *testing.T, db *sql.DB) {
	// check that any bundle entry has csv/bundle content unpacked if there is no bundlepath
	rows, err := db.Query(`
		select name,length(csv),length(bundle) from operatorbundle
		where bundlepath="" or bundlepath=null`)
	require.NoError(t, err)

	for rows.Next() {
		var name sql.NullString
		var csvlen sql.NullInt64
		var bundlelen sql.NullInt64
		err := rows.Scan(&name, &csvlen, &bundlelen)
		require.NoError(t, err)
		t.Logf("bundle %s has csvlen %d and bundlelen %d", name.String, csvlen.Int64, bundlelen.Int64)
		require.NotZero(t, csvlen.Int64, "length of csv for %s should not be zero, it has no bundle path", name.String)
		require.NotZero(t, bundlelen.Int64, "length of bundle for %s should not be zero, it has no bundle path", name.String)
	}
}

func TestDirectoryPopulator(t *testing.T) {
	db, cleanup := CreateTestDb(t)
	defer cleanup()

	loader, err := sqlite.NewSQLLiteLoader(db)
	require.NoError(t, err)
	require.NoError(t, loader.Migrate(context.TODO()))

	graphLoader, err := sqlite.NewSQLGraphLoaderFromDB(db)
	require.NoError(t, err)

	query := sqlite.NewSQLLiteQuerierFromDb(db)

	populate := func(bundles map[image.Reference]string) error {
		return registry.NewDirectoryPopulator(
			loader,
			graphLoader,
			query,
			bundles,
			make(map[string]map[image.Reference]string),
			false).Populate(registry.ReplacesMode)
	}
	add := map[image.Reference]string{
		image.SimpleReference("quay.io/test/etcd.0.9.2"):        "../../bundles/etcd.0.9.2",
		image.SimpleReference("quay.io/test/prometheus.0.22.2"): "../../bundles/prometheus.0.22.2",
	}

	err = populate(add)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("Invalid bundle %s, replaces nonexistent bundle %s", "etcdoperator.v0.9.2", "etcdoperator.v0.9.0"))
	require.Contains(t, err.Error(), fmt.Sprintf("Invalid bundle %s, replaces nonexistent bundle %s", "prometheusoperator.0.22.2", "prometheusoperator.0.15.0"))
}

func TestDeprecateBundle(t *testing.T) {
	type args struct {
		bundles []string
	}
	type pkgChannel map[string][]string
	type expected struct {
		err                  error
		remainingBundles     []string
		deprecatedBundles    []string
		remainingPkgChannels pkgChannel
	}
	tests := []struct {
		description string
		args        args
		expected    expected
	}{
		{
			description: "BundleDeprecated/IgnoreIfNotInIndex",
			args: args{
				bundles: []string{
					"quay.io/test/etcd.0.6.0",
				},
			},
			expected: expected{
				err: errors.NewAggregate([]error{fmt.Errorf("error deprecating bundle quay.io/test/etcd.0.6.0: %s", registry.ErrBundleImageNotInDatabase)}),
				remainingBundles: []string{
					"quay.io/test/etcd.0.9.0/alpha",
					"quay.io/test/etcd.0.9.0/beta",
					"quay.io/test/etcd.0.9.0/stable",
					"quay.io/test/etcd.0.9.2/stable",
					"quay.io/test/etcd.0.9.2/alpha",
					"quay.io/test/prometheus.0.22.2/preview",
					"quay.io/test/prometheus.0.15.0/preview",
					"quay.io/test/prometheus.0.15.0/stable",
					"quay.io/test/prometheus.0.14.0/preview",
					"quay.io/test/prometheus.0.14.0/stable",
				},
				deprecatedBundles: []string{},
				remainingPkgChannels: pkgChannel{
					"etcd": []string{
						"beta",
						"alpha",
						"stable",
					},
					"prometheus": []string{
						"preview",
						"stable",
					},
				},
			},
		},
		{
			description: "BundleDeprecated/SingleChannel",
			args: args{
				bundles: []string{
					"quay.io/test/prometheus.0.15.0",
				},
			},
			expected: expected{
				err: nil,
				remainingBundles: []string{
					"quay.io/test/etcd.0.9.0/alpha",
					"quay.io/test/etcd.0.9.0/beta",
					"quay.io/test/etcd.0.9.0/stable",
					"quay.io/test/etcd.0.9.2/stable",
					"quay.io/test/etcd.0.9.2/alpha",
					"quay.io/test/prometheus.0.15.0/preview",
					"quay.io/test/prometheus.0.15.0/stable",
					"quay.io/test/prometheus.0.22.2/preview",
				},
				deprecatedBundles: []string{
					"quay.io/test/prometheus.0.15.0/preview",
					"quay.io/test/prometheus.0.15.0/stable",
				},
				remainingPkgChannels: pkgChannel{
					"etcd": []string{
						"beta",
						"alpha",
						"stable",
					},
					"prometheus": []string{
						"preview",
						"stable",
					},
				},
			},
		},
		{
			description: "BundleDeprecated/ChannelRemoved",
			args: args{
				bundles: []string{
					"quay.io/test/etcd.0.9.2",
				},
			},
			expected: expected{
				err: nil,
				remainingBundles: []string{
					"quay.io/test/etcd.0.9.2/alpha",
					"quay.io/test/etcd.0.9.2/stable",
					"quay.io/test/prometheus.0.22.2/preview",
					"quay.io/test/prometheus.0.14.0/preview",
					"quay.io/test/prometheus.0.14.0/stable",
					"quay.io/test/prometheus.0.15.0/preview",
					"quay.io/test/prometheus.0.15.0/stable",
				},
				deprecatedBundles: []string{
					"quay.io/test/etcd.0.9.2/alpha",
					"quay.io/test/etcd.0.9.2/stable",
				},
				remainingPkgChannels: pkgChannel{
					"etcd": []string{
						"alpha",
						"stable",
					},
					"prometheus": []string{
						"preview",
						"stable",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			logrus.SetLevel(logrus.DebugLevel)
			db, cleanup := CreateTestDb(t)
			defer cleanup()

			querier, err := createAndPopulateDB(db)
			require.NoError(t, err)

			store, err := sqlite.NewSQLLiteLoader(db)
			require.NoError(t, err)

			deprecator := sqlite.NewSQLDeprecatorForBundles(store, querier, tt.args.bundles)
			err = deprecator.Deprecate()
			fmt.Printf("error: %s\n", err)
			require.Equal(t, tt.expected.err, err)

			// Ensure remaining bundlePaths in db match
			bundles, err := querier.ListBundles(context.Background())
			require.NoError(t, err)
			var bundlePaths []string
			for _, bundle := range bundles {
				bundlePaths = append(bundlePaths, strings.Join([]string{bundle.BundlePath, bundle.ChannelName}, "/"))
			}
			fmt.Println("remaining", bundlePaths)
			require.ElementsMatch(t, tt.expected.remainingBundles, bundlePaths)

			// Ensure deprecated bundles match
			var deprecatedBundles []string
			deprecatedProperty, err := json.Marshal(registry.DeprecatedProperty{})
			require.NoError(t, err)
			for _, bundle := range bundles {
				for _, prop := range bundle.Properties {
					if prop.Type == registry.DeprecatedType && prop.Value == string(deprecatedProperty) {
						deprecatedBundles = append(deprecatedBundles, strings.Join([]string{bundle.BundlePath, bundle.ChannelName}, "/"))
					}
				}
			}
			fmt.Println("deprecated", deprecatedBundles)

			require.ElementsMatch(t, tt.expected.deprecatedBundles, deprecatedBundles)

			// Ensure remaining channels match
			packages, err := querier.ListPackages(context.Background())
			require.NoError(t, err)

			for _, pkg := range packages {
				channelEntries, err := querier.GetChannelEntriesFromPackage(context.Background(), pkg)
				require.NoError(t, err)

				uniqueChannels := make(map[string]struct{})
				var channels []string
				for _, ch := range channelEntries {
					uniqueChannels[ch.ChannelName] = struct{}{}
				}
				for k := range uniqueChannels {
					channels = append(channels, k)
				}
				require.ElementsMatch(t, tt.expected.remainingPkgChannels[pkg], channels)
			}
		})
	}
}

func TestAddAfterDeprecate(t *testing.T) {
	type args struct {
		firstBundles      []string
		deprecatedBundles []string
		secondBundles     []string
	}
	type pkgChannel map[string][]string
	type expected struct {
		err                  error
		remainingBundles     []string
		deprecatedBundles    []string
		remainingPkgChannels pkgChannel
	}
	tests := []struct {
		description string
		args        args
		expected    expected
	}{
		{
			description: "SimpleAdd",
			args: args{
				firstBundles: []string{
					"prometheus.0.14.0",
					"prometheus.0.15.0",
				},
				deprecatedBundles: []string{
					"quay.io/test/prometheus.0.15.0",
				},
				secondBundles: []string{
					"prometheus.0.22.2",
				},
			},
			expected: expected{
				err: nil,
				remainingBundles: []string{
					"quay.io/test/prometheus.0.15.0/preview",
					"quay.io/test/prometheus.0.15.0/stable",
					"quay.io/test/prometheus.0.22.2/preview",
				},
				deprecatedBundles: []string{
					"quay.io/test/prometheus.0.15.0/preview",
					"quay.io/test/prometheus.0.15.0/stable",
				},
				remainingPkgChannels: pkgChannel{
					"prometheus": []string{
						"preview",
						"stable",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			logrus.SetLevel(logrus.DebugLevel)
			db, cleanup := CreateTestDb(t)
			defer cleanup()

			load, err := sqlite.NewSQLLiteLoader(db, sqlite.WithEnableAlpha(true))
			require.NoError(t, err)
			err = load.Migrate(context.TODO())
			require.NoError(t, err)
			query := sqlite.NewSQLLiteQuerierFromDb(db)

			graphLoader, err := sqlite.NewSQLGraphLoaderFromDB(db)
			require.NoError(t, err)

			populate := func(names []string) error {
				refMap := make(map[image.Reference]string, 0)
				for _, name := range names {
					refMap[image.SimpleReference("quay.io/test/"+name)] = "../../bundles/" + name
				}
				return registry.NewDirectoryPopulator(
					load,
					graphLoader,
					query,
					refMap,
					make(map[string]map[image.Reference]string, 0), false).Populate(registry.ReplacesMode)
			}
			// Initialize index with some bundles
			require.NoError(t, populate(tt.args.firstBundles))

			deprecator := sqlite.NewSQLDeprecatorForBundles(load, query, tt.args.deprecatedBundles)
			err = deprecator.Deprecate()
			require.Equal(t, tt.expected.err, err)

			require.NoError(t, populate(tt.args.secondBundles))

			// Ensure remaining bundlePaths in db match
			bundles, err := query.ListBundles(context.Background())
			require.NoError(t, err)
			var bundlePaths []string
			for _, bundle := range bundles {
				bundlePaths = append(bundlePaths, strings.Join([]string{bundle.BundlePath, bundle.ChannelName}, "/"))
			}
			require.ElementsMatch(t, tt.expected.remainingBundles, bundlePaths)

			// Ensure deprecated bundles match
			var deprecatedBundles []string
			deprecatedProperty, err := json.Marshal(registry.DeprecatedProperty{})
			require.NoError(t, err)
			for _, bundle := range bundles {
				for _, prop := range bundle.Properties {
					if prop.Type == registry.DeprecatedType && prop.Value == string(deprecatedProperty) {
						deprecatedBundles = append(deprecatedBundles, strings.Join([]string{bundle.BundlePath, bundle.ChannelName}, "/"))
					}
				}
			}

			require.ElementsMatch(t, tt.expected.deprecatedBundles, deprecatedBundles)

			// Ensure remaining channels match
			packages, err := query.ListPackages(context.Background())
			require.NoError(t, err)

			for _, pkg := range packages {
				channelEntries, err := query.GetChannelEntriesFromPackage(context.Background(), pkg)
				require.NoError(t, err)

				uniqueChannels := make(map[string]struct{})
				var channels []string
				for _, ch := range channelEntries {
					uniqueChannels[ch.ChannelName] = struct{}{}
				}
				for k := range uniqueChannels {
					channels = append(channels, k)
				}
				require.ElementsMatch(t, tt.expected.remainingPkgChannels[pkg], channels)
			}
		})
	}
}

func TestOverwrite(t *testing.T) {
	type args struct {
		firstAdd   map[image.Reference]string
		secondAdd  map[image.Reference]string
		overwrites map[string]map[image.Reference]string
	}
	type pkgChannel map[string][]string
	type expected struct {
		errs                     []error
		remainingBundles         []string
		remainingPkgChannels     pkgChannel
		remainingDefaultChannels map[string]string
	}
	getBundleRefs := func(names []string) map[image.Reference]string {
		refs := map[image.Reference]string{}
		for _, name := range names {
			refs[image.SimpleReference("quay.io/test/"+name)] = "../../bundles/" + name
		}
		return refs
	}

	tests := []struct {
		description string
		args        args
		expected    expected
	}{
		{
			description: "DefaultBehavior",
			args: args{
				firstAdd: getBundleRefs([]string{"prometheus.0.14.0"}),
				secondAdd: map[image.Reference]string{
					image.SimpleReference("quay.io/test/etcd.0.9.2"):        "../../bundles/etcd.0.9.2",
					image.SimpleReference("quay.io/test/prometheus.0.22.2"): "../../bundles/prometheus.0.22.2",
				},
				overwrites: nil,
			},
			expected: expected{
				errs: []error{
					fmt.Errorf("Invalid bundle %s, replaces nonexistent bundle %s", "etcdoperator.v0.9.2", "etcdoperator.v0.9.0"),
					fmt.Errorf("Invalid bundle %s, replaces nonexistent bundle %s", "prometheusoperator.0.22.2", "prometheusoperator.0.15.0"),
				},
				remainingBundles: []string{
					"quay.io/test/prometheus.0.14.0/preview",
				},
				remainingPkgChannels: pkgChannel{
					"prometheus": []string{
						"preview",
					},
				},
				remainingDefaultChannels: map[string]string{
					"prometheus": "preview",
				},
			},
		},
		{
			description: "SimpleCsvChange",
			args: args{
				firstAdd: getBundleRefs([]string{"etcd.0.9.0", "prometheus.0.14.0", "prometheus.0.15.0"}),
				secondAdd: map[image.Reference]string{
					image.SimpleReference("quay.io/test/new-etcd.0.9.0"):    "testdata/overwrite/etcd.0.9.0",
					image.SimpleReference("quay.io/test/prometheus.0.22.2"): "../../bundles/prometheus.0.22.2",
				},
				overwrites: map[string]map[image.Reference]string{"etcd": {}},
			},
			expected: expected{
				errs: nil,
				remainingBundles: []string{
					"quay.io/test/new-etcd.0.9.0/alpha",
					"quay.io/test/new-etcd.0.9.0/beta",
					"quay.io/test/new-etcd.0.9.0/stable",
					"quay.io/test/prometheus.0.22.2/preview",
					"quay.io/test/prometheus.0.15.0/preview",
					"quay.io/test/prometheus.0.15.0/stable",
					"quay.io/test/prometheus.0.14.0/preview",
					"quay.io/test/prometheus.0.14.0/stable",
				},
				remainingPkgChannels: pkgChannel{
					"etcd": []string{
						"beta",
						"alpha",
						"stable",
					},
					"prometheus": []string{
						"preview",
						"stable",
					},
				},
				remainingDefaultChannels: map[string]string{
					"etcd":       "stable",
					"prometheus": "preview",
				},
			},
		},
		{
			description: "ChannelRemove",
			args: args{
				firstAdd: getBundleRefs([]string{"etcd.0.9.0", "etcd.0.9.2", "prometheus.0.14.0", "prometheus.0.15.0"}),
				secondAdd: map[image.Reference]string{
					image.SimpleReference("quay.io/test/new-etcd.0.9.2"):    "testdata/overwrite/etcd.0.9.2",
					image.SimpleReference("quay.io/test/prometheus.0.22.2"): "../../bundles/prometheus.0.22.2",
				},
				overwrites: map[string]map[image.Reference]string{"etcd": getBundleRefs([]string{"etcd.0.9.0"})},
			},
			expected: expected{
				errs: nil,
				remainingBundles: []string{
					"quay.io/test/etcd.0.9.0/alpha",
					"quay.io/test/etcd.0.9.0/beta",
					"quay.io/test/etcd.0.9.0/stable",
					"quay.io/test/new-etcd.0.9.2/alpha",
					"quay.io/test/prometheus.0.14.0/preview",
					"quay.io/test/prometheus.0.14.0/stable",
					"quay.io/test/prometheus.0.15.0/preview",
					"quay.io/test/prometheus.0.15.0/stable",
					"quay.io/test/prometheus.0.22.2/preview",
				},
				remainingPkgChannels: pkgChannel{
					"etcd": []string{
						"alpha",
						"beta",
						"stable",
					},
					"prometheus": []string{
						"preview",
						"stable",
					},
				},
				remainingDefaultChannels: map[string]string{
					"etcd":       "alpha",
					"prometheus": "preview",
				},
			},
		},
		{
			description: "OverwriteBundle/ChannelSwitch",
			args: args{
				firstAdd: getBundleRefs([]string{"prometheus.0.14.0", "prometheus.0.15.0", "prometheus.0.22.2"}),
				secondAdd: map[image.Reference]string{
					image.SimpleReference("quay.io/test/etcd.0.9.0"):            "../../bundles/etcd.0.9.0",
					image.SimpleReference("quay.io/test/etcd.0.9.2"):            "../../bundles/etcd.0.9.2",
					image.SimpleReference("quay.io/test/new-prometheus.0.22.2"): "testdata/overwrite/prometheus.0.22.2",
				},
				overwrites: map[string]map[image.Reference]string{"prometheus": getBundleRefs([]string{"prometheus.0.14.0", "prometheus.0.15.0"})},
			},
			expected: expected{
				errs: nil,
				remainingBundles: []string{
					"quay.io/test/etcd.0.9.0/alpha",
					"quay.io/test/etcd.0.9.0/beta",
					"quay.io/test/etcd.0.9.0/stable",
					"quay.io/test/etcd.0.9.2/alpha",
					"quay.io/test/etcd.0.9.2/stable",
					"quay.io/test/prometheus.0.14.0/preview",
					"quay.io/test/prometheus.0.14.0/stable",
					"quay.io/test/prometheus.0.15.0/preview",
					"quay.io/test/prometheus.0.15.0/stable",
					"quay.io/test/new-prometheus.0.22.2/alpha",
					"quay.io/test/prometheus.0.15.0/alpha",
					"quay.io/test/prometheus.0.14.0/alpha",
				},
				remainingPkgChannels: pkgChannel{
					"etcd": []string{
						"beta",
						"alpha",
						"stable",
					},
					"prometheus": []string{
						"preview",
						"stable",
						"alpha",
					},
				},
				remainingDefaultChannels: map[string]string{
					"etcd":       "alpha",
					"prometheus": "preview",
				},
			},
		},
		{
			description: "OverwriteBundle/DefaultChannelChange",
			args: args{
				firstAdd: getBundleRefs([]string{"prometheus.0.14.0", "prometheus.0.15.0"}),
				secondAdd: map[image.Reference]string{
					image.SimpleReference("quay.io/test/etcd.0.9.0"):            "../../bundles/etcd.0.9.0",
					image.SimpleReference("quay.io/test/etcd.0.9.2"):            "../../bundles/etcd.0.9.2",
					image.SimpleReference("quay.io/test/new-prometheus.0.15.0"): "testdata/overwrite/prometheus.0.15.0",
				},
				overwrites: map[string]map[image.Reference]string{"prometheus": getBundleRefs([]string{"prometheus.0.14.0"})},
			},
			expected: expected{
				errs: nil,
				remainingBundles: []string{
					"quay.io/test/etcd.0.9.0/alpha",
					"quay.io/test/etcd.0.9.0/beta",
					"quay.io/test/etcd.0.9.0/stable",
					"quay.io/test/etcd.0.9.2/alpha",
					"quay.io/test/etcd.0.9.2/stable",
					"quay.io/test/prometheus.0.14.0/preview",
					"quay.io/test/prometheus.0.14.0/stable",
					"quay.io/test/new-prometheus.0.15.0/preview",
					"quay.io/test/new-prometheus.0.15.0/stable",
				},
				remainingPkgChannels: pkgChannel{
					"etcd": []string{
						"alpha",
						"beta",
						"stable",
					},
					"prometheus": []string{
						"preview",
						"stable",
					},
				},
				remainingDefaultChannels: map[string]string{
					"etcd":       "alpha",
					"prometheus": "stable",
				},
			},
		},
		{
			description: "OverwriteBundle/NonLatestOverwrite",
			args: args{
				firstAdd: getBundleRefs([]string{"prometheus.0.14.0", "prometheus.0.15.0", "prometheus.0.22.2"}),
				secondAdd: map[image.Reference]string{
					image.SimpleReference("quay.io/test/etcd.0.9.0"):            "../../bundles/etcd.0.9.0",
					image.SimpleReference("quay.io/test/etcd.0.9.2"):            "../../bundles/etcd.0.9.2",
					image.SimpleReference("quay.io/test/new-prometheus.0.15.0"): "testdata/overwrite/prometheus.0.15.0",
				},
				overwrites: map[string]map[image.Reference]string{"prometheus": getBundleRefs([]string{"prometheus.0.14.0"})},
			},
			expected: expected{
				errs: []error{registry.OverwriteErr{ErrorString: "Cannot overwrite a bundle that is not at the head of a channel using --overwrite-latest"}},
				remainingBundles: []string{
					"quay.io/test/prometheus.0.14.0/preview",
					"quay.io/test/prometheus.0.14.0/stable",
					"quay.io/test/prometheus.0.15.0/preview",
					"quay.io/test/prometheus.0.15.0/stable",
					"quay.io/test/prometheus.0.22.2/preview",
				},
				remainingPkgChannels: pkgChannel{
					"prometheus": []string{
						"preview",
						"stable",
					},
				},
				remainingDefaultChannels: map[string]string{
					"prometheus": "preview",
				},
			},
		},
		{
			description: "OverwriteBundle/MultipleOverwrites",
			args: args{
				firstAdd: getBundleRefs([]string{"etcd.0.9.0", "etcd.0.9.2", "prometheus.0.14.0", "prometheus.0.15.0", "prometheus.0.22.2"}),
				secondAdd: map[image.Reference]string{
					image.SimpleReference("quay.io/test/new-etcd.0.9.2"):        "testdata/overwrite/etcd.0.9.2",
					image.SimpleReference("quay.io/test/new-prometheus.0.22.2"): "testdata/overwrite/prometheus.0.22.2",
				},
				overwrites: map[string]map[image.Reference]string{
					"prometheus": getBundleRefs([]string{"prometheus.0.14.0", "prometheus.0.15.0"}),
					"etcd":       getBundleRefs([]string{"etcd.0.9.0"}),
				},
			},
			expected: expected{
				errs: nil,
				remainingBundles: []string{
					"quay.io/test/etcd.0.9.0/alpha",
					"quay.io/test/etcd.0.9.0/beta",
					"quay.io/test/etcd.0.9.0/stable",
					"quay.io/test/new-etcd.0.9.2/alpha",
					"quay.io/test/prometheus.0.14.0/preview",
					"quay.io/test/prometheus.0.14.0/stable",
					"quay.io/test/prometheus.0.15.0/preview",
					"quay.io/test/prometheus.0.15.0/stable",
					"quay.io/test/new-prometheus.0.22.2/alpha",
					"quay.io/test/prometheus.0.15.0/alpha",
					"quay.io/test/prometheus.0.14.0/alpha",
				},
				remainingPkgChannels: pkgChannel{
					"etcd": []string{
						"beta",
						"alpha",
						"stable",
					},
					"prometheus": []string{
						"preview",
						"stable",
						"alpha",
					},
				},
				remainingDefaultChannels: map[string]string{
					"etcd":       "alpha",
					"prometheus": "preview",
				},
			},
		},
		{
			description: "OverwriteBundle/MultipleOverwritesPerPackage",
			args: args{
				firstAdd: getBundleRefs([]string{"etcd.0.9.0", "etcd.0.9.2", "prometheus.0.14.0", "prometheus.0.15.0", "prometheus.0.22.2"}),
				secondAdd: map[image.Reference]string{
					image.SimpleReference("quay.io/test/new-etcd.0.9.2"):     "testdata/overwrite/etcd.0.9.2",
					image.SimpleReference("quay.io/test/new-new-etcd.0.9.2"): "testdata/overwrite/etcd.0.9.2",
				},
				overwrites: map[string]map[image.Reference]string{
					"etcd": getBundleRefs([]string{"etcd.0.9.0"}),
				},
			},
			expected: expected{
				errs: []error{registry.OverwriteErr{ErrorString: "Cannot overwrite more than one bundle at a time for a given package using --overwrite-latest"}},
				remainingBundles: []string{
					"quay.io/test/etcd.0.9.0/alpha",
					"quay.io/test/etcd.0.9.0/beta",
					"quay.io/test/etcd.0.9.0/stable",
					"quay.io/test/etcd.0.9.2/alpha",
					"quay.io/test/etcd.0.9.2/stable",
					"quay.io/test/prometheus.0.22.2/preview",
					"quay.io/test/prometheus.0.14.0/preview",
					"quay.io/test/prometheus.0.14.0/stable",
					"quay.io/test/prometheus.0.15.0/preview",
					"quay.io/test/prometheus.0.15.0/stable",
				},
				remainingPkgChannels: pkgChannel{
					"etcd": []string{
						"alpha",
						"beta",
						"stable",
					},
					"prometheus": []string{
						"preview",
						"stable",
					},
				},
				remainingDefaultChannels: map[string]string{
					"etcd":       "alpha",
					"prometheus": "preview",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			logrus.SetLevel(logrus.DebugLevel)
			db, cleanup := CreateTestDb(t)
			defer cleanup()

			store, err := sqlite.NewSQLLiteLoader(db)
			require.NoError(t, err)
			require.NoError(t, store.Migrate(context.TODO()))

			graphLoader, err := sqlite.NewSQLGraphLoaderFromDB(db)
			require.NoError(t, err)

			query := sqlite.NewSQLLiteQuerierFromDb(db)

			populate := func(bundles map[image.Reference]string, overwrites map[string]map[image.Reference]string) error {
				return registry.NewDirectoryPopulator(
					store,
					graphLoader,
					query,
					bundles,
					overwrites,
					true).Populate(registry.ReplacesMode)
			}
			require.NoError(t, populate(tt.args.firstAdd, nil))

			err = populate(tt.args.secondAdd, tt.args.overwrites)
			if len(tt.expected.errs) < 1 {
				require.NoError(t, err)
			}
			for _, e := range tt.expected.errs {
				require.Contains(t, err.Error(), e.Error())
			}

			// Ensure remaining bundlePaths in db match
			bundles, err := query.ListBundles(context.Background())
			require.NoError(t, err)
			var bundlePaths []string
			for _, bundle := range bundles {
				bundlePaths = append(bundlePaths, strings.Join([]string{bundle.BundlePath, bundle.ChannelName}, "/"))
			}
			require.ElementsMatch(t, tt.expected.remainingBundles, bundlePaths)

			// Ensure remaining channels and default channel match
			packages, err := query.ListPackages(context.Background())
			require.NoError(t, err)

			for _, pkg := range packages {
				channelEntries, err := query.GetChannelEntriesFromPackage(context.Background(), pkg)
				require.NoError(t, err)

				uniqueChannels := make(map[string]struct{})
				var channels []string
				for _, ch := range channelEntries {
					uniqueChannels[ch.ChannelName] = struct{}{}
				}
				for k := range uniqueChannels {
					channels = append(channels, k)
				}
				defaultChannel, err := query.GetDefaultChannelForPackage(context.Background(), pkg)
				require.NoError(t, err)
				require.ElementsMatch(t, tt.expected.remainingPkgChannels[pkg], channels)
				require.Equal(t, tt.expected.remainingDefaultChannels[pkg], defaultChannel)
			}
		})
	}
}

func TestSemverPackageManifest(t *testing.T) {
	bundle := func(name, version, pkg, defaultChannel, channels string) *registry.Bundle {
		b, err := registry.NewBundleFromStrings(name, version, pkg, defaultChannel, channels, "")
		require.NoError(t, err)
		return b
	}
	type args struct {
		bundles []*registry.Bundle
	}
	type expect struct {
		packageManifest *registry.PackageManifest
		hasError        bool
	}
	for _, tt := range []struct {
		description string
		args        args
		expect      expect
	}{
		{
			description: "OneUnversioned",
			args: args{
				bundles: []*registry.Bundle{
					bundle("operator", "", "package", "stable", "stable"), // version "" is interpreted as 0.0.0-z
				},
			},
			expect: expect{
				packageManifest: &registry.PackageManifest{
					PackageName:        "package",
					DefaultChannelName: "stable",
					Channels: []registry.PackageChannel{
						{
							Name:           "stable",
							CurrentCSVName: "operator",
						},
					},
				},
			},
		},
		{
			description: "TwoUnversioned",
			args: args{
				bundles: []*registry.Bundle{
					bundle("operator-1", "", "package", "stable", "stable"),
					bundle("operator-2", "", "package", "stable", "stable"),
				},
			},
			expect: expect{
				hasError: true,
			},
		},
		{
			description: "UnversionedAndVersioned",
			args: args{
				bundles: []*registry.Bundle{
					bundle("operator-1", "", "package", "", "stable"),
					bundle("operator-2", "", "package", "", "stable"),
					bundle("operator-3", "0.0.1", "package", "", "stable"), // As long as there is one version, we should be good
				},
			},
			expect: expect{
				packageManifest: &registry.PackageManifest{
					PackageName:        "package",
					DefaultChannelName: "stable",
					Channels: []registry.PackageChannel{
						{
							Name:           "stable",
							CurrentCSVName: "operator-3",
						},
					},
				},
			},
		},
		{
			description: "MaxVersionsAreChannelHeads",
			args: args{
				bundles: []*registry.Bundle{
					bundle("operator-1", "1.0.0", "package", "slow", "slow"),
					bundle("operator-2", "1.1.0", "package", "stable", "slow,stable"),
					bundle("operator-3", "2.1.0", "package", "stable", "edge"),
				},
			},
			expect: expect{
				packageManifest: &registry.PackageManifest{
					PackageName:        "package",
					DefaultChannelName: "stable",
					Channels: []registry.PackageChannel{
						{
							Name:           "slow",
							CurrentCSVName: "operator-2",
						},
						{
							Name:           "stable",
							CurrentCSVName: "operator-2",
						},
						{
							Name:           "edge",
							CurrentCSVName: "operator-3",
						},
					},
				},
			},
		},
		{
			description: "DuplicateVersionsNotTolerated",
			args: args{
				bundles: []*registry.Bundle{
					bundle("operator-1", "1.0.0", "package", "slow", "slow"),
					bundle("operator-2", "1.0.0", "package", "stable", "slow,stable"),
					bundle("operator-3", "2.1.0", "package", "stable", "edge"),
				},
			},
			expect: expect{
				hasError: true,
			},
		},
		{
			description: "DuplicateVersionsInSeparateChannelsAreTolerated",
			args: args{
				bundles: []*registry.Bundle{
					bundle("operator-1", "1.0.0", "package", "slow", "slow"),
					bundle("operator-2", "1.0.0", "package", "stable", "stable"),
					bundle("operator-3", "2.1.0", "package", "edge", "edge"), // Should only be tolerated if we have a global max
				},
			},
			expect: expect{
				packageManifest: &registry.PackageManifest{
					PackageName:        "package",
					DefaultChannelName: "edge",
					Channels: []registry.PackageChannel{
						{
							Name:           "slow",
							CurrentCSVName: "operator-1",
						},
						{
							Name:           "stable",
							CurrentCSVName: "operator-2",
						},
						{
							Name:           "edge",
							CurrentCSVName: "operator-3",
						},
					},
				},
			},
		},
		{
			description: "DuplicateMaxVersionsAreNotTolerated",
			args: args{
				bundles: []*registry.Bundle{
					bundle("operator-1", "1.0.0", "package", "slow", "slow"),
					bundle("operator-2", "1.0.0", "package", "stable", "stable"),
				},
			},
			expect: expect{
				hasError: true,
			},
		},
		{
			description: "UnknownDefaultChannel",
			args: args{
				bundles: []*registry.Bundle{
					bundle("operator-1", "1.0.0", "package", "stable", "stable"),
					bundle("operator-2", "2.0.0", "package", "edge", "stable"),
				},
			},
			expect: expect{
				hasError: true,
			},
		},
		{
			description: "BuildIDAndPreReleaseHeads",
			args: args{
				bundles: []*registry.Bundle{
					bundle("operator-1", "1.0.0", "package", "stable", "stable"),
					bundle("operator-2", "1.0.0+1", "package", "stable", "stable"),
					bundle("operator-3", "1.0.0+2", "package", "stable", "stable"),
					bundle("operator-4", "2.0.0-pre", "package", "stable", "edge"),
				},
			},
			expect: expect{
				packageManifest: &registry.PackageManifest{
					PackageName:        "package",
					DefaultChannelName: "stable",
					Channels: []registry.PackageChannel{
						{
							Name:           "stable",
							CurrentCSVName: "operator-3",
						},
						{
							Name:           "edge",
							CurrentCSVName: "operator-4",
						},
					},
				},
			},
		},
	} {
		t.Run(tt.description, func(t *testing.T) {
			packageManifest, err := registry.SemverPackageManifest(tt.args.bundles)
			if tt.expect.hasError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, packageManifest)

			expected := tt.expect.packageManifest
			require.Equal(t, expected.PackageName, packageManifest.PackageName)
			require.Equal(t, expected.DefaultChannelName, packageManifest.DefaultChannelName)
			require.ElementsMatch(t, expected.Channels, packageManifest.Channels)
		})
	}
}

func TestSubstitutesFor(t *testing.T) {
	type args struct {
		bundles  []string
		rebuilds []string
	}
	type replaces map[string]string
	type expected struct {
		bundles        []string
		substitutions  map[string]string
		whatReplaces   map[string]replaces
		defaultChannel string
	}
	tests := []struct {
		description string
		args        args
		expected    expected
	}{
		{
			description: "FirstSubstitutionReplaces",
			args: args{
				bundles: []string{
					"prometheus.0.22.2",
					"prometheus.0.14.0",
					"prometheus.0.15.0",
				},
				rebuilds: []string{
					"prometheus.0.15.0.substitutesfor",
				},
			},
			expected: expected{
				bundles: []string{
					"quay.io/test/prometheus.0.22.2",
					"quay.io/test/prometheus.0.14.0",
					"quay.io/test/prometheus.0.15.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.14.0",
				},
				substitutions: map[string]string{
					"prometheusoperator.0.22.2":                      "",
					"prometheusoperator.0.14.0":                      "",
					"prometheusoperator.0.15.0":                      "",
					"prometheusoperator.0.15.0+1-freshmaker-rebuild": "prometheusoperator.0.15.0",
				},
				whatReplaces: map[string]replaces{
					"prometheusoperator.0.22.2": {
						"preview": "",
					},
					"prometheusoperator.0.14.0": {
						"preview": "prometheusoperator.0.15.0+1-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+1-freshmaker-rebuild",
					},
					"prometheusoperator.0.15.0": {
						"preview": "prometheusoperator.0.15.0+1-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+1-freshmaker-rebuild",
					},
					"prometheusoperator.0.15.0+1-freshmaker-rebuild": {
						"preview": "prometheusoperator.0.22.2",
						"stable":  "",
					},
				},
				defaultChannel: "preview",
			},
		},
		{
			description: "FirstSubstitutionReplacesNonHead",
			args: args{
				bundles: []string{
					"prometheus.0.22.2",
					"prometheus.0.14.0",
					"prometheus.0.15.0",
				},
				rebuilds: []string{
					"prometheus.0.14.0.substitutesfor",
				},
			},
			expected: expected{
				bundles: []string{
					"quay.io/test/prometheus.0.22.2",
					"quay.io/test/prometheus.0.14.0",
					"quay.io/test/prometheus.0.14.0.substitutesfor",
					"quay.io/test/prometheus.0.14.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.14.0",
				},
				substitutions: map[string]string{
					"prometheusoperator.0.22.2":                "",
					"prometheusoperator.0.14.0":                "",
					"prometheusoperator.0.15.0":                "",
					"prometheusoperator.0.14.0+0.1234-rebuild": "prometheusoperator.0.14.0",
				},
				whatReplaces: map[string]replaces{
					"prometheusoperator.0.22.2": {
						"preview": "",
					},
					"prometheusoperator.0.14.0": {
						"preview": "prometheusoperator.0.14.0+0.1234-rebuild",
						"stable":  "prometheusoperator.0.14.0+0.1234-rebuild",
					},
					"prometheusoperator.0.15.0": {
						"preview": "prometheusoperator.0.22.2",
						"stable":  "",
					},
					"prometheusoperator.0.14.0+0.1234-rebuild": {
						"preview": "prometheusoperator.0.15.0",
						"stable":  "prometheusoperator.0.15.0",
					},
				},
				defaultChannel: "preview",
			},
		},
		{
			description: "SecondSubstitutionReplaces",
			args: args{
				bundles: []string{
					"prometheus.0.22.2",
					"prometheus.0.14.0",
					"prometheus.0.15.0",
				},
				rebuilds: []string{
					"prometheus.0.15.0.substitutesfor",
					"prometheus.0.15.0.substitutesfor2",
				},
			},
			expected: expected{
				bundles: []string{
					"quay.io/test/prometheus.0.22.2",
					"quay.io/test/prometheus.0.14.0",
					"quay.io/test/prometheus.0.15.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0.substitutesfor2",
					"quay.io/test/prometheus.0.15.0.substitutesfor2",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.14.0",
				},
				substitutions: map[string]string{
					"prometheusoperator.0.22.2":                      "",
					"prometheusoperator.0.14.0":                      "",
					"prometheusoperator.0.15.0":                      "",
					"prometheusoperator.0.15.0+1-freshmaker-rebuild": "prometheusoperator.0.15.0",
					"prometheusoperator.0.15.0+2-freshmaker-rebuild": "prometheusoperator.0.15.0+1-freshmaker-rebuild",
				},
				whatReplaces: map[string]replaces{
					"prometheusoperator.0.22.2": {
						"preview": "",
					},
					"prometheusoperator.0.14.0": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
					"prometheusoperator.0.15.0": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
					"prometheusoperator.0.15.0+2-freshmaker-rebuild": {
						"preview": "prometheusoperator.0.22.2",
						"stable":  "",
					},
					"prometheusoperator.0.15.0+1-freshmaker-rebuild": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
				},
				defaultChannel: "preview",
			},
		},
		{
			description: "SecondSubstitutionReplacesWithNonHead",
			args: args{
				bundles: []string{
					"prometheus.0.22.2",
					"prometheus.0.14.0",
					"prometheus.0.15.0",
				},
				rebuilds: []string{
					"prometheus.0.14.0.substitutesfor",
					"prometheus.0.15.0.substitutesfor",
					"prometheus.0.15.0.substitutesfor2",
				},
			},
			expected: expected{
				bundles: []string{
					"quay.io/test/prometheus.0.22.2",
					"quay.io/test/prometheus.0.14.0",
					"quay.io/test/prometheus.0.15.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0.substitutesfor2",
					"quay.io/test/prometheus.0.15.0.substitutesfor2",
					"quay.io/test/prometheus.0.14.0.substitutesfor",
					"quay.io/test/prometheus.0.14.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.14.0",
				},
				substitutions: map[string]string{
					"prometheusoperator.0.22.2":                      "",
					"prometheusoperator.0.14.0":                      "",
					"prometheusoperator.0.15.0":                      "",
					"prometheusoperator.0.14.0+0.1234-rebuild":       "prometheusoperator.0.14.0",
					"prometheusoperator.0.15.0+1-freshmaker-rebuild": "prometheusoperator.0.15.0",
					"prometheusoperator.0.15.0+2-freshmaker-rebuild": "prometheusoperator.0.15.0+1-freshmaker-rebuild",
				},
				whatReplaces: map[string]replaces{
					"prometheusoperator.0.22.2": {
						"preview": "",
					},
					"prometheusoperator.0.15.0": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
					"prometheusoperator.0.15.0+2-freshmaker-rebuild": {
						"preview": "prometheusoperator.0.22.2",
						"stable":  "",
					},
					"prometheusoperator.0.15.0+1-freshmaker-rebuild": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
					"prometheusoperator.0.14.0": {
						"preview": "prometheusoperator.0.14.0+0.1234-rebuild",
						"stable":  "prometheusoperator.0.14.0+0.1234-rebuild",
					},
					"prometheusoperator.0.14.0+0.1234-rebuild": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
				},
				defaultChannel: "preview",
			},
		},
		{
			description: "CanBatchAddSubstitutesFor",
			args: args{
				bundles: []string{
					"prometheus.0.14.0",
					"prometheus.0.15.0.substitutesfor",
					"prometheus.0.15.0.substitutesfor2",
					"prometheus.0.15.0",
				},
				rebuilds: []string{},
			},
			expected: expected{
				bundles: []string{
					"quay.io/test/prometheus.0.14.0",
					"quay.io/test/prometheus.0.15.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0.substitutesfor2",
					"quay.io/test/prometheus.0.15.0.substitutesfor2",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.14.0",
				},
				substitutions: map[string]string{
					"prometheusoperator.0.14.0":                      "",
					"prometheusoperator.0.15.0":                      "",
					"prometheusoperator.0.15.0+1-freshmaker-rebuild": "prometheusoperator.0.15.0",
					"prometheusoperator.0.15.0+2-freshmaker-rebuild": "prometheusoperator.0.15.0+1-freshmaker-rebuild",
				},
				whatReplaces: map[string]replaces{
					"prometheusoperator.0.15.0": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
					"prometheusoperator.0.15.0+2-freshmaker-rebuild": {
						"preview": "",
						"stable":  "",
					},
					"prometheusoperator.0.15.0+1-freshmaker-rebuild": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
					"prometheusoperator.0.14.0": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
				},
				defaultChannel: "preview",
			},
		},
		{
			description: "CanBatchAddSubstitutesForWithNonHead",
			args: args{
				bundles: []string{
					"prometheus.0.14.0",
					"prometheus.0.14.0.substitutesfor",
					"prometheus.0.15.0.substitutesfor",
					"prometheus.0.15.0.substitutesfor2",
					"prometheus.0.15.0",
				},
				rebuilds: []string{},
			},
			expected: expected{
				bundles: []string{
					"quay.io/test/prometheus.0.14.0",
					"quay.io/test/prometheus.0.15.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0.substitutesfor2",
					"quay.io/test/prometheus.0.15.0.substitutesfor2",
					"quay.io/test/prometheus.0.14.0.substitutesfor",
					"quay.io/test/prometheus.0.14.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.14.0",
				},
				substitutions: map[string]string{
					"prometheusoperator.0.14.0":                      "",
					"prometheusoperator.0.15.0":                      "",
					"prometheusoperator.0.14.0+0.1234-rebuild":       "prometheusoperator.0.14.0",
					"prometheusoperator.0.15.0+1-freshmaker-rebuild": "prometheusoperator.0.15.0",
					"prometheusoperator.0.15.0+2-freshmaker-rebuild": "prometheusoperator.0.15.0+1-freshmaker-rebuild",
				},
				whatReplaces: map[string]replaces{
					"prometheusoperator.0.15.0": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
					"prometheusoperator.0.15.0+2-freshmaker-rebuild": {
						"preview": "",
						"stable":  "",
					},
					"prometheusoperator.0.15.0+1-freshmaker-rebuild": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
					"prometheusoperator.0.14.0": {
						"preview": "prometheusoperator.0.14.0+0.1234-rebuild",
						"stable":  "prometheusoperator.0.14.0+0.1234-rebuild",
					},
					"prometheusoperator.0.14.0+0.1234-rebuild": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
				},
				defaultChannel: "preview",
			},
		},
		{
			description: "CanAddABundleThatReplacesSubstitutedOne",
			args: args{
				bundles: []string{
					"prometheus.0.14.0",
					"prometheus.0.15.0",
					"prometheus.0.15.0.substitutesfor",
					"prometheus.0.15.0.substitutesfor2",
				},
				rebuilds: []string{
					"prometheus.0.22.2",
				},
			},
			expected: expected{
				bundles: []string{
					"quay.io/test/prometheus.0.22.2",
					"quay.io/test/prometheus.0.14.0",
					"quay.io/test/prometheus.0.15.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0.substitutesfor2",
					"quay.io/test/prometheus.0.15.0.substitutesfor2",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.14.0",
				},
				substitutions: map[string]string{
					"prometheusoperator.0.22.2":                      "",
					"prometheusoperator.0.14.0":                      "",
					"prometheusoperator.0.15.0":                      "",
					"prometheusoperator.0.15.0+1-freshmaker-rebuild": "prometheusoperator.0.15.0",
					"prometheusoperator.0.15.0+2-freshmaker-rebuild": "prometheusoperator.0.15.0+1-freshmaker-rebuild",
				},
				whatReplaces: map[string]replaces{
					"prometheusoperator.0.22.2": {
						"preview": "",
					},
					"prometheusoperator.0.14.0": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
					"prometheusoperator.0.15.0": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
					"prometheusoperator.0.15.0+2-freshmaker-rebuild": {
						"preview": "prometheusoperator.0.22.2",
						"stable":  "",
					},
					"prometheusoperator.0.15.0+1-freshmaker-rebuild": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
				},
				defaultChannel: "preview",
			},
		},
		{
			description: "CanAddABundleThatReplacesSubstitutedOneWithNonHead",
			args: args{
				bundles: []string{
					"prometheus.0.14.0",
					"prometheus.0.15.0",
					"prometheus.0.15.0.substitutesfor",
					"prometheus.0.15.0.substitutesfor2",
					"prometheus.0.14.0.substitutesfor",
				},
				rebuilds: []string{
					"prometheus.0.22.2",
				},
			},
			expected: expected{
				bundles: []string{
					"quay.io/test/prometheus.0.22.2",
					"quay.io/test/prometheus.0.14.0",
					"quay.io/test/prometheus.0.15.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0.substitutesfor2",
					"quay.io/test/prometheus.0.15.0.substitutesfor2",
					"quay.io/test/prometheus.0.14.0.substitutesfor",
					"quay.io/test/prometheus.0.14.0.substitutesfor",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.15.0",
					"quay.io/test/prometheus.0.14.0",
				},
				substitutions: map[string]string{
					"prometheusoperator.0.22.2":                      "",
					"prometheusoperator.0.14.0":                      "",
					"prometheusoperator.0.15.0":                      "",
					"prometheusoperator.0.14.0+0.1234-rebuild":       "prometheusoperator.0.14.0",
					"prometheusoperator.0.15.0+1-freshmaker-rebuild": "prometheusoperator.0.15.0",
					"prometheusoperator.0.15.0+2-freshmaker-rebuild": "prometheusoperator.0.15.0+1-freshmaker-rebuild",
				},
				whatReplaces: map[string]replaces{
					"prometheusoperator.0.22.2": {
						"preview": "",
					},
					"prometheusoperator.0.15.0": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
					"prometheusoperator.0.15.0+2-freshmaker-rebuild": {
						"preview": "prometheusoperator.0.22.2",
						"stable":  "",
					},
					"prometheusoperator.0.15.0+1-freshmaker-rebuild": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
					"prometheusoperator.0.14.0": {
						"preview": "prometheusoperator.0.14.0+0.1234-rebuild",
						"stable":  "prometheusoperator.0.14.0+0.1234-rebuild",
					},
					"prometheusoperator.0.14.0+0.1234-rebuild": {
						"preview": "prometheusoperator.0.15.0+2-freshmaker-rebuild",
						"stable":  "prometheusoperator.0.15.0+2-freshmaker-rebuild",
					},
				},
				defaultChannel: "preview",
			},
		},
		{
			description: "MultipleBundlesSubstitutingForTheSameBundleAddedInWrongOrder",
			args: args{
				bundles: []string{
					"prometheus.0.14.0",
				},
				rebuilds: []string{
					"prometheus.0.14.0.substitutesfor",
					"prometheus.0.14.0.substitutesfor3",
					"prometheus.0.14.0.substitutesfor2",
				},
			},
			expected: expected{
				bundles: []string{
					"quay.io/test/prometheus.0.14.0",
					"quay.io/test/prometheus.0.14.0.substitutesfor2",
					"quay.io/test/prometheus.0.14.0.substitutesfor3",
					"quay.io/test/prometheus.0.14.0.substitutesfor",
				},
				substitutions: map[string]string{
					"prometheusoperator.0.14.0":                "",
					"prometheusoperator.0.14.0+0.1234-rebuild": "prometheusoperator.0.14.0",
					"prometheusoperator.0.14.0+2-rebuild":      "prometheusoperator.0.14.0+0.1234-rebuild",
					"prometheusoperator.0.14.0+3-rebuild":      "prometheusoperator.0.14.0+2-rebuild",
				},
				whatReplaces: map[string]replaces{
					"prometheusoperator.0.14.0": {
						"preview": "prometheusoperator.0.14.0+3-rebuild",
					},
					"prometheusoperator.0.14.0+0.1234-rebuild": {
						"preview": "prometheusoperator.0.14.0+3-rebuild",
					},
					"prometheusoperator.0.14.0+2-rebuild": {
						"preview": "prometheusoperator.0.14.0+3-rebuild",
					},
					"prometheusoperator.0.14.0+3-rebuild": {
						"preview": "",
					},
				},
				defaultChannel: "preview",
			},
		},
		{
			description: "BatchAddWithMultipleBundlesSubstitutingForTheSameBundle",
			args: args{
				bundles: []string{
					"prometheus.0.14.0",
					"prometheus.0.14.0.substitutesfor",
					"prometheus.0.14.0.substitutesfor3",
					"prometheus.0.14.0.substitutesfor2",
				},
				rebuilds: []string{},
			},
			expected: expected{
				bundles: []string{
					"quay.io/test/prometheus.0.14.0",
					"quay.io/test/prometheus.0.14.0.substitutesfor3",
					"quay.io/test/prometheus.0.14.0.substitutesfor2",
					"quay.io/test/prometheus.0.14.0.substitutesfor",
				},
				substitutions: map[string]string{
					"prometheusoperator.0.14.0":                "",
					"prometheusoperator.0.14.0+0.1234-rebuild": "prometheusoperator.0.14.0",
					"prometheusoperator.0.14.0+2-rebuild":      "prometheusoperator.0.14.0+0.1234-rebuild",
					"prometheusoperator.0.14.0+3-rebuild":      "prometheusoperator.0.14.0+2-rebuild",
				},
				whatReplaces: map[string]replaces{
					"prometheusoperator.0.14.0": {
						"preview": "prometheusoperator.0.14.0+3-rebuild",
					},
					"prometheusoperator.0.14.0+0.1234-rebuild": {
						"preview": "prometheusoperator.0.14.0+3-rebuild",
					},
					"prometheusoperator.0.14.0+2-rebuild": {
						"preview": "prometheusoperator.0.14.0+3-rebuild",
					},
					"prometheusoperator.0.14.0+3-rebuild": {
						"preview": "",
					},
				},
				defaultChannel: "preview",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			logrus.SetLevel(logrus.DebugLevel)
			db, cleanup := CreateTestDb(t)
			defer cleanup()

			load, err := sqlite.NewSQLLiteLoader(db, sqlite.WithEnableAlpha(true))
			require.NoError(t, err)
			err = load.Migrate(context.TODO())
			require.NoError(t, err)
			query := sqlite.NewSQLLiteQuerierFromDb(db)

			graphLoader, err := sqlite.NewSQLGraphLoaderFromDB(db)
			require.NoError(t, err)

			populate := func(names []string) error {
				refMap := make(map[image.Reference]string, 0)
				for _, name := range names {
					refMap[image.SimpleReference("quay.io/test/"+name)] = "../../bundles/" + name
				}
				return registry.NewDirectoryPopulator(
					load,
					graphLoader,
					query,
					refMap,
					make(map[string]map[image.Reference]string, 0), false).Populate(registry.ReplacesMode)
			}
			// Initialize index with some bundles
			require.NoError(t, populate(tt.args.bundles))

			// Add the rebuilds one at a time
			for _, rb := range tt.args.rebuilds {
				require.NoError(t, populate([]string{rb}))
			}

			// Check graph is unchanged but has new csv name + version
			// Ensure bundlePaths in db match
			bundles, err := query.ListBundles(context.Background())
			require.NoError(t, err)
			var bundlePaths []string
			for _, bundle := range bundles {
				bundlePaths = append(bundlePaths, bundle.BundlePath)
				bundleThatReplaces, _ := query.GetBundleThatReplaces(context.Background(), bundle.CsvName, bundle.PackageName, bundle.ChannelName)
				if bundleThatReplaces != nil {
					require.Equal(t, tt.expected.whatReplaces[bundle.CsvName][bundle.ChannelName], bundleThatReplaces.CsvName)
				} else {
					require.Equal(t, tt.expected.whatReplaces[bundle.CsvName][bundle.ChannelName], "")
				}
				substitution, err := getBundleSubstitution(context.Background(), db, bundle.CsvName)
				require.NoError(t, err)
				require.Equal(t, tt.expected.substitutions[bundle.CsvName], substitution)
			}
			require.ElementsMatch(t, tt.expected.bundles, bundlePaths)

			// check default channel
			defaultChannel, err := query.GetDefaultChannelForPackage(context.Background(), "prometheus")
			require.NoError(t, err)
			require.Equal(t, tt.expected.defaultChannel, defaultChannel)
		})
	}
}

func getBundleSubstitution(ctx context.Context, db *sql.DB, name string) (string, error) {
	query := `SELECT substitutesfor FROM operatorbundle WHERE name=?`
	rows, err := db.QueryContext(ctx, query, name)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var substitutesFor sql.NullString
	if rows.Next() {
		if err := rows.Scan(&substitutesFor); err != nil {
			return "", err
		}
	}
	return substitutesFor.String, nil
}

func TestEnableAlpha(t *testing.T) {
	type args struct {
		bundles     []string
		enableAlpha bool
	}
	type expected struct {
		err error
	}
	tests := []struct {
		description string
		args        args
		expected    expected
	}{
		{
			description: "SubstitutesForTrue",
			args: args{
				bundles: []string{
					"prometheus.0.22.2",
					"prometheus.0.14.0",
					"prometheus.0.15.0",
					"prometheus.0.15.0.substitutesfor",
				},
				enableAlpha: true,
			},
			expected: expected{
				err: nil,
			},
		},
		{
			description: "SubstitutesForFalse",
			args: args{
				bundles: []string{
					"prometheus.0.22.2",
					"prometheus.0.14.0",
					"prometheus.0.15.0",
					"prometheus.0.15.0.substitutesfor",
				},
				enableAlpha: false,
			},
			expected: expected{
				err: errors.NewAggregate([]error{fmt.Errorf("SubstitutesFor is an alpha-only feature. You must enable alpha features with the flag --enable-alpha in order to use this feature.")}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			logrus.SetLevel(logrus.DebugLevel)
			db, cleanup := CreateTestDb(t)
			defer cleanup()

			load, err := sqlite.NewSQLLiteLoader(db, sqlite.WithEnableAlpha(tt.args.enableAlpha))
			require.NoError(t, err)
			err = load.Migrate(context.TODO())
			require.NoError(t, err)
			query := sqlite.NewSQLLiteQuerierFromDb(db)

			graphLoader, err := sqlite.NewSQLGraphLoaderFromDB(db)
			require.NoError(t, err)

			populate := func(names []string) error {
				refMap := make(map[image.Reference]string, 0)
				for _, name := range names {
					refMap[image.SimpleReference("quay.io/test/"+name)] = "../../bundles/" + name
				}
				return registry.NewDirectoryPopulator(
					load,
					graphLoader,
					query,
					refMap,
					make(map[string]map[image.Reference]string, 0), false).Populate(registry.ReplacesMode)
			}
			require.Equal(t, tt.expected.err, populate(tt.args.bundles))
		})
	}
}
