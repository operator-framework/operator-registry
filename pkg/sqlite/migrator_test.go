package sqlite

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Ensure that the migrator is able to pull the latest migration version from the real migration folder
func TestValidMigratorVersion(t *testing.T) {
	migrationsPath := "./db_migrations"

	store, err := NewSQLLiteLoader(WithDBName("test.db"), WithMigrationsPath(migrationsPath))
	require.NoError(t, err)
	defer os.Remove("test.db")

	migrator, err := NewSQLLiteMigrator(store.db, migrationsPath)
	require.NoError(t, err, "Unable to initialize migrator")

	version, err := migrator.CurrentVersion()
	require.NoError(t, err, "Could not parse latest migration version from db_migrations folder")
	require.NotNil(t, version)
}

// Ensure that the migrator gets the right version
func TestGetMigrationVersion(t *testing.T) {
	expectedVersion := uint(200412250000)
	migrationsPath := "./testdata/test_db_migrations/valid"

	store, err := NewSQLLiteLoader(WithDBName("test.db"), WithMigrationsPath(migrationsPath))
	require.NoError(t, err)
	defer os.Remove("test.db")

	migrator, err := NewSQLLiteMigrator(store.db, migrationsPath)
	require.NoError(t, err, "Unable to initialize migrator")

	version, err := migrator.CurrentVersion()
	require.NoError(t, err, "Could not parse latest migration version from db_migrations folder")
	require.NotNil(t, version)
	require.Equal(t, expectedVersion, version) // make sure our migration is from christmas morning 2004
}

// Attempt to initialize a database with a bad migration and ensure it fails
func TestGetMigrationError(t *testing.T) {
	migrationsPath := "./testdata/test_db_migrations/invalid"

	_, err := NewSQLLiteLoader(WithDBName("test.db"), WithMigrationsPath(migrationsPath))
	require.Error(t, err)
	defer os.Remove("test.db")
}

func TestGeneratedMigrations(t *testing.T) {
	store, err := NewSQLLiteLoader(WithDBName("test.db"))
	require.NoError(t, err)
	defer os.Remove("test.db")
	
	migrator, err := NewSQLLiteMigrator(store.db, "")
	defer migrator.CleanUpMigrator()

	require.NoError(t, err, "Unable to initialize migrator with generated migrations")
}
