package migrations_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/sqlite/migrations"
)

func TestNonemptyRelatedImageMigrationUp(t *testing.T) {
	require := require.New(t)

	db, migrator, cleanup := CreateTestDbAt(t, migrations.NonemptyRelatedImageMigrationKey-1)
	defer cleanup()

	_, err := db.Exec(`INSERT INTO operatorbundle(name) VALUES(?)`, "test")
	require.NoError(err)

	for _, image := range []string{"one", "", "three"} {
		_, err := db.Exec(`INSERT INTO related_image(image, operatorbundle_name) VALUES(?,"test")`, image)
		require.NoError(err)
	}

	err = migrator.Up(context.TODO(), migrations.Only(migrations.NonemptyRelatedImageMigrationKey))
	require.NoError(err)

	rows, err := db.Query(`SELECT image, operatorbundle_name FROM related_image`)
	require.NoError(err)

	var images []string
	for rows.Next() {
		var image, bundle string
		require.NoError(rows.Scan(&image, &bundle))
		require.Equal("test", bundle)
		images = append(images, image)
	}

	require.ElementsMatch([]string{"one", "three"}, images)
}

func TestNonemptyRelatedImageMigrationImageConstraint(t *testing.T) {
	require := require.New(t)

	db, _, cleanup := CreateTestDbAt(t, migrations.NonemptyRelatedImageMigrationKey)
	defer cleanup()

	_, err := db.Exec(`INSERT INTO operatorbundle(name) VALUES(?)`, "test")
	require.NoError(err)

	_, err = db.Exec(`INSERT INTO related_image(image, operatorbundle_name) VALUES("","test")`)
	require.Error(err)
	require.Contains(err.Error(), migrations.NonemptyRelatedImageConstraintName)
}

func TestNonemptyRelatedImageMigrationDown(t *testing.T) {
	require := require.New(t)

	db, migrator, cleanup := CreateTestDbAt(t, migrations.NonemptyRelatedImageMigrationKey)
	defer cleanup()

	_, err := db.Exec(`INSERT INTO operatorbundle(name) VALUES(?)`, "test")
	require.NoError(err)

	for _, image := range []string{"one", "two", "three"} {
		_, err := db.Exec(`INSERT INTO related_image(image, operatorbundle_name) VALUES(?,"test")`, image)
		require.NoError(err)
	}

	err = migrator.Down(context.TODO(), migrations.Only(migrations.NonemptyRelatedImageMigrationKey))
	require.NoError(err)

	rows, err := db.Query(`SELECT image, operatorbundle_name FROM related_image`)
	require.NoError(err)

	var images []string
	for rows.Next() {
		var image, bundle string
		require.NoError(rows.Scan(&image, &bundle))
		require.Equal("test", bundle)
		images = append(images, image)
	}

	require.ElementsMatch([]string{"one", "two", "three"}, images)
}
