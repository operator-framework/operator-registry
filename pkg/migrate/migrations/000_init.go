package migrations

import (
	"context"
	"fmt"
)

var InitMigrationKey = 0

func init() {
	registerMigration(InitMigrationKey, initMigration)
}

var initMigration = &Migration{
	Id: InitMigrationKey,
	Up: func(ctx context.Context, file string) error {
		return nil
	},
	Down: func(ctx context.Context, file string) error {
		return fmt.Errorf("cannot migrate down - no conversion from bolt backend to sqlite backend")
	},
}
