package boltdb

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/registry/test"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func TestEnsureBolt(t *testing.T) {
	prefix := fmt.Sprintf("test-%d", rand.Int())
	dbName := prefix + ".db"
	backupName := dbName + ".bak"
	db, err := sql.Open("sqlite3", dbName)
	require.NoError(t, err)
	defer os.Remove(dbName)
	defer os.Remove(backupName)

	sqliteLoader, err := sqlite.NewSQLLiteLoader(db)
	require.NoError(t, err)
	require.NoError(t, sqliteLoader.Migrate(context.TODO()))
	require.NoError(t, registry.NewDirectoryPopulator(sqliteLoader, "../../manifests").Populate())
	require.NoError(t, db.Close())

	require.NoError(t, EnsureBolt(dbName, backupName))
	bdb, err := storm.Open(dbName)
	require.NoError(t, err)
	defer bdb.Close()

	t.Run("queriable", func(t *testing.T) {
		test.ContentQueriable(t, NewStormQuerier(bdb))
	})
}
