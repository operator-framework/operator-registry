package migrations

import (
	"context"
	"database/sql"
)

const PackageAddModeMigrationKey = 14

// Register this migration
func init() {
	registerMigration(PackageAddModeMigrationKey, packageAddModeMigration)
}

var packageAddModeMigration = &Migration{
	Id: PackageAddModeMigrationKey,
	Up: func(ctx context.Context, tx *sql.Tx) error {
		sql := `
		ALTER TABLE package
		ADD COLUMN add_mode TEXT;
		`
		_, err := tx.ExecContext(ctx, sql)
		return err
	},
	Down: func(ctx context.Context, tx *sql.Tx) error {
		foreignKeyOff := `PRAGMA foreign_keys = 0`
		createTempTable := `CREATE TABLE package_backup (name TEXT, default_channel TEXT)`
		backupTargetTable := `INSERT INTO package_backup SELECT name, default_channel FROM package`
		dropTargetTable := `DROP TABLE package`
		renameBackUpTable := `ALTER TABLE package_backup RENAME TO package;`
		foreignKeyOn := `PRAGMA foreign_keys = 1`
		_, err := tx.ExecContext(ctx, foreignKeyOff)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, createTempTable)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, backupTargetTable)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, dropTargetTable)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, renameBackUpTable)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, foreignKeyOn)
		return err
	},
}
