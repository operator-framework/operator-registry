package migrations_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/sqlite/migrations"
)

func TestBeforeCascadeDeleteUp(t *testing.T) {
	// migrate up to, but not including, this migration
	db, _, cleanup := CreateTestDbAt(t, migrations.CascadeDeleteMigrationKey-1)
	defer cleanup()

	tx, err := db.Begin()
	require.NoError(t, err)

	err = checkMigrationInPreviousState(t, tx)
	require.NoError(t, err)
}

func TestAfterCascadeDeleteUp(t *testing.T) {
	// migrate up to, but not including, this migration
	db, migrator, cleanup := CreateTestDbAt(t, migrations.CascadeDeleteMigrationKey-1)
	defer cleanup()

	// run up migration
	err := migrator.Up(context.TODO(), migrations.Only(migrations.CascadeDeleteMigrationKey))
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)

	err = checkMigrationInNextState(t, tx)
	require.NoError(t, err)
}

func TestBeforeCascadeDeleteDown(t *testing.T) {
	db, _, cleanup := CreateTestDbAt(t, migrations.CascadeDeleteMigrationKey)
	defer cleanup()

	tx, err := db.Begin()
	require.NoError(t, err)

	err = checkMigrationInNextState(t, tx)
	require.NoError(t, err)
}

func TestAferCascadeDeleteDown(t *testing.T) {
	db, migrator, cleanup := CreateTestDbAt(t, migrations.CascadeDeleteMigrationKey)
	defer cleanup()

	// run down migration
	err := migrator.Down(context.TODO(), migrations.Only(migrations.CascadeDeleteMigrationKey))
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)

	err = checkMigrationInPreviousState(t, tx)
	require.NoError(t, err)
}

func removeWhiteSpaces(s string) string {
	unwanted := []string{" ", "\n", "\t"}
	for _, char := range unwanted {
		s = strings.Replace(s, char, "", -1)
	}
	return s
}

func checkMigrationInPreviousState(t *testing.T, tx *sql.Tx) error {
	getCreateTableStatement := func(table string) string {
		return `SELECT sql FROM sqlite_master where name="` + table + `"`
	}

	createNewOperatorBundleTable := `CREATE TABLE operatorbundle (
			name TEXT PRIMARY KEY,
			csv TEXT UNIQUE,
			bundle TEXT,
			bundlepath TEXT)`
	createNewPackageTable := `CREATE TABLE package (
			name TEXT PRIMARY KEY,
			default_channel TEXT,
			FOREIGN KEY(name, default_channel) REFERENCES channel(package_name,name)
		)`
	createNewChannelTable := `CREATE TABLE channel (
			name TEXT,
			package_name TEXT,
			head_operatorbundle_name TEXT,
			PRIMARY KEY(name, package_name),
			FOREIGN KEY(package_name) REFERENCES package(name),
			FOREIGN KEY(head_operatorbundle_name) REFERENCES operatorbundle(name)
		)`
	createNewChannelEntryTable := `CREATE TABLE channel_entry (
			entry_id INTEGER PRIMARY KEY,
			channel_name TEXT,
			package_name TEXT,
			operatorbundle_name TEXT,
			replaces INTEGER,
			depth INTEGER,
			FOREIGN KEY(replaces) REFERENCES channel_entry(entry_id)  DEFERRABLE INITIALLY DEFERRED,
			FOREIGN KEY(channel_name, package_name) REFERENCES channel(name, package_name)
		)`
	createNewAPIProviderTable := `CREATE TABLE api_provider (
			group_name TEXT,
			version TEXT,
			kind TEXT,
			channel_entry_id INTEGER,
			FOREIGN KEY(channel_entry_id) REFERENCES channel_entry(entry_id),
			FOREIGN KEY(group_name, version, kind) REFERENCES api(group_name, version, kind)
		)`
	createNewRelatedImageTable := `CREATE TABLE related_image (
			image TEXT,
     		operatorbundle_name TEXT,
     		FOREIGN KEY(operatorbundle_name) REFERENCES operatorbundle(name)
		)`
	createNewAPIRequirerTable := `CREATE TABLE api_requirer (
			group_name TEXT,
			version TEXT,
			kind TEXT,
			channel_entry_id INTEGER,
			FOREIGN KEY(channel_entry_id) REFERENCES channel_entry(entry_id),
			FOREIGN KEY(group_name, version, kind) REFERENCES api(group_name, version, kind)
		)`
	var createStatement string

	table, err := tx.Query(getCreateTableStatement("operatorbundle"))
	hasRows := table.Next()
	require.True(t, hasRows)
	err = table.Scan(&createStatement)
	require.NoError(t, err)
	require.Equal(t, removeWhiteSpaces(createNewOperatorBundleTable), removeWhiteSpaces(createStatement))
	err = table.Close()
	require.NoError(t, err)

	table, err = tx.Query(getCreateTableStatement("package"))
	hasRows = table.Next()
	require.True(t, hasRows)
	err = table.Scan(&createStatement)
	require.NoError(t, err)
	require.Equal(t, removeWhiteSpaces(createNewPackageTable), removeWhiteSpaces(createStatement))
	err = table.Close()
	require.NoError(t, err)

	table, err = tx.Query(getCreateTableStatement("channel"))
	hasRows = table.Next()
	require.True(t, hasRows)
	err = table.Scan(&createStatement)
	require.NoError(t, err)
	require.Equal(t, removeWhiteSpaces(createNewChannelTable), removeWhiteSpaces(createStatement))
	err = table.Close()
	require.NoError(t, err)

	table, err = tx.Query(getCreateTableStatement("channel_entry"))
	hasRows = table.Next()
	require.True(t, hasRows)
	err = table.Scan(&createStatement)
	require.NoError(t, err)
	require.Equal(t, removeWhiteSpaces(createNewChannelEntryTable), removeWhiteSpaces(createStatement))
	err = table.Close()
	require.NoError(t, err)

	table, err = tx.Query(getCreateTableStatement("api_requirer"))
	hasRows = table.Next()
	require.True(t, hasRows)
	err = table.Scan(&createStatement)
	require.NoError(t, err)
	require.Equal(t, removeWhiteSpaces(createNewAPIRequirerTable), removeWhiteSpaces(createStatement))
	err = table.Close()
	require.NoError(t, err)

	table, err = tx.Query(getCreateTableStatement("api_provider"))
	hasRows = table.Next()
	require.True(t, hasRows)
	err = table.Scan(&createStatement)
	require.NoError(t, err)
	require.Equal(t, removeWhiteSpaces(createNewAPIProviderTable), removeWhiteSpaces(createStatement))
	err = table.Close()
	require.NoError(t, err)

	table, err = tx.Query(getCreateTableStatement("related_image"))
	hasRows = table.Next()
	require.True(t, hasRows)
	err = table.Scan(&createStatement)
	require.NoError(t, err)
	require.Equal(t, removeWhiteSpaces(createNewRelatedImageTable), removeWhiteSpaces(createStatement))
	err = table.Close()
	require.NoError(t, err)

	return nil
}

func checkMigrationInNextState(t *testing.T, tx *sql.Tx) error {
	getCreateTableStatement := func(table string) string {
		return `SELECT sql FROM sqlite_master where name="` + table + `"`
	}
	createNewOperatorBundleTable := `CREATE TABLE operatorbundle (
			name TEXT PRIMARY KEY,
			csv TEXT,
			bundle TEXT,
			bundlepath TEXT)`
	createNewPackageTable := `CREATE TABLE package (
			name TEXT PRIMARY KEY,
			default_channel TEXT,
			FOREIGN KEY(name, default_channel) REFERENCES channel(package_name,name) ON DELETE CASCADE
		)`
	createNewChannelTable := `CREATE TABLE channel (
			name TEXT,
			package_name TEXT,
			head_operatorbundle_name TEXT,
			PRIMARY KEY(name, package_name),
			FOREIGN KEY(head_operatorbundle_name) REFERENCES operatorbundle(name) ON DELETE CASCADE
		)`
	createNewChannelEntryTable := `CREATE TABLE channel_entry (
			entry_id INTEGER PRIMARY KEY,
			channel_name TEXT,
			package_name TEXT,
			operatorbundle_name TEXT,
			replaces INTEGER,
			depth INTEGER,
			FOREIGN KEY(replaces) REFERENCES channel_entry(entry_id) DEFERRABLE INITIALLY DEFERRED, 
			FOREIGN KEY(channel_name, package_name) REFERENCES channel(name, package_name) ON DELETE CASCADE
		)`
	createNewAPIProviderTable := `CREATE TABLE api_provider (
			group_name TEXT,
			version TEXT,
			kind TEXT,
			channel_entry_id INTEGER,
			PRIMARY KEY(group_name, version, kind, channel_entry_id),
			FOREIGN KEY(channel_entry_id) REFERENCES channel_entry(entry_id) ON DELETE CASCADE,
			FOREIGN KEY(group_name, version, kind) REFERENCES api(group_name, version, kind)
		)`
	createNewRelatedImageTable := `CREATE TABLE related_image (
			image TEXT,
     		operatorbundle_name TEXT,
     		FOREIGN KEY(operatorbundle_name) REFERENCES operatorbundle(name) ON DELETE CASCADE
		)`
	createNewAPIRequirerTable := `CREATE TABLE api_requirer (
			group_name TEXT,
			version TEXT,
			kind TEXT,
			channel_entry_id INTEGER,
			PRIMARY KEY(group_name, version, kind, channel_entry_id),
			FOREIGN KEY(channel_entry_id) REFERENCES channel_entry(entry_id) ON DELETE CASCADE,
			FOREIGN KEY(group_name, version, kind) REFERENCES api(group_name, version, kind)
		)`
	var createStatement string

	table, err := tx.Query(getCreateTableStatement("operatorbundle"))
	hasRows := table.Next()
	require.True(t, hasRows)
	err = table.Scan(&createStatement)
	require.NoError(t, err)
	require.Equal(t, removeWhiteSpaces(createNewOperatorBundleTable), removeWhiteSpaces(createStatement))
	err = table.Close()
	require.NoError(t, err)

	table, err = tx.Query(getCreateTableStatement("package"))
	hasRows = table.Next()
	require.True(t, hasRows)
	err = table.Scan(&createStatement)
	require.NoError(t, err)
	require.Equal(t, removeWhiteSpaces(createNewPackageTable), removeWhiteSpaces(createStatement))
	err = table.Close()
	require.NoError(t, err)

	table, err = tx.Query(getCreateTableStatement("channel"))
	hasRows = table.Next()
	require.True(t, hasRows)
	err = table.Scan(&createStatement)
	require.NoError(t, err)
	require.Equal(t, removeWhiteSpaces(createNewChannelTable), removeWhiteSpaces(createStatement))
	err = table.Close()
	require.NoError(t, err)

	table, err = tx.Query(getCreateTableStatement("channel_entry"))
	hasRows = table.Next()
	require.True(t, hasRows)
	err = table.Scan(&createStatement)
	require.NoError(t, err)
	require.Equal(t, removeWhiteSpaces(createNewChannelEntryTable), removeWhiteSpaces(createStatement))
	err = table.Close()
	require.NoError(t, err)

	table, err = tx.Query(getCreateTableStatement("api_requirer"))
	hasRows = table.Next()
	require.True(t, hasRows)
	err = table.Scan(&createStatement)
	require.NoError(t, err)
	require.Equal(t, removeWhiteSpaces(createNewAPIRequirerTable), removeWhiteSpaces(createStatement))
	err = table.Close()
	require.NoError(t, err)

	table, err = tx.Query(getCreateTableStatement("api_provider"))
	hasRows = table.Next()
	require.True(t, hasRows)
	err = table.Scan(&createStatement)
	require.NoError(t, err)
	require.Equal(t, removeWhiteSpaces(createNewAPIProviderTable), removeWhiteSpaces(createStatement))
	err = table.Close()
	require.NoError(t, err)

	table, err = tx.Query(getCreateTableStatement("related_image"))
	hasRows = table.Next()
	require.True(t, hasRows)
	err = table.Scan(&createStatement)
	require.NoError(t, err)
	require.Equal(t, removeWhiteSpaces(createNewRelatedImageTable), removeWhiteSpaces(createStatement))
	err = table.Close()
	require.NoError(t, err)

	return nil
}
