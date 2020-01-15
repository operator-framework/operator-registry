package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/ql/driver"

	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

type SQLQuerier struct {
	db *sql.DB
}

var _ registry.Query = &SQLQuerier{}

func NewSQLLiteQuerier(dbFilename string) (*SQLQuerier, error) {
	db, err := sql.Open("ql", dbFilename)
	if err != nil {
		return nil, err
	}

	return &SQLQuerier{db}, nil
}

func NewSQLLiteQuerierFromDb(db *sql.DB) *SQLQuerier {
	return &SQLQuerier{db}
}

func (s *SQLQuerier) ListTables(ctx context.Context) ([]string, error) {
	query := "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;"
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := []string{}
	for rows.Next() {
		var tableName sql.NullString
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		if tableName.Valid {
			tables = append(tables, tableName.String)
		}
	}
	return tables, nil
}

// ListPackages returns a list of package names as strings
func (s *SQLQuerier) ListPackages(ctx context.Context) ([]string, error) {
	query := "SELECT DISTINCT name FROM package"
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	packages := []string{}
	for rows.Next() {
		var pkgName sql.NullString
		if err := rows.Scan(&pkgName); err != nil {
			return nil, err
		}
		if pkgName.Valid {
			packages = append(packages, pkgName.String)
		}
	}
	return packages, nil
}

// listChannelEntries returns a list of channel entries (debug)
func (s *SQLQuerier) listChannelEntries(ctx context.Context) ([]int64, error) {
	query := "SELECT id(), channel_name, package_name, operatorbundle_name, replaces, depth FROM channel_entry"
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []int64{}
	for rows.Next() {
		var entryId sql.NullInt64
		var pkgName sql.NullString
		var chanName sql.NullString
		var bundleName sql.NullString
		var replaces sql.NullInt64
		var depth sql.NullInt64
		if err := rows.Scan(&entryId, &chanName, &pkgName, &bundleName, &replaces, &depth); err != nil {
			return nil, err
		}
		if entryId.Valid {
			entries = append(entries, entryId.Int64)
		}
		//TODO: replaces is always nil - last insert id?
	}
	return entries, nil
}

func (s *SQLQuerier) GetPackage(ctx context.Context, name string) (*registry.PackageManifest, error) {
	query := `SELECT package.name, package.default_channel, channel.name, channel.head_operatorbundle_name
              FROM package LEFT JOIN channel ON channel.package_name=package.name
              WHERE package.name=$1`
	rows, err := s.db.QueryContext(ctx, query, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pkgName sql.NullString
	var defaultChannel sql.NullString
	var channelName sql.NullString
	var bundleName sql.NullString
	if !rows.Next() {
		return nil, fmt.Errorf("package %s not found", name)
	}
	if err := rows.Scan(&pkgName, &defaultChannel, &channelName, &bundleName); err != nil {
		return nil, err
	}
	pkg := &registry.PackageManifest{
		PackageName:        pkgName.String,
		DefaultChannelName: defaultChannel.String,
		Channels: []registry.PackageChannel{
			{
				Name:           channelName.String,
				CurrentCSVName: bundleName.String,
			},
		},
	}

	for rows.Next() {
		if err := rows.Scan(&pkgName, &defaultChannel, &channelName, &bundleName); err != nil {
			return nil, err
		}
		pkg.Channels = append(pkg.Channels, registry.PackageChannel{Name: channelName.String, CurrentCSVName: bundleName.String})
	}
	return pkg, nil
}

func (s *SQLQuerier) GetBundle(ctx context.Context, pkgName, channelName, csvName string) (*api.Bundle, error) {
	query := `SELECT id(channel_entry), operatorbundle.name, operatorbundle.bundle, operatorbundle.bundlepath, operatorbundle.version, operatorbundle.skiprange
			  FROM operatorbundle LEFT JOIN channel_entry ON operatorbundle.name=channel_entry.operatorbundle_name
              WHERE channel_entry.package_name=$1 AND channel_entry.channel_name=$2 AND channel_entry.operatorbundle_name=$3 LIMIT 1`
	rows, err := s.db.QueryContext(ctx, query, pkgName, channelName, csvName)
	if err != nil {
		return nil, err
	}

	if !rows.Next() {
		return nil,  fmt.Errorf("no entry found for %s %s %s", pkgName, channelName, csvName)
	}
	var entryId sql.NullInt64
	var name sql.NullString
	var bundle sql.NullString
	var bundlePath sql.NullString
	var version sql.NullString
	var skipRange sql.NullString
	if err := rows.Scan(&entryId, &name, &bundle, &bundlePath, &version, &skipRange); err != nil {
		return nil, err
	}

	out := &api.Bundle{}
	if bundle.Valid && bundle.String != "" {
		out, err = registry.BundleStringToAPIBundle(bundle.String)
		if err != nil {
			return nil, err
		}
	}
	out.CsvName = name.String
	out.PackageName = pkgName
	out.ChannelName = channelName
	out.BundlePath = bundlePath.String
	out.Version = version.String
	out.SkipRange = skipRange.String

	if err := rows.Close(); err != nil {
		return nil, err
	}

	provided, required, err := s.GetApisForEntry(ctx, entryId.Int64)
	if err != nil {
		return nil, err
	}
	out.ProvidedApis = provided
	out.RequiredApis = required

	return out, nil
}

func (s *SQLQuerier) GetBundleForChannel(ctx context.Context, pkgName string, channelName string) (*api.Bundle, error) {
	query := `SELECT operatorbundle.name, operatorbundle.bundle, operatorbundle.bundlepath, operatorbundle.version, operatorbundle.skiprange 
			  FROM channel LEFT JOIN operatorbundle ON channel.head_operatorbundle_name=operatorbundle.name
              WHERE channel.package_name=$1 AND channel.name=$2`

	rows, err := s.db.QueryContext(ctx, query, pkgName, channelName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil,  fmt.Errorf("no entry found for %s %s", pkgName, channelName)
	}
	var name sql.NullString
	var bundle sql.NullString
	var bundlePath sql.NullString
	var version sql.NullString
	var skipRange sql.NullString
	if err := rows.Scan(&name, &bundle, &bundlePath, &version, &skipRange); err != nil {
		return nil, err
	}

	out := &api.Bundle{}
	if bundle.Valid && bundle.String != "" {
		out, err = registry.BundleStringToAPIBundle(bundle.String)
		if err != nil {
			return nil, err
		}
	}
	out.CsvName = name.String
	out.PackageName = pkgName
	out.ChannelName = channelName
	out.BundlePath = bundlePath.String
	out.Version = version.String
	out.SkipRange = skipRange.String

	entryQuery := `SELECT id() FROM channel_entry
                   WHERE package_name=$1 AND channel_name=$2 AND operatorbundle_name=$3 LIMIT 1`
	entryRows, err := s.db.QueryContext(ctx, entryQuery, pkgName, channelName, name.String)
	if err != nil {
		return nil, err
	}
	defer entryRows.Close()

	if !entryRows.Next() {
		return nil,  fmt.Errorf("no entry found for %s %s %s", pkgName, channelName, name.String)
	}
	var entryId sql.NullInt64
	if err := entryRows.Scan(&entryId); err != nil {
		return nil, err
	}
	if !entryId.Valid {
		return nil,  fmt.Errorf("no entry found for %s %s %s", pkgName, channelName, name.String)
	}

	provided, required, err := s.GetApisForEntry(ctx, entryId.Int64)
	if err != nil {
		return nil, err
	}
	out.ProvidedApis = provided
	out.RequiredApis = required

	return out, nil
}

func (s *SQLQuerier) GetChannelEntriesThatReplace(ctx context.Context, name string) (entries []*registry.ChannelEntry, err error) {
	query := `select package_name, channel_name, operatorbundle_name from channel_entry 
              where replaces in (
                  select id() from channel_entry where operatorbundle_name = $1
              )
              `
	rows, err := s.db.QueryContext(ctx, query, name)
	if err != nil {
		return
	}
	defer rows.Close()

	entries = []*registry.ChannelEntry{}

	for rows.Next() {
		var pkgNameSQL sql.NullString
		var channelNameSQL sql.NullString
		var bundleNameSQL sql.NullString

		if err = rows.Scan(&pkgNameSQL, &channelNameSQL, &bundleNameSQL); err != nil {
			return
		}
		entries = append(entries, &registry.ChannelEntry{
			PackageName: pkgNameSQL.String,
			ChannelName: channelNameSQL.String,
			BundleName:  bundleNameSQL.String,
			Replaces:    name,
		})
	}
	if len(entries) == 0 {
		err = fmt.Errorf("no channel entries found that replace %s", name)
		return
	}
	return
}

func (s *SQLQuerier) GetBundleThatReplaces(ctx context.Context, name, pkgName, channelName string) (*api.Bundle, error) {
	query := `select id(channel_entry), operatorbundle.name, operatorbundle.bundle, operatorbundle.bundlepath, operatorbundle.version, operatorbundle.skiprange
			  from channel_entry left join operatorbundle on channel_entry.operatorbundle_name=operatorbundle.name 
              where channel_entry.replaces in (
                  select id() from channel_entry where operatorbundle_name = $1 AND package_name = $2 AND channel_name = $3
              )
		      LIMIT 1
              `
	rows, err := s.db.QueryContext(ctx, query, name, pkgName, channelName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()


	if !rows.Next() {
		return nil,  fmt.Errorf("no entry found for %s %s", pkgName, channelName)
	}
	var entryId sql.NullInt64
	var outName sql.NullString
	var bundle sql.NullString
	var bundlePath sql.NullString
	var version sql.NullString
	var skipRange sql.NullString
	if err := rows.Scan(&entryId, &outName, &bundle, &bundlePath, &version, &skipRange); err != nil {
		return nil, err
	}

	out := &api.Bundle{}
	if bundle.Valid && bundle.String != "" {
		out, err = registry.BundleStringToAPIBundle(bundle.String)
		if err != nil {
			return nil, err
		}
	}
	out.CsvName = outName.String
	out.PackageName = pkgName
	out.ChannelName = channelName
	out.BundlePath = bundlePath.String
	out.Version = version.String
	out.SkipRange = skipRange.String

	provided, required, err := s.GetApisForEntry(ctx, entryId.Int64)
	if err != nil {
		return nil, err
	}
	out.ProvidedApis = provided
	out.RequiredApis = required

	return out, nil
}

func (s *SQLQuerier) GetChannelEntriesThatProvide(ctx context.Context, group, version, kind string) (entries []*registry.ChannelEntry, err error) {
	query := `SELECT e.pkg, e.chan, e.bundle, e.replace_name, FROM
			  (SELECT id(c) as ID, c.package_name as pkg, c.channel_name as chan, c.operatorbundle_name as bundle, r.operatorbundle_name as replace_name
			   FROM channel_entry as c
				   LEFT JOIN (SELECT id() AS ID, operatorbundle_name FROM channel_entry) AS r
				   on c.replaces==r.ID
			   ) as e left join api_provider as a on a.channel_entry_id=e.ID
			  WHERE a.group_name = $1 AND a.version = $2 AND a.kind = $3`
	rows, err := s.db.QueryContext(ctx, query, group, version, kind)
	if err != nil {
		return
	}
	defer rows.Close()

	entries = []*registry.ChannelEntry{}

	for rows.Next() {
		var pkgNameSQL sql.NullString
		var channelNameSQL sql.NullString
		var bundleNameSQL sql.NullString
		var replacesSQL sql.NullString
		if err = rows.Scan(&pkgNameSQL, &channelNameSQL, &bundleNameSQL, &replacesSQL); err != nil {
			return
		}

		entries = append(entries, &registry.ChannelEntry{
			PackageName: pkgNameSQL.String,
			ChannelName: channelNameSQL.String,
			BundleName:  bundleNameSQL.String,
			Replaces:    replacesSQL.String,
		})
	}
	if len(entries) == 0 {
		err = fmt.Errorf("no channel entries found that provide %s %s %s", group, version, kind)
		return
	}
	return
}

//// Get latest channel entries that provide an api
//func (s *SQLQuerier) GetLatestChannelEntriesThatProvide(ctx context.Context, group, version, kind string) (entries []*registry.ChannelEntry, err error) {
//	query := `SELECT e.pkg, e.chan, e.bundle, e.replace_name, min(e.depth) FROM
//			  (SELECT id(c) as ID, c.package_name as pkg, c.channel_name as chan, c.operatorbundle_name as bundle, r.operatorbundle_name as replace_name, c.depth as depth
//			   FROM channel_entry as c
//				   LEFT JOIN (SELECT id() AS ID, operatorbundle_name FROM channel_entry) AS r
//				   on c.replaces==r.ID
//			   ) as e left join api_provider as a on a.channel_entry_id=e.ID
//			  WHERE a.group_name = $1 AND a.version = $2 AND a.kind = $3
//			  GROUP BY e.chan
//			  `
//
//	//query := `SELECT DISTINCT channel_entry.package_name, channel_entry.channel_name, channel_entry.operatorbundle_name, replaces.operatorbundle_name, min(channel_entry.depth)
//    //      FROM channel_entry
//    //      INNER JOIN api_provider ON channel_entry.entry_id = api_provider.channel_entry_id
//	//	  LEFT OUTER JOIN channel_entry replaces ON channel_entry.replaces = replaces.entry_id
//	//	  WHERE api_provider.group_name = $1 AND api_provider.version = $2 AND api_provider.kind = $3
//	//	  GROUP BY channel_entry.package_name, channel_entry.channel_name`
//	rows, err := s.db.QueryContext(ctx, query, group, version, kind)
//	if err != nil {
//		return nil, err
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
//		var min_depth sql.NullInt64
//		if err = rows.Scan(&pkgNameSQL, &channelNameSQL, &bundleNameSQL, &replacesSQL, &min_depth); err != nil {
//			return nil, err
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
//		return nil, err
//	}
//	return entries, nil
//}


// Get the the latest bundle that provides the API in a default channel, error unless there is ONLY one
func (s *SQLQuerier) GetBundleThatProvides(ctx context.Context, group, apiVersion, kind string) (*api.Bundle, error) {
	// TODO: refactor / tidy
    // similar to the real query, but only returns the target depth
	depthQuery := `
SELECT min(g.g_d) FROM
( SELECT f.f_id as g_id, operatorbundle.bundle as g_bundle, operatorbundle.bundlepath as g_bundlepath, f.f_d as g_d, f.f_name as g_name, f.f_p as g_p, f.f_c as g_c, f.f_r as g_r, operatorbundle.version as g_v, operatorbundle.skiprange as g_s
FROM
(SELECT entry.ID as f_id,
entry.bundlename as f_name,
entry.pkg as f_p,
entry.chan as f_c,
entry.replaces as f_r,
entry.depth as f_d from
         package as p,
        (SELECT id(channel_entry) as ID,
               channel_entry.operatorbundle_name as bundlename,
               channel_entry.package_name as pkg,
               channel_entry.channel_name as chan,
               channel_entry.replaces as replaces,
               channel_entry.depth as depth
            FROM channel_entry LEFT JOIN api_provider ON id(channel_entry) = api_provider.channel_entry_id
         ) as entry
         WHERE p.name = entry.pkg AND p.default_channel = entry.chan
) as f
LEFT JOIN operatorbundle ON operatorbundle.name = f.f_name) as g
LEFT JOIN api_provider ON g.g_id = api_provider.channel_entry_id WHERE api_provider.kind="EtcdCluster"
`

	drows, err := s.db.QueryContext(ctx, depthQuery, group, apiVersion, kind)
	if err != nil {
		return nil, err
	}
	defer drows.Close()
	if !drows.Next() {
		return nil, fmt.Errorf("no entry found that provides %s %s %s", group, apiVersion, kind)
	}
	var depth sql.NullInt64
	if err := drows.Scan(&depth); err != nil {
		return nil, err
	}
	if !depth.Valid {
		return nil, fmt.Errorf("no entry found that provides %s %s %s", group, apiVersion, kind)
	}

	query := `
SELECT g.g_id, g.g_bundle, g.g_bundlepath, g.g_d, g.g_name, g.g_p, g.g_c, g.g_r, g.g_v, g.g_s FROM 
( SELECT f.f_id as g_id, operatorbundle.bundle as g_bundle, operatorbundle.bundlepath as g_bundlepath, f.f_d as g_d, f.f_name as g_name, f.f_p as g_p, f.f_c as g_c, f.f_r as g_r, operatorbundle.version as g_v, operatorbundle.skiprange as g_s
FROM
(SELECT entry.ID as f_id,
entry.bundlename as f_name,
entry.pkg as f_p, 
entry.chan as f_c, 
entry.replaces as f_r,
entry.depth as f_d from 
         package as p,
	(SELECT id(channel_entry) as ID,
               channel_entry.operatorbundle_name as bundlename,
               channel_entry.package_name as pkg,
               channel_entry.channel_name as chan,
               channel_entry.replaces as replaces,
               channel_entry.depth as depth
            FROM channel_entry LEFT JOIN api_provider ON id(channel_entry) = api_provider.channel_entry_id
         ) as entry
         WHERE p.name = entry.pkg AND p.default_channel = entry.chan
) as f
LEFT JOIN operatorbundle ON operatorbundle.name = f.f_name) as g
LEFT JOIN api_provider ON g.g_id = api_provider.channel_entry_id
WHERE api_provider.group_name = $1 AND api_provider.version = $2 AND api_provider.kind = $3 AND g.g_d = $4
`

	rows, err := s.db.QueryContext(ctx, query, group, apiVersion, kind, depth.Int64)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("no entry found that provides %s %s %s", group, apiVersion, kind)
	}
	var entryId sql.NullInt64
	var bundle sql.NullString
	var bundlePath sql.NullString
	var min_depth sql.NullInt64
	var bundleName sql.NullString
	var pkgName sql.NullString
	var channelName sql.NullString
	var replaces sql.NullString
	var version sql.NullString
	var skipRange sql.NullString
	if err := rows.Scan(&entryId, &bundle, &bundlePath, &min_depth, &bundleName, &pkgName, &channelName, &replaces, &version, &skipRange); err != nil {
		return nil, err
	}

	if !bundle.Valid {
		return nil, fmt.Errorf("no entry found that provides %s %s %s", group, apiVersion, kind)
	}

	out := &api.Bundle{}
	if bundle.Valid && bundle.String != "" {
		out, err = registry.BundleStringToAPIBundle(bundle.String)
		if err != nil {
			return nil, err
		}
	}
	out.CsvName = bundleName.String
	out.PackageName = pkgName.String
	out.ChannelName = channelName.String
	out.BundlePath = bundlePath.String
	out.Version = version.String
	out.SkipRange = skipRange.String

	provided, required, err := s.GetApisForEntry(ctx, entryId.Int64)
	if err != nil {
		return nil, err
	}
	out.ProvidedApis = provided
	out.RequiredApis = required

	return out, nil
}

func (s *SQLQuerier) ListImages(ctx context.Context) ([]string, error) {
	query := "SELECT DISTINCT image FROM related_image"
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	images := []string{}
	for rows.Next() {
		var imgName sql.NullString
		if err := rows.Scan(&imgName); err != nil {
			return nil, err
		}
		if imgName.Valid {
			images = append(images, imgName.String)
		}
	}
	return images, nil
}

func (s *SQLQuerier) GetImagesForBundle(ctx context.Context, csvName string) ([]string, error) {
	query := "SELECT DISTINCT image FROM related_image WHERE operatorbundle_name=$1"
	rows, err := s.db.QueryContext(ctx, query, csvName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	images := []string{}
	for rows.Next() {
		var imgName sql.NullString
		if err := rows.Scan(&imgName); err != nil {
			return nil, err
		}
		if imgName.Valid {
			images = append(images, imgName.String)
		}
	}
	return images, nil
}

func (s *SQLQuerier) GetApisForEntry(ctx context.Context, entryId int64) (provided []*api.GroupVersionKind, required []*api.GroupVersionKind, err error) {
	providedQuery := `SELECT group_name, version, kind, plural FROM api_provider
			  		  WHERE channel_entry_id=$1`

	providedRows, err := s.db.QueryContext(ctx, providedQuery, entryId)
	if err != nil {
		return nil,nil, err
	}

	provided = []*api.GroupVersionKind{}
	for providedRows.Next() {
		var groupName sql.NullString
		var versionName sql.NullString
		var kindName sql.NullString
		var pluralName sql.NullString

		if err := providedRows.Scan(&groupName, &versionName, &kindName, &pluralName); err != nil {
			return nil, nil, err
		}
		if !groupName.Valid || !versionName.Valid || !kindName.Valid || !pluralName.Valid {
			return nil, nil, err
		}
		provided = append(provided, &api.GroupVersionKind{
			Group:  groupName.String,
			Version: versionName.String,
			Kind:   kindName.String,
			Plural:   pluralName.String,
		})
	}
	if err := providedRows.Close(); err != nil {
		return nil, nil, err
	}

	requiredQuery := `SELECT group_name, version, kind, plural FROM api_requirer
			  		  WHERE channel_entry_id=$1`

	requiredRows, err := s.db.QueryContext(ctx, requiredQuery, entryId)
	if err != nil {
		return nil,nil, err
	}
	required = []*api.GroupVersionKind{}
	for requiredRows.Next() {
		var groupName sql.NullString
		var versionName sql.NullString
		var kindName sql.NullString
		var pluralName sql.NullString

		if err := requiredRows.Scan(&groupName, &versionName, &kindName, &pluralName); err != nil {
			return nil, nil, err
		}
		if !groupName.Valid || !versionName.Valid || !kindName.Valid || !pluralName.Valid {
			return nil, nil, err
		}
		required = append(required, &api.GroupVersionKind{
			Group:  groupName.String,
			Version: versionName.String,
			Kind:   kindName.String,
			Plural:   pluralName.String,
		})
	}
	if err := requiredRows.Close(); err != nil {
		return nil, nil, err
	}

	return
}
