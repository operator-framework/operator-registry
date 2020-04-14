package migrations

import (
	"context"
	"database/sql"

	"github.com/operator-framework/operator-registry/pkg/registry"
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
			FOREIGN KEY(operatorbundle_name, operatorbundle_version, operatorbundle_path) REFERENCES operatorbundle(name, version, bundlepath) ON DELETE CASCADE
		);
		`
		_, err := tx.ExecContext(ctx, sql)
		if err != nil {
			return err
		}

		insertRequired := `INSERT INTO dependencies(type, package_name, group_name, version, kind, operatorbundle_name, operatorbundle_version, operatorbundle_path) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

		bundleApis, err := getRequiredAPIs(ctx, tx)
		if err != nil {
			return err
		}
		for bundle, apis := range bundleApis {
			for required := range apis.required {
				_, err := tx.ExecContext(ctx, insertRequired, "olm.gvk", nil, required.Group, required.Version, required.Kind, bundle.CsvName, bundle.Version, bundle.BundlePath)
				if err != nil {
					return err
				}
			}
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

func getRequiredAPIs(ctx context.Context, tx *sql.Tx) (map[bundleKey]apis, error) {
	bundles := map[bundleKey]apis{}

	requiredQuery := `SELECT api_requirer.group_name, api_requirer.version, api_requirer.kind, api_requirer.operatorbundle_name, api_requirer.operatorbundle_version, api_requirer.operatorbundle_path
  FROM api_requirer`

	requiredRows, err := tx.QueryContext(ctx, requiredQuery)
	if err != nil {
		return nil, err
	}
	for requiredRows.Next() {
		var group sql.NullString
		var apiVersion sql.NullString
		var kind sql.NullString
		var name sql.NullString
		var bundleVersion sql.NullString
		var path sql.NullString
		if err = requiredRows.Scan(&group, &apiVersion, &kind, &name, &bundleVersion, &path); err != nil {
			return nil, err
		}
		if !group.Valid || !apiVersion.Valid || !kind.Valid || !name.Valid {
			continue
		}
		key := bundleKey{
			BundlePath: path,
			Version:    bundleVersion,
			CsvName:    name,
		}
		bundleApis, ok := bundles[key]
		if !ok {
			bundleApis = apis{
				provided: map[registry.APIKey]struct{}{},
				required: map[registry.APIKey]struct{}{},
			}
		}

		bundleApis.required[registry.APIKey{
			Group:   group.String,
			Version: apiVersion.String,
			Kind:    kind.String,
		}] = struct{}{}

		bundles[key] = bundleApis
	}

	return bundles, nil
}
