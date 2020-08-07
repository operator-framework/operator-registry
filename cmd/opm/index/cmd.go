package index

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// AddCommand adds the index subcommand to the given parent command.
func AddCommand(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "generate operator index container images",
		Long:  `generate operator index container images from preexisting operator bundles`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			if debug, _ := cmd.Flags().GetBool("debug"); debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil
		},
	}

	parent.AddCommand(cmd)
	cmd.AddCommand(newIndexDeleteCmd())
	addIndexAddCmd(cmd)
	cmd.AddCommand(newIndexExportCmd())
	cmd.AddCommand(newIndexPruneCmd())
	cmd.AddCommand(newIndexDeprecateTruncateCmd())
	cmd.AddCommand(newIndexPruneStrandedCmd())
}
