package migrations

import (
	"fmt"
	"slices"
	"strings"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

type Migration interface {
	Name() string
	Help() string
	Migrate(*declcfg.DeclarativeConfig) error
}

func newMigration(name string, help string, fn func(config *declcfg.DeclarativeConfig) error) Migration {
	return &simpleMigration{name: name, help: help, fn: fn}
}

type simpleMigration struct {
	name string
	help string
	fn   func(*declcfg.DeclarativeConfig) error
}

func (s simpleMigration) Name() string {
	return s.name
}

func (s simpleMigration) Migrate(config *declcfg.DeclarativeConfig) error {
	return s.fn(config)
}

func (s simpleMigration) Help() string {
	return s.help
}

type Migrations struct {
	Migrations []Migration
}

// allMigrations represents the migration catalog
// the order of these migrations is important
var allMigrations = []Migration{
	BundleObjectToCSVMetadata,
}

func NewMigrations(level string) (*Migrations, error) {
	if level == "" {
		return &Migrations{}, nil
	}

	migrations := slices.Clone(allMigrations)

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
	return &Migrations{Migrations: keep}, nil
}

func HelpText() string {
	var help strings.Builder
	help.WriteString("  The migrator will run all migrations up to and including the selected level.\n\n")
	help.WriteString("  Available migrators:\n")
	if len(allMigrations) == 0 {
		help.WriteString("   (no migrations available in this version)\n")
	}
	for _, migration := range allMigrations {
		help.WriteString(fmt.Sprintf("  - %s\n", migration.Name()))
	}
	return help.String()
}

func (m *Migrations) Migrate(config *declcfg.DeclarativeConfig) error {
	for _, migration := range m.Migrations {
		if err := migration.Migrate(config); err != nil {
			return err
		}
	}
	return nil
}
