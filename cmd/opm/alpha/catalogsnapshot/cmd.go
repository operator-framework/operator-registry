package catalogsnapshot

import (
	"context"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/internal/action"
	"github.com/operator-framework/operator-registry/internal/declcfg"
)

func NewCmd() *cobra.Command {
	var (
		snapshot action.CatalogSnapshot

		output  string
		timeout time.Duration
	)
	cmd := &cobra.Command{
		Use:   "catalog-snapshot [args]",
		Short: "Generate a snapshot of all installed operators in the cluster as a declarative config",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			var write func(declcfg.DeclarativeConfig, io.Writer) error
			switch output {
			case "yaml":
				write = declcfg.WriteYAML
			case "json":
				write = declcfg.WriteJSON
			default:
				log.Fatalf("invalid --output value %q, expected (json|yaml)", output)
			}

			// The bundle loading impl is somewhat verbose, even on the happy path,
			// so discard all logrus default logger logs. Any important failures will be
			// returned from snapshot.Run and logged as fatal errors.
			logrus.SetOutput(ioutil.Discard)

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			cfg, err := snapshot.Run(ctx)
			if err != nil {
				log.Fatal(err)
			}

			if err := write(*cfg, os.Stdout); err != nil {
				log.Fatal(err)
			}
		},
	}

	cmd.Flags().StringVarP(&snapshot.KubeconfigPath, "output", "o", "json", "Output format (json|yaml)")
	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "Command timeout")
	return cmd
}
