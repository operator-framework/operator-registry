package sqlite

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestListBundlesQuery(t *testing.T) {
	for _, tt := range []struct {
		Name   string
		Setup  func(t *testing.T, db *sql.DB)
		Expect func(t *testing.T, rows *sql.Rows)
	}{
		{
			Name: "replacement comes from channel entry",
			Setup: func(t *testing.T, db *sql.DB) {
				for _, stmt := range []string{
					`insert into package (name, default_channel) values ("package", "channel")`,
					`insert into channel (name, package_name, head_operatorbundle_name) values ("channel", "package", "bundle")`,
					`insert into operatorbundle (name) values ("bundle-a"), ("bundle-b")`,
					`insert into channel_entry (package_name, channel_name, operatorbundle_name, entry_id, depth, replaces) values ("package", "channel", "bundle-a", 1, 0, 2)`,
					`insert into channel_entry (package_name, channel_name, operatorbundle_name, entry_id, depth) values ("package", "channel", "bundle-b", 2, 1)`,
				} {
					if _, err := db.Exec(stmt); err != nil {
						t.Fatalf("unexpected error executing setup statements: %v", err)
					}
				}

			},
			Expect: func(t *testing.T, rows *sql.Rows) {
				replacements := map[sql.NullString]sql.NullString{
					{String: "bundle-a", Valid: true}: {String: "bundle-b", Valid: true},
					{String: "bundle-b", Valid: true}: {Valid: false},
				}
				for rows.Next() {
					var (
						c            interface{}
						name, actual sql.NullString
					)
					if err := rows.Scan(&c, &c, &c, &name, &c, &c, &actual, &c, &c, &c, &c, &c, &c, &c); err != nil {
						t.Fatalf("unexpected error during row scan: %v", err)
					}
					expected, ok := replacements[name]
					if !ok {
						t.Errorf("unexpected name: %v", name)
						continue
					}
					delete(replacements, name)
					if actual != expected {
						t.Errorf("got replacement %v for %v, expected %v", actual, name, expected)
						continue
					}
				}
				for replacer, replacee := range replacements {
					t.Errorf("missing expected result row: %v replaces %v", replacer, replacee)
				}
			},
		},
		{
			Name: "skips populated from multiple channel entries",
			Setup: func(t *testing.T, db *sql.DB) {
				for _, stmt := range []string{
					`insert into package (name, default_channel) values ("package", "channel")`,
					`insert into channel (name, package_name, head_operatorbundle_name) values ("channel", "package", "bundle")`,
					`insert into operatorbundle (name) values ("bundle-a"), ("bundle-b"), ("bundle-c")`,
					`insert into channel_entry (package_name, channel_name, operatorbundle_name, entry_id, replaces) values ("package", "channel", "bundle-a", 1, 2)`,
					`insert into channel_entry (package_name, channel_name, operatorbundle_name, entry_id) values ("package", "channel", "bundle-b", 2)`,
					`insert into channel_entry (package_name, channel_name, operatorbundle_name, entry_id, replaces) values ("package", "channel", "bundle-a", 3, 4)`,
					`insert into channel_entry (package_name, channel_name, operatorbundle_name, entry_id) values ("package", "channel", "bundle-c", 4)`,
				} {
					if _, err := db.Exec(stmt); err != nil {
						t.Fatalf("unexpected error executing setup statements: %v", err)
					}
				}

			},
			Expect: func(t *testing.T, rows *sql.Rows) {
				type result struct {
					Name     sql.NullString
					Replaces sql.NullString
					Skips    sql.NullString
				}
				expected := map[sql.NullString]result{
					{String: "bundle-a", Valid: true}: {
						Name:     sql.NullString{String: "bundle-a", Valid: true},
						Replaces: sql.NullString{Valid: false},
						Skips:    sql.NullString{String: "bundle-b,bundle-c", Valid: true},
					},
					{String: "bundle-b", Valid: true}: {
						Name:     sql.NullString{String: "bundle-b", Valid: true},
						Replaces: sql.NullString{Valid: false},
						Skips:    sql.NullString{Valid: false},
					},
					{String: "bundle-c", Valid: true}: {
						Name:     sql.NullString{String: "bundle-c", Valid: true},
						Replaces: sql.NullString{Valid: false},
						Skips:    sql.NullString{Valid: false},
					},
				}
				for rows.Next() {
					var (
						c      interface{}
						actual result
					)
					if err := rows.Scan(&c, &c, &c, &actual.Name, &c, &c, &actual.Replaces, &actual.Skips, &c, &c, &c, &c, &c, &c); err != nil {
						t.Fatalf("unexpected error during row scan: %v", err)
					}
					r, ok := expected[actual.Name]
					if !ok {
						t.Errorf("unexpected name: %v", actual.Name)
						continue
					}
					delete(expected, actual.Name)
					if actual != r {
						t.Errorf("got row %v, expected %v for name %v", actual, r, actual.Name)
						continue
					}
				}
				for _, e := range expected {
					t.Errorf("missing expected result row: %v", e)
				}
			},
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			ctx := context.Background()

			db, err := sql.Open("sqlite3", ":memory:")
			if err != nil {
				t.Fatalf("unable to open in-memory sqlite database: %v", err)
			}

			m, err := NewSQLLiteMigrator(db)
			if err != nil {
				t.Fatalf("unable to create database migrator: %v", err)
			}

			if err := m.Migrate(ctx); err != nil {
				t.Fatalf("failed to perform initial schema migration: %v", err)
			}

			tt.Setup(t, db)

			rows, err := db.QueryContext(ctx, listBundlesQuery)
			if err != nil {
				t.Fatalf("unexpected error executing list bundles query: %v", err)
			}
			defer rows.Close()

			tt.Expect(t, rows)
		})
	}
}
