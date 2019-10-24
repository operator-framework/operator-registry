# Add a new migration

Migrations live in `pkg/sqlite/migrations`.

Create a new file (and tests!) that increments the migration number:

```sh
touch pkg/sqlite/migrations/002_migration_description.go
touch pkg/sqlite/migrations/002_migration_description_test.go
```

Create the migration instance and register it:

```go
package migrations

import (
	"context"
	"database/sql"
)

// This should increment the value from the previous migration
const MyMigrationKey = 2


var myNewMigration = &Migration{
    // The id for this migration
	Id: MyMigrationKey,
	Up: func(ctx context.Context, tx *sql.Tx) error {
	    // the up version of this migration
		return nil
	},
	Down: func(ctx context.Context, tx *sql.Tx) error {
	    // the down version of this migration
		return nil
	},
}

// Register this migration 
func init() {
	migrations[MyMigrationKey] = myNewMigration
}
```

See the existing migrations in the `pkg/sqlite/migrations` for examples of migrations and tests.
