package sqlite_test

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"

	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
	"github.com/operator-framework/operator-registry/pkg/sqlite/sqlitefakes"
	"github.com/stretchr/testify/assert"
)

func TestListBundles(t *testing.T) {
	type Columns struct {
		EntryID         sql.NullInt64
		Bundle          sql.NullString
		BundlePath      sql.NullString
		BundleName      sql.NullString
		PackageName     sql.NullString
		ChannelName     sql.NullString
		Replaces        sql.NullString
		Skips           sql.NullString
		Version         sql.NullString
		SkipRange       sql.NullString
		DependencyType  sql.NullString
		DependencyValue sql.NullString
		PropertyType    sql.NullString
		PropertyValue   sql.NullString
	}

	var NoRows sqlitefakes.FakeRowScanner
	NoRows.NextReturns(false)

	ScanFromColumns := func(t *testing.T, dsts []interface{}, cols Columns) {
		ct := reflect.TypeOf(cols)
		if len(dsts) != ct.NumField() {
			t.Fatalf("expected %d columns, got %d", ct.NumField(), len(dsts))
		}
		for i, dst := range dsts {
			f := ct.Field(i)
			dv := reflect.ValueOf(dst)
			if dv.Kind() != reflect.Ptr {
				t.Fatalf("scan argument at index %d is not a pointer", i)
			}
			if !f.Type.AssignableTo(dv.Elem().Type()) {
				t.Fatalf("%s is not assignable to argument %s at index %d", f.Type, dv.Elem().Type(), i)
			}
			dv.Elem().Set(reflect.ValueOf(cols).Field(i))
		}
	}

	for _, tc := range []struct {
		Name         string
		Querier      func(t *testing.T) sqlite.Querier
		Bundles      []*api.Bundle
		ErrorMessage string
	}{
		{
			Name: "returns error when query returns error",
			Querier: func(t *testing.T) sqlite.Querier {
				var q sqlitefakes.FakeQuerier
				q.QueryContextReturns(nil, fmt.Errorf("test"))
				return &q
			},
			ErrorMessage: "test",
		},
		{
			Name: "returns error when scan returns error",
			Querier: func(t *testing.T) sqlite.Querier {
				var (
					q sqlitefakes.FakeQuerier
					r sqlitefakes.FakeRowScanner
				)
				q.QueryContextReturns(&r, nil)
				r.NextReturnsOnCall(0, true)
				r.ScanReturns(fmt.Errorf("test"))
				return &q
			},
			ErrorMessage: "test",
		},
		{
			Name: "skips row without valid bundle name",
			Querier: func(t *testing.T) sqlite.Querier {
				var (
					q sqlitefakes.FakeQuerier
					r sqlitefakes.FakeRowScanner
				)
				q.QueryContextReturns(&r, nil)
				r.NextReturnsOnCall(0, true)
				r.ScanCalls(func(args ...interface{}) error {
					ScanFromColumns(t, args, Columns{
						Version:     sql.NullString{Valid: true},
						BundlePath:  sql.NullString{Valid: true},
						ChannelName: sql.NullString{Valid: true},
					})
					return nil
				})
				return &q
			},
		},
		{
			Name: "skips row without valid version",
			Querier: func(t *testing.T) sqlite.Querier {
				var (
					q sqlitefakes.FakeQuerier
					r sqlitefakes.FakeRowScanner
				)
				q.QueryContextReturns(&r, nil)
				r.NextReturnsOnCall(0, true)
				r.ScanCalls(func(args ...interface{}) error {
					ScanFromColumns(t, args, Columns{
						BundleName:  sql.NullString{Valid: true},
						BundlePath:  sql.NullString{Valid: true},
						ChannelName: sql.NullString{Valid: true},
					})
					return nil
				})
				return &q
			},
		},
		{
			Name: "skips row without valid bundle path",
			Querier: func(t *testing.T) sqlite.Querier {
				var (
					q sqlitefakes.FakeQuerier
					r sqlitefakes.FakeRowScanner
				)
				q.QueryContextReturns(&r, nil)
				r.NextReturnsOnCall(0, true)
				r.ScanCalls(func(args ...interface{}) error {
					ScanFromColumns(t, args, Columns{
						BundleName:  sql.NullString{Valid: true},
						Version:     sql.NullString{Valid: true},
						ChannelName: sql.NullString{Valid: true},
					})
					return nil
				})
				return &q
			},
		},
		{
			Name: "skips row without valid channel name",
			Querier: func(t *testing.T) sqlite.Querier {
				var (
					q sqlitefakes.FakeQuerier
					r sqlitefakes.FakeRowScanner
				)
				q.QueryContextReturns(&r, nil)
				r.NextReturnsOnCall(0, true)
				r.ScanCalls(func(args ...interface{}) error {
					ScanFromColumns(t, args, Columns{
						BundleName: sql.NullString{Valid: true},
						Version:    sql.NullString{Valid: true},
						BundlePath: sql.NullString{Valid: true},
					})
					return nil
				})
				return &q
			},
		},
		{
			Name: "bundle dependencies are null when dependency type or value is null",
			Querier: func(t *testing.T) sqlite.Querier {
				var (
					q sqlitefakes.FakeQuerier
					r sqlitefakes.FakeRowScanner
				)
				q.QueryContextReturns(&r, nil)
				r.NextReturnsOnCall(0, true)
				r.ScanCalls(func(args ...interface{}) error {
					ScanFromColumns(t, args, Columns{
						BundleName:  sql.NullString{Valid: true},
						Version:     sql.NullString{Valid: true},
						ChannelName: sql.NullString{Valid: true},
						BundlePath:  sql.NullString{Valid: true},
					})
					return nil
				})
				return &q
			},
			Bundles: []*api.Bundle{
				{},
			},
		},
		{
			Name: "all dependencies and properties are returned",
			Querier: func(t *testing.T) sqlite.Querier {
				var (
					q sqlitefakes.FakeQuerier
					r sqlitefakes.FakeRowScanner
				)
				q.QueryContextReturns(&NoRows, nil)
				q.QueryContextReturnsOnCall(0, &r, nil)
				r.NextReturnsOnCall(0, true)
				r.NextReturnsOnCall(1, true)
				cols := []Columns{
					{
						BundleName:      sql.NullString{Valid: true, String: "BundleName"},
						Version:         sql.NullString{Valid: true, String: "Version"},
						ChannelName:     sql.NullString{Valid: true, String: "ChannelName"},
						BundlePath:      sql.NullString{Valid: true, String: "BundlePath"},
						DependencyType:  sql.NullString{Valid: true, String: "Dependency1Type"},
						DependencyValue: sql.NullString{Valid: true, String: "Dependency1Value"},
						PropertyType:    sql.NullString{Valid: true, String: "Property1Type"},
						PropertyValue:   sql.NullString{Valid: true, String: "Property1Value"},
					},
					{
						BundleName:      sql.NullString{Valid: true, String: "BundleName"},
						Version:         sql.NullString{Valid: true, String: "Version"},
						ChannelName:     sql.NullString{Valid: true, String: "ChannelName"},
						BundlePath:      sql.NullString{Valid: true, String: "BundlePath"},
						DependencyType:  sql.NullString{Valid: true, String: "Dependency2Type"},
						DependencyValue: sql.NullString{Valid: true, String: "Dependency2Value"},
						PropertyType:    sql.NullString{Valid: true, String: "Property2Type"},
						PropertyValue:   sql.NullString{Valid: true, String: "Property2Value"},
					},
				}
				var i int
				r.ScanCalls(func(args ...interface{}) error {
					if i < len(cols) {
						ScanFromColumns(t, args, cols[i])
						i++
					}
					return nil
				})
				return &q
			},
			Bundles: []*api.Bundle{
				{
					CsvName:     "BundleName",
					ChannelName: "ChannelName",
					BundlePath:  "BundlePath",
					Version:     "Version",
					Dependencies: []*api.Dependency{
						{
							Type:  "Dependency1Type",
							Value: "Dependency1Value",
						},
						{
							Type:  "Dependency2Type",
							Value: "Dependency2Value",
						},
					},
					Properties: []*api.Property{
						{
							Type:  "Property1Type",
							Value: "Property1Value",
						},
						{
							Type:  "Property2Type",
							Value: "Property2Value",
						},
					},
				},
			},
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			var q sqlite.Querier
			if tc.Querier != nil {
				q = tc.Querier(t)
			}
			sq := sqlite.NewSQLLiteQuerierFromDBQuerier(q)
			bundles, err := sq.ListBundles(context.Background())

			assert := assert.New(t)
			assert.Equal(tc.Bundles, bundles)
			if tc.ErrorMessage == "" {
				assert.NoError(err)
			} else {
				assert.EqualError(err, tc.ErrorMessage)
			}
		})
	}
}

func TestGetBundleReplacesDepth(t *testing.T) {
	for _, tt := range []struct {
		Name  string
		Setup func(t *testing.T, db *sql.DB)
		Want  map[string]int64
	}{
		{
			Name: "list depth for real replaces entries from channel_entry",
			Setup: func(t *testing.T, db *sql.DB) {
				for _, stmt := range []string{
					`insert into operatorbundle (name) values ("0.0.1"), ("0.2.0"), ("1.0.1-rc2"), ("1.1.1"), ("1.2.0")`,
					`insert into channel_entry (package_name, channel_name, operatorbundle_name, entry_id, depth) values 
							("pkg", "stable", "0.0.0", 1, 0),
							("pkg", "fast", "0.0.0", 2, 0),
							("pkg", "stable", "0.0.1", 3, 0),
							("pkg", "stable", "0.1.0", 4, 0),
							("pkg", "stable", "0.2.0", 5, 0),
							("pkg", "fast", "0.2.0", 6, 0),
							("pkg", "fast", "1.0.1-rc1", 7, 0),
							("pkg", "fast", "1.0.1-rc2", 8, 0),
							("pkg", "stable", "1.2.0", 9, 0),
							("pkg", "fast", "1.2.0", 10, 0)`,
					`insert into channel_entry (package_name, channel_name, operatorbundle_name, entry_id, replaces, depth) values 
							("pkg", "stable", "1.2.0", 11, 1, 4),
							("pkg", "stable", "1.2.0", 12, 3, 3),
							("pkg", "stable", "1.2.0", 13, 4, 2),
							("pkg", "stable", "1.2.0", 14, 5, 1),
							("pkg", "fast", "1.2.0", 15, 2, 4),
							("pkg", "fast", "1.2.0", 16, 6, 3),
							("pkg", "fast", "1.2.0", 17, 7, 2),
							("pkg", "fast", "1.2.0", 18, 8, 1)`,
				} {
					if _, err := db.Exec(stmt); err != nil {
						t.Fatalf("unexpected error executing setup statements: %v", err)
					}
				}
			},
			Want: map[string]int64{
				"0.0.1":     3,
				"0.2.0":     1,
				"1.0.1-rc2": 1,
			},
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			ctx := context.Background()

			db, err := sql.Open("sqlite3", ":memory:")
			if err != nil {
				t.Fatalf("unable to open in-memory sqlite database: %v", err)
			}

			m, err := sqlite.NewSQLLiteMigrator(db)
			if err != nil {
				t.Fatalf("unable to create database migrator: %v", err)
			}

			if err := m.Migrate(ctx); err != nil {
				t.Fatalf("failed to perform initial schema migration: %v", err)
			}

			tt.Setup(t, db)

			q := sqlite.NewSQLLiteQuerierFromDb(db)
			replaces, err := q.GetBundleReplacesDepth(ctx, "pkg", "1.2.0")
			require.NoError(t, err)
			require.EqualValues(t, tt.Want, replaces)
		})
	}
}
