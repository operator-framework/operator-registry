package template

import (
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/action/migrations"
)

func NewCmd() *cobra.Command {
	var output, migrateLevel string

	runCmd := &cobra.Command{
		Use:   "render-template [TYPE] [FILE]",
		Short: "Render a catalog template (auto-detects type from schema if TYPE not specified)",
		Long: `Render a catalog template with optional type specification.

If TYPE is specified, it must be one of: basic, semver, substitutes
If TYPE is not specified, the template type will be auto-detected from the schema field in the input file.

When FILE is '-' or not provided, the template is read from standard input.

Examples:
  opm alpha render-template basic template.yaml
  opm alpha render-template semver template.yaml  
  opm alpha render-template substitutes template.yaml
  opm alpha render-template template.yaml  # auto-detect type
  opm alpha render-template < template.yaml  # auto-detect from stdin`,
		Args: cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRenderTemplate(cmd, args)
		},
	}

	runCmd.PersistentFlags().StringVarP(&output, "output", "o", "json", "Output format (json|yaml|mermaid)")
	runCmd.PersistentFlags().StringVar(&migrateLevel, "migrate-level", "", "Name of the last migration to run (default: none)\n"+migrations.HelpText())

	return runCmd
}
