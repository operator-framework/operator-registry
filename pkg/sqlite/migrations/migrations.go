package migrations

import (
	"context"
	"database/sql"
	"sort"
)

type Migration struct {
	Id int
	Up func(context.Context, *sql.Tx) error
	Down func(context.Context, *sql.Tx) error
}

type MigrationSet map[int]*Migration

type Migrations []*Migration

func (m Migrations) Len() int           { return len(m) }
func (m Migrations) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m Migrations) Less(i, j int) bool { return m[i].Id < m[j].Id }

var migrations MigrationSet = make(map[int]*Migration)

// From returns a set of migrations, starting at key
func From(key int) Migrations {
	keys := make([]int, 0)
	for k := range migrations {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	sorted := []*Migration{}
	for _, k := range keys {
		if k < key {
			continue
		}
		sorted = append(sorted, migrations[k])
	}
	return sorted
}

// To returns a set of migrations, up to and including key
func To(key int) Migrations {
	keys := make([]int, 0)
	for k := range migrations {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	sorted := []*Migration{}
	for _, k := range keys {
		if k > key {
			continue
		}
		sorted = append(sorted, migrations[k])
	}
	return sorted
}

// Only returns a set of one migration
func Only(key int) Migrations {
	return []*Migration{migrations[key]}
}
