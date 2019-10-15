package sqlite

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Ensure that the migrator is able to pull the latest migration version from the real migration folder
func TestValidMigratorVersion(t *testing.T) {
	migrationsPath := "./db_migrations"

	store, err := NewSQLLiteLoader("test.db", migrationsPath)
	require.NoError(t, err)
	defer os.Remove("test.db")

	migrator := NewSQLLiteMigrator(store.db, migrationsPath)

	version, err := migrator.CurrentVersion()
	require.NoError(t, err, "Could not parse latest migration version from db_migrations folder")
	require.NotNil(t, version)
}

// Ensure that the migrator gets the right version
func TestGetMigrationVersion(t *testing.T) {
	expectedVersion := uint(200412250000)
	migrationsPath := "./testdata/test_db_migrations/valid"

	store, err := NewSQLLiteLoader("test.db", migrationsPath)
	require.NoError(t, err)
	defer os.Remove("test.db")

	migrator := NewSQLLiteMigrator(store.db, migrationsPath)

	version, err := migrator.CurrentVersion()
	require.NoError(t, err, "Could not parse latest migration version from db_migrations folder")
	require.NotNil(t, version)
	require.Equal(t, expectedVersion, version) // make sure our migration is from christmas morning 2004
}

// Attempt to initialize a database with a bad migration and ensure it fails
func TestGetMigrationError(t *testing.T) {
	migrationsPath := "./testdata/test_db_migrations/invalid"

	_, err := NewSQLLiteLoader("test.db", migrationsPath)
	require.Error(t, err)
	defer os.Remove("test.db")
}
