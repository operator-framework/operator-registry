package alpha

import (
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/cmd/opm/alpha/bundle"
	"github.com/operator-framework/operator-registry/cmd/opm/alpha/list"
	rendergraph "github.com/operator-framework/operator-registry/cmd/opm/alpha/render-graph"
	renderveneer "github.com/operator-framework/operator-registry/cmd/opm/alpha/render-veneer"
)

func NewCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Hidden: true,
		Use:    "alpha",
		Short:  "Run an alpha subcommand",
		Args:   cobra.NoArgs,
	}

	runCmd.AddCommand(
		bundle.NewCmd(),
		list.NewCmd(),
		rendergraph.NewCmd(),
		renderveneer.NewCmd(),
	)
	return runCmd
}
