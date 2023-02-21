package alpha

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/cmd/opm/alpha/bundle"
	"github.com/operator-framework/operator-registry/cmd/opm/alpha/list"
	rendergraph "github.com/operator-framework/operator-registry/cmd/opm/alpha/render-graph"
	"github.com/operator-framework/operator-registry/cmd/opm/alpha/template"
)

func NewCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Hidden: true,
		Use:    "alpha",
		Short:  "Run an alpha subcommand",
		Args:   cobra.NoArgs,
	}

	runCmd.SetUsageFunc(func(c *cobra.Command) error {
		fmt.Printf("usage\n")
		fmt.Fprintf(c.OutOrStderr(), c.UsageTemplate(), c)
		os.Exit(3)
		return nil
	})

	runCmd.SetFlagErrorFunc(func(c *cobra.Command, e error) error {
		if e == nil {
			return nil
		}
		fmt.Printf("error\n")
		return fmt.Errorf("flag error func")
	})

	runCmd.AddCommand(
		bundle.NewCmd(),
		list.NewCmd(),
		rendergraph.NewCmd(),
		template.NewCmd(),
	)
	return runCmd
}
