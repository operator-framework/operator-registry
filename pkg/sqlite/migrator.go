package sqlite

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file" // indirect import required by golang-migrate package
)

type SQLMigrator struct {
	db             *sql.DB
	migrationsPath string
	generated      bool
}

// NewSQLLiteMigrator returns a SQLMigrator. The SQLMigrator takes a sql database and directory for migrations
// and exposes a set of functions that allow the golang-migrate project to apply migrations to that database.
func NewSQLLiteMigrator(db *sql.DB, migrationsPath string) (*SQLMigrator, error) {
	// If no migrations folder is set, use the generated migrations
	if migrationsPath == "" {
		// Create a temp dir for the generated migrations
		tempDir, err := ioutil.TempDir(".", "db_migrations_")
		if err != nil {
			return nil, err
		}

		migrationsFolder := "pkg/sqlite/db_migrations"

		dirData, err := AssetDir(migrationsFolder)
		if err != nil {
			return nil, err
		}

		for _, file := range dirData {
			fileData, err := Asset(fmt.Sprintf("%s/%s", migrationsFolder, file))
			if err != nil {
				return nil, err
			}

			f, err := os.Create(fmt.Sprintf("%s/%s", tempDir, file))
			if err != nil {
				return nil, err
			}
			defer f.Close()

			_, err = f.Write(fileData)
			if err != nil {
				return nil, err
			}
		}

		return &SQLMigrator{
			db:             db,
			migrationsPath: tempDir,
			generated:      true,
		}, nil
	}

	return &SQLMigrator{
		db:             db,
		migrationsPath: migrationsPath,
		generated:      false,
	}, nil
}

// CleanUpMigrator deletes any unnecessary data generated just for the scope of the migrator.
// Call this function once the scope of the Migrator is no longer required
func (m *SQLMigrator) CleanUpMigrator() {
	if m.generated {
		os.RemoveAll(m.migrationsPath)
	}

	return
}

// InitMigrationVersion parses the db_migrations for the latest migration version, then applies that
// version manually to the database after the DB in initialized. This is to ensure that the correct
// future db migration will be run once the database is built.
//
// This function does not take any input and relies on the db defined in the SQLMigrator object
// to connect to the database.
func (m *SQLMigrator) InitMigrationVersion() error {
	instance, err := sqlite3.WithInstance(m.db, &sqlite3.Config{})
	if err != nil {
		return err
	}

	migrator, err := migrate.NewWithDatabaseInstance(fmt.Sprintf("file://%s", m.migrationsPath), "registrydb", instance)
	if err != nil {
		return err
	}

	initVersion, err := getLatestMigrationVersion(m.migrationsPath)
	if err != nil {
		return err
	}

	err = migrator.Force(initVersion)
	if err != nil {
		return err
	}

	return nil
}

// MigrateUp is a wrapper around golang-migrate's Up. Up
// looks at the currently active migration version and will
// migrate all the way up (applying all up migrations).
func (m *SQLMigrator) MigrateUp(dbName string) error {
	instance, err := sqlite3.WithInstance(m.db, &sqlite3.Config{DatabaseName: dbName})
	if err != nil {
		return err
	}

	migrator, err := migrate.NewWithDatabaseInstance(fmt.Sprintf("file://%s", m.migrationsPath), "registrydb", instance)
	if err != nil {
		return err
	}

	err = migrator.Up()
	if err != nil {
		if err == migrate.ErrNoChange {
			return nil
		}
		return err
	}

	return nil
}

// CurrentVersion returns the version of the database associated with the migrator
func (m *SQLMigrator) CurrentVersion() (uint, error) {
	instance, err := sqlite3.WithInstance(m.db, &sqlite3.Config{})
	if err != nil {
		return 0, err
	}

	migrator, err := migrate.NewWithDatabaseInstance(fmt.Sprintf("file://%s", m.migrationsPath), "registrydb", instance)
	if err != nil {
		return 0, err
	}

	version, _, err := migrator.Version()
	if err != nil {
		return 0, err
	}

	return version, nil
}

// getLatestMigrationVersion returns the latest migration version by parsing the files in the db_migrations
// folder and returning the highest value
//
// This function makes the assumption that all files in the db_migrations follow the naming convention
// ${VERSION}_${NAME}.${DIRECTION(down/up)}.sql, which is also expected by the migration package
func getLatestMigrationVersion(path string) (int, error) {
	var versions []int
	err := filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".sql") {
			return nil
		}

		versionString := strings.Split(f.Name(), "_")[0]
		version, err := strconv.Atoi(versionString)
		if err != nil {
			return err
		}

		versions = append(versions, version)

		return nil
	})
	if err != nil {
		return 0, err
	}

	sort.Ints(versions)

	return versions[len(versions)-1], nil
}
