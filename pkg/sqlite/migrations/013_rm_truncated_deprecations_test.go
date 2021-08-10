package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/sqlite/migrations"
)

func TestRmTruncatedDeprecations(t *testing.T) {
	db, migrator, cleanup := CreateTestDbAt(t, migrations.RmTruncatedDeprecationsMigrationKey-1)
	defer cleanup()

	// Insert fixtures to satisfy foreign key constraints
	insertBundle := "INSERT INTO operatorbundle(name, version, bundlepath, csv) VALUES (?, ?, ?, ?)"
	insertChannel := "INSERT INTO channel(name, package_name, head_operatorbundle_name) VALUES (?, ?, ?)"
	insertChannelEntry := "INSERT INTO channel_entry(entry_id, channel_name, package_name, operatorbundle_name) VALUES (?, ?, ?, ?)"
	insertDeprecated := "INSERT INTO deprecated(operatorbundle_name) VALUES (?)"

	// Add a deprecated bundle
	_, err := db.Exec(insertBundle, "operator.v1.0.0", "1.0.0", "quay.io/operator:v1.0.0", "operator.v1.0.0's csv")
	require.NoError(t, err)
	_, err = db.Exec(insertChannel, "stable", "apple", "operator.v1.0.0")
	require.NoError(t, err)
	_, err = db.Exec(insertChannelEntry, 0, "stable", "apple", "operator.v1.0.0")
	require.NoError(t, err)
	_, err = db.Exec(insertDeprecated, "operator.v1.0.0")
	require.NoError(t, err)

	// Add a truncated bundle; i.e. doesn't exist in the channel_entry table
	_, err = db.Exec(insertDeprecated, "operator.v1.0.0-pre")

	// This migration should delete all bundles that are not referenced by the channel_entry table
	require.NoError(t, migrator.Up(context.Background(), migrations.Only(migrations.RmTruncatedDeprecationsMigrationKey)))

	deprecated, err := db.Query("SELECT * FROM deprecated")
	require.NoError(t, err)
	defer deprecated.Close()

	require.True(t, deprecated.Next(), "failed to detect deprecated bundle")
	var name sql.NullString
	require.NoError(t, deprecated.Scan(&name))
	require.True(t, name.Valid)
	require.Equal(t, "operator.v1.0.0", name.String)
	require.False(t, deprecated.Next(), "incorrect number of deprecated bundles")
}
