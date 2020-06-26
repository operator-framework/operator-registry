package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/operator-framework/operator-registry/pkg/sqlite/migrations"
	"github.com/stretchr/testify/require"
)

func TestBundlePackageUp(t *testing.T) {
	db, migrator, cleanup := CreateTestDbAt(t, migrations.BundlePackageMigrationKey-1)
	defer cleanup()

	tx, err := db.Begin()
	require.NoError(t, err)
	// Add a bundle
	insert := "insert into operatorbundle(name) values(?)"
	_, err = db.Exec(insert, "etcdoperator.v0.6.1")
	require.NoError(t, err)
	_, err = tx.Exec("insert into channel_entry(entry_id, package_name, operatorbundle_name) values(?, ?, ?)", 1, "etcd", "etcdoperator.v0.6.1")
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	err = migrator.Up(context.TODO(), migrations.Only(migrations.BundlePackageMigrationKey))
	require.NoError(t, err)

	pkgQuery := "SELECT package_name FROM operatorbundle WHERE name=?"

	rows, err := db.Query(pkgQuery, "etcdoperator.v0.6.1")
	require.NoError(t, err)
	defer rows.Close()
	rows.Next()
	var package_name sql.NullString
	require.NoError(t, rows.Scan(&package_name))
	require.Equal(t, "etcd", package_name.String)
	require.NoError(t, rows.Close())
}

func TestBundlePackageDown(t *testing.T) {
	db, migrator, cleanup := CreateTestDbAt(t, migrations.BundlePackageMigrationKey)
	defer cleanup()

	_, err := db.Exec(`PRAGMA foreign_keys = 0`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	// Add a bundle
	insert := "insert into operatorbundle(name, package_name) values(?, ?)"
	_, err = db.Exec(insert, "etcdoperator.v0.6.1", "etcd")
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	// run down migration
	err = migrator.Down(context.TODO(), migrations.Only(migrations.BundlePackageMigrationKey))
	require.NoError(t, err)

	pkgQuery := "SELECT package_name FROM operatorbundle WHERE name=?"
	rows, err := db.Query(pkgQuery, "etcdoperator.v0.6.1")
	require.NoError(t, err)
	defer rows.Close()
	require.False(t, rows.Next())
}
