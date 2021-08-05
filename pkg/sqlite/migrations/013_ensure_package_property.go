package migrations

import (
	"context"
	"database/sql"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

const EnsurePackagePropertyMigrationKey = 13

// Register this migration
func init() {
	registerMigration(EnsurePackagePropertyMigrationKey, ensurePackagePropertyMigration)
}

var ensurePackagePropertyMigration = &Migration{
	Id: EnsurePackagePropertyMigrationKey,
	Up: func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO properties(type, value, operatorbundle_name, operatorbundle_version, operatorbundle_path)
  SELECT DISTINCT :property_type, json_object('packageName', channel_entry.package_name, 'version', operatorbundle.version), operatorbundle.name, operatorbundle.version, operatorbundle.bundlepath
  FROM channel_entry INNER JOIN operatorbundle ON operatorbundle.name = channel_entry.operatorbundle_name
  WHERE NOT EXISTS (SELECT operatorbundle_name FROM properties WHERE type = :property_type AND operatorbundle_name = channel_entry.operatorbundle_name)`,
			sql.Named("property_type", registry.PackageType),
		); err != nil {
			return err
		}
		return nil
	},
	Down: func(ctx context.Context, tx *sql.Tx) error {
		return nil
	},
}
