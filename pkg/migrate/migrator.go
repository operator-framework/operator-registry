package migrate

import (
	"context"

	"github.com/operator-framework/operator-registry/pkg/migrate/migrations"
)

type Migrator interface {
	Migrate(ctx context.Context) error
	Up(ctx context.Context, migrations migrations.Migrations) error
	Down(ctx context.Context, migrations migrations.Migrations) error
}