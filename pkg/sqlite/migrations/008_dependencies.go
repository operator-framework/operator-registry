package migrations

import (
	"context"
	"database/sql"
)

const DependenciesMigrationKey = 8

// Register this migration
func init() {
	registerMigration(DependenciesMigrationKey, dependenciesMigration)
}

var dependenciesMigration = &Migration{
	Id: DependenciesMigrationKey,
	Up: func(ctx context.Context, tx *sql.Tx) error {
		sql := `
		CREATE TABLE IF NOT EXISTS dependencies (
			type TEXT,
			package_name TEXT,
			group_name TEXT,
			version TEXT,
			kind TEXT,
			operatorbundle_name TEXT,
			operatorbundle_version TEXT,
			operatorbundle_path TEXT,
			FOREIGN KEY(operatorbundle_name, operatorbundle_version, operatorbundle_path) REFERENCES operatorbundle(name, version, bundlepath) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
			FOREIGN KEY(type, package_name, group_name, version, kind) REFERENCES dependencies(type, package_name, group_name, version, kind) ON DELETE CASCADE
		);
		`
		_, err := tx.ExecContext(ctx, sql)
		if err != nil {
			return err
		}

		return nil
	},
	Down: func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DROP TABLE dependencies`)
		if err != nil {
			return err
		}

		return err
	},
}
