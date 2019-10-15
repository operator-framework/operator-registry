package sqlite

type DbOptions struct {
	// OutFileName is used to define the database file name that is generated from the loader
	OutFileName string

	// Migrator refers to the SQL migrator used to initialize the database with
	MigrationsPath string
}

type DbOption func(*DbOptions)

func WithDBName(name string) DbOption {
	return func(o *DbOptions) {
        o.OutFileName = name
    }
}

func WithMigrationsPath(path string) DbOption {
	return func(o *DbOptions) {
        o.MigrationsPath = path
    }
}
