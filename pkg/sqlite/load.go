package sqlite

import (
	"database/sql"
	"encoding/json"

	_ "github.com/mattn/go-sqlite3"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

type SQLLoader struct {
	db *sql.DB
}

var _ registry.Load = &SQLLoader{}

func NewSQLLiteLoader(outFilename string) (*SQLLoader, error) {
	db, err := sql.Open("sqlite3", outFilename) // TODO: ?immutable=true
	if err != nil {
		return nil, err
	}

	createTable := `
	CREATE TABLE operatorbundle (
		name TEXT PRIMARY KEY,  
		csv TEXT UNIQUE, 
		bundle TEXT
	);
	CREATE TABLE package (
		name TEXT PRIMARY KEY,
		default_channel TEXT,
		FOREIGN KEY(default_channel) REFERENCES channel(name)
	);
	CREATE TABLE channel (
		name TEXT, 
		package_name TEXT, 
		head_operatorbundle_name TEXT,
		PRIMARY KEY(name, package_name),
		FOREIGN KEY(package_name) REFERENCES package(name),
		FOREIGN KEY(head_operatorbundle_name) REFERENCES operatorbundle(name)
	);
	CREATE TABLE channel_entry (
		entry_id INTEGER PRIMARY KEY,
		channel_name TEXT,
		package_name TEXT,
		operatorbundle_name TEXT,
		replaces INTEGER,
		depth INTEGER,
		FOREIGN KEY(replaces) REFERENCES channel_entry(entry_id)  DEFERRABLE INITIALLY DEFERRED, 
		FOREIGN KEY(channel_name) REFERENCES channel(name),
		FOREIGN KEY(package_name) REFERENCES channel(package_name),
		FOREIGN KEY(operatorbundle_name) REFERENCES operatorbundle(name)
	);
	CREATE TABLE api (
		groupOrName TEXT,
		version TEXT,
		kind TEXT,
		PRIMARY KEY(groupOrName,version,kind)
	);
	CREATE TABLE api_provider (
		groupOrName TEXT,
		version TEXT,
		kind TEXT,
		channel_entry_id INTEGER,
		FOREIGN KEY(channel_entry_id) REFERENCES channel_entry(entry_id),
		FOREIGN KEY(groupOrName,version,kind) REFERENCES api(groupOrName, version, kind) 
	);
	CREATE INDEX replaces ON operatorbundle(json_extract(csv, '$.spec.replaces'));
	`

	if _, err = db.Exec(createTable); err != nil {
		return nil, err
	}
	return &SQLLoader{db}, nil
}

func (s *SQLLoader) AddOperatorBundle(bundle *registry.Bundle) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("insert into operatorbundle(name, csv, bundle) values(?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	csvName, csvBytes, bundleBytes, err := bundle.Serialize()
	if err != nil {
		return err
	}

	if _, err := stmt.Exec(csvName, csvBytes, bundleBytes); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLLoader) AddPackageChannels(manifest registry.PackageManifest) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	addPackage, err := tx.Prepare("insert into package(name) values(?)")
	if err != nil {
		return err
	}
	defer addPackage.Close()

	addDefaultChannel, err := tx.Prepare("update package set default_channel = ? where name = ?")
	if err != nil {
		return err
	}
	defer addPackage.Close()

	if _, err := addPackage.Exec(manifest.PackageName); err != nil {
		return err
	}

	addChannel, err := tx.Prepare("insert into channel(name, package_name, head_operatorbundle_name) values(?, ?, ?)")
	if err != nil {
		return err
	}
	defer addChannel.Close()

	for _, c := range manifest.Channels {
		if _, err := addChannel.Exec(c.Name, manifest.PackageName, c.CurrentCSVName); err != nil {
			return err
		}
		if c.IsDefaultChannel(manifest) {
			if _, err := addDefaultChannel.Exec(c.Name, manifest.PackageName); err != nil {
				return err
			}
		}
	}

	addChannelEntry, err := tx.Prepare("insert into channel_entry(channel_name,package_name,operatorbundle_name,depth) values(?,?,?,?)")
	if err != nil {
		return err
	}
	defer addChannelEntry.Close()

	getReplaces, err := tx.Prepare(`
	 SELECT DISTINCT json_extract(operatorbundle.csv, '$.spec.replaces')
	 FROM operatorbundle,json_tree(operatorbundle.csv)
	 WHERE operatorbundle.name IS ?
	`)
	defer getReplaces.Close()

	addReplaces, err := tx.Prepare("update channel_entry set replaces = ? where entry_id = ?")
	if err != nil {
		return err
	}
	defer addReplaces.Close()

	for _, c := range manifest.Channels {
		res, err := addChannelEntry.Exec(c.Name, manifest.PackageName, c.CurrentCSVName, 0)
		if err != nil {
			return err
		}
		currentID, err := res.LastInsertId()
		if err != nil {
			return err
		}

		channelEntryCSVName := c.CurrentCSVName
		depth := 1
		for {
			rows, err := getReplaces.Query(channelEntryCSVName)
			if err != nil {
				return err
			}

			if rows.Next() {
				var replaced sql.NullString
				if err := rows.Scan(&replaced); err != nil {
					return err
				}

				if !replaced.Valid || replaced.String == "" {
					break
				}

				replacedChannelEntry, err := addChannelEntry.Exec(c.Name, manifest.PackageName, replaced.String, depth)
				if err != nil {
					return err
				}
				replacedID, err := replacedChannelEntry.LastInsertId()
				if err != nil {
					return err
				}
				addReplaces.Exec(replacedID, currentID)
				currentID = replacedID
				channelEntryCSVName = replaced.String
				depth += 1
			}
		}
	}
	return tx.Commit()
}

func (s *SQLLoader) AddProvidedApis() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	addApi, err := tx.Prepare("insert or replace into api(groupOrName,version,kind) values(?,?,?)")
	if err != nil {
		return err
	}
	defer addApi.Close()

	addApiProvider, err := tx.Prepare("insert into api_provider(groupOrName,version,kind,channel_entry_id) values(?,?,?,?)")
	if err != nil {
		return err
	}
	defer addApiProvider.Close()

	// get CRD provided APIs
	getChannelEntryProvidedAPIs, err := tx.Prepare(`
	SELECT DISTINCT channel_entry.entry_id, json_extract(json_each.value, '$.name', '$.version', '$.kind')
	FROM channel_entry INNER JOIN operatorbundle,json_each(operatorbundle.csv, '$.spec.customresourcedefinitions.owned')
	ON channel_entry.operatorbundle_name = operatorbundle.name`)
	if err != nil {
		return err
	}
	defer getChannelEntryProvidedAPIs.Close()

	rows, err := getChannelEntryProvidedAPIs.Query()
	if err != nil {
		return err
	}
	for rows.Next() {
		var channelId sql.NullInt64
		var gvkSQL sql.NullString

		if err := rows.Scan(&channelId, &gvkSQL); err != nil {
			return err
		}
		apigvk := []string{}
		if err := json.Unmarshal([]byte(gvkSQL.String), &apigvk); err != nil {
			return err
		}
		if _, err := addApi.Exec(apigvk[0], apigvk[1], apigvk[2]); err != nil {
			return err
		}
		if _, err := addApiProvider.Exec(apigvk[0], apigvk[1], apigvk[2], channelId.Int64); err != nil {
			return err
		}
	}

	getChannelEntryProvidedAPIsAPIservice, err := tx.Prepare(`
	SELECT DISTINCT channel_entry.entry_id, json_extract(json_each.value, '$.group', '$.version', '$.kind')
	FROM channel_entry INNER JOIN operatorbundle,json_each(operatorbundle.csv, '$.spec.apiservicedefinitions.owned')
	ON channel_entry.operatorbundle_name = operatorbundle.name`)
	if err != nil {
		return err
	}
	defer getChannelEntryProvidedAPIsAPIservice.Close()

	rows, err = getChannelEntryProvidedAPIsAPIservice.Query()
	if err != nil {
		return err
	}
	for rows.Next() {
		var channelId sql.NullInt64
		var gvkSQL sql.NullString

		if err := rows.Scan(&channelId, &gvkSQL); err != nil {
			return err
		}
		apigvk := []string{}
		if err := json.Unmarshal([]byte(gvkSQL.String), &apigvk); err != nil {
			return err
		}
		if _, err := addApi.Exec(apigvk[0], apigvk[1], apigvk[2]); err != nil {
			return err
		}
		if _, err := addApiProvider.Exec(apigvk[0], apigvk[1], apigvk[2], channelId.Int64); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLLoader) Close() {
	s.db.Close()
}
