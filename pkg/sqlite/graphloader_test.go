package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

func createLoadedTestDb(t *testing.T) (*sql.DB, func()) {
	dbName := fmt.Sprintf("test-%d.db", rand.Int())

	db, err := sql.Open("sqlite3", dbName)
	require.NoError(t, err)

	dbLoader, err := NewSQLLiteLoader(db)
	require.NoError(t, err)

	err = dbLoader.Migrate(context.TODO())
	require.NoError(t, err)

	loader := NewSQLLoaderForDirectory(dbLoader, "./testdata/loader_data")
	err = loader.Populate()
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

func TestLoadPackageGraph_Etcd(t *testing.T) {
	version092, _ := semver.Make("0.9.2")
	version090, _ := semver.Make("0.9.0")
	version061, _ := semver.Make("0.6.1")

	expectedChannels := map[string]registry.Channel{
		"alpha": registry.Channel{
			Name: "alpha",
			OperatorBundles: []registry.OperatorBundle{
				registry.OperatorBundle{
					Version: version092,
					CsvName: "etcdoperator.v0.9.2",
					Replaces: []registry.BundleRef{
						registry.BundleRef{
							Version: version090,
							CsvName: "etcdoperator.v0.9.0",
						},
					},
					ReplacesBundles: []registry.OperatorBundle{},
				},
				registry.OperatorBundle{
					CsvName:         "etcdoperator.v0.9.1",
					Replaces:        []registry.BundleRef{},
					ReplacesBundles: []registry.OperatorBundle{},
				},
				registry.OperatorBundle{
					Version: version090,
					CsvName: "etcdoperator.v0.9.0",
					Replaces: []registry.BundleRef{
						registry.BundleRef{
							Version: version061,
							CsvName: "etcdoperator.v0.6.1",
						},
					},
					ReplacesBundles: []registry.OperatorBundle{},
				},
				registry.OperatorBundle{
					Version:         version061,
					CsvName:         "etcdoperator.v0.6.1",
					Replaces:        []registry.BundleRef{},
					ReplacesBundles: []registry.OperatorBundle{},
				},
			},
			Head: registry.BundleRef{
				Version: version092,
				CsvName: "etcdoperator.v0.9.2",
			},
		},
		"beta": registry.Channel{
			Name: "beta",
			OperatorBundles: []registry.OperatorBundle{
				registry.OperatorBundle{
					Version: version090,
					CsvName: "etcdoperator.v0.9.0",
					Replaces: []registry.BundleRef{
						registry.BundleRef{
							Version: version061,
							CsvName: "etcdoperator.v0.6.1",
						},
					},
					ReplacesBundles: []registry.OperatorBundle{},
				},
				registry.OperatorBundle{
					Version:         version061,
					CsvName:         "etcdoperator.v0.6.1",
					Replaces:        []registry.BundleRef{},
					ReplacesBundles: []registry.OperatorBundle{},
				},
			},
			Head: registry.BundleRef{
				Version: version090,
				CsvName: "etcdoperator.v0.9.0",
			},
		},
		"stable": registry.Channel{
			Name: "stable",
			OperatorBundles: []registry.OperatorBundle{
				registry.OperatorBundle{
					Version: version092,
					CsvName: "etcdoperator.v0.9.2",
					Replaces: []registry.BundleRef{
						registry.BundleRef{
							Version: version090,
							CsvName: "etcdoperator.v0.9.0",
						},
					},
					ReplacesBundles: []registry.OperatorBundle{},
				},
				registry.OperatorBundle{
					CsvName:         "etcdoperator.v0.9.1",
					Replaces:        []registry.BundleRef{},
					ReplacesBundles: []registry.OperatorBundle{},
				},
				registry.OperatorBundle{
					Version: version090,
					CsvName: "etcdoperator.v0.9.0",
					Replaces: []registry.BundleRef{
						registry.BundleRef{
							Version: version061,
							CsvName: "etcdoperator.v0.6.1",
						},
					},
					ReplacesBundles: []registry.OperatorBundle{},
				},
				registry.OperatorBundle{
					Version:         version061,
					CsvName:         "etcdoperator.v0.6.1",
					Replaces:        []registry.BundleRef{},
					ReplacesBundles: []registry.OperatorBundle{},
				},
			},
			Head: registry.BundleRef{
				Version: version092,
				CsvName: "etcdoperator.v0.9.2",
			},
		},
	}

	db, cleanup := createLoadedTestDb(t)
	defer cleanup()

	graphLoader, err := NewSQLGraphLoaderFromDB(db, "etcd")
	require.NoError(t, err)

	result, err := graphLoader.Generate()
	require.NoError(t, err)

	require.Equal(t, "etcd", result.Name)
	require.Equal(t, 3, len(result.Channels))

	for _, channel := range result.Channels {
		expectedChannel := expectedChannels[channel.Name]
		require.Equal(t, expectedChannel.Head, channel.Head)
		require.Equal(t, len(expectedChannel.OperatorBundles), len(channel.OperatorBundles))
		require.Equal(t, expectedChannel.OperatorBundles, channel.OperatorBundles)
	}
}
