package migrations

import (
	"context"
	"database/sql"
)

const BundlePackageMigrationKey = 9

// Register this migration
func init() {
	registerMigration(BundlePackageMigrationKey, bundlePackageMigration)
}

var bundlePackageMigration = &Migration{
	Id: BundlePackageMigrationKey,
	Up: func(ctx context.Context, tx *sql.Tx) error {
		addColumnStmt := `
		ALTER TABLE operatorbundle
		ADD COLUMN package_name TEXT;
		`
		_, err := tx.ExecContext(ctx, addColumnStmt)
		if err != nil {
			return err
		}

		bundlePkgs, err := getBundlePackages(ctx, tx)
		if err != nil {
			return err
		}

		for bundle, pkgName := range bundlePkgs {
			err := setPackage(ctx, tx, pkgName, bundle)
			if err != nil {
				return err
			}
		}

		return nil
	},
	Down: func(ctx context.Context, tx *sql.Tx) error {
		foreignKeyOff := `PRAGMA foreign_keys = 0`
		createTempTable := `CREATE TABLE operatorbundle_backup (name TEXT, csv TEXT, bundle TEXT, bundlepath TEXT, skiprange TEXT, version TEXT, replaces TEXT, skips TEXT)`
		backupTargetTable := `INSERT INTO operatorbundle_backup SELECT name, csv, bundle, bundlepath, skiprange, version, replaces, skips FROM operatorbundle`
		dropTargetTable := `DROP TABLE operatorbundle`
		renameBackUpTable := `ALTER TABLE operatorbundle_backup RENAME TO operatorbundle;`
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

func setPackage(ctx context.Context, tx *sql.Tx, pkgName, bundle string) error {
	updateSql := `UPDATE operatorbundle SET package_name = ? WHERE name = ?;`
	_, err := tx.ExecContext(ctx, updateSql, pkgName, bundle)
	return err
}

func getBundlePackages(ctx context.Context, tx *sql.Tx) (map[string]string, error) {
	bundlePkgMap := make(map[string]string, 0)
	selectEntryPackageQuery := `SELECT DISTINCT operatorbundle_name, package_name FROM channel_entry`
	rows, err := tx.QueryContext(ctx, selectEntryPackageQuery)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var bundle, pkgName sql.NullString

		if err = rows.Scan(&bundle, &pkgName); err != nil {
			return nil, err
		}

		if bundle.Valid && pkgName.Valid {
			bundlePkgMap[bundle.String] = pkgName.String
		}
	}

	return bundlePkgMap, nil
}
