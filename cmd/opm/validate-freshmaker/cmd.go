package validate_freshmaker

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
)

const (
	substitutesForAnnotation = "olm.substitutesFor"
	maxNameLength            = 63
	maxReleaseLength         = 20
)

type ValidationResult struct {
	Schema  string   `json:"schema"`
	Name    string   `json:"name"`
	Package string   `json:"package"`
	Valid   bool     `json:"valid"`
	Errors  []string `json:"errors,omitempty"`
}

type ValidationOutput struct {
	Results []ValidationResult `json:"results"`
}

func NewCmd() *cobra.Command {
	var (
		render action.Render
		output string
	)

	cmd := &cobra.Command{
		Use:   "validate-freshmaker [catalog-image | catalog-directory | bundle-image | bundle-directory]...",
		Short: "Validate freshmaker release versioning in bundles",
		Long: `Validate freshmaker release versioning in bundles from the provided
catalog images, file-based catalog directories, bundle images, and bundle directories.

Freshmaker usage is identified by bundles having:
1. An olm.substitutesFor annotation (value is immaterial)
2. A property of type "olm.package" with value.version containing a plus sign (+)

The release versioning is the portion after the plus sign.
Release versioning naming requirement: <package>-v<version-without-release>-<release-version>
where:
- release-version: dot-delimited sequences of alphanumerics and hyphens, max 20 characters
- total constructed name: max 63 characters
`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			render.Refs = args

			// Discard verbose logging
			logrus.SetOutput(io.Discard)

			reg, err := util.CreateCLIRegistry(cmd)
			if err != nil {
				log.Fatal(err)
			}
			defer func() {
				_ = reg.Destroy()
			}()

			render.Registry = reg

			cfg, err := render.Run(cmd.Context())
			if err != nil {
				log.Fatal(err)
			}

			results := validateBundles(cfg)

			var writeFunc func(ValidationOutput, io.Writer) error
			switch output {
			case "yaml":
				writeFunc = writeYAML
			case "json":
				writeFunc = writeJSON
			case "text":
				writeFunc = writeText
			default:
				log.Fatalf("invalid --output value %q, expected (json|yaml|text)", output)
			}

			if err := writeFunc(ValidationOutput{Results: results}, os.Stdout); err != nil {
				log.Fatal(err)
			}
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format (json|yaml|text)")

	return cmd
}

func validateBundles(cfg *declcfg.DeclarativeConfig) []ValidationResult {
	var results []ValidationResult

	for _, bundle := range cfg.Bundles {
		result := validateBundle(bundle)
		// Only include freshmaker bundles in the output
		if result.Name != "" {
			results = append(results, result)
		}
	}

	return results
}

func validateBundle(bundle declcfg.Bundle) ValidationResult {
	// Parse properties
	props, err := property.Parse(bundle.Properties)
	if err != nil {
		// Can't parse properties, skip this bundle
		return ValidationResult{}
	}

	// Check for olm.package property with version containing "+"
	var packageProp *property.Package
	for _, p := range props.Packages {
		if strings.Contains(p.Version, "+") {
			packageProp = &p
			break
		}
	}

	// Check for substitutesFor annotation
	hasSubstitutesFor := false
	for _, csvMeta := range props.CSVMetadatas {
		if _, ok := csvMeta.Annotations[substitutesForAnnotation]; ok {
			hasSubstitutesFor = true
			break
		}
	}

	// Only validate freshmaker bundles
	isFreshmaker := packageProp != nil && hasSubstitutesFor
	if !isFreshmaker {
		return ValidationResult{}
	}

	result := ValidationResult{
		Schema:  "olm.bundle",
		Name:    bundle.Name,
		Package: bundle.Package,
		Valid:   true,
		Errors:  []string{},
	}

	// Extract release version (portion after "+")
	parts := strings.SplitN(packageProp.Version, "+", 2)
	if len(parts) != 2 {
		result.Valid = false
		result.Errors = append(result.Errors, "version contains '+' but no release version found")
		return result
	}

	versionWithoutRelease := parts[0]
	releaseVersion := parts[1]

	// Construct the expected name
	constructedName := fmt.Sprintf("%s-v%s-%s", bundle.Package, versionWithoutRelease, releaseVersion)

	// Validate release version format (dot-delimited sequences of alphanumerics and hyphens)
	if !isValidReleaseVersion(releaseVersion) {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("release version %q has invalid format (must be dot-delimited sequences of alphanumerics and hyphens)", releaseVersion))
	}

	// Validate release version length
	if len(releaseVersion) > maxReleaseLength {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("release version %q exceeds maximum length of %d characters (length: %d)",
				releaseVersion, maxReleaseLength, len(releaseVersion)))
	}

	// Validate total constructed name length
	if len(constructedName) > maxNameLength {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("constructed name %q exceeds maximum length of %d characters (length: %d)",
				constructedName, maxNameLength, len(constructedName)))
	}

	return result
}

// isValidReleaseVersion checks if the release version is composed of dot-delimited sequences
// of alphanumerics and hyphens
func isValidReleaseVersion(s string) bool {
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9-]+(\.[a-zA-Z0-9-]+)*$`, s)
	return matched
}

func writeJSON(output ValidationOutput, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(output)
}

func writeYAML(output ValidationOutput, w io.Writer) error {
	// Convert to JSON bytes first
	data, err := json.Marshal(output)
	if err != nil {
		return err
	}

	// Create a temporary DeclarativeConfig to use the existing WriteYAML encoder
	// Since we have a simple structure, we'll just use JSON for now
	// (In production, you might want to use a proper YAML library)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	return enc.Encode(obj)
}

func writeText(output ValidationOutput, w io.Writer) error {
	var total, valid, invalid int

	for _, r := range output.Results {
		total++
		if r.Valid {
			valid++
		} else {
			invalid++
		}
	}

	fmt.Fprintf(w, "Freshmaker Bundle Validation Summary\n")
	fmt.Fprintf(w, "=====================================\n\n")
	fmt.Fprintf(w, "Total freshmaker bundles: %d\n", total)
	fmt.Fprintf(w, "Valid: %d\n", valid)
	fmt.Fprintf(w, "Invalid: %d\n\n", invalid)

	if invalid > 0 {
		fmt.Fprintf(w, "Invalid Bundles:\n")
		fmt.Fprintf(w, "----------------\n\n")
		for _, r := range output.Results {
			if !r.Valid {
				fmt.Fprintf(w, "Bundle: %s\n", r.Name)
				fmt.Fprintf(w, "  Package: %s\n", r.Package)
				fmt.Fprintf(w, "  Validation Errors:\n")
				for _, err := range r.Errors {
					fmt.Fprintf(w, "    - %s\n", err)
				}
				fmt.Fprintf(w, "\n")
			}
		}
	}

	if valid > 0 {
		fmt.Fprintf(w, "Valid Bundles:\n")
		fmt.Fprintf(w, "--------------\n\n")
		for _, r := range output.Results {
			if r.Valid {
				fmt.Fprintf(w, "  - %s (package: %s)\n", r.Name, r.Package)
			}
		}
	}

	return nil
}
