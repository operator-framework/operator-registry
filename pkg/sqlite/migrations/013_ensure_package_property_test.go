package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/operator-framework/operator-registry/pkg/sqlite/migrations"
	"github.com/stretchr/testify/require"
)

func TestEnsurePackagePropertyUp(t *testing.T) {
	db, migrator, cleanup := CreateTestDbAt(t, migrations.EnsurePackagePropertyMigrationKey-1)
	defer cleanup()

	_, err := db.Exec(`PRAGMA foreign_keys = 0`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)

	insert := "insert into operatorbundle(name, bundlepath, version, skiprange, replaces, skips) values(?, ?, ?, ?, ?, ?)"
	_, err = db.Exec(insert, "etcdoperator.v0.6.1", "quay.io/image", "0.6.1", ">0.5.0 <0.6.1", "0.9.0", "0.9.1,0.9.2")
	require.NoError(t, err)
	_, err = db.Exec(insert, "otheroperator.v1.2.3", "quay.io/image", "1.2.3", "", "", "")
	require.NoError(t, err)

	channel_entries := `INSERT INTO channel_entry("entry_id", "channel_name", "package_name", "operatorbundle_name", "replaces", "depth") VALUES ('1', 'alpha', 'etcd', 'etcdoperator.v0.6.1', '', '0'), ('2', 'beta', 'otheroperator', 'otheroperator.v1.2.3', '', '0');`
	_, err = tx.Exec(channel_entries)
	require.NoError(t, err)

	_, err = tx.Exec(`INSERT INTO properties (type, value, operatorbundle_name, operatorbundle_version, operatorbundle_path) VALUES ('olm.package', json_object('packageName', 'otheroperator', 'version', '1.2.3'), 'otheroperator.v1.2.3', '1.2.3', 'quay.io/image')`)
	require.NoError(t, err)

	require.NoError(t, tx.Commit())

	err = migrator.Up(context.TODO(), migrations.Only(migrations.EnsurePackagePropertyMigrationKey))
	require.NoError(t, err)

	rows, err := db.Query(`SELECT operatorbundle_name, type, value FROM properties`)
	require.NoError(t, err)
	defer rows.Close()

	type prop struct {
		bundle   string
		typeName string
		value    string
	}
	properties := []prop{}
	for rows.Next() {
		var (
			bundle   sql.NullString
			typeName sql.NullString
			value    sql.NullString
		)
		require.NoError(t, rows.Scan(&bundle, &typeName, &value))
		require.True(t, bundle.Valid)
		require.True(t, typeName.Valid)
		require.True(t, value.Valid)
		properties = append(properties, prop{
			bundle:   bundle.String,
			typeName: typeName.String,
			value:    value.String,
		})
	}

	expectedProperties := []prop{
		{
			bundle:   "etcdoperator.v0.6.1",
			typeName: "olm.package",
			value:    `{"packageName":"etcd","version":"0.6.1"}`,
		},
		{
			bundle:   "otheroperator.v1.2.3",
			typeName: "olm.package",
			value:    `{"packageName":"otheroperator","version":"1.2.3"}`,
		},
	}
	require.ElementsMatch(t, expectedProperties, properties)
}

func TestEnsurePackagePropertyDown(t *testing.T) {
	_, migrator, cleanup := CreateTestDbAt(t, migrations.EnsurePackagePropertyMigrationKey)
	defer cleanup()

	// down migration is a no-op
	require.NoError(t, migrator.Down(context.TODO(), migrations.Only(migrations.EnsurePackagePropertyMigrationKey)))
}
