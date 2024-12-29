package migrations_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/sqlite"
	"github.com/operator-framework/operator-registry/pkg/sqlite/migrations"
)

func TestBundlePathUp(t *testing.T) {
	db, migrator, cleanup := CreateTestDBAt(t, migrations.BundlePathMigrationKey-1)
	defer cleanup()

	err := migrator.Up(context.TODO(), migrations.Only(migrations.BundlePathMigrationKey))
	require.NoError(t, err)

	// Adding row with bundlepath colum should not fail after migrating up
	tx, err := db.Begin()
	require.NoError(t, err)
	stmt, err := tx.Prepare("insert into operatorbundle(name, csv, bundle, bundlepath) values(?, ?, ?, ?)")
	require.NoError(t, err)
	defer stmt.Close()

	_, err = stmt.Exec("testName", "testCSV", "testBundle", "quay.io/test")
	require.NoError(t, err)
}

func TestBundlePathDown(t *testing.T) {
	db, migrator, cleanup := CreateTestDBAt(t, migrations.BundlePathMigrationKey)
	defer cleanup()

	querier := sqlite.NewSQLLiteQuerierFromDb(db)
	imagesBeforeMigration, err := querier.GetImagesForBundle(context.TODO(), "etcdoperator.v0.6.1")
	require.NoError(t, err)

	err = migrator.Down(context.TODO(), migrations.Only(migrations.BundlePathMigrationKey))
	require.NoError(t, err)

	imagesAfterMigration, err := querier.GetImagesForBundle(context.TODO(), "etcdoperator.v0.6.1")
	require.NoError(t, err)

	// Migrating down entails sensitive operations. Ensure data is preserved across down migration
	require.Equal(t, len(imagesBeforeMigration), len(imagesAfterMigration))
}
