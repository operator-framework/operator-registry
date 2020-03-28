package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const ReplacesSkipsMigrationKey = 7

// Register this migration
func init() {
	registerMigration(ReplacesSkipsMigrationKey, replacesSkipsMigration)
}

var replacesSkipsMigration = &Migration{
	Id: ReplacesSkipsMigrationKey,
	Up: func(ctx context.Context, tx *sql.Tx) error {
		sql := `
		ALTER TABLE operatorbundle 
		ADD COLUMN replaces TEXT;

		ALTER TABLE operatorbundle 
		ADD COLUMN skips TEXT;
		`
		_, err := tx.ExecContext(ctx, sql)
		if err != nil {
			return err
		}

		bundles, err := listBundles(ctx, tx)
		if err != nil {
			return err
		}
		for _, bundle := range bundles {
			if err := extractReplaces(ctx, tx, bundle); err != nil {
				return fmt.Errorf("error backfilling replaces and skips: %v", err)
			}
		}
		return err
	},
	Down: func(ctx context.Context, tx *sql.Tx) error {
			foreignKeyOff := `PRAGMA foreign_keys = 0`
			createTempTable := `CREATE TABLE operatorbundle_backup (name TEXT, csv TEXT, bundle TEXT, bundlepath TEXT, version TEXT, skiprange TEXT)`
			backupTargetTable := `INSERT INTO operatorbundle_backup SELECT name, csv, bundle, bundlepath, version, skiprange FROM operatorbundle`
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

func extractReplaces(ctx context.Context, tx *sql.Tx, name string) error {
	replaces, skips, err := getReplacesAndSkips(ctx, tx, name)
	if err != nil {
		return err
	}
	updateSql := `update operatorbundle SET replaces = ?, skips = ? WHERE name = ?;`
	_, err = tx.ExecContext(ctx, updateSql, replaces, strings.Join(skips, ","), name)
	return err
}

func getReplacesAndSkips(ctx context.Context, tx *sql.Tx, name string) (replaces string, skips []string, err error) {
	getReplacees := `
		SELECT DISTINCT replaces.operatorbundle_name
		FROM channel_entry
		LEFT OUTER JOIN channel_entry replaces ON channel_entry.replaces = replaces.entry_id
    	WHERE channel_entry.operatorbundle_name = ? 
    	ORDER BY channel_entry.depth ASC
	`

	rows, err := tx.QueryContext(ctx, getReplacees, name)
	if err != nil {
		return "", nil, fmt.Errorf("error backfilling replaces and skips: %v", err)
	}
	defer rows.Close()

	if rows.Next() {
		var replaceeName sql.NullString
		if err = rows.Scan(&replaceeName); err != nil {
			return
		}
		if replaceeName.Valid {
			replaces = replaceeName.String
		}
	}

	skips = []string{}
	for rows.Next() {
		var skipName sql.NullString
		if err = rows.Scan(&skipName); err != nil {
			return
		}
		if !skipName.Valid {
			continue
		}
		skips = append(skips, skipName.String)
	}
	return
}
