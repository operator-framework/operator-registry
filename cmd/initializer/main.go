package main

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

var rootCmd = &cobra.Command{
	Short: "initializer",
	Long:  `initializer takes a directory of OLM manifests and outputs a sqlite database containing them`,

	PreRunE: func(cmd *cobra.Command, args []string) error {
		if debug, _ := cmd.Flags().GetBool("debug"); debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	},

	RunE: runCmdFunc,
}

func init() {
	rootCmd.Flags().Bool("debug", false, "enable debug logging")
	rootCmd.Flags().StringP("manifests", "m", "manifests", "relative path to directory of manifests")
	rootCmd.Flags().StringP("output", "o", "bundles.db", "relative path to a sqlite file to create or overwrite")
	rootCmd.Flags().Bool("permissive", false, "allow registry load errors")
	if err := rootCmd.Flags().MarkHidden("debug"); err != nil {
		panic(err)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

func runCmdFunc(cmd *cobra.Command, args []string) error {
	outFilename, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}
	manifestDir, err := cmd.Flags().GetString("manifests")
	if err != nil {
		return err
	}
	permissive, err := cmd.Flags().GetBool("permissive")
	if err != nil {
		return err
	}

	db, err := sqlite.Open(outFilename)
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

	loader := sqlite.NewSQLLoaderForDirectory(dbLoader, manifestDir)
	if err := loader.Populate(); err != nil {
		err = fmt.Errorf("error loading manifests from directory: %s", err)
		if !permissive {
			logrus.WithError(err).Fatal("permissive mode disabled")
			return err
		}
		logrus.WithError(err).Warn("permissive mode enabled")
	}

	return nil
}
