package server

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

const (
	dbPort    = ":50052"
	dbAddress = "localhost" + dbPort
	dbName    = "test.db"

	cfgPort    = ":50053"
	cfgAddress = "localhost" + cfgPort
)

func dbStore(dbPath string) *sqlite.SQLQuerier {
	_ = os.Remove(dbPath)

	db, err := sqlite.Open(dbPath)
	if err != nil {
		logrus.Fatal(err)
	}
	load, err := sqlite.NewSQLLiteLoader(db)
	if err != nil {
		logrus.Fatal(err)
	}
	if err := load.Migrate(context.TODO()); err != nil {
		logrus.Fatal(err)
	}

	loader := sqlite.NewSQLLoaderForDirectory(load, "../../manifests")
	if err := loader.Populate(); err != nil {
		logrus.Fatal(err)
	}
	if err := db.Close(); err != nil {
		logrus.Fatal(err)
	}
	store, err := sqlite.NewSQLLiteQuerier(dbPath)
	if err != nil {
		logrus.Fatal(err)
	}
	return store
}

func cfgStore() *registry.Querier {
	tmpDir, err := ioutil.TempDir("", "server_test-")
	if err != nil {
		logrus.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbFile := filepath.Join(tmpDir, "test.db")

	dbStore := dbStore(dbFile)
	m, err := sqlite.ToModel(context.TODO(), dbStore)
	if err != nil {
		logrus.Fatal(err)
	}
	store := registry.NewQuerier(m)
	return store
}

func server(store registry.GRPCQuery) *grpc.Server {
	s := grpc.NewServer()
	api.RegisterRegistryServer(s, NewRegistryServer(store))
	return s
}

func TestMain(m *testing.M) {
	s1 := server(dbStore(dbName))
	s2 := server(cfgStore())
	go func() {
		lis, err := net.Listen("tcp", dbPort)
		if err != nil {
			logrus.Fatalf("failed to listen: %v", err)
		}
		if err := s1.Serve(lis); err != nil {
			logrus.Fatalf("failed to serve db: %v", err)
		}
	}()
	go func() {
		lis, err := net.Listen("tcp", cfgPort)
		if err != nil {
			logrus.Fatalf("failed to listen: %v", err)
		}
		if err := s2.Serve(lis); err != nil {
			logrus.Fatalf("failed to serve configs: %v", err)
		}
	}()
	exit := m.Run()
	if err := os.Remove(dbName); err != nil {
		logrus.Fatalf("couldn't remove db")
	}
	os.Exit(exit)
}

func client(t *testing.T, address string) (api.RegistryClient, *grpc.ClientConn) {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}

	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	conn.WaitForStateChange(ctx, connectivity.TransientFailure)

	return api.NewRegistryClient(conn), conn
}

func TestListPackages(t *testing.T) {
	t.Run("Sqlite", testListPackages(dbAddress))
	t.Run("DeclarativeConfig", testListPackages(cfgAddress))
}

func testListPackages(addr string) func(*testing.T) {
	return func(t *testing.T) {
		c, conn := client(t, addr)
		defer conn.Close()

		stream, err := c.ListPackages(context.TODO(), &api.ListPackageRequest{})
		require.NoError(t, err)

		packages := []string{}
		waitc := make(chan struct{})
		go func(t *testing.T) {
			for {
				in, err := stream.Recv()
				if err == io.EOF {
					// read done.
					close(waitc)
					return
				}
				require.NoError(t, err)
				packages = append(packages, in.Name)
			}
		}(t)
		<-waitc
		require.ElementsMatch(t, []string{"etcd", "prometheus", "strimzi-kafka-operator"}, packages)
	}
}

func TestGetPackage(t *testing.T) {
	t.Run("Sqlite", testGetPackage(dbAddress))
	t.Run("DeclarativeConfig", testGetPackage(cfgAddress))
}

func testGetPackage(addr string) func(*testing.T) {
	return func(t *testing.T) {
		c, conn := client(t, addr)
		defer conn.Close()

		pkg, err := c.GetPackage(context.TODO(), &api.GetPackageRequest{Name: "etcd"})
		require.NoError(t, err)
		expected := &api.Package{
			Name: "etcd",
			Channels: []*api.Channel{
				{
					Name:    "alpha",
					CsvName: "etcdoperator.v0.9.2",
				},
				{
					Name:    "beta",
					CsvName: "etcdoperator.v0.9.0",
				},
				{
					Name:    "stable",
					CsvName: "etcdoperator.v0.9.2",
				},
			},
			DefaultChannelName: "alpha",
		}
		opts := []cmp.Option{
			cmpopts.IgnoreUnexported(api.Package{}),
			cmpopts.IgnoreUnexported(api.Channel{}),
			cmpopts.SortSlices(func(x, y *api.Channel) bool {
				return x.Name < y.Name
			}),
		}
		require.True(t, cmp.Equal(expected, pkg, opts...), cmp.Diff(expected, pkg, opts...))
	}
}

func TestGetBundle(t *testing.T) {
	t.Run("Sqlite", testGetBundle(dbAddress, etcdoperator_v0_9_2("alpha", false, false)))
	t.Run("DeclarativeConfig", testGetBundle(cfgAddress, etcdoperator_v0_9_2("alpha", false, true)))
}

func testGetBundle(addr string, expected *api.Bundle) func(*testing.T) {
	return func(t *testing.T) {
		c, conn := client(t, addr)
		defer conn.Close()

		bundle, err := c.GetBundle(context.TODO(), &api.GetBundleRequest{PkgName: "etcd", ChannelName: "alpha", CsvName: "etcdoperator.v0.9.2"})
		require.NoError(t, err)

		EqualBundles(t, *expected, *bundle)
	}
}

func TestGetBundleForChannel(t *testing.T) {
	t.Run("Sqlite", testGetBundleForChannel(dbAddress, etcdoperator_v0_9_2("alpha", false, false)))
	t.Run("DeclarativeConfig", testGetBundleForChannel(cfgAddress, etcdoperator_v0_9_2("alpha", false, true)))
}

func testGetBundleForChannel(addr string, expected *api.Bundle) func(*testing.T) {
	return func(t *testing.T) {
		c, conn := client(t, addr)
		defer conn.Close()

		bundle, err := c.GetBundleForChannel(context.TODO(), &api.GetBundleInChannelRequest{PkgName: "etcd", ChannelName: "alpha"})
		require.NoError(t, err)
		EqualBundles(t, *expected, *bundle)
	}
}

func TestGetChannelEntriesThatReplace(t *testing.T) {
	t.Run("Sqlite", testGetChannelEntriesThatReplace(dbAddress))
	t.Run("DeclarativeConfig", testGetChannelEntriesThatReplace(cfgAddress))
}

func testGetChannelEntriesThatReplace(addr string) func(*testing.T) {
	return func(t *testing.T) {
		c, conn := client(t, addr)
		defer conn.Close()

		stream, err := c.GetChannelEntriesThatReplace(context.TODO(), &api.GetAllReplacementsRequest{CsvName: "etcdoperator.v0.6.1"})
		require.NoError(t, err)

		channelEntries := []*api.ChannelEntry{}
		waitc := make(chan struct{})
		go func(t *testing.T) {
			for {
				in, err := stream.Recv()
				if err == io.EOF {
					// read done.
					close(waitc)
					return
				}
				if err != nil {
					t.Error(err)
					close(waitc)
					return
				}
				channelEntries = append(channelEntries, in)
			}
		}(t)
		<-waitc

		expected := []*api.ChannelEntry{
			{
				PackageName: "etcd",
				ChannelName: "alpha",
				BundleName:  "etcdoperator.v0.9.0",
				Replaces:    "etcdoperator.v0.6.1",
			},
			{
				PackageName: "etcd",
				ChannelName: "beta",
				BundleName:  "etcdoperator.v0.9.0",
				Replaces:    "etcdoperator.v0.6.1",
			},
			{
				PackageName: "etcd",
				ChannelName: "stable",
				BundleName:  "etcdoperator.v0.9.0",
				Replaces:    "etcdoperator.v0.6.1",
			},
		}
		opts := []cmp.Option{
			cmpopts.IgnoreUnexported(api.ChannelEntry{}),
			cmpopts.SortSlices(func(x, y *api.ChannelEntry) bool {
				if x.PackageName != y.PackageName {
					return x.PackageName < y.PackageName
				}
				if x.ChannelName != y.ChannelName {
					return x.ChannelName < y.ChannelName
				}
				if x.BundleName != y.BundleName {
					return x.BundleName < y.BundleName
				}
				if x.Replaces != y.Replaces {
					return x.Replaces < y.Replaces
				}
				return false
			}),
		}

		require.Truef(t, cmp.Equal(expected, channelEntries, opts...), cmp.Diff(expected, channelEntries, opts...))
	}
}

func TestGetBundleThatReplaces(t *testing.T) {
	t.Run("Sqlite", testGetBundleThatReplaces(dbAddress, etcdoperator_v0_9_2("alpha", false, false)))
	t.Run("DeclarativeConfig", testGetBundleThatReplaces(cfgAddress, etcdoperator_v0_9_2("alpha", false, true)))
}

func testGetBundleThatReplaces(addr string, expected *api.Bundle) func(*testing.T) {
	return func(t *testing.T) {
		c, conn := client(t, addr)
		defer conn.Close()

		bundle, err := c.GetBundleThatReplaces(context.TODO(), &api.GetReplacementRequest{CsvName: "etcdoperator.v0.9.0", PkgName: "etcd", ChannelName: "alpha"})
		require.NoError(t, err)
		EqualBundles(t, *expected, *bundle)
	}
}

func TestGetBundleThatReplacesSynthetic(t *testing.T) {
	t.Run("Sqlite", testGetBundleThatReplacesSynthetic(dbAddress, etcdoperator_v0_9_2("alpha", false, false)))
	t.Run("DeclarativeConfig", testGetBundleThatReplacesSynthetic(cfgAddress, etcdoperator_v0_9_2("alpha", false, true)))
}

func testGetBundleThatReplacesSynthetic(addr string, expected *api.Bundle) func(*testing.T) {
	return func(t *testing.T) {
		c, conn := client(t, addr)
		defer conn.Close()

		// 0.9.1 is not actually a bundle in the registry
		bundle, err := c.GetBundleThatReplaces(context.TODO(), &api.GetReplacementRequest{CsvName: "etcdoperator.v0.9.1", PkgName: "etcd", ChannelName: "alpha"})
		require.NoError(t, err)
		EqualBundles(t, *expected, *bundle)
	}
}

func TestGetChannelEntriesThatProvide(t *testing.T) {
	t.Run("Sqlite", testGetChannelEntriesThatProvide(dbAddress))
	t.Run("DeclarativeConfig", testGetChannelEntriesThatProvide(cfgAddress))
}

func testGetChannelEntriesThatProvide(addr string) func(t *testing.T) {
	return func(t *testing.T) {
		c, conn := client(t, addr)
		defer conn.Close()

		stream, err := c.GetChannelEntriesThatProvide(context.TODO(), &api.GetAllProvidersRequest{Group: "etcd.database.coreos.com", Version: "v1beta2", Kind: "EtcdCluster"})
		require.NoError(t, err)

		channelEntries := []api.ChannelEntry{}
		waitc := make(chan struct{})
		go func(t *testing.T) {
			for {
				in, err := stream.Recv()
				if err == io.EOF {
					// read done.
					close(waitc)
					return
				}
				if err != nil {
					t.Error(err)
					close(waitc)
					return
				}
				channelEntries = append(channelEntries, *in)
			}
		}(t)
		<-waitc

		expected := []api.ChannelEntry{
			{
				PackageName: "etcd",
				ChannelName: "alpha",
				BundleName:  "etcdoperator.v0.6.1",
				Replaces:    "",
			},
			{
				PackageName: "etcd",
				ChannelName: "alpha",
				BundleName:  "etcdoperator.v0.9.0",
				Replaces:    "etcdoperator.v0.6.1",
			},
			{
				PackageName: "etcd",
				ChannelName: "alpha",
				BundleName:  "etcdoperator.v0.9.2",
				Replaces:    "etcdoperator.v0.9.1",
			},
			{
				PackageName: "etcd",
				ChannelName: "alpha",
				BundleName:  "etcdoperator.v0.9.2",
				Replaces:    "etcdoperator.v0.9.0",
			},
			{
				PackageName: "etcd",
				ChannelName: "beta",
				BundleName:  "etcdoperator.v0.6.1",
				Replaces:    "",
			},
			{
				PackageName: "etcd",
				ChannelName: "beta",
				BundleName:  "etcdoperator.v0.9.0",
				Replaces:    "etcdoperator.v0.6.1",
			},
			{
				PackageName: "etcd",
				ChannelName: "stable",
				BundleName:  "etcdoperator.v0.6.1",
				Replaces:    "",
			},
			{
				PackageName: "etcd",
				ChannelName: "stable",
				BundleName:  "etcdoperator.v0.9.0",
				Replaces:    "etcdoperator.v0.6.1",
			},
			{
				PackageName: "etcd",
				ChannelName: "stable",
				BundleName:  "etcdoperator.v0.9.2",
				Replaces:    "etcdoperator.v0.9.1",
			},
			{
				PackageName: "etcd",
				ChannelName: "stable",
				BundleName:  "etcdoperator.v0.9.2",
				Replaces:    "etcdoperator.v0.9.0",
			},
		}
		opts := []cmp.Option{
			cmpopts.IgnoreUnexported(api.ChannelEntry{}),
			cmpopts.SortSlices(func(x, y api.ChannelEntry) bool {
				if x.PackageName != y.PackageName {
					return x.PackageName < y.PackageName
				}
				if x.ChannelName != y.ChannelName {
					return x.ChannelName < y.ChannelName
				}
				if x.BundleName != y.BundleName {
					return x.BundleName < y.BundleName
				}
				if x.Replaces != y.Replaces {
					return x.Replaces < y.Replaces
				}
				return false
			}),
		}
		require.Truef(t, cmp.Equal(expected, channelEntries, opts...), cmp.Diff(expected, channelEntries, opts...))
	}
}

func TestGetLatestChannelEntriesThatProvide(t *testing.T) {
	t.Run("Sqlite", testGetLatestChannelEntriesThatProvide(dbAddress))
	t.Run("DeclarativeConfig", testGetLatestChannelEntriesThatProvide(cfgAddress))
}

func testGetLatestChannelEntriesThatProvide(addr string) func(t *testing.T) {
	return func(t *testing.T) {
		c, conn := client(t, addr)
		defer conn.Close()

		stream, err := c.GetLatestChannelEntriesThatProvide(context.TODO(), &api.GetLatestProvidersRequest{Group: "etcd.database.coreos.com", Version: "v1beta2", Kind: "EtcdCluster"})
		require.NoError(t, err)

		channelEntries := []*api.ChannelEntry{}
		waitc := make(chan struct{})
		go func(t *testing.T) {
			for {
				in, err := stream.Recv()
				if err == io.EOF {
					// read done.
					close(waitc)
					return
				}
				if err != nil {
					t.Error(err)
					close(waitc)
					return
				}
				channelEntries = append(channelEntries, in)
			}
		}(t)
		<-waitc

		expected := []*api.ChannelEntry{
			{
				PackageName: "etcd",
				ChannelName: "alpha",
				BundleName:  "etcdoperator.v0.9.2",
				Replaces:    "etcdoperator.v0.9.0",
			},
			{
				PackageName: "etcd",
				ChannelName: "beta",
				BundleName:  "etcdoperator.v0.9.0",
				Replaces:    "etcdoperator.v0.6.1",
			},
			{
				PackageName: "etcd",
				ChannelName: "stable",
				BundleName:  "etcdoperator.v0.9.2",
				Replaces:    "etcdoperator.v0.9.0",
			},
		}

		opts := []cmp.Option{
			cmpopts.IgnoreUnexported(api.ChannelEntry{}),
			cmpopts.SortSlices(func(x, y *api.ChannelEntry) bool {
				if x.PackageName != y.PackageName {
					return x.PackageName < y.PackageName
				}
				if x.ChannelName != y.ChannelName {
					return x.ChannelName < y.ChannelName
				}
				if x.BundleName != y.BundleName {
					return x.BundleName < y.BundleName
				}
				if x.Replaces != y.Replaces {
					return x.Replaces < y.Replaces
				}
				return false
			}),
		}
		require.Truef(t, cmp.Equal(expected, channelEntries, opts...), cmp.Diff(expected, channelEntries, opts...))
	}
}

func TestGetDefaultBundleThatProvides(t *testing.T) {
	t.Run("Sqlite", testGetDefaultBundleThatProvides(dbAddress, etcdoperator_v0_9_2("alpha", false, false)))
	t.Run("DeclarativeConfig", testGetDefaultBundleThatProvides(cfgAddress, etcdoperator_v0_9_2("alpha", false, true)))
}

func testGetDefaultBundleThatProvides(addr string, expected *api.Bundle) func(*testing.T) {
	return func(t *testing.T) {
		c, conn := client(t, addr)
		defer conn.Close()

		bundle, err := c.GetDefaultBundleThatProvides(context.TODO(), &api.GetDefaultProviderRequest{Group: "etcd.database.coreos.com", Version: "v1beta2", Kind: "EtcdCluster"})
		require.NoError(t, err)
		EqualBundles(t, *expected, *bundle)
	}
}

func TestListBundles(t *testing.T) {
	t.Run("Sqlite", testListBundles(dbAddress,
		etcdoperator_v0_9_2("alpha", true, false),
		etcdoperator_v0_9_2("stable", true, false)))
	t.Run("DeclarativeConfig", testListBundles(cfgAddress,
		etcdoperator_v0_9_2("alpha", true, true),
		etcdoperator_v0_9_2("stable", true, true)))
}

func testListBundles(addr string, etcdAlpha *api.Bundle, etcdStable *api.Bundle) func(*testing.T) {
	return func(t *testing.T) {
		require := require.New(t)

		c, conn := client(t, addr)
		defer conn.Close()

		stream, err := c.ListBundles(context.TODO(), &api.ListBundlesRequest{})
		require.NoError(err)

		expected := []string{
			"etcdoperator.v0.6.1",
			"prometheusoperator.0.22.2",
			"strimzi-cluster-operator.v0.11.0",
			"strimzi-cluster-operator.v0.11.1",
			"strimzi-cluster-operator.v0.12.2",
			"etcdoperator.v0.9.0",
			"prometheusoperator.0.15.0",
			"prometheusoperator.0.14.0",
			"etcdoperator.v0.6.1",
			"etcdoperator.v0.6.1",
			"etcdoperator.v0.9.0",
			"strimzi-cluster-operator.v0.12.1",
			"strimzi-cluster-operator.v0.11.0",
			"etcdoperator.v0.9.2",
			"etcdoperator.v0.9.2",
			"strimzi-cluster-operator.v0.11.1",
			"strimzi-cluster-operator.v0.11.0",
			"strimzi-cluster-operator.v0.12.1",
			"strimzi-cluster-operator.v0.11.1",
			"etcdoperator.v0.9.0",
		}

		var names []string
		var gotBundles = make([]*api.Bundle, 0)

		waitc := make(chan struct{})
		go func(t *testing.T) {
			tt := t
			for {
				in, err := stream.Recv()

				if err == io.EOF {
					// read done.
					close(waitc)
					return
				}
				if err != nil {
					tt.Error(err)
					close(waitc)
					return
				}
				names = append(names, in.CsvName)
				if in.CsvName == etcdAlpha.CsvName {
					gotBundles = append(gotBundles, in)
				}
			}
		}(t)
		<-waitc

		require.ElementsMatch(expected, names, "%#v\n%#v", expected, names)

		// TODO: this test needs better expectations
		// check that one of the entries has all of the fields we expect
		checked := 0
		for _, b := range gotBundles {
			if b.CsvName != "etcdoperator.v0.9.2" {
				continue
			}
			if b.ChannelName == "stable" {
				EqualBundles(t, *etcdStable, *b)
				checked++
			}
			if b.ChannelName == "alpha" {
				EqualBundles(t, *etcdAlpha, *b)
				checked++
			}
		}
		require.Equal(2, checked)
	}
}

func EqualBundles(t *testing.T, expected, actual api.Bundle) {
	stripPlural(actual.ProvidedApis)
	stripPlural(actual.RequiredApis)

	require.ElementsMatch(t, expected.ProvidedApis, actual.ProvidedApis, "provided apis don't match: %#v\n%#v", expected.ProvidedApis, actual.ProvidedApis)
	require.ElementsMatch(t, expected.RequiredApis, actual.RequiredApis, "required apis don't match: %#v\n%#v", expected.RequiredApis, actual.RequiredApis)
	require.ElementsMatch(t, expected.Dependencies, actual.Dependencies, "dependencies don't match: %#v\n%#v", expected.Dependencies, actual.Dependencies)
	require.ElementsMatch(t, expected.Properties, actual.Properties, "properties don't match: %#v\n%#v", expected.Properties, actual.Properties)
	require.ElementsMatch(t, expected.Object, actual.Object, "objects don't match: %#v\n%#v", expected.Object, actual.Object)

	expected.RequiredApis, expected.ProvidedApis, actual.RequiredApis, actual.ProvidedApis = nil, nil, nil, nil
	expected.Dependencies, expected.Properties, actual.Dependencies, actual.Properties = nil, nil, nil, nil
	expected.Object, actual.Object = nil, nil

	opts := []cmp.Option{
		cmpopts.IgnoreUnexported(api.Bundle{}),
		cmpopts.IgnoreUnexported(api.GroupVersionKind{}),
		cmpopts.IgnoreUnexported(api.Property{}),
		cmpopts.IgnoreUnexported(api.Dependency{}),
	}

	require.Truef(t, cmp.Equal(expected, actual, opts...), cmp.Diff(expected, actual, opts...))
}

func stripPlural(gvks []*api.GroupVersionKind) {
	for i := range gvks {
		gvks[i].Plural = ""
	}
}

func etcdoperator_v0_9_2(channel string, addSkipsReplaces, addExtraProperties bool) *api.Bundle {
	b := &api.Bundle{
		CsvName:     "etcdoperator.v0.9.2",
		PackageName: "etcd",
		ChannelName: channel,
		CsvJson:     "{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"annotations\":{\"alm-examples\":\"[{\\\"apiVersion\\\":\\\"etcd.database.coreos.com/v1beta2\\\",\\\"kind\\\":\\\"EtcdCluster\\\",\\\"metadata\\\":{\\\"name\\\":\\\"example\\\",\\\"namespace\\\":\\\"default\\\"},\\\"spec\\\":{\\\"size\\\":3,\\\"version\\\":\\\"3.2.13\\\"}},{\\\"apiVersion\\\":\\\"etcd.database.coreos.com/v1beta2\\\",\\\"kind\\\":\\\"EtcdRestore\\\",\\\"metadata\\\":{\\\"name\\\":\\\"example-etcd-cluster\\\"},\\\"spec\\\":{\\\"etcdCluster\\\":{\\\"name\\\":\\\"example-etcd-cluster\\\"},\\\"backupStorageType\\\":\\\"S3\\\",\\\"s3\\\":{\\\"path\\\":\\\"\\u003cfull-s3-path\\u003e\\\",\\\"awsSecret\\\":\\\"\\u003caws-secret\\u003e\\\"}}},{\\\"apiVersion\\\":\\\"etcd.database.coreos.com/v1beta2\\\",\\\"kind\\\":\\\"EtcdBackup\\\",\\\"metadata\\\":{\\\"name\\\":\\\"example-etcd-cluster-backup\\\"},\\\"spec\\\":{\\\"etcdEndpoints\\\":[\\\"\\u003cetcd-cluster-endpoints\\u003e\\\"],\\\"storageType\\\":\\\"S3\\\",\\\"s3\\\":{\\\"path\\\":\\\"\\u003cfull-s3-path\\u003e\\\",\\\"awsSecret\\\":\\\"\\u003caws-secret\\u003e\\\"}}}]\",\"olm.properties\":\"[{\\\"type\\\":\\\"other\\\",\\\"value\\\":{\\\"its\\\":\\\"notdefined\\\"}},{\\\"type\\\":\\\"olm.label\\\",\\\"value\\\":{\\\"label\\\":\\\"testlabel\\\"}},{\\\"type\\\":\\\"olm.label\\\",\\\"value\\\":{\\\"label\\\":\\\"testlabel1\\\"}}]\",\"olm.skipRange\":\"\\u003c 0.6.0\",\"tectonic-visibility\":\"ocs\"},\"name\":\"etcdoperator.v0.9.2\",\"namespace\":\"placeholder\"},\"spec\":{\"customresourcedefinitions\":{\"owned\":[{\"description\":\"Represents a cluster of etcd nodes.\",\"displayName\":\"etcd Cluster\",\"kind\":\"EtcdCluster\",\"name\":\"etcdclusters.etcd.database.coreos.com\",\"resources\":[{\"kind\":\"Service\",\"version\":\"v1\"},{\"kind\":\"Pod\",\"version\":\"v1\"}],\"specDescriptors\":[{\"description\":\"The desired number of member Pods for the etcd cluster.\",\"displayName\":\"Size\",\"path\":\"size\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:podCount\"]},{\"description\":\"Limits describes the minimum/maximum amount of compute resources required/allowed\",\"displayName\":\"Resource Requirements\",\"path\":\"pod.resources\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:resourceRequirements\"]}],\"statusDescriptors\":[{\"description\":\"The status of each of the member Pods for the etcd cluster.\",\"displayName\":\"Member Status\",\"path\":\"members\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:podStatuses\"]},{\"description\":\"The service at which the running etcd cluster can be accessed.\",\"displayName\":\"Service\",\"path\":\"serviceName\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Service\"]},{\"description\":\"The current size of the etcd cluster.\",\"displayName\":\"Cluster Size\",\"path\":\"size\"},{\"description\":\"The current version of the etcd cluster.\",\"displayName\":\"Current Version\",\"path\":\"currentVersion\"},{\"description\":\"The target version of the etcd cluster, after upgrading.\",\"displayName\":\"Target Version\",\"path\":\"targetVersion\"},{\"description\":\"The current status of the etcd cluster.\",\"displayName\":\"Status\",\"path\":\"phase\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase\"]},{\"description\":\"Explanation for the current status of the cluster.\",\"displayName\":\"Status Details\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"},{\"description\":\"Represents the intent to backup an etcd cluster.\",\"displayName\":\"etcd Backup\",\"kind\":\"EtcdBackup\",\"name\":\"etcdbackups.etcd.database.coreos.com\",\"specDescriptors\":[{\"description\":\"Specifies the endpoints of an etcd cluster.\",\"displayName\":\"etcd Endpoint(s)\",\"path\":\"etcdEndpoints\",\"x-descriptors\":[\"urn:alm:descriptor:etcd:endpoint\"]},{\"description\":\"The full AWS S3 path where the backup is saved.\",\"displayName\":\"S3 Path\",\"path\":\"s3.path\",\"x-descriptors\":[\"urn:alm:descriptor:aws:s3:path\"]},{\"description\":\"The name of the secret object that stores the AWS credential and config files.\",\"displayName\":\"AWS Secret\",\"path\":\"s3.awsSecret\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Secret\"]}],\"statusDescriptors\":[{\"description\":\"Indicates if the backup was successful.\",\"displayName\":\"Succeeded\",\"path\":\"succeeded\",\"x-descriptors\":[\"urn:alm:descriptor:text\"]},{\"description\":\"Indicates the reason for any backup related failures.\",\"displayName\":\"Reason\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"},{\"description\":\"Represents the intent to restore an etcd cluster from a backup.\",\"displayName\":\"etcd Restore\",\"kind\":\"EtcdRestore\",\"name\":\"etcdrestores.etcd.database.coreos.com\",\"specDescriptors\":[{\"description\":\"References the EtcdCluster which should be restored,\",\"displayName\":\"etcd Cluster\",\"path\":\"etcdCluster.name\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:EtcdCluster\",\"urn:alm:descriptor:text\"]},{\"description\":\"The full AWS S3 path where the backup is saved.\",\"displayName\":\"S3 Path\",\"path\":\"s3.path\",\"x-descriptors\":[\"urn:alm:descriptor:aws:s3:path\"]},{\"description\":\"The name of the secret object that stores the AWS credential and config files.\",\"displayName\":\"AWS Secret\",\"path\":\"s3.awsSecret\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Secret\"]}],\"statusDescriptors\":[{\"description\":\"Indicates if the restore was successful.\",\"displayName\":\"Succeeded\",\"path\":\"succeeded\",\"x-descriptors\":[\"urn:alm:descriptor:text\"]},{\"description\":\"Indicates the reason for any restore related failures.\",\"displayName\":\"Reason\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"}],\"required\":[{\"description\":\"Represents a cluster of etcd nodes.\",\"displayName\":\"etcd Cluster\",\"kind\":\"EtcdCluster\",\"name\":\"etcdclusters.etcd.database.coreos.com\",\"resources\":[{\"kind\":\"Service\",\"version\":\"v1\"},{\"kind\":\"Pod\",\"version\":\"v1\"}],\"specDescriptors\":[{\"description\":\"The desired number of member Pods for the etcd cluster.\",\"displayName\":\"Size\",\"path\":\"size\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:podCount\"]}],\"version\":\"v1beta2\"}]},\"description\":\"etcd is a distributed key value store that provides a reliable way to store data across a cluster of machines. It’s open-source and available on GitHub. etcd gracefully handles leader elections during network partitions and will tolerate machine failure, including the leader. Your applications can read and write data into etcd.\\nA simple use-case is to store database connection details or feature flags within etcd as key value pairs. These values can be watched, allowing your app to reconfigure itself when they change. Advanced uses take advantage of the consistency guarantees to implement database leader elections or do distributed locking across a cluster of workers.\\n\\n_The etcd Open Cloud Service is Public Alpha. The goal before Beta is to fully implement backup features._\\n\\n### Reading and writing to etcd\\n\\nCommunicate with etcd though its command line utility `etcdctl` or with the API using the automatically generated Kubernetes Service.\\n\\n[Read the complete guide to using the etcd Open Cloud Service](https://coreos.com/tectonic/docs/latest/alm/etcd-ocs.html)\\n\\n### Supported Features\\n\\n\\n**High availability**\\n\\n\\nMultiple instances of etcd are networked together and secured. Individual failures or networking issues are transparently handled to keep your cluster up and running.\\n\\n\\n**Automated updates**\\n\\n\\nRolling out a new etcd version works like all Kubernetes rolling updates. Simply declare the desired version, and the etcd service starts a safe rolling update to the new version automatically.\\n\\n\\n**Backups included**\\n\\n\\nComing soon, the ability to schedule backups to happen on or off cluster.\\n\",\"displayName\":\"etcd\",\"icon\":[{\"base64data\":\"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\",\"mediatype\":\"image/png\"}],\"install\":{\"spec\":{\"deployments\":[{\"name\":\"etcd-operator\",\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"name\":\"etcd-operator-alm-owned\"}},\"template\":{\"metadata\":{\"labels\":{\"name\":\"etcd-operator-alm-owned\"},\"name\":\"etcd-operator-alm-owned\"},\"spec\":{\"containers\":[{\"command\":[\"etcd-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2\",\"name\":\"etcd-operator\"},{\"command\":[\"etcd-backup-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2\",\"name\":\"etcd-backup-operator\"},{\"command\":[\"etcd-restore-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2\",\"name\":\"etcd-restore-operator\"}],\"serviceAccountName\":\"etcd-operator\"}}}}],\"permissions\":[{\"rules\":[{\"apiGroups\":[\"etcd.database.coreos.com\"],\"resources\":[\"etcdclusters\",\"etcdbackups\",\"etcdrestores\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"\"],\"resources\":[\"pods\",\"services\",\"endpoints\",\"persistentvolumeclaims\",\"events\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"apps\"],\"resources\":[\"deployments\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"\"],\"resources\":[\"secrets\"],\"verbs\":[\"get\"]}],\"serviceAccountName\":\"etcd-operator\"}]},\"strategy\":\"deployment\"},\"keywords\":[\"etcd\",\"key value\",\"database\",\"coreos\",\"open source\"],\"labels\":{\"alm-owner-etcd\":\"etcdoperator\",\"operated-by\":\"etcdoperator\"},\"links\":[{\"name\":\"Blog\",\"url\":\"https://coreos.com/etcd\"},{\"name\":\"Documentation\",\"url\":\"https://coreos.com/operators/etcd/docs/latest/\"},{\"name\":\"etcd Operator Source Code\",\"url\":\"https://github.com/coreos/etcd-operator\"}],\"maintainers\":[{\"email\":\"support@coreos.com\",\"name\":\"CoreOS, Inc\"}],\"maturity\":\"alpha\",\"provider\":{\"name\":\"CoreOS, Inc\"},\"relatedImages\":[{\"image\":\"quay.io/coreos/etcd@sha256:3816b6daf9b66d6ced6f0f966314e2d4f894982c6b1493061502f8c2bf86ac84\",\"name\":\"etcd-v3.4.0\"},{\"image\":\"quay.io/coreos/etcd@sha256:49d3d4a81e0d030d3f689e7167f23e120abf955f7d08dbedf3ea246485acee9f\",\"name\":\"etcd-3.4.1\"}],\"replaces\":\"etcdoperator.v0.9.0\",\"selector\":{\"matchLabels\":{\"alm-owner-etcd\":\"etcdoperator\",\"operated-by\":\"etcdoperator\"}},\"skips\":[\"etcdoperator.v0.9.1\"],\"version\":\"0.9.2\"}}",
		Object: []string{
			"{\"apiVersion\":\"apiextensions.k8s.io/v1beta1\",\"kind\":\"CustomResourceDefinition\",\"metadata\":{\"name\":\"etcdbackups.etcd.database.coreos.com\"},\"spec\":{\"group\":\"etcd.database.coreos.com\",\"names\":{\"kind\":\"EtcdBackup\",\"listKind\":\"EtcdBackupList\",\"plural\":\"etcdbackups\",\"singular\":\"etcdbackup\"},\"scope\":\"Namespaced\",\"version\":\"v1beta2\"}}",
			"{\"apiVersion\":\"apiextensions.k8s.io/v1beta1\",\"kind\":\"CustomResourceDefinition\",\"metadata\":{\"name\":\"etcdclusters.etcd.database.coreos.com\"},\"spec\":{\"group\":\"etcd.database.coreos.com\",\"names\":{\"kind\":\"EtcdCluster\",\"listKind\":\"EtcdClusterList\",\"plural\":\"etcdclusters\",\"shortNames\":[\"etcdclus\",\"etcd\"],\"singular\":\"etcdcluster\"},\"scope\":\"Namespaced\",\"version\":\"v1beta2\"}}",
			"{\"apiVersion\":\"operators.coreos.com/v1alpha1\",\"kind\":\"ClusterServiceVersion\",\"metadata\":{\"annotations\":{\"alm-examples\":\"[{\\\"apiVersion\\\":\\\"etcd.database.coreos.com/v1beta2\\\",\\\"kind\\\":\\\"EtcdCluster\\\",\\\"metadata\\\":{\\\"name\\\":\\\"example\\\",\\\"namespace\\\":\\\"default\\\"},\\\"spec\\\":{\\\"size\\\":3,\\\"version\\\":\\\"3.2.13\\\"}},{\\\"apiVersion\\\":\\\"etcd.database.coreos.com/v1beta2\\\",\\\"kind\\\":\\\"EtcdRestore\\\",\\\"metadata\\\":{\\\"name\\\":\\\"example-etcd-cluster\\\"},\\\"spec\\\":{\\\"etcdCluster\\\":{\\\"name\\\":\\\"example-etcd-cluster\\\"},\\\"backupStorageType\\\":\\\"S3\\\",\\\"s3\\\":{\\\"path\\\":\\\"\\u003cfull-s3-path\\u003e\\\",\\\"awsSecret\\\":\\\"\\u003caws-secret\\u003e\\\"}}},{\\\"apiVersion\\\":\\\"etcd.database.coreos.com/v1beta2\\\",\\\"kind\\\":\\\"EtcdBackup\\\",\\\"metadata\\\":{\\\"name\\\":\\\"example-etcd-cluster-backup\\\"},\\\"spec\\\":{\\\"etcdEndpoints\\\":[\\\"\\u003cetcd-cluster-endpoints\\u003e\\\"],\\\"storageType\\\":\\\"S3\\\",\\\"s3\\\":{\\\"path\\\":\\\"\\u003cfull-s3-path\\u003e\\\",\\\"awsSecret\\\":\\\"\\u003caws-secret\\u003e\\\"}}}]\",\"olm.properties\":\"[{\\\"type\\\":\\\"other\\\",\\\"value\\\":{\\\"its\\\":\\\"notdefined\\\"}},{\\\"type\\\":\\\"olm.label\\\",\\\"value\\\":{\\\"label\\\":\\\"testlabel\\\"}},{\\\"type\\\":\\\"olm.label\\\",\\\"value\\\":{\\\"label\\\":\\\"testlabel1\\\"}}]\",\"olm.skipRange\":\"\\u003c 0.6.0\",\"tectonic-visibility\":\"ocs\"},\"name\":\"etcdoperator.v0.9.2\",\"namespace\":\"placeholder\"},\"spec\":{\"customresourcedefinitions\":{\"owned\":[{\"description\":\"Represents a cluster of etcd nodes.\",\"displayName\":\"etcd Cluster\",\"kind\":\"EtcdCluster\",\"name\":\"etcdclusters.etcd.database.coreos.com\",\"resources\":[{\"kind\":\"Service\",\"version\":\"v1\"},{\"kind\":\"Pod\",\"version\":\"v1\"}],\"specDescriptors\":[{\"description\":\"The desired number of member Pods for the etcd cluster.\",\"displayName\":\"Size\",\"path\":\"size\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:podCount\"]},{\"description\":\"Limits describes the minimum/maximum amount of compute resources required/allowed\",\"displayName\":\"Resource Requirements\",\"path\":\"pod.resources\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:resourceRequirements\"]}],\"statusDescriptors\":[{\"description\":\"The status of each of the member Pods for the etcd cluster.\",\"displayName\":\"Member Status\",\"path\":\"members\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:podStatuses\"]},{\"description\":\"The service at which the running etcd cluster can be accessed.\",\"displayName\":\"Service\",\"path\":\"serviceName\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Service\"]},{\"description\":\"The current size of the etcd cluster.\",\"displayName\":\"Cluster Size\",\"path\":\"size\"},{\"description\":\"The current version of the etcd cluster.\",\"displayName\":\"Current Version\",\"path\":\"currentVersion\"},{\"description\":\"The target version of the etcd cluster, after upgrading.\",\"displayName\":\"Target Version\",\"path\":\"targetVersion\"},{\"description\":\"The current status of the etcd cluster.\",\"displayName\":\"Status\",\"path\":\"phase\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase\"]},{\"description\":\"Explanation for the current status of the cluster.\",\"displayName\":\"Status Details\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"},{\"description\":\"Represents the intent to backup an etcd cluster.\",\"displayName\":\"etcd Backup\",\"kind\":\"EtcdBackup\",\"name\":\"etcdbackups.etcd.database.coreos.com\",\"specDescriptors\":[{\"description\":\"Specifies the endpoints of an etcd cluster.\",\"displayName\":\"etcd Endpoint(s)\",\"path\":\"etcdEndpoints\",\"x-descriptors\":[\"urn:alm:descriptor:etcd:endpoint\"]},{\"description\":\"The full AWS S3 path where the backup is saved.\",\"displayName\":\"S3 Path\",\"path\":\"s3.path\",\"x-descriptors\":[\"urn:alm:descriptor:aws:s3:path\"]},{\"description\":\"The name of the secret object that stores the AWS credential and config files.\",\"displayName\":\"AWS Secret\",\"path\":\"s3.awsSecret\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Secret\"]}],\"statusDescriptors\":[{\"description\":\"Indicates if the backup was successful.\",\"displayName\":\"Succeeded\",\"path\":\"succeeded\",\"x-descriptors\":[\"urn:alm:descriptor:text\"]},{\"description\":\"Indicates the reason for any backup related failures.\",\"displayName\":\"Reason\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"},{\"description\":\"Represents the intent to restore an etcd cluster from a backup.\",\"displayName\":\"etcd Restore\",\"kind\":\"EtcdRestore\",\"name\":\"etcdrestores.etcd.database.coreos.com\",\"specDescriptors\":[{\"description\":\"References the EtcdCluster which should be restored,\",\"displayName\":\"etcd Cluster\",\"path\":\"etcdCluster.name\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:EtcdCluster\",\"urn:alm:descriptor:text\"]},{\"description\":\"The full AWS S3 path where the backup is saved.\",\"displayName\":\"S3 Path\",\"path\":\"s3.path\",\"x-descriptors\":[\"urn:alm:descriptor:aws:s3:path\"]},{\"description\":\"The name of the secret object that stores the AWS credential and config files.\",\"displayName\":\"AWS Secret\",\"path\":\"s3.awsSecret\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes:Secret\"]}],\"statusDescriptors\":[{\"description\":\"Indicates if the restore was successful.\",\"displayName\":\"Succeeded\",\"path\":\"succeeded\",\"x-descriptors\":[\"urn:alm:descriptor:text\"]},{\"description\":\"Indicates the reason for any restore related failures.\",\"displayName\":\"Reason\",\"path\":\"reason\",\"x-descriptors\":[\"urn:alm:descriptor:io.kubernetes.phase:reason\"]}],\"version\":\"v1beta2\"}],\"required\":[{\"description\":\"Represents a cluster of etcd nodes.\",\"displayName\":\"etcd Cluster\",\"kind\":\"EtcdCluster\",\"name\":\"etcdclusters.etcd.database.coreos.com\",\"resources\":[{\"kind\":\"Service\",\"version\":\"v1\"},{\"kind\":\"Pod\",\"version\":\"v1\"}],\"specDescriptors\":[{\"description\":\"The desired number of member Pods for the etcd cluster.\",\"displayName\":\"Size\",\"path\":\"size\",\"x-descriptors\":[\"urn:alm:descriptor:com.tectonic.ui:podCount\"]}],\"version\":\"v1beta2\"}]},\"description\":\"etcd is a distributed key value store that provides a reliable way to store data across a cluster of machines. It’s open-source and available on GitHub. etcd gracefully handles leader elections during network partitions and will tolerate machine failure, including the leader. Your applications can read and write data into etcd.\\nA simple use-case is to store database connection details or feature flags within etcd as key value pairs. These values can be watched, allowing your app to reconfigure itself when they change. Advanced uses take advantage of the consistency guarantees to implement database leader elections or do distributed locking across a cluster of workers.\\n\\n_The etcd Open Cloud Service is Public Alpha. The goal before Beta is to fully implement backup features._\\n\\n### Reading and writing to etcd\\n\\nCommunicate with etcd though its command line utility `etcdctl` or with the API using the automatically generated Kubernetes Service.\\n\\n[Read the complete guide to using the etcd Open Cloud Service](https://coreos.com/tectonic/docs/latest/alm/etcd-ocs.html)\\n\\n### Supported Features\\n\\n\\n**High availability**\\n\\n\\nMultiple instances of etcd are networked together and secured. Individual failures or networking issues are transparently handled to keep your cluster up and running.\\n\\n\\n**Automated updates**\\n\\n\\nRolling out a new etcd version works like all Kubernetes rolling updates. Simply declare the desired version, and the etcd service starts a safe rolling update to the new version automatically.\\n\\n\\n**Backups included**\\n\\n\\nComing soon, the ability to schedule backups to happen on or off cluster.\\n\",\"displayName\":\"etcd\",\"icon\":[{\"base64data\":\"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\",\"mediatype\":\"image/png\"}],\"install\":{\"spec\":{\"deployments\":[{\"name\":\"etcd-operator\",\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"name\":\"etcd-operator-alm-owned\"}},\"template\":{\"metadata\":{\"labels\":{\"name\":\"etcd-operator-alm-owned\"},\"name\":\"etcd-operator-alm-owned\"},\"spec\":{\"containers\":[{\"command\":[\"etcd-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2\",\"name\":\"etcd-operator\"},{\"command\":[\"etcd-backup-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2\",\"name\":\"etcd-backup-operator\"},{\"command\":[\"etcd-restore-operator\",\"--create-crd=false\"],\"env\":[{\"name\":\"MY_POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}},{\"name\":\"MY_POD_NAME\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.name\"}}}],\"image\":\"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2\",\"name\":\"etcd-restore-operator\"}],\"serviceAccountName\":\"etcd-operator\"}}}}],\"permissions\":[{\"rules\":[{\"apiGroups\":[\"etcd.database.coreos.com\"],\"resources\":[\"etcdclusters\",\"etcdbackups\",\"etcdrestores\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"\"],\"resources\":[\"pods\",\"services\",\"endpoints\",\"persistentvolumeclaims\",\"events\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"apps\"],\"resources\":[\"deployments\"],\"verbs\":[\"*\"]},{\"apiGroups\":[\"\"],\"resources\":[\"secrets\"],\"verbs\":[\"get\"]}],\"serviceAccountName\":\"etcd-operator\"}]},\"strategy\":\"deployment\"},\"keywords\":[\"etcd\",\"key value\",\"database\",\"coreos\",\"open source\"],\"labels\":{\"alm-owner-etcd\":\"etcdoperator\",\"operated-by\":\"etcdoperator\"},\"links\":[{\"name\":\"Blog\",\"url\":\"https://coreos.com/etcd\"},{\"name\":\"Documentation\",\"url\":\"https://coreos.com/operators/etcd/docs/latest/\"},{\"name\":\"etcd Operator Source Code\",\"url\":\"https://github.com/coreos/etcd-operator\"}],\"maintainers\":[{\"email\":\"support@coreos.com\",\"name\":\"CoreOS, Inc\"}],\"maturity\":\"alpha\",\"provider\":{\"name\":\"CoreOS, Inc\"},\"relatedImages\":[{\"image\":\"quay.io/coreos/etcd@sha256:3816b6daf9b66d6ced6f0f966314e2d4f894982c6b1493061502f8c2bf86ac84\",\"name\":\"etcd-v3.4.0\"},{\"image\":\"quay.io/coreos/etcd@sha256:49d3d4a81e0d030d3f689e7167f23e120abf955f7d08dbedf3ea246485acee9f\",\"name\":\"etcd-3.4.1\"}],\"replaces\":\"etcdoperator.v0.9.0\",\"selector\":{\"matchLabels\":{\"alm-owner-etcd\":\"etcdoperator\",\"operated-by\":\"etcdoperator\"}},\"skips\":[\"etcdoperator.v0.9.1\"],\"version\":\"0.9.2\"}}",
			"{\"apiVersion\":\"apiextensions.k8s.io/v1beta1\",\"kind\":\"CustomResourceDefinition\",\"metadata\":{\"name\":\"etcdrestores.etcd.database.coreos.com\"},\"spec\":{\"group\":\"etcd.database.coreos.com\",\"names\":{\"kind\":\"EtcdRestore\",\"listKind\":\"EtcdRestoreList\",\"plural\":\"etcdrestores\",\"singular\":\"etcdrestore\"},\"scope\":\"Namespaced\",\"version\":\"v1beta2\"}}",
		},
		BundlePath: "",
		Dependencies: []*api.Dependency{
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
				Value: `{"group":"etcd.database.coreos.com","kind":"EtcdRestore","version":"v1beta2"}`,
			},
			{
				Type:  "olm.gvk",
				Value: `{"group":"etcd.database.coreos.com","kind":"EtcdBackup","version":"v1beta2"}`,
			},
			{
				Type:  "olm.label",
				Value: `{"label":"testlabel"}`,
			},
			{
				Type:  "olm.label",
				Value: `{"label":"testlabel1"}`,
			},
			{
				Type:  "other",
				Value: `{"its":"notdefined"}`,
			},
		},
		ProvidedApis: []*api.GroupVersionKind{
			{Group: "etcd.database.coreos.com", Version: "v1beta2", Kind: "EtcdCluster"},
			{Group: "etcd.database.coreos.com", Version: "v1beta2", Kind: "EtcdBackup"},
			{Group: "etcd.database.coreos.com", Version: "v1beta2", Kind: "EtcdRestore"},
		},
		RequiredApis: []*api.GroupVersionKind{
			{Group: "etcd.database.coreos.com", Version: "v1beta2", Kind: "EtcdCluster"},
		},
		Version:   "0.9.2",
		SkipRange: "< 0.6.0",
	}
	if addSkipsReplaces {
		b.Replaces = "etcdoperator.v0.9.0"
		b.Skips = []string{"etcdoperator.v0.9.1"}
	}
	if addExtraProperties {
		b.Properties = append(b.Properties, []*api.Property{
			{Type: "olm.skipRange", Value: `"< 0.6.0"`},
			{Type: "olm.skips", Value: `"etcdoperator.v0.9.1"`},
			{Type: "olm.channel", Value: fmt.Sprintf(`{"name":%q,"replaces":"etcdoperator.v0.9.0"}`, channel)},
			{Type: "olm.gvk.required", Value: `{"group":"etcd.database.coreos.com","kind":"EtcdCluster","version":"v1beta2"}`},
		}...)
	}
	return b
}
