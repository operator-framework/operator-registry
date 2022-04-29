package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/sqlite/migrations"
)

func TestPackageAddModeUp(t *testing.T) {
	db, migrator, cleanup := CreateTestDbAt(t, migrations.PackageAddModeMigrationKey-1)
	defer cleanup()

	tx, err := db.Begin()
	require.NoError(t, err)

	// Add a bundle, channel, and package
	for _, f := range []func() (sql.Result, error){
		func() (sql.Result, error) {
			return db.Exec("INSERT INTO operatorbundle(name) values(?)", "pkg.v0.1.0")
		},
		func() (sql.Result, error) {
			return db.Exec("INSERT INTO channel(name, package_name) values(?, ?)", "channel", "pkg")
		},
		func() (sql.Result, error) {
			return db.Exec("INSERT INTO package(name, default_channel) VALUES(?, ?)", "pkg", "channel")
		},
	} {
		_, err := f()
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit())

	err = migrator.Up(context.TODO(), migrations.Only(migrations.PackageAddModeMigrationKey))
	require.NoError(t, err)

	// Check package table add_mode field is empty
	query := `SELECT add_mode FROM package WHERE name=?`
	rows, err := db.Query(query, "pkg")
	require.NoError(t, err)
	defer rows.Close()

	addMode := &sql.NullString{}
	if rows.Next() {
		require.NoError(t, rows.Scan(addMode))
	}
	require.False(t, addMode.Valid)
}

func TestPackageAddModeDown(t *testing.T) {
	db, migrator, cleanup := CreateTestDbAt(t, migrations.PackageAddModeMigrationKey)
	defer cleanup()

	tx, err := db.Begin()
	require.NoError(t, err)

	// Add a bundle, channel, and package
	for _, f := range []func() (sql.Result, error){
		func() (sql.Result, error) {
			return db.Exec("INSERT INTO operatorbundle(name) values(?)", "pkg.v0.1.0")
		},
		func() (sql.Result, error) {
			return db.Exec("INSERT INTO channel(name, package_name) values(?, ?)", "channel", "pkg")
		},
		func() (sql.Result, error) {
			return db.Exec("INSERT INTO package(name, default_channel) VALUES(?, ?)", "pkg", "channel")
		},
	} {
		_, err := f()
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit())

	err = migrator.Down(context.TODO(), migrations.Only(migrations.PackageAddModeMigrationKey))
	require.NoError(t, err)
}
