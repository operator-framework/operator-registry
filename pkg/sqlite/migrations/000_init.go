package migrations

import (
	"context"
	"database/sql"
)

var InitMigrationKey = 0

func init() {
	registerMigration(InitMigrationKey, initMigration)
}

var initMigration = &Migration{
	Id: InitMigrationKey,
	Up: func(ctx context.Context, tx *sql.Tx) error {
		sql := `
		CREATE TABLE IF NOT EXISTS operatorbundle (
			name string,
			csv blob,
			bundle blob,
		    skiprange string,
		    version string,
		    bundlepath string
		);
		CREATE TABLE IF NOT EXISTS package (
			name string,
			default_channel string
		);
		CREATE TABLE IF NOT EXISTS channel (
			name string,
			package_name string,
			head_operatorbundle_name string
		);
		CREATE TABLE IF NOT EXISTS channel_entry (
			channel_name string,
			package_name string,
			operatorbundle_name string,
			replaces int,
			depth int 
		);
		CREATE TABLE IF NOT EXISTS api_provider (
			group_name string,
			version string,
			kind string,
			plural string,
			channel_entry_id int
		);
		CREATE TABLE IF NOT EXISTS api_requirer (
			group_name string,
			version string,
			kind string,
			plural string,
			channel_entry_id int
		);
		CREATE TABLE IF NOT EXISTS related_image (
			image string,
     		operatorbundle_name string
		);
		`
		_, err := tx.ExecContext(ctx, sql)
		return err
	},
	Down: func(ctx context.Context, tx *sql.Tx) error {
		sql := `
			DROP TABLE operatorbundle;
			DROP TABLE package;
			DROP TABLE channel;
			DROP TABLE channel_entry;
			DROP TABLE api;
			DROP TABLE api_provider;
			DROP TABLE api_requirer;
			DROP TABLE related_image;
		`
		_, err := tx.ExecContext(ctx, sql)

		return err
	},
}
