package alpha

import (
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/cmd/opm/alpha/bundle"
	"github.com/operator-framework/operator-registry/cmd/opm/alpha/generate"
	"github.com/operator-framework/operator-registry/cmd/opm/alpha/list"
)

func NewCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Hidden: true,
		Use:    "alpha",
		Short:  "Run an alpha subcommand",
	}

	runCmd.AddCommand(
		bundle.NewCmd(),
		generate.NewCmd(),
		list.NewCmd(),
	)
	return runCmd
}
