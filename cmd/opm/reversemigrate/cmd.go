package reversemigrate

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

func NewCmd() *cobra.Command {
	var (
		reverseMigrate action.ReverseMigrate
	)
	cmd := &cobra.Command{
		Use:   "reverse-migrate <indexRef> <outputDir>",
		Short: "Migrate a file-based catalog to an sqlite-based catalog",
		Long: `Migrate a file-based catalog to an sqlite-based catalog.

` + sqlite.DeprecationMessage,
		Args: cobra.ExactArgs(2),
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			sqlite.LogSqliteDeprecation()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			reverseMigrate.CatalogRef = args[0]
			reverseMigrate.OutputFile = args[1]

			logrus.Infof("rendering index %q as sqlite-based catalog", reverseMigrate.CatalogRef)
			if err := reverseMigrate.Run(cmd.Context()); err != nil {
				logrus.New().Fatal(err)
			}
			logrus.Infof("wrote sqlite-based catalog to %q\n", reverseMigrate.OutputFile)
			return nil
		},
	}
	return cmd
}
