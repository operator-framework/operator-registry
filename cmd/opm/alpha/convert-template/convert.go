package converttemplate

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/template/converter"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert-template",
		Short: "Convert existing FBC to a supported template type",
	}
	cmd.AddCommand(
		newBasicConvertCmd(),
	)
	return cmd
}

func newBasicConvertCmd() *cobra.Command {
	var (
		converter converter.Converter
		output    string
	)
	cmd := &cobra.Command{
		Use:   "basic [<fbc-file> | -]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Generate a basic template from existing FBC",
		Long: `Generate a basic template from existing FBC.

This command outputs a basic catalog template to STDOUT from input FBC.
If no argument is specified or is '-' input is assumed from STDIN.
`,
		RunE: func(c *cobra.Command, args []string) error {
			switch output {
			case "yaml", "json":
				converter.OutputFormat = output
			default:
				log.Fatalf("invalid --output value %q, expected (json|yaml)", output)
			}

			reader, name, err := util.OpenFileOrStdin(c, args)
			if err != nil {
				return fmt.Errorf("unable to open input: %q", name)
			}

			converter.FbcReader = reader
			err = converter.Convert()
			if err != nil {
				return fmt.Errorf("converting: %v", err)
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format (json|yaml)")

	return cmd
}
