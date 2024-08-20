package template

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/action/migrations"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/template/semver"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
)

func newSemverTemplateCmd() *cobra.Command {
	var (
		migrateLevel string
	)

	cmd := &cobra.Command{
		Use: "semver [FILE]",
		Short: `Generate a file-based catalog from a single 'semver template' file
When FILE is '-' or not provided, the template is read from standard input`,
		Long: `Generate a file-based catalog from a single 'semver template' file
When FILE is '-' or not provided, the template is read from standard input`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle different input argument types
			// When no arguments or "-" is passed to the command,
			// assume input is coming from stdin
			// Otherwise open the file passed to the command
			data, source, err := util.OpenFileOrStdin(cmd, args)
			if err != nil {
				return err
			}
			defer data.Close()

			var write func(declcfg.DeclarativeConfig, io.Writer) error
			output, err := cmd.Flags().GetString("output")
			if err != nil {
				log.Fatalf("unable to determine output format")
			}
			switch output {
			case "json":
				write = declcfg.WriteJSON
			case "yaml":
				write = declcfg.WriteYAML
			case "mermaid":
				write = func(cfg declcfg.DeclarativeConfig, writer io.Writer) error {
					mermaidWriter := declcfg.NewMermaidWriter()
					return mermaidWriter.WriteChannels(cfg, writer)
				}
			default:
				return fmt.Errorf("invalid output format %q", output)
			}

			// The bundle loading impl is somewhat verbose, even on the happy path,
			// so discard all logrus default logger logs. Any important failures will be
			// returned from template.Render and logged as fatal errors.
			logrus.SetOutput(io.Discard)

			reg, err := util.CreateCLIRegistry(cmd)
			if err != nil {
				log.Fatalf("creating containerd registry: %v", err)
			}
			defer reg.Destroy()

			var m *migrations.Migrations
			if migrateLevel != "" {
				m, err = migrations.NewMigrations(migrateLevel)
				if err != nil {
					log.Fatal(err)
				}
			}

			template := semver.Template{
				Data: data,
				RenderBundle: func(ctx context.Context, ref string) (*declcfg.DeclarativeConfig, error) {
					renderer := action.Render{
						Refs:           []string{ref},
						Registry:       reg,
						AllowedRefMask: action.RefBundleImage,
						Migrations:     m,
					}
					return renderer.Run(ctx)
				},
			}

			out, err := template.Render(cmd.Context())
			if err != nil {
				log.Fatalf("semver %q: %v", source, err)
			}

			if out != nil {
				if err := write(*out, os.Stdout); err != nil {
					log.Fatal(err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&migrateLevel, "migrate-level", "", "Name of the last migration to run (default: none)\n"+migrations.HelpText())

	return cmd
}
