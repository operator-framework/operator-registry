package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

func main() {
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

	rootCmd.Flags().Bool("debug", false, "enable debug logging")
	rootCmd.Flags().StringP("manifests", "m", "manifests", "relative path to directory of manifests")
	rootCmd.Flags().StringP("output", "o", "bundles.db", "relative path to a sqlite file to create or overwrite")
	rootCmd.Flags().Bool("permissive", false, "permit failures during registry build time")

	if err := rootCmd.Flags().MarkHidden("debug"); err != nil {
		panic(err)
	}

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

	dbLoader := sqlite.NewErrorSupressingSQLLoader(outFilename)
	defer dbLoader.Close()

	loader := sqlite.NewSQLLoaderForDirectory(dbLoader, manifestDir)
	if err := loader.Populate(); err != nil {
		logrus.Fatal(err)
	}

	if err = registry.NewAggregate(dbLoader); err != nil {
		// TODO: Pass LoadErrors/Database to validator(s) to determine what is permitted vs. not
		if !permissive {
			logrus.Fatalf("failing on load errors:\n%v", err)
		}

		logrus.Warnf("permissive mode enabled, permitting load errors:\n%v", err)
	}

	return nil
}
