package migrations

import (
	"context"
	"database/sql"
)

const (
	NonemptyRelatedImageMigrationKey   = 9
	NonemptyRelatedImageConstraintName = "nonempty_related_image"
)

func init() {
	registerMigration(NonemptyRelatedImageMigrationKey, nonemptyRelatedImageMigration)
}

var nonemptyRelatedImageMigration = &Migration{
	Id: NonemptyRelatedImageMigrationKey,
	Up: func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
ALTER TABLE related_image RENAME TO related_image_migrating;
CREATE TABLE related_image (
  image TEXT,
  operatorbundle_name TEXT,
  FOREIGN KEY(operatorbundle_name) REFERENCES operatorbundle(name) ON DELETE CASCADE,
  CONSTRAINT `+NonemptyRelatedImageConstraintName+` CHECK (length(image))
);
INSERT INTO related_image
  SELECT * FROM related_image_migrating
  WHERE length(image) > 0;
DROP TABLE related_image_migrating;`)
		return err
	},
	Down: func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
ALTER TABLE related_image RENAME TO related_image_migrating;
CREATE TABLE related_image (
  image TEXT,
  operatorbundle_name TEXT,
  FOREIGN KEY(operatorbundle_name) REFERENCES operatorbundle(name) ON DELETE CASCADE
);
INSERT INTO related_image SELECT * FROM related_image_migrating;
DROP TABLE related_image_migrating;`)
		return err
	},
}
