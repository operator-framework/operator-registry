# Add database versioning and migrations to output database

Status: Pending

Version: alpha

Implementation owner: TBD

## Abstract

Proposal to add database versioning and migration capability to the output of operator-registry commands

## Motivation

In an attempt at optimizing the registry database usage in OLM to allow the registry database to be stored in a container image, a method is needed to add to the database over time. Since these database files will be long living, it becomes a requirement that the database can migrate to a new schema over time. This proposal attempts to solve that problem by implementing a method with which database versions can be migrated with existing db files over time.

## Proposal

### Migration Method

At a high level, the migration method is relatively straightforward. The database will contain a current migration version and when any additive operation is called with an existing database as input, the first step will be to run the migration to upgrade the database schema to the latest version.

Rather than reinventing the database migration process, operator-registry will use an existing database migration tool that has already defined a set of semantics and conventions. This proposal suggests using the [golang-migrate](https://github.com/golang-migrate/migrate) tool for this purpose. Golang-migrate uses a set of conventions to define migrations.

Firstly, it defines migrations as a set of sql files that live in a flat database folder. Migrations are individually defined as a pair of up and down migration scripts. These up and down scripts each define a method of going to and back from a particular database version and should be opposites semantically. Each script pair has a unique 64 bit unsigned integer identifier followed by an underscore, as well as a title, a direction, and ends in the `.sql` extension: `${version}_${title}.${direction(down/up)}.sql`. The versions are ordered, with the lowest integer value coming first and the latest version defining the current database migration version in the db. For more details on the migration format, please see https://github.com/golang-migrate/migrate/blob/master/MIGRATIONS.md.

```
 # example migrations folder
 db_migrations
 ├── 201909251522_add_users_table.up.sql
 ├── 201909251522_add_users_table.down.sql
 ├── 201909251510_first_migration.up.sql
 └── 201909251510_first_migration.down.sql
```

Once that migration schema is defined, whenever any additive operation is run against the database we will first use the migrate API to upgrade the schema to the latest version. See https://godoc.org/github.com/golang-migrate/migrate#Migrate.Up for more details on that API.

### Versioning fresh databases

One other consideration of note here is that the operator-registry database is not a long living database in the traditional sense, since the database is often created from scratch. All common migration tools do not generally account for such an edge case, so this proposal also defines a method of initializing the database at a particular migration version. Since the migration version is a matter of convention on file names, we can infer the migration version by parsing the migration folder and finding the latest migration script version. Then, once we initialize the database on startup we can use `golang-migrate` API to force the initial migration version. See https://godoc.org/github.com/golang-migrate/migrate#Migrate.Force for more details on that API.

### Schema Definition

One thing to note is the choice of how the database schema is defined. Currently that schema is defined in source code here in [/pkg/sqlite/load.go](https://github.com/operator-framework/operator-registry/blob/master/pkg/sqlite/load.go#L29) as a list of create table statements.

However, once sql migrations are written the database schema is sometimes generated in two ways (through a migration upgrade or from scratch) as defined above. In that case, there are a few ways to do that. One is to leave the initial schema as is and use the migrations on a clean install as well -- but that means that the schema is not defined in one single human readable place. This can commonly be worked around by using a migration tool that can output an example of the sql schema for the purpose of development.

The other option, and the one that this proposal defines as the solution, is to keep the schema definition in `load.go` in sync with the changes defined in each migration. In this case, it is possible for the database to go out of sync on migration vs from scratch in the case where the migration was written properly. This can be easily mitigated by writing an automated test that always ensures that some initial bundle database can be migrated to a latest version and have the same schema as a clean database.

As a result, this implementation will require writing tests to ensure that the schema versioning is in sync between upgrade and scratch creation.

### Choice of Migration Tool

One point of this proposal is deciding to use the `golang-migrate` project as a method of driving migration conventions and the migrations themselves. Below is a list of criteria that this tool fulfilled when making that choice:

- Popular and well supported
- Good documentation
- Supports lots of different database drivers, in the event that we move away from sqlite we can maintain the use of workflows around migrations
- No need to ship a migration binary, migration can be handled by importing `golang-migrate` and running in source
- Has a nice API for upgrading and reading the schema version for our scratch scenario that doesn't require us to expect a particular convention of database versioning
- Allows defining migration versions as timestamps
