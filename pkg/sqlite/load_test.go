package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/registry/test"
)

func TestSQLiteLoading(t *testing.T) {
	test.RunLoadSuite(t, setup)
}

func setup(t *testing.T) (loader registry.Load, querier registry.Query, teardown func(t *testing.T)) {
	dbName := fmt.Sprintf("test-%d.db", rand.Int())
	db, err := sql.Open("sqlite3", dbName)
	require.NoError(t, err)

	sqliteLoader, err := NewSQLLiteLoader(db)
	require.NoError(t, err)
	require.NoError(t, sqliteLoader.Migrate(context.TODO()))
	loader = sqliteLoader
	querier = NewSQLLiteQuerierFromDb(db)

	teardown = func(t *testing.T) {
		defer func() {
			require.NoError(t, os.Remove(dbName))
		}()
		require.NoError(t, db.Close())
	}

	return
}
