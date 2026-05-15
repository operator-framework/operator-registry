package migrate

import (
	"log"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/action/migrations"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

func NewCmd() *cobra.Command {
	var (
		migrate      action.Migrate
		migrateLevel string
		output       string
	)
	cmd := &cobra.Command{
		Use:   "migrate <catalogRef> <outputDir>",
		Short: "Migrate a file-based catalog to a new file-based catalog applying optional migrations",
		Long: `Migrate a file-based catalog image or directory to a new file-based catalog,
optionally applying migrations.

The input catalogRef may be a container image reference or a local directory
containing a file-based catalog.

NOTE: the --output=json format produces streamable, concatenated JSON files.
These are suitable to opm and jq, but may not be supported by arbitrary JSON
parsers that assume that a file contains exactly one valid JSON object.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			migrate.CatalogRef = args[0]
			migrate.OutputDir = args[1]

			switch output {
			case "yaml":
				migrate.WriteFunc = declcfg.WriteYAML
				migrate.FileExt = ".yaml"
			case "json":
				migrate.WriteFunc = declcfg.WriteJSON
				migrate.FileExt = ".json"
			default:
				log.Fatalf("invalid --output value %q, expected (json|yaml)", output)
			}

			if migrateLevel != "" {
				m, err := migrations.NewMigrations(migrateLevel)
				if err != nil {
					log.Fatal(err)
				}
				migrate.Migrations = m
			}

			logrus.Infof("rendering catalog %q as file-based catalog", migrate.CatalogRef)
			if err := migrate.Run(cmd.Context()); err != nil {
				logrus.New().Fatal(err)
			}
			logrus.Infof("wrote rendered file-based catalog to %q\n", migrate.OutputDir)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format (json|yaml)")
	cmd.Flags().StringVar(&migrateLevel, "migrate-level", "", "Name of the last migration to run (default: none)\n"+migrations.HelpText())

	return cmd
}
