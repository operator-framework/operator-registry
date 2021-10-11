package migrate

import (
	"log"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

func NewCmd() *cobra.Command {
	var (
		migrate    action.Migrate
		output     string
		singleFile bool
	)
	cmd := &cobra.Command{
		Use:   "migrate <indexRef> <outputDir>",
		Short: "Migrate a sqlite-based index image or database file to a file-based catalog",
		Long: `Migrate a sqlite-based index image or database file to a file-based catalog.

` + sqlite.DeprecationMessage,
		Args: cobra.ExactArgs(2),
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			sqlite.LogSqliteDeprecation()
		},
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
			migrate.SingleFile = singleFile
			logrus.Infof("rendering index %q as file-based catalog", migrate.CatalogRef)
			if err := migrate.Run(cmd.Context()); err != nil {
				logrus.New().Fatal(err)
			}
			logrus.Infof("wrote rendered file-based catalog to %q\n", migrate.OutputDir)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format (json|yaml)")
	cmd.Flags().BoolVarP(&singleFile, "singleFile", "s", false, "Output is written to a single file, rather than multiple files split into bundles/channels")
	return cmd
}
