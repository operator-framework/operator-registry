package validate

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/operator-framework/operator-registry/pkg/lib/config"
)

const (
	outputJSON = "json"
	outputYAML = "yaml"
)

// ValidationResult represents the structured output of validation
type ValidationResult struct {
	Passed bool             `json:"passed" yaml:"passed"`
	Error  *ValidationError `json:"error,omitempty" yaml:"error,omitempty"`
}

// ValidationError represents a structured validation error that can be nested
type ValidationError struct {
	Message string            `json:"message" yaml:"message"`
	Errors  []ValidationError `json:"errors,omitempty" yaml:"errors,omitempty"`
}

// errorToValidationError converts a Go error into a structured ValidationError
func errorToValidationError(err error) *ValidationError {
	if err == nil {
		return nil
	}

	// For now, handle the error as a simple string message
	// The error tree structure is already formatted in the Error() output
	return &ValidationError{
		Message: err.Error(),
	}
}

func NewCmd() *cobra.Command {
	logger := logrus.New()
	var output string

	validate := &cobra.Command{
		Use:   "validate <directory>",
		Short: "Validate the declarative index config",
		Long:  "Validate the declarative config JSON file(s) in a given directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			directory := args[0]
			s, err := os.Stat(directory)
			if err != nil {
				return err
			}
			if !s.IsDir() {
				return fmt.Errorf("%q is not a directory", directory)
			}

			// Perform validation
			validationErr := config.Validate(c.Context(), os.DirFS(directory))

			// Handle structured output
			if output != "" {
				result := ValidationResult{
					Passed: validationErr == nil,
					Error:  errorToValidationError(validationErr),
				}

				var data []byte
				var marshalErr error

				switch output {
				case outputJSON:
					data, marshalErr = json.MarshalIndent(result, "", "  ")
				case outputYAML:
					data, marshalErr = yaml.Marshal(result)
				default:
					return fmt.Errorf("invalid --output value %q, expected (json|yaml)", output)
				}

				if marshalErr != nil {
					return fmt.Errorf("failed to marshal output: %w", marshalErr)
				}

				fmt.Fprintln(os.Stdout, string(data))

				// Exit directly with appropriate code to avoid cobra printing error
				if validationErr != nil {
					os.Exit(1)
				}
				return nil
			}

			// Default behavior: use logger.Fatal on error
			if validationErr != nil {
				logger.Fatal(validationErr)
			}
			return nil
		},
	}

	validate.Flags().StringVarP(&output, "output", "o", "", "Output format for validation results (json|yaml)")

	return validate
}
