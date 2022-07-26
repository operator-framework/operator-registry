package veneer

import (
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "render-veneer",
		Short: "Render a veneer type",
		Args:  cobra.NoArgs,
	}

	runCmd.AddCommand(newBasicVeneerRenderCmd())
	runCmd.AddCommand(newSemverCmd())

	return runCmd
}
