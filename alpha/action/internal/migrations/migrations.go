package migrations

import (
	"fmt"
	"slices"
	"strings"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

type Migration interface {
	Name() string
	Migrate(*declcfg.DeclarativeConfig) error
}

func NewMigration(name string, fn func(config *declcfg.DeclarativeConfig) error) Migration {
	return &simpleMigration{name: name, fn: fn}
}

type simpleMigration struct {
	name string
	fn   func(*declcfg.DeclarativeConfig) error
}

func (s simpleMigration) Name() string {
	return s.name
}

func (s simpleMigration) Migrate(config *declcfg.DeclarativeConfig) error {
	return s.fn(config)
}

type Migrations struct {
	migrations []Migration
}

var allMigrations = []Migration{
	BundleObjectToCSVMetadata,
}

func NewMigrations(level string) (*Migrations, error) {
	migrations := slices.Clone(allMigrations)
	if level == "" {
		return &Migrations{migrations: migrations}, nil
	}

	found := false
	keep := migrations[:0]
	for _, migration := range migrations {
		keep = append(keep, migration)
		if migration.Name() == level {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("unknown migration level %q", level)
	}
	return &Migrations{migrations: keep}, nil
}

func (m *Migrations) HelpText() string {
	var help strings.Builder
	help.WriteString("-- Migrations --\n")
	help.WriteString("  To run a migration, use the --level flag with the migration name.\n")
	help.WriteString("  The migrator will run all migrations up to and including the selected level.\n\n")
	help.WriteString("  Available migration levels:\n")
	if len(m.migrations) == 0 {
		help.WriteString("   (no migrations available in this version)\n")
	}
	for i, migration := range m.migrations {
		help.WriteString(fmt.Sprintf("   - %s\n", i+1, migration.Name()))
	}
	return help.String()
}

func (m *Migrations) Migrate(config *declcfg.DeclarativeConfig) error {
	for _, migration := range m.migrations {
		if err := migration.Migrate(config); err != nil {
			return err
		}
	}
	return nil
}
