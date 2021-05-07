package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite/migrations"
)

func TestDeprecated(t *testing.T) {
	db, migrator, cleanup := CreateTestDbAt(t, migrations.DeprecatedMigrationKey-1)
	defer cleanup()

	// Insert fixture bundles to satisfy foreign key constraint in properties table
	insertBundle := "INSERT INTO operatorbundle(name, version, bundlepath, csv) VALUES (?, ?, ?, ?)"
	insertProperty := "INSERT INTO properties(type, operatorbundle_name, operatorbundle_version, operatorbundle_path) VALUES (?, ?, ?, ?)"

	// Add unique bundles both with and without the deprecated property
	// The content of the bundles is otherwise unimportant
	// Deprecated:
	_, err := db.Exec(insertBundle, "operator.v1.0.0", "1.0.0", "quay.io/operator:v1.0.0", "operator.v1.0.0's csv")
	require.NoError(t, err)
	_, err = db.Exec(insertProperty, registry.DeprecatedType, "operator.v1.0.0", "1.0.0", "quay.io/operator:v1.0.0")
	require.NoError(t, err)
	_, err = db.Exec(insertProperty, "extraneous", "operator.v1.0.0", "1.0.0", "quay.io/operator:v1.0.0")
	require.NoError(t, err)

	// Not deprecated:
	_, err = db.Exec(insertBundle, "operator.v2.0.0", "2.0.0", "quay.io/operator:v2.0.0", "operator.v2.0.0's csv")
	require.NoError(t, err)
	_, err = db.Exec(insertProperty, "extraneous", "operator.v2.0.0", "2.0.0", "quay.io/operator:v2.0.0")
	require.NoError(t, err)

	// This migration should populate the deprecated table with the names of all bundles that have the deprecated property
	require.NoError(t, migrator.Up(context.Background(), migrations.Only(migrations.DeprecatedMigrationKey)))

	deprecated, err := db.Query("SELECT * FROM deprecated")
	require.NoError(t, err)
	defer deprecated.Close()

	require.True(t, deprecated.Next(), "failed to detect deprecated bundle")
	var name sql.NullString
	require.NoError(t, deprecated.Scan(&name))
	require.True(t, name.Valid)
	require.Equal(t, "operator.v1.0.0", name.String)
	require.False(t, deprecated.Next(), "incorrect number of deprecated bundles")

	// This migration should drop the deprecated table
	require.NoError(t, migrator.Down(context.Background(), migrations.Only(migrations.DeprecatedMigrationKey)))

	table, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name='deprecated'")
	require.NoError(t, err)
	defer table.Close()
	require.False(t, table.Next(), "deprecated table wasn't properly cleaned up on downgrade")
}
