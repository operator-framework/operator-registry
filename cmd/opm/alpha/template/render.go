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
	"github.com/operator-framework/operator-registry/alpha/template"
	"github.com/operator-framework/operator-registry/alpha/template/basic"
	"github.com/operator-framework/operator-registry/alpha/template/semver"
	"github.com/operator-framework/operator-registry/alpha/template/substitutes"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
)

// runRenderTemplate handles the unified template rendering logic
func runRenderTemplate(cmd *cobra.Command, args []string) error {
	var templateType, filePath string

	// Parse arguments based on number provided
	switch len(args) {
	case 0:
		// No arguments - read from stdin, auto-detect type
		filePath = "-"
	case 1:
		// One argument - could be type or file
		if isValidTemplateType(args[0]) {
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
		if !isValidTemplateType(templateType) {
			return fmt.Errorf("invalid template type %q, must be one of: basic, semver, substitutes", templateType)
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
	// so discard all logrus default logger logs. Any important failures will be
	// returned from template.Render and logged as fatal errors.
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
	renderBundle := template.BundleRenderer(func(ctx context.Context, image string) (*declcfg.DeclarativeConfig, error) {
		renderer := action.Render{
			Refs:           []string{image},
			Registry:       reg,
			AllowedRefMask: action.RefBundleImage,
			Migrations:     m,
		}
		return renderer.Run(ctx)
	})

	// Create template registry and register factories
	registry := template.NewRegistry()
	registry.Register(&basic.Factory{})
	registry.Register(&semver.Factory{})
	registry.Register(&substitutes.Factory{})

	var tmpl template.Template

	// nolint:nestif
	if templateType != "" {
		// Use specified template type
		tmpl, err = createTemplateByType(templateType, renderBundle)
		if err != nil {
			return err
		}
	} else {
		// Auto-detect template type from schema
		// We need to re-open the file since schema detection consumes the reader
		data.Close()
		data, source, err = util.OpenFileOrStdin(cmd, []string{filePath})
		if err != nil {
			return fmt.Errorf("unable to reopen %q: %v", source, err)
		}
		defer data.Close()

		tmpl, err = registry.CreateTemplate(data, renderBundle)
		if err != nil {
			return fmt.Errorf("auto-detecting template type: %v", err)
		}

		// Re-open again for rendering
		data.Close()
		data, source, err = util.OpenFileOrStdin(cmd, []string{filePath})
		if err != nil {
			return fmt.Errorf("unable to reopen %q for rendering: %v", source, err)
		}
		defer data.Close()
	}

	// Render the template
	cfg, err := tmpl.Render(cmd.Context(), data)
	if err != nil {
		return fmt.Errorf("rendering template: %v", err)
	}

	// Write output
	if err := write(*cfg, os.Stdout); err != nil {
		return fmt.Errorf("writing output: %v", err)
	}

	return nil
}

// isValidTemplateType checks if the provided string is a valid template type
func isValidTemplateType(templateType string) bool {
	switch templateType {
	case "basic", "semver", "substitutes":
		return true
	default:
		return false
	}
}

// createTemplateByType creates a template instance of the specified type
func createTemplateByType(templateType string, renderBundle template.BundleRenderer) (template.Template, error) {
	switch templateType {
	case "basic":
		return basic.NewTemplate(renderBundle), nil
	case "semver":
		return semver.NewTemplate(renderBundle), nil
	case "substitutes":
		return substitutes.NewTemplate(renderBundle), nil
	default:
		return nil, fmt.Errorf("unknown template type: %s", templateType)
	}
}
