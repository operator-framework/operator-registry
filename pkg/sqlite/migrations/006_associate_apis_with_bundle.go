package migrations

import (
	"context"
	"database/sql"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

const AssociateApisWithBundleMigrationKey = 6

// Register this migration
func init() {
	registerMigration(AssociateApisWithBundleMigrationKey, bundleApiMigration)
}


// This migration moves the link between the provided and required apis table from the channel_entry to the
// bundle itself. This simplifies loading and minimizes changes that need to happen when a new bundle is
// inserted into an existing database.
// Before:
// api_provider: FOREIGN KEY(channel_entry_id) REFERENCES channel_entry(entry_id),
// api_requirer: FOREIGN KEY(channel_entry_id) REFERENCES channel_entry(entry_id),
// After:
// api_provider: FOREIGN KEY(operatorbundle_name, operatorbundle_version, operatorbundle_path) REFERENCES operatorbundle(name, version, bundlepath),
// api_requirer: FOREIGN KEY(operatorbundle_name, operatorbundle_version, operatorbundle_path) REFERENCES operatorbundle(name, version, bundlepath),

var bundleApiMigration = &Migration{
	Id: AssociateApisWithBundleMigrationKey,
	Up: func(ctx context.Context, tx *sql.Tx) error {
		createNew := `
		CREATE TABLE api_provider_new (
			group_name TEXT,
			version TEXT,
			kind TEXT,
			operatorbundle_name TEXT,
			operatorbundle_version TEXT,
			operatorbundle_path TEXT,
			FOREIGN KEY (operatorbundle_name) REFERENCES operatorbundle(name)  ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
			FOREIGN KEY(group_name, version, kind) REFERENCES api(group_name, version, kind)
		);
		CREATE TABLE api_requirer_new (
			group_name TEXT,
			version TEXT,
			kind TEXT,
			operatorbundle_name TEXT,
			operatorbundle_version TEXT,
			operatorbundle_path TEXT,
			FOREIGN KEY (operatorbundle_name) REFERENCES operatorbundle(name)  ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
			FOREIGN KEY(group_name, version, kind) REFERENCES api(group_name, version, kind)
		);
		`
		_, err := tx.ExecContext(ctx, createNew)
		if err != nil {
			return err
		}

		insertProvided := `INSERT INTO api_provider_new(group_name, version, kind, operatorbundle_name, operatorbundle_version, operatorbundle_path) VALUES (?, ?, ?, ?, ?, ?)`
		insertRequired := `INSERT INTO api_requirer_new(group_name, version, kind, operatorbundle_name, operatorbundle_version, operatorbundle_path) VALUES (?, ?, ?, ?, ?, ?)`

		bundleApis, err := getApisForBundles(ctx, tx)
		if err != nil {
			return err
		}
		for bundle, apis := range bundleApis {
			for provided := range apis.provided {
				_, err := tx.ExecContext(ctx, insertProvided, provided.Group, provided.Version, provided.Kind, bundle.CsvName, bundle.Version, bundle.BundlePath)
				if err != nil {
					return err
				}
			}
			for required := range apis.required {
				_, err := tx.ExecContext(ctx, insertRequired, required.Group, required.Version, required.Kind, bundle.CsvName, bundle.Version, bundle.BundlePath)
				if err != nil {
					return err
				}
			}
		}

		renameNewAndDropOld := `
		DROP TABLE api_provider;
		DROP TABLE api_requirer;
		ALTER TABLE api_provider_new RENAME TO api_provider;
		ALTER TABLE api_requirer_new RENAME TO api_requirer;
		`
		_, err = tx.ExecContext(ctx, renameNewAndDropOld)
		if err != nil {
			return err
		}

		return err
	},
	Down: func(ctx context.Context, tx *sql.Tx) error {
		// TODO
		return nil
	},
}

type apis struct {
	provided map[registry.APIKey]struct{}
	required map[registry.APIKey]struct{}
}

func getApisForBundles(ctx context.Context, tx *sql.Tx) (map[registry.BundleKey]apis, error) {
	bundles := map[registry.BundleKey]apis{}

	providedQuery := `SELECT api_provider.group_name, api_provider.version, api_provider.kind, operatorbundle.name, operatorbundle.version, operatorbundle.bundlepath 
                       FROM api_provider
			  		   INNER JOIN channel_entry ON channel_entry.entry_id = api_provider.channel_entry_id
			           INNER JOIN operatorbundle ON operatorbundle.name = channel_entry.operatorbundle_name`

	requiredQuery := `SELECT api_requirer.group_name, api_requirer.version, api_requirer.kind, operatorbundle.name, operatorbundle.version, operatorbundle.bundlepath 
                       FROM api_requirer
			  		   INNER JOIN channel_entry ON channel_entry.entry_id = api_requirer.channel_entry_id
			           INNER JOIN operatorbundle ON operatorbundle.name = channel_entry.operatorbundle_name`

	providedRows, err := tx.QueryContext(ctx, providedQuery)
	if err != nil {
		return nil, err
	}
	for providedRows.Next() {
		var group sql.NullString
		var apiVersion sql.NullString
		var kind sql.NullString
		var name sql.NullString
		var bundleVersion sql.NullString
		var path sql.NullString
		if err = providedRows.Scan(&group, &apiVersion, &kind, &name, &bundleVersion, &path); err != nil {
			return nil, err
		}
		if !group.Valid || !apiVersion.Valid || !kind.Valid || !name.Valid || !bundleVersion.Valid || !path.Valid {
			continue
		}
		key := registry.BundleKey{
			BundlePath: path.String,
			Version:    bundleVersion.String,
			CsvName:    name.String,
		}
		bundleApis, ok := bundles[key]
		if !ok {
			bundleApis = apis{
				provided: map[registry.APIKey]struct{}{},
				required: map[registry.APIKey]struct{}{},
			}
		}

		bundleApis.provided[registry.APIKey{
			Group:   group.String,
			Version: apiVersion.String,
			Kind:    kind.String,
		}] = struct {}{}

		bundles[key] = bundleApis
	}

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
		if !group.Valid || !apiVersion.Valid || !kind.Valid || !name.Valid || !bundleVersion.Valid || !path.Valid {
			continue
		}
		key := registry.BundleKey{
			BundlePath: path.String,
			Version:    bundleVersion.String,
			CsvName:    name.String,
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
		}] = struct {}{}

		bundles[key] = bundleApis
	}

	return bundles, nil
}


// OLD queries
//
//func GetChannelEntriesThatProvide(ctx context.Context, group, version, kind string) (entries []*registry.ChannelEntry, err error) {
//	query := `SELECT DISTINCT channel_entry.package_name, channel_entry.channel_name, channel_entry.operatorbundle_name, replaces.operatorbundle_name
//          FROM channel_entry
//          INNER JOIN api_provider ON channel_entry.entry_id = api_provider.channel_entry_id
//          LEFT OUTER JOIN channel_entry replaces ON channel_entry.replaces = replaces.entry_id
//		  WHERE api_provider.group_name = ? AND api_provider.version = ? AND api_provider.kind = ?`
//
//	rows, err := s.db.QueryContext(ctx, query, group, version, kind)
//	if err != nil {
//		return
//	}
//	defer rows.Close()
//
//	entries = []*registry.ChannelEntry{}
//
//	for rows.Next() {
//		var pkgNameSQL sql.NullString
//		var channelNameSQL sql.NullString
//		var bundleNameSQL sql.NullString
//		var replacesSQL sql.NullString
//		if err = rows.Scan(&pkgNameSQL, &channelNameSQL, &bundleNameSQL, &replacesSQL); err != nil {
//			return
//		}
//
//		entries = append(entries, &registry.ChannelEntry{
//			PackageName: pkgNameSQL.String,
//			ChannelName: channelNameSQL.String,
//			BundleName:  bundleNameSQL.String,
//			Replaces:    replacesSQL.String,
//		})
//	}
//	if len(entries) == 0 {
//		err = fmt.Errorf("no channel entries found that provide %s %s %s", group, version, kind)
//		return
//	}
//	return
//}