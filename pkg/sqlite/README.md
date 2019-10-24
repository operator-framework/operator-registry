# Operator Registry Database

The Operator Registry generates a [SQLite](https://www.sqlite.org) database from operator manifests to define Operator 
Data used in [OLM](https://github.com/operator-framework/operator-lifecycle-manager). The database is stored in a 
container image and the content is maintained throughout every image and operator update in the long term. The schema
of the registry database is evolving based on usage. To avoid the overhead of rebuilding database every time schema 
is updated and ensure database is compatible to any current operations, we migrate the registry database to the latest 
version upon additive operations. The latest schema of the database is defined in 
[/pkg/sqlite/load.go](https://github.com/operator-framework/operator-registry/blob/master/pkg/sqlite/load.go#L29) file. 


## Database Creation

The registry database are initialized from operator manifests with example manifests available in the 
[/manifests](https://github.com/operator-framework/operator-registry/tree/master/manifests) directory. Upon creation,
the database is [forced](https://godoc.org/github.com/golang-migrate/migrate#Migrate.Force) to use the latest schema. 

## Database Migration

The registry database migration is handled by [golang-migrate](https://github.com/golang-migrate/migrate) and is 
necessary upon additive operation interacting with the database. The migrations are defined in 
[/pkg/sqlite/db_migrations](https://github.com/operator-framework/operator-registry/tree/master/pkg/sqlite/db_migrations) 
directory as SQL files.

Each logical migration is represented by a pair of separate migration files defining `up` and `down` migration 
between a pair of older and newer database versions. The ordering and direction of the migration files are determined
by the filenames using the following format:
```
{version}_{title}.up.sql
{version}_{title}.down.sql
```
where version is represented by date (YYYYMMDD) and time (HHMM) or any 64-bit unsigned integer and title is for 
readability. For example, the 
[/pkg/sqlite/db_migrations](https://github.com/operator-framework/operator-registry/tree/master/pkg/sqlite/db_migrations) 
directory can have the following:

```
201909251211_initialize_version.down.sql
201909251211_initialize_version.up.sql
201909251522_add_table.down.sql
201909251522_add_table.up.sql
...
```

Note: each schema migration should be kept in one transaction in case any commands in a migration fail.

### How to write a migration

To upgrade registry database schema and allow migration from and back to an older version, we need to provide a pair
of SQLite migration files in the
[/pkg/sqlite/db_migrations](https://github.com/operator-framework/operator-registry/tree/master/pkg/sqlite/db_migrations) 
directory. The migration should specify schema transition logic concretely. The `up` logic migrates database schema 
from the older version to the newer where `down` logic migrates schema back to the older version for backup purpose. To 
test the validity of the migration logic, database should be migrated to the latest version, then back down to an earlier 
version and again upgrade to the latest to see if both down and up logic transitions of the schema are as expected.
