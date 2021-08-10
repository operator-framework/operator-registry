package migrations

import (
	"context"
	"database/sql"
)

const RmTruncatedDeprecationsMigrationKey = 13

// Register this migration
func init() {
	registerMigration(RmTruncatedDeprecationsMigrationKey, rmTruncatedDeprecationsMigration)
}

var rmTruncatedDeprecationsMigration = &Migration{
	Id: RmTruncatedDeprecationsMigrationKey,
	Up: func(ctx context.Context, tx *sql.Tx) error {

		// Delete deprecation history for all bundles that no longer exist in the channel_entries table
		// These bundles have been truncated by more recent deprecations and would only confuse future operations on an index;
		// e.g. adding a previously truncated bundle to a package removed via `opm index|registry rm` would lead to that bundle
		// being deprecated
		_, err := tx.ExecContext(ctx, `DELETE FROM deprecated WHERE deprecated.operatorbundle_name NOT IN (SELECT DISTINCT deprecated.operatorbundle_name FROM (deprecated INNER JOIN channel_entry ON deprecated.operatorbundle_name = channel_entry.operatorbundle_name))`)

		return err
	},
	Down: func(ctx context.Context, tx *sql.Tx) error {
		// No-op
		return nil
	},
}
