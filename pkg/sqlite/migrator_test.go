package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/sqlite/migrations"
)

func TestNewSQLLiteMigrator(t *testing.T) {
	type args struct {
		db *sql.DB
	}
	tests := []struct {
		name    string
		args    args
		want    Migrator
		wantErr bool
	}{
		{
			name: "uses default table",
			args: args{&sql.DB{}},
			want: &SQLLiteMigrator{db: &sql.DB{}, migrationsTable: DefaultMigrationsTable},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSQLLiteMigrator(tt.args.db)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSQLLiteMigrator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewSQLLiteMigrator() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSQLLiteMigrator_Down(t *testing.T) {
	var up bool
	var down bool

	type fields struct {
		migrationsTable string
	}
	type args struct {
		ctx        context.Context
		migrations []*migrations.Migration
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		wantUp bool
		wantDown bool
		wantVersion int
	}{
		{
			name: "run test migration",
			fields: fields {migrationsTable: DefaultMigrationsTable},
			args: args{ctx: context.TODO(), migrations: []*migrations.Migration{{
				Id:   0,
				Up: func(ctx context.Context, tx *sql.Tx) error {
					up = true
					return nil
				},
				Down: func(ctx context.Context, tx *sql.Tx) error {
					down = true
					return nil
				},
			}}},
			wantUp: false,
			wantDown: true,
			wantVersion: -1,
		},
		{
			name: "run migration out of order",
			fields: fields {migrationsTable: DefaultMigrationsTable},
			args: args{ctx: context.TODO(), migrations: []*migrations.Migration{{
				Id:   1,
				Up: func(ctx context.Context, tx *sql.Tx) error {
					up = true
					return nil
				},
				Down: func(ctx context.Context, tx *sql.Tx) error {
					down = true
					return nil
				},
			}}},
			wantUp: false,
			wantDown: false,
			wantErr: true,
			wantVersion: 0,
		},
		{
			name: "run error migration",
			fields: fields {migrationsTable: DefaultMigrationsTable},
			args: args{ctx: context.TODO(), migrations: []*migrations.Migration{{
				Id:   0,
				Up: func(ctx context.Context, tx *sql.Tx) error {
					return fmt.Errorf("error")
				},
				Down: func(ctx context.Context, tx *sql.Tx) error {
					return fmt.Errorf("error")
				},
			}}},
			wantErr: true,
			wantUp: false,
			wantDown: false,
			wantVersion: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			up = false
			down = false
			db, cleanup := CreateTestDb(t)
			defer cleanup()
			m := &SQLLiteMigrator{
				db:              db,
				migrationsTable: tt.fields.migrationsTable,
			}
			{
				tx, err := db.Begin()
				require.NoError(t, err)
				require.NoError(t, m.setVersion(context.TODO(), tx, 0))
				require.NoError(t, err)
				require.NoError(t, tx.Commit())
				tx.Rollback()
			}
			if err := m.Down(tt.args.ctx, tt.args.migrations); (err != nil) != tt.wantErr {
				t.Errorf("Down() error = %v, wantErr %v", err, tt.wantErr)
			}
			require.Equal(t, tt.wantUp, up)
			require.Equal(t, tt.wantDown, down)

			// verify the version is correct
			var version int
			{
				tx, err := db.Begin()
				require.NoError(t, err)
				version, err = m.version(context.TODO(), tx)
				require.NoError(t, err)
				require.NoError(t, tx.Commit())
				tx.Rollback()
			}
			require.Equal(t, tt.wantVersion, version)
		})
	}
}

func TestSQLLiteMigrator_Up(t *testing.T) {
	var up int
	var down bool

	type fields struct {
		migrationsTable string
	}
	type args struct {
		ctx        context.Context
		migrations migrations.Migrations
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		wantUp int
		wantDown bool
		wantVersion int
	}{
		{
			name: "run test migration",
			fields: fields {migrationsTable: DefaultMigrationsTable},
			args: args{ctx: context.TODO(), migrations: migrations.Migrations{{
				Id:   0,
				Up: func(ctx context.Context, tx *sql.Tx) error {
					up++
					return nil
				},
				Down: func(ctx context.Context, tx *sql.Tx) error {
					down = true
					return nil
				},
			}}},
			wantUp: 1,
			wantDown: false,
			wantVersion: 0,
		},
		{
			name: "run multiple test migration",
			fields: fields {migrationsTable: DefaultMigrationsTable},
			args: args{ctx: context.TODO(), migrations: migrations.Migrations{
				{
					Id:   0,
					Up: func(ctx context.Context, tx *sql.Tx) error {
						up++
						return nil
					},
					Down: func(ctx context.Context, tx *sql.Tx) error {
						down = true
						return nil
					},
				},
				{
					Id:   1,
					Up: func(ctx context.Context, tx *sql.Tx) error {
						up++
						return nil
					},
					Down: func(ctx context.Context, tx *sql.Tx) error {
						down = true
						return nil
					},
				},
			}},
			wantUp: 2,
			wantDown: false,
			wantVersion: 1,
		},
		{
			name: "run migrations out of order",
			fields: fields {migrationsTable: DefaultMigrationsTable},
			args: args{ctx: context.TODO(), migrations: migrations.Migrations{
				{
					Id:   1,
					Up: func(ctx context.Context, tx *sql.Tx) error {
						up++
						return nil
					},
					Down: func(ctx context.Context, tx *sql.Tx) error {
						down = true
						return nil
					},
				},
				{
					Id:   0,
					Up: func(ctx context.Context, tx *sql.Tx) error {
						up++
						return nil
					},
					Down: func(ctx context.Context, tx *sql.Tx) error {
						down = true
						return nil
					},
				},
			}},
			wantUp: 0,
			wantDown: false,
			wantErr: true,
			wantVersion: -1,
		},
		{
			name: "run error migration",
			fields: fields {migrationsTable: DefaultMigrationsTable},
			args: args{ctx: context.TODO(), migrations: migrations.Migrations{{
				Id:   0,
				Up: func(ctx context.Context, tx *sql.Tx) error {
					return fmt.Errorf("error")
				},
				Down: func(ctx context.Context, tx *sql.Tx) error {
					return fmt.Errorf("error")
				},
			}}},
			wantErr: true,
			wantUp: 0,
			wantDown: false,
			wantVersion: -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			up = 0
			down = false
			db, cleanup := CreateTestDb(t)
			defer cleanup()
			m := &SQLLiteMigrator{
				db:              db,
				migrationsTable: tt.fields.migrationsTable,
			}
			{
				tx, err := db.Begin()
				require.NoError(t, err)
				require.NoError(t, m.setVersion(context.TODO(), tx, -1))
				require.NoError(t, err)
				require.NoError(t, tx.Commit())
				tx.Rollback()
			}
			if err := m.Up(tt.args.ctx, tt.args.migrations); (err != nil) != tt.wantErr {
				t.Errorf("Up() error = %v, wantErr %v", err, tt.wantErr)
			}
			require.Equal(t, tt.wantUp, up)
			require.Equal(t, tt.wantDown, down)

			// verify the version is correct
			var version int
			{
				tx, err := db.Begin()
				require.NoError(t, err)
				version, err = m.version(context.TODO(), tx)
				require.NoError(t, err)
				require.NoError(t, tx.Commit())
				tx.Rollback()
			}
			require.Equal(t, tt.wantVersion, version)

			if tt.wantErr {
				return
			}

			// walk backwards back to zero
			sort.Sort(sort.Reverse(tt.args.migrations))
			err := m.Down(tt.args.ctx, tt.args.migrations)
			require.NoError(t, err)
			{
				tx, err := db.Begin()
				require.NoError(t, err)
				version, err = m.version(context.TODO(), tx)
				require.NoError(t, err)
				require.NoError(t, tx.Commit())
				tx.Rollback()
			}
			require.Equal(t, NilVersion, version)
		})
	}
}
