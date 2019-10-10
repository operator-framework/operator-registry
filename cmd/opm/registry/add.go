package registry

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

func newRegistryAddCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "add",
		Short: "add operator bundle to operator registry DB",
		Long:  `add operator bundle to operator registry DB`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			if debug, _ := cmd.Flags().GetBool("debug"); debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil
		},

		RunE: addFunc,
	}

	rootCmd.Flags().Bool("debug", false, "enable debug logging")
	rootCmd.Flags().StringP("database", "d", "bundles.db", "relative path to database file")
	rootCmd.Flags().StringSliceP("bundle-images", "b", []string{}, "comma separated list of links to bundle image")
	rootCmd.Flags().Bool("permissive", false, "allow registry load errors")

	return rootCmd
}

func addFunc(cmd *cobra.Command, args []string) error {
	bundleImages, err := cmd.Flags().GetStringSlice("bundle-images")
	if err != nil {
		return err
	}

	fromFilename, err := cmd.Flags().GetString("database")
	if err != nil {
		return err
	}
	permissive, err := cmd.Flags().GetBool("permissive")
	if err != nil {
		return err
	}

	var errs []error

	db, err := sql.Open("sqlite3", fromFilename)
	if err != nil {
		return err
	}
	defer db.Close()

	dbLoader, err := sqlite.NewSQLLiteLoader(db)
	if err != nil {
		return err
	}
	if err := dbLoader.Migrate(context.TODO()); err != nil {
		return err
	}

	for _, bundleImage := range bundleImages {
		loader := sqlite.NewSQLLoaderForImage(dbLoader, bundleImage)
		if err := loader.Populate(); err != nil {
			err = fmt.Errorf("error loading bundle from image: %s", err)
			if !permissive {
				logrus.WithError(err).Fatal("permissive mode disabled")
				errs = append(errs, err)
			}
			logrus.WithError(err).Warn("permissive mode enabled")
		}
	}
	return nil
}
