package main

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/cmd/opm/alpha"
	"github.com/operator-framework/operator-registry/cmd/opm/index"
	"github.com/operator-framework/operator-registry/cmd/opm/registry"
	"github.com/operator-framework/operator-registry/cmd/opm/version"
	registrylib "github.com/operator-framework/operator-registry/pkg/registry"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "opm",
		Short: "operator package manager",
		Long:  "CLI to interact with operator-registry and build indexes of operator content",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if debug, _ := cmd.Flags().GetBool("debug"); debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil
		},
	}

	rootCmd.AddCommand(registry.NewOpmRegistryCmd(), alpha.NewCmd())
	index.AddCommand(rootCmd)
	version.AddCommand(rootCmd)

	rootCmd.Flags().Bool("debug", false, "enable debug logging")
	if err := rootCmd.Flags().MarkHidden("debug"); err != nil {
		logrus.Panic(err.Error())
	}

	if err := rootCmd.Execute(); err != nil {
		if agg, ok := err.(utilerrors.Aggregate); ok {
			for _, e := range agg.Errors() {
				if _, ok := e.(registrylib.BundleAlreadyAddedError); ok {
					os.Exit(2)
				}
				if _, ok := e.(registrylib.BundleAlreadyInDatabaseError); ok {
					os.Exit(3)
				}
			}
		}
		os.Exit(1)
	}
}
