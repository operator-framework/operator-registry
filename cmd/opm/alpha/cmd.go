package alpha

import (
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/cmd/opm/alpha/bundle"
	initcmd "github.com/operator-framework/operator-registry/cmd/opm/alpha/init"
	"github.com/operator-framework/operator-registry/cmd/opm/alpha/render"
	"github.com/operator-framework/operator-registry/cmd/opm/alpha/serve"
	"github.com/operator-framework/operator-registry/cmd/opm/alpha/validate"
)

func NewCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Hidden: true,
		Use:    "alpha",
		Short:  "Run an alpha subcommand",
	}

	runCmd.AddCommand(bundle.NewCmd(), initcmd.NewCmd(), serve.NewCmd(), render.NewCmd(), validate.NewCmd())
	return runCmd
}
