package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/operator-framework/operator-registry/pkg/sqlite/migrations"
	"github.com/stretchr/testify/require"
)

func TestDependenciesUp(t *testing.T) {
	db, migrator, cleanup := CreateTestDbAt(t, migrations.DependenciesMigrationKey-1)
	defer cleanup()

	_, err := db.Exec(`PRAGMA foreign_keys = 0`)
	require.NoError(t, err)

	tx, _ := db.Begin()
	err := migrator.Up(context.TODO(), migrations.Only(migrations.DependenciesMigrationKey))
	require.NoError(t, err)

	query, err := tx.Prepare("insert into dependencies(type, package_name, group_name, version, kind, operatorbundle_name, operatorbundle_version, operatorbundle_path) values(?, ?, ?, ?, ?, ?, ?, ?)")
	require.NoError(t, err)
	defer query.Close()

	_, err = query.Exec("olm.gvk", "prometheus", "monitoring.coreos.com", "v1", "Prometheus", "prometheusoperator.0.32.0", "0.32.0", "quay.io/coreos/prometheus-operator")
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	depQuery := `SELECT DISTINCT dependencies.type, dependencies.package_name, dependencies.group_name, dependencies.version, dependencies.kind FROM dependencies
	WHERE dependencies.operatorbundle_name=? AND dependencies.operatorbundle_version=? AND dependencies.operatorbundle_path=?`

	rows, err := db.Query(depQuery, "prometheusoperator.0.32.0", "0.32.0", "quay.io/coreos/prometheus-operator")
	require.NoError(t, err)
	var typeName sql.NullString
	var name sql.NullString
	var group sql.NullString
	var version sql.NullString
	var kind sql.NullString
	rows.Next()
	require.NoError(t, rows.Scan(&typeName, &name, &group, &version, &kind))
	require.Equal(t, typeName.String, "olm.gvk")
	require.Equal(t, name.String, "prometheus")
	require.Equal(t, group.String, "monitoring.coreos.com")
	require.Equal(t, version.String, "v1")
	require.Equal(t, kind.String, "Prometheus")
	require.NoError(t, rows.Close())
}

func TestDependenciesDown(t *testing.T) {
	db, migrator, cleanup := CreateTestDbAt(t, migrations.DependenciesMigrationKey)
	defer cleanup()

	_, err := db.Exec(`PRAGMA foreign_keys = 0`)
	require.NoError(t, err)

	tx, _ := db.Begin()
	query, err := tx.Prepare("insert into dependencies(type, package_name, group_name, version, kind, operatorbundle_name, operatorbundle_version, operatorbundle_path) values(?, ?, ?, ?, ?, ?, ?, ?)")
	require.NoError(t, err)
	defer query.Close()

	_, err = query.Exec("olm.gvk", "prometheus", "monitoring.coreos.com", "v1", "Prometheus", "prometheusoperator.0.32.0", "0.32.0", "quay.io/coreos/prometheus-operator")
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	depQuery := `SELECT DISTINCT dependencies.type, dependencies.package_name, dependencies.group_name, dependencies.version, dependencies.kind FROM dependencies
	WHERE dependencies.operatorbundle_name=? AND dependencies.operatorbundle_version=? AND dependencies.operatorbundle_path=?`

	// run down migration
	err = migrator.Down(context.TODO(), migrations.Only(migrations.DependenciesMigrationKey))
	require.NoError(t, err)

	// check that no dependencies were extracted.
	_, err := db.Query(depQuery, "prometheusoperator.0.32.0", "0.32.0", "quay.io/coreos/prometheus-operator")
	require.Error(t, err)
}
