package template

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/action/migrations"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	alphatemplate "github.com/operator-framework/operator-registry/alpha/template"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
)

// runRenderTemplate handles the unified template rendering logic
func runRenderTemplate(cmd *cobra.Command, args []string, tr alphatemplate.Registry) error {
	var templateType, filePath string

	// Parse arguments based on number provided
	switch len(args) {
	case 0:
		// No arguments - read from stdin, auto-detect type
		filePath = "-"
	case 1:
		// One argument - could be type or file
		if tr.HasType(args[0]) {
			// It's a template type, read from stdin
			templateType = args[0]
			filePath = "-"
		} else {
			// It's a file path, auto-detect type
			filePath = args[0]
		}
	case 2:
		// Two arguments - type and file
		templateType = args[0]
		filePath = args[1]
		if !tr.HasType(templateType) {
			return fmt.Errorf("invalid template type %q, must be one of: %s", templateType, tr.GetSupportedTypes())
		}
	}

	// Handle different input argument types
	data, source, err := util.OpenFileOrStdin(cmd, []string{filePath})
	if err != nil {
		return fmt.Errorf("unable to open %q: %v", source, err)
	}
	defer data.Close()

	// Determine output format
	var write func(declcfg.DeclarativeConfig, io.Writer) error
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return fmt.Errorf("unable to determine output format")
	}
	switch output {
	case "yaml":
		write = declcfg.WriteYAML
	case "json":
		write = declcfg.WriteJSON
	case "mermaid":
		write = func(cfg declcfg.DeclarativeConfig, writer io.Writer) error {
			mermaidWriter := declcfg.NewMermaidWriter()
			return mermaidWriter.WriteChannels(cfg, writer)
		}
	default:
		return fmt.Errorf("invalid --output value %q, expected (json|yaml|mermaid)", output)
	}

	// The bundle loading impl is somewhat verbose, even on the happy path,
	// so discard all logrus default logger logs.
	logrus.SetOutput(io.Discard)

	// Create registry and registry client
	reg, err := util.CreateCLIRegistry(cmd)
	if err != nil {
		return fmt.Errorf("creating containerd registry: %v", err)
	}
	defer func() {
		_ = reg.Destroy()
	}()

	// Handle migrations
	var m *migrations.Migrations
	migrateLevel, err := cmd.Flags().GetString("migrate-level")
	if err == nil && migrateLevel != "" {
		m, err = migrations.NewMigrations(migrateLevel)
		if err != nil {
			return err
		}
	}

	// Create render bundle function
	renderBundle := alphatemplate.BundleRenderer(func(ctx context.Context, image string) (*declcfg.DeclarativeConfig, error) {
		renderer := action.Render{
			Refs:           []string{image},
			Registry:       reg,
			AllowedRefMask: action.RefBundleImage,
			Migrations:     m,
		}
		return renderer.Run(ctx)
	})

	var tmpl alphatemplate.Template
	// a reader for the schema data.  in the simple case, this is just 'data'.
	// in the case where we auto-detect the schema, this is a reader that
	// includes the consumed schema data plus any remainder.
	var renderReader io.Reader

	if templateType != "" {
		// Use specified template type
		tmpl, err = tr.CreateTemplateByType(templateType, renderBundle)
		if err != nil {
			return fmt.Errorf("creating template by type: %v", err)
		}
		renderReader = data
	} else {
		// Auto-detect template type from schema
		tmpl, renderReader, err = tr.CreateTemplateBySchema(data, renderBundle)
		if err != nil {
			return fmt.Errorf("auto-detecting template type: %v", err)
		}
	}

	// Render the template
	cfg, err := tmpl.Render(cmd.Context(), renderReader)
	if err != nil {
		return fmt.Errorf("rendering template: %v", err)
	}

	// Write output
	if err := write(*cfg, os.Stdout); err != nil {
		return fmt.Errorf("writing output: %v", err)
	}

	return nil
}
