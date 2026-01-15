package substitutes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

// Helper function to create a mock template for testing
func createMockTemplate() template {
	return template{
		renderBundle: func(ctx context.Context, imageRef string) (*declcfg.DeclarativeConfig, error) {
			// Extract package name from image reference
			packageName := "testoperator"
			if strings.Contains(imageRef, "test-bundle") {
				packageName = "test"
			}

			// Extract version from image reference if it contains a version tag
			version := "1.0.0"
			release := ""
			if strings.Contains(imageRef, ":v") {
				parts := strings.Split(imageRef, ":v")
				if len(parts) == 2 {
					versionTag := parts[1]
					// Check if version has a prerelease/build metadata (e.g., "1.2.0-alpha")
					if strings.Contains(versionTag, "-") {
						versionParts := strings.SplitN(versionTag, "-", 2)
						version = versionParts[0]
						release = versionParts[1]
					} else {
						version = versionTag
					}
				}
			}

			// Create bundle name based on whether it has a release version
			var bundleName string
			var properties []property.Property

			if release != "" {
				// substitutesFor bundle: package-vversion-release
				bundleName = fmt.Sprintf("%s-v%s-%s", packageName, version, release)
				properties = []property.Property{
					property.MustBuildPackageRelease(packageName, version, release),
					property.MustBuildBundleObject([]byte(fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, imageRef))),
					property.MustBuildBundleObject([]byte(`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`)),
				}
			} else {
				// normal bundle: package.vversion
				bundleName = fmt.Sprintf("%s.v%s", packageName, version)
				properties = []property.Property{
					property.MustBuildPackage(packageName, version),
					property.MustBuildBundleObject([]byte(fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, imageRef))),
					property.MustBuildBundleObject([]byte(`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`)),
				}
			}

			return &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema:         "olm.package",
						Name:           packageName,
						DefaultChannel: "stable",
					},
				},
				Bundles: []declcfg.Bundle{
					{
						Schema:     "olm.bundle",
						Name:       bundleName,
						Package:    packageName,
						Image:      imageRef,
						Properties: properties,
						RelatedImages: []declcfg.RelatedImage{
							{Name: "bundle", Image: imageRef},
						},
						CsvJSON: fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, imageRef),
						Objects: []string{
							fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, imageRef),
							`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`,
						},
					},
				},
			}, nil
		},
	}
}

// Helper function to create a test DeclarativeConfig
func createTestDeclarativeConfig() *declcfg.DeclarativeConfig {
	return &declcfg.DeclarativeConfig{
		Packages: []declcfg.Package{
			{
				Schema:         "olm.package",
				Name:           "testoperator",
				DefaultChannel: "stable",
			},
		},
		Channels: []declcfg.Channel{
			{
				Schema:  "olm.channel",
				Name:    "stable",
				Package: "testoperator",
				Entries: []declcfg.ChannelEntry{
					{Name: "testoperator.v1.0.0"},
					{Name: "testoperator.v1.1.0", Replaces: "testoperator.v1.0.0"},
					{Name: "testoperator.v1.2.0", Replaces: "testoperator.v1.1.0", Skips: []string{"testoperator.v1.0.0"}},
				},
			},
		},
		Bundles: []declcfg.Bundle{
			{
				Schema:  "olm.bundle",
				Name:    "testoperator.v1.0.0",
				Package: "testoperator",
				Image:   "quay.io/test/testoperator-bundle:v1.0.0",
				Properties: []property.Property{
					property.MustBuildPackage("testoperator", "1.0.0"),
				},
			},
			{
				Schema:  "olm.bundle",
				Name:    "testoperator.v1.1.0",
				Package: "testoperator",
				Image:   "quay.io/test/testoperator-bundle:v1.1.0",
				Properties: []property.Property{
					property.MustBuildPackage("testoperator", "1.1.0"),
				},
			},
			{
				Schema:  "olm.bundle",
				Name:    "testoperator.v1.2.0",
				Package: "testoperator",
				Image:   "quay.io/test/testoperator-bundle:v1.2.0",
				Properties: []property.Property{
					property.MustBuildPackage("testoperator", "1.2.0"),
				},
			},
		},
	}
}

// Helper function to create a valid test package Meta entry
// nolint: unparam
func createValidTestPackageMeta(name, defaultChannel string) *declcfg.Meta {
	pkg := declcfg.Package{
		Schema:         "olm.package",
		Name:           name,
		DefaultChannel: defaultChannel,
		Description:    fmt.Sprintf("%s operator", name),
	}

	blob, err := json.Marshal(pkg)
	if err != nil {
		panic(err)
	}

	return &declcfg.Meta{
		Schema:  "olm.package",
		Name:    name,
		Package: name,
		Blob:    json.RawMessage(blob),
	}
}

// Helper function to create a valid test bundle Meta entry with proper naming convention
// nolint: unparam
func createValidTestBundleMeta(name, packageName, version, release string) *declcfg.Meta {
	var bundleName string
	var properties []property.Property

	if release != "" {
		// Create bundle name following the normalizeName convention: package-vversion-release
		bundleName = fmt.Sprintf("%s-v%s-%s", packageName, version, release)
		properties = []property.Property{
			property.MustBuildPackageRelease(packageName, version, release),
			property.MustBuildBundleObject([]byte(fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, bundleName))),
			property.MustBuildBundleObject([]byte(`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`)),
		}
	} else {
		// Use simple naming convention for bundles without release version
		bundleName = name
		properties = []property.Property{
			property.MustBuildPackage(packageName, version),
			property.MustBuildBundleObject([]byte(fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, bundleName))),
			property.MustBuildBundleObject([]byte(`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`)),
		}
	}

	bundle := declcfg.Bundle{
		Schema:     "olm.bundle",
		Name:       bundleName,
		Package:    packageName,
		Image:      fmt.Sprintf("quay.io/test/%s-bundle:v%s", packageName, version),
		Properties: properties,
		RelatedImages: []declcfg.RelatedImage{
			{
				Name:  "bundle",
				Image: fmt.Sprintf("quay.io/test/%s-bundle:v%s", packageName, version),
			},
		},
		CsvJSON: fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, bundleName),
		Objects: []string{
			fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, bundleName),
			`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`,
		},
	}

	blob, err := json.Marshal(bundle)
	if err != nil {
		panic(err)
	}

	return &declcfg.Meta{
		Schema:  "olm.bundle",
		Name:    bundleName,
		Package: packageName,
		Blob:    json.RawMessage(blob),
	}
}

// Helper function to create a valid test channel Meta entry with proper bundle names
// nolint: unparam
func createValidTestChannelMeta(name, packageName string, entries []declcfg.ChannelEntry) *declcfg.Meta {
	channel := declcfg.Channel{
		Schema:  "olm.channel",
		Name:    name,
		Package: packageName,
		Entries: entries,
	}

	blob, err := json.Marshal(channel)
	if err != nil {
		panic(err)
	}

	return &declcfg.Meta{
		Schema:  "olm.channel",
		Name:    name,
		Package: packageName,
		Blob:    json.RawMessage(blob),
	}
}

func TestParseSpec(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    *SubstitutesTemplateData
		expectError bool
		errorMsg    string
	}{
		{
			name: "Success/valid template with substitutions",
			input: `
schema: olm.template.substitutes
entries:
  - schema: olm.channel
    name: stable
    package: testoperator
    blob: '{"schema":"olm.channel","name":"stable","package":"testoperator","entries":[{"name":"testoperator.v1.0.0"}]}'
substitutions:
  - name: testoperator.v1.1.0
    base: testoperator.v1.0.0
`,
			expected: &SubstitutesTemplateData{
				Schema: "olm.template.substitutes",
				Entries: []*declcfg.Meta{
					{
						Schema:  "olm.channel",
						Name:    "stable",
						Package: "testoperator",
						Blob:    json.RawMessage(`{"schema":"olm.channel","name":"stable","package":"testoperator","entries":[{"name":"testoperator.v1.0.0"}]}`),
					},
				},
				Substitutions: []Substitute{
					{Name: "testoperator.v1.1.0", Base: "testoperator.v1.0.0"},
				},
			},
			expectError: false,
		},
		{
			name: "Error/invalid schema",
			input: `
schema: olm.template.invalid
entries: []
substitutions: []
`,
			expectError: true,
			errorMsg:    "template has unknown schema",
		},
		{
			name: "Error/missing schema",
			input: `
entries: []
substitutions: []
`,
			expectError: true,
			errorMsg:    "template has unknown schema",
		},
		{
			name: "Error/invalid YAML",
			input: `
schema: olm.template.substitutes
entries: [
substitutions: []
`,
			expectError: true,
			errorMsg:    "decoding template schema",
		},
		{
			name: "Success/empty template",
			input: `
schema: olm.template.substitutes
entries: []
substitutions: []
`,
			expected: &SubstitutesTemplateData{
				Schema:        "olm.template.substitutes",
				Entries:       []*declcfg.Meta{},
				Substitutions: []Substitute{},
			},
			expectError: false,
		},
		{
			name: "Success/multiple substitutions",
			input: `
schema: olm.template.substitutes
entries:
  - schema: olm.channel
    name: stable
    package: testoperator
    blob: '{"schema":"olm.channel","name":"stable","package":"testoperator","entries":[{"name":"testoperator.v1.0.0"}]}'
substitutions:
  - name: testoperator.v1.1.0
    base: testoperator.v1.0.0
  - name: testoperator.v1.2.0
    base: testoperator.v1.1.0
`,
			expected: &SubstitutesTemplateData{
				Schema: "olm.template.substitutes",
				Entries: []*declcfg.Meta{
					{
						Schema:  "olm.channel",
						Name:    "stable",
						Package: "testoperator",
						Blob:    json.RawMessage(`{"schema":"olm.channel","name":"stable","package":"testoperator","entries":[{"name":"testoperator.v1.0.0"}]}`),
					},
				},
				Substitutions: []Substitute{
					{Name: "testoperator.v1.1.0", Base: "testoperator.v1.0.0"},
					{Name: "testoperator.v1.2.0", Base: "testoperator.v1.1.0"},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result, err := parseSpec(reader)

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected.Schema, result.Schema)
				require.Len(t, result.Entries, len(tt.expected.Entries))
				require.Len(t, result.Substitutions, len(tt.expected.Substitutions))

				// Check substitutions
				for i, expectedSub := range tt.expected.Substitutions {
					require.Equal(t, expectedSub.Name, result.Substitutions[i].Name)
					require.Equal(t, expectedSub.Base, result.Substitutions[i].Base)
				}
			}
		})
	}
}

func TestRender(t *testing.T) {
	tests := []struct {
		name          string
		entries       []*declcfg.Meta
		substitutions []Substitute
		expectError   bool
		errorContains string
		validate      func(t *testing.T, cfg *declcfg.DeclarativeConfig)
	}{
		{
			name: "Success/render with single substitution",
			entries: []*declcfg.Meta{
				createValidTestPackageMeta("testoperator", "stable"),
				createValidTestChannelMeta("stable", "testoperator", []declcfg.ChannelEntry{
					{Name: "testoperator.v1.0.0"},
					{Name: "testoperator.v1.1.0", Replaces: "testoperator.v1.0.0"}, // Base bundle must be in channel entries
				}),
				createValidTestBundleMeta("testoperator.v1.0.0", "testoperator", "1.0.0", ""),
				createValidTestBundleMeta("testoperator.v1.1.0", "testoperator", "1.1.0", ""),
				// Substitute.name bundle (rendered from image ref) must NOT be in template entries
			},
			substitutions: []Substitute{
				{Name: "quay.io/test/testoperator-bundle:v1.1.0-alpha", Base: "testoperator.v1.1.0"}, // Use bundle image reference
			},
			expectError: false,
			validate: func(t *testing.T, cfg *declcfg.DeclarativeConfig) {
				require.Len(t, cfg.Channels, 1)
				channel := cfg.Channels[0]
				require.Len(t, channel.Entries, 3) // Original 2 + 1 new substitution

				// Find the new substitution entry
				var substituteEntry *declcfg.ChannelEntry
				for i := range channel.Entries {
					if channel.Entries[i].Name == "testoperator-v1.1.0-alpha" {
						substituteEntry = &channel.Entries[i]
						break
					}
				}
				require.NotNil(t, substituteEntry)
				require.Equal(t, "testoperator.v1.0.0", substituteEntry.Replaces)
				require.Contains(t, substituteEntry.Skips, "testoperator.v1.1.0")
			},
		},
		{
			name: "Success/render with multiple substitutions",
			entries: []*declcfg.Meta{
				createValidTestPackageMeta("testoperator", "stable"),
				createValidTestChannelMeta("stable", "testoperator", []declcfg.ChannelEntry{
					{Name: "testoperator.v1.0.0"},
					{Name: "testoperator.v1.1.0", Replaces: "testoperator.v1.0.0"},
					// Don't include substitution bundles in channel entries initially - they will be added by the substitution process
				}),
				createValidTestBundleMeta("testoperator.v1.0.0", "testoperator", "1.0.0", ""),
				createValidTestBundleMeta("testoperator.v1.1.0", "testoperator", "1.1.0", ""),
				// Don't include substitution bundles in entries - they will be added by the substitution process
			},
			substitutions: []Substitute{
				{Name: "quay.io/test/testoperator-bundle:v1.2.0-alpha", Base: "testoperator.v1.1.0"},
				{Name: "quay.io/test/testoperator-bundle:v1.3.0-alpha", Base: "testoperator-v1.2.0-alpha"},
			},
			expectError: false,
			validate: func(t *testing.T, cfg *declcfg.DeclarativeConfig) {
				require.Len(t, cfg.Channels, 1)
				channel := cfg.Channels[0]
				require.Len(t, channel.Entries, 4) // Original 2 + 2 new substitutions

				// Check first substitution (it gets cleared by the second substitution)
				var firstSub *declcfg.ChannelEntry
				for i := range channel.Entries {
					if channel.Entries[i].Name == "testoperator-v1.2.0-alpha" {
						firstSub = &channel.Entries[i]
						break
					}
				}
				require.NotNil(t, firstSub)
				require.Empty(t, firstSub.Replaces) // Cleared by second substitution
				require.Nil(t, firstSub.Skips)      // Cleared by second substitution

				// Check second substitution
				var secondSub *declcfg.ChannelEntry
				for i := range channel.Entries {
					if channel.Entries[i].Name == "testoperator-v1.3.0-alpha" {
						secondSub = &channel.Entries[i]
						break
					}
				}
				require.NotNil(t, secondSub)
				require.Equal(t, "testoperator.v1.0.0", secondSub.Replaces)
				require.Contains(t, secondSub.Skips, "testoperator-v1.2.0-alpha")
			},
		},
		{
			name: "Success/render with no substitutions",
			entries: []*declcfg.Meta{
				createValidTestPackageMeta("testoperator", "stable"),
				createValidTestChannelMeta("stable", "testoperator", []declcfg.ChannelEntry{
					{Name: "testoperator.v1.0.0"},
				}),
				createValidTestBundleMeta("testoperator.v1.0.0", "testoperator", "1.0.0", ""),
			},
			substitutions: []Substitute{},
			expectError:   false,
			validate: func(t *testing.T, cfg *declcfg.DeclarativeConfig) {
				require.Len(t, cfg.Channels, 1)
				channel := cfg.Channels[0]
				require.Len(t, channel.Entries, 1)
				require.Equal(t, "testoperator.v1.0.0", channel.Entries[0].Name)
			},
		},
		{
			name: "Error/render with substitution that has no matching base",
			entries: []*declcfg.Meta{
				createValidTestPackageMeta("testoperator", "stable"),
				createValidTestChannelMeta("stable", "testoperator", []declcfg.ChannelEntry{
					{Name: "testoperator.v1.0.0"},
				}),
				createValidTestBundleMeta("testoperator.v1.0.0", "testoperator", "1.0.0", ""),
			},
			substitutions: []Substitute{
				{Name: "quay.io/test/testoperator-bundle:v1.2.0-alpha", Base: "nonexistent.v1.0.0"},
			},
			expectError: true,
			validate: func(t *testing.T, cfg *declcfg.DeclarativeConfig) {
				require.Len(t, cfg.Channels, 1)
				channel := cfg.Channels[0]
				require.Len(t, channel.Entries, 1) // No new entries added
				require.Equal(t, "testoperator.v1.0.0", channel.Entries[0].Name)
			},
		},
		{
			name: "Error/render with invalid substitution (empty name)",
			entries: []*declcfg.Meta{
				createValidTestPackageMeta("testoperator", "stable"),
				createValidTestChannelMeta("stable", "testoperator", []declcfg.ChannelEntry{
					{Name: "testoperator.v1.0.0"},
					{Name: "testoperator.v1.1.0", Replaces: "testoperator.v1.0.0"},
				}),
				createValidTestBundleMeta("testoperator.v1.0.0", "testoperator", "1.0.0", ""),
				createValidTestBundleMeta("testoperator.v1.1.0", "testoperator", "1.1.0", ""),
			},
			substitutions: []Substitute{
				{Name: "", Base: "testoperator.v1.1.0"}, // Invalid: empty name
			},
			expectError:   true,
			errorContains: "substitution name cannot be empty",
		},
		{
			name: "Error/render with invalid substitution (empty base)",
			entries: []*declcfg.Meta{
				createValidTestPackageMeta("testoperator", "stable"),
				createValidTestChannelMeta("stable", "testoperator", []declcfg.ChannelEntry{
					{Name: "testoperator.v1.0.0"},
					{Name: "testoperator.v1.1.0", Replaces: "testoperator.v1.0.0"},
				}),
				createValidTestBundleMeta("testoperator.v1.0.0", "testoperator", "1.0.0", ""),
				createValidTestBundleMeta("testoperator.v1.1.0", "testoperator", "1.1.0", ""),
			},
			substitutions: []Substitute{
				{Name: "quay.io/test/testoperator-bundle:v1.2.0-alpha", Base: ""}, // Invalid: empty base
			},
			expectError:   true,
			errorContains: "substitution base cannot be empty",
		},
		{
			name: "Error/render with invalid substitution (same name and base)",
			entries: []*declcfg.Meta{
				createValidTestPackageMeta("testoperator", "stable"),
				createValidTestChannelMeta("stable", "testoperator", []declcfg.ChannelEntry{
					{Name: "testoperator.v1.0.0"},
					{Name: "testoperator.v1.1.0", Replaces: "testoperator.v1.0.0"},
				}),
				createValidTestBundleMeta("testoperator.v1.0.0", "testoperator", "1.0.0", ""),
				createValidTestBundleMeta("testoperator.v1.1.0", "testoperator", "1.1.0", ""),
			},
			substitutions: []Substitute{
				{Name: "testoperator.v1.1.0", Base: "testoperator.v1.1.0"}, // Invalid: same name and base
			},
			expectError:   true,
			errorContains: "substitution name and base cannot be the same",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create template with test data
			templateData := SubstitutesTemplateData{
				Schema:        "olm.template.substitutes",
				Entries:       tt.entries,
				Substitutions: tt.substitutions,
			}

			// Convert to JSON and create reader
			templateJSON, err := json.Marshal(templateData)
			require.NoError(t, err)

			reader := strings.NewReader(string(templateJSON))
			templateInstance := template{
				renderBundle: func(ctx context.Context, imageRef string) (*declcfg.DeclarativeConfig, error) {
					// Mock implementation that creates a bundle from the image reference
					packageName := "testoperator"
					version := "1.0.0"
					release := ""

					// Extract version from image reference if it contains a version tag
					if strings.Contains(imageRef, ":v") {
						parts := strings.Split(imageRef, ":v")
						if len(parts) == 2 {
							versionTag := parts[1]
							// Check if version has a prerelease/build metadata (e.g., "1.2.0-alpha")
							if strings.Contains(versionTag, "-") {
								versionParts := strings.SplitN(versionTag, "-", 2)
								version = versionParts[0]
								release = versionParts[1]
							} else {
								version = versionTag
							}
						}
					}

					// Create bundle name based on whether it has a release version
					var bundleName string
					var properties []property.Property

					if release != "" {
						// substitutesFor bundle: package-vversion-release
						bundleName = fmt.Sprintf("%s-v%s-%s", packageName, version, release)
						properties = []property.Property{
							property.MustBuildPackageRelease(packageName, version, release),
							property.MustBuildBundleObject([]byte(fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, imageRef))),
							property.MustBuildBundleObject([]byte(`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`)),
						}
					} else {
						// normal bundle: package.vversion
						bundleName = fmt.Sprintf("%s.v%s", packageName, version)
						properties = []property.Property{
							property.MustBuildPackage(packageName, version),
							property.MustBuildBundleObject([]byte(fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, imageRef))),
							property.MustBuildBundleObject([]byte(`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`)),
						}
					}

					return &declcfg.DeclarativeConfig{
						Bundles: []declcfg.Bundle{
							{
								Schema:     "olm.bundle",
								Name:       bundleName,
								Package:    packageName,
								Image:      imageRef,
								Properties: properties,
								RelatedImages: []declcfg.RelatedImage{
									{Name: "bundle", Image: imageRef},
								},
								CsvJSON: fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, imageRef),
								Objects: []string{
									fmt.Sprintf(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":%q}}`, imageRef),
									`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`,
								},
							},
						},
					}, nil
				},
			}
			ctx := context.Background()

			result, err := templateInstance.Render(ctx, reader)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestProcessSubstitution(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *declcfg.DeclarativeConfig
		substitution Substitute
		validate     func(t *testing.T, cfg *declcfg.DeclarativeConfig)
	}{
		{
			name: "Success/substitution with replaces relationship",
			cfg: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema:         "olm.package",
						Name:           "testoperator",
						DefaultChannel: "stable",
						Description:    "testoperator operator",
					},
				},
				Channels: []declcfg.Channel{
					{
						Schema:  "olm.channel",
						Name:    "stable",
						Package: "testoperator",
						Entries: []declcfg.ChannelEntry{
							{Name: "testoperator-v1.0.0-alpha"},
							{Name: "testoperator-v1.1.0-alpha", Replaces: "testoperator-v1.0.0-alpha"},
						},
					},
				},
				Bundles: []declcfg.Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "testoperator-v1.0.0-alpha",
						Package: "testoperator",
						Image:   "quay.io/test/testoperator-bundle:v1.0.0",
						Properties: []property.Property{
							property.MustBuildPackageRelease("testoperator", "1.0.0", "alpha"),
							property.MustBuildBundleObject([]byte(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":"testoperator-v1.0.0-alpha"}}`)),
							property.MustBuildBundleObject([]byte(`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`)),
						},
						RelatedImages: []declcfg.RelatedImage{
							{Name: "bundle", Image: "quay.io/test/testoperator-bundle:v1.0.0"},
						},
						CsvJSON: `{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":"testoperator-v1.0.0-alpha"}}`,
						Objects: []string{
							`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":"testoperator-v1.0.0-alpha"}}`,
							`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`,
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "testoperator-v1.1.0-alpha",
						Package: "testoperator",
						Image:   "quay.io/test/testoperator-bundle:v1.1.0-alpha",
						Properties: []property.Property{
							property.MustBuildPackageRelease("testoperator", "1.1.0", "alpha"),
							property.MustBuildBundleObject([]byte(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":"testoperator-v1.1.0-alpha"}}`)),
							property.MustBuildBundleObject([]byte(`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`)),
						},
						RelatedImages: []declcfg.RelatedImage{
							{Name: "bundle", Image: "quay.io/test/testoperator-bundle:v1.1.0-alpha"},
						},
						CsvJSON: `{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":"testoperator-v1.1.0-alpha"}}`,
						Objects: []string{
							`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":"testoperator-v1.1.0-alpha"}}`,
							`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`,
						},
					},
				},
			},
			substitution: Substitute{Name: "quay.io/test/testoperator-bundle:v1.2.0-alpha", Base: "testoperator-v1.1.0-alpha"},
			validate: func(t *testing.T, cfg *declcfg.DeclarativeConfig) {
				channel := cfg.Channels[0]
				require.Len(t, channel.Entries, 3)

				// Find the new substitution entry
				var substituteEntry *declcfg.ChannelEntry
				for i := range channel.Entries {
					if channel.Entries[i].Name == "testoperator-v1.2.0-alpha" {
						substituteEntry = &channel.Entries[i]
						break
					}
				}
				require.NotNil(t, substituteEntry)
				require.Equal(t, "testoperator-v1.0.0-alpha", substituteEntry.Replaces)
				require.Contains(t, substituteEntry.Skips, "testoperator-v1.1.0-alpha")

				// Check that original entry was cleared
				var originalEntry *declcfg.ChannelEntry
				for i := range channel.Entries {
					if channel.Entries[i].Name == "testoperator-v1.1.0-alpha" {
						originalEntry = &channel.Entries[i]
						break
					}
				}
				require.NotNil(t, originalEntry)
				require.Empty(t, originalEntry.Replaces)
				require.Empty(t, originalEntry.Skips)
				require.Empty(t, originalEntry.SkipRange)
			},
		},
		{
			name: "Success/substitution with skips and skipRange",
			cfg: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema:         "olm.package",
						Name:           "testoperator",
						DefaultChannel: "stable",
						Description:    "testoperator operator",
					},
				},
				Channels: []declcfg.Channel{
					{
						Schema:  "olm.channel",
						Name:    "stable",
						Package: "testoperator",
						Entries: []declcfg.ChannelEntry{
							{Name: "testoperator-v1.0.0-alpha"},
							{Name: "testoperator-v1.1.0-alpha", Replaces: "testoperator-v1.0.0-alpha", Skips: []string{"testoperator-v0.9.0-alpha"}, SkipRange: ">=0.9.0 <1.1.0"},
						},
					},
				},
				Bundles: []declcfg.Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "testoperator-v1.0.0-alpha",
						Package: "testoperator",
						Image:   "quay.io/test/testoperator-bundle:v1.0.0",
						Properties: []property.Property{
							property.MustBuildPackageRelease("testoperator", "1.0.0", "alpha"),
							property.MustBuildBundleObject([]byte(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":"testoperator-v1.0.0-alpha"}}`)),
							property.MustBuildBundleObject([]byte(`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`)),
						},
						RelatedImages: []declcfg.RelatedImage{
							{Name: "bundle", Image: "quay.io/test/testoperator-bundle:v1.0.0"},
						},
						CsvJSON: `{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":"testoperator-v1.0.0-alpha"}}`,
						Objects: []string{
							`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":"testoperator-v1.0.0-alpha"}}`,
							`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`,
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "testoperator-v1.1.0-alpha",
						Package: "testoperator",
						Image:   "quay.io/test/testoperator-bundle:v1.1.0-alpha",
						Properties: []property.Property{
							property.MustBuildPackageRelease("testoperator", "1.1.0", "alpha"),
							property.MustBuildBundleObject([]byte(`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":"testoperator-v1.1.0-alpha"}}`)),
							property.MustBuildBundleObject([]byte(`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`)),
						},
						RelatedImages: []declcfg.RelatedImage{
							{Name: "bundle", Image: "quay.io/test/testoperator-bundle:v1.1.0-alpha"},
						},
						CsvJSON: `{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":"testoperator-v1.1.0-alpha"}}`,
						Objects: []string{
							`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1", "metadata":{"name":"testoperator-v1.1.0-alpha"}}`,
							`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`,
						},
					},
				},
			},
			substitution: Substitute{Name: "quay.io/test/testoperator-bundle:v1.2.0-alpha", Base: "testoperator-v1.1.0-alpha"},
			validate: func(t *testing.T, cfg *declcfg.DeclarativeConfig) {
				channel := cfg.Channels[0]
				require.Len(t, channel.Entries, 3)

				// Find the new substitution entry
				var substituteEntry *declcfg.ChannelEntry
				for i := range channel.Entries {
					if channel.Entries[i].Name == "testoperator-v1.2.0-alpha" {
						substituteEntry = &channel.Entries[i]
						break
					}
				}
				require.NotNil(t, substituteEntry)
				require.Equal(t, "testoperator-v1.0.0-alpha", substituteEntry.Replaces)
				require.Contains(t, substituteEntry.Skips, "testoperator-v0.9.0-alpha")
				require.Contains(t, substituteEntry.Skips, "testoperator-v1.1.0-alpha")
				require.Equal(t, ">=0.9.0 <1.1.0", substituteEntry.SkipRange)
			},
		},
		{
			name: "Error/substitution with no matching base",
			cfg: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema:         "olm.package",
						Name:           "testoperator",
						DefaultChannel: "stable",
					},
				},
				Channels: []declcfg.Channel{
					{
						Schema:  "olm.channel",
						Name:    "stable",
						Package: "testoperator",
						Entries: []declcfg.ChannelEntry{
							{Name: "testoperator.v1.0.0"},
						},
					},
				},
				Bundles: []declcfg.Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "testoperator.v1.0.0",
						Package: "testoperator",
						Image:   "quay.io/test/testoperator-bundle:v1.0.0",
						Properties: []property.Property{
							property.MustBuildPackage("testoperator", "1.0.0"),
						},
					},
				},
			},
			substitution: Substitute{Name: "quay.io/test/testoperator-bundle:v1.2.0-alpha", Base: "nonexistent.v1.0.0"},
			validate: func(t *testing.T, cfg *declcfg.DeclarativeConfig) {
				// This test should fail, so this validation should not be called
				t.Fatal("This test should have failed")
			},
		},
		{
			name: "Success/substitution with multiple channels",
			cfg: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema:         "olm.package",
						Name:           "testoperator",
						DefaultChannel: "stable",
					},
				},
				Channels: []declcfg.Channel{
					{
						Schema:  "olm.channel",
						Name:    "stable",
						Package: "testoperator",
						Entries: []declcfg.ChannelEntry{
							{Name: "testoperator-v1.0.0-alpha"},
							{Name: "testoperator-v1.1.0-alpha", Replaces: "testoperator-v1.0.0-alpha"},
						},
					},
					{
						Schema:  "olm.channel",
						Name:    "beta",
						Package: "testoperator",
						Entries: []declcfg.ChannelEntry{
							{Name: "testoperator-v1.0.0-alpha"},
							{Name: "testoperator-v1.1.0-alpha", Replaces: "testoperator-v1.0.0-alpha"},
						},
					},
				},
				Bundles: []declcfg.Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "testoperator-v1.0.0-alpha",
						Package: "testoperator",
						Image:   "quay.io/test/testoperator-bundle:v1.0.0",
						Properties: []property.Property{
							property.MustBuildPackageRelease("testoperator", "1.0.0", "alpha"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "testoperator-v1.1.0-alpha",
						Package: "testoperator",
						Image:   "quay.io/test/testoperator-bundle:v1.1.0-alpha",
						Properties: []property.Property{
							property.MustBuildPackageRelease("testoperator", "1.1.0", "alpha"),
						},
					},
				},
			},
			substitution: Substitute{Name: "quay.io/test/testoperator-bundle:v1.2.0-alpha", Base: "testoperator-v1.1.0-alpha"},
			validate: func(t *testing.T, cfg *declcfg.DeclarativeConfig) {
				require.Len(t, cfg.Channels, 2)

				// Check stable channel
				stableChannel := cfg.Channels[0]
				require.Len(t, stableChannel.Entries, 3)

				// Check beta channel
				betaChannel := cfg.Channels[1]
				require.Len(t, betaChannel.Entries, 3)

				// Both channels should have the substitution
				for _, channel := range cfg.Channels {
					var substituteEntry *declcfg.ChannelEntry
					for i := range channel.Entries {
						if channel.Entries[i].Name == "testoperator-v1.2.0-alpha" {
							substituteEntry = &channel.Entries[i]
							break
						}
					}
					require.NotNil(t, substituteEntry)
					require.Equal(t, "testoperator-v1.0.0-alpha", substituteEntry.Replaces)
					require.Contains(t, substituteEntry.Skips, "testoperator-v1.1.0-alpha")
				}
			},
		},
		{
			name: "Success/substitution updates existing references",
			cfg: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema:         "olm.package",
						Name:           "testoperator",
						DefaultChannel: "stable",
					},
				},
				Channels: []declcfg.Channel{
					{
						Schema:  "olm.channel",
						Name:    "stable",
						Package: "testoperator",
						Entries: []declcfg.ChannelEntry{
							{Name: "testoperator.v1.0.0"},
							{Name: "testoperator.v1.1.0", Replaces: "testoperator.v1.0.0"},
							{Name: "testoperator.v1.2.0", Replaces: "testoperator.v1.1.0"},
						},
					},
				},
				Bundles: []declcfg.Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "testoperator.v1.0.0",
						Package: "testoperator",
						Image:   "quay.io/test/testoperator-bundle:v1.0.0",
						Properties: []property.Property{
							property.MustBuildPackage("testoperator", "1.0.0"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "testoperator.v1.1.0",
						Package: "testoperator",
						Image:   "quay.io/test/testoperator-bundle:v1.1.0",
						Properties: []property.Property{
							property.MustBuildPackage("testoperator", "1.1.0"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "testoperator.v1.2.0",
						Package: "testoperator",
						Image:   "quay.io/test/testoperator-bundle:v1.2.0",
						Properties: []property.Property{
							property.MustBuildPackage("testoperator", "1.2.0"),
						},
					},
				},
			},
			substitution: Substitute{Name: "quay.io/test/testoperator-bundle:v1.1.0-alpha", Base: "testoperator.v1.1.0"},
			validate: func(t *testing.T, cfg *declcfg.DeclarativeConfig) {
				channel := cfg.Channels[0]
				require.Len(t, channel.Entries, 4) // Original 3 + 1 new substitution

				// Find the entry that originally replaced testoperator.v1.1.0
				var updatedEntry *declcfg.ChannelEntry
				for i := range channel.Entries {
					if channel.Entries[i].Name == "testoperator.v1.2.0" {
						updatedEntry = &channel.Entries[i]
						break
					}
				}
				require.NotNil(t, updatedEntry)
				require.Equal(t, "testoperator-v1.1.0-alpha", updatedEntry.Replaces) // Should now reference the substitute
				require.Contains(t, updatedEntry.Skips, "testoperator.v1.1.0")       // Should skip the original base
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := createMockTemplate()
			ctx := context.Background()
			err := template.processSubstitution(ctx, tt.cfg, tt.substitution)
			if strings.Contains(tt.name, "Error/") {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.validate(t, tt.cfg)
			}
		})
	}
}

func TestBoundaryCases(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "Error/empty DeclarativeConfig",
			testFunc: func(t *testing.T) {
				cfg := &declcfg.DeclarativeConfig{}
				substitution := Substitute{Name: "quay.io/test/test-bundle:v1.0.0-alpha", Base: "test.v0.9.0"}
				template := createMockTemplate()
				ctx := context.Background()
				err := template.processSubstitution(ctx, cfg, substitution)
				require.Error(t, err)
				require.Contains(t, err.Error(), "unknown package")
			},
		},
		{
			name: "Error/DeclarativeConfig with empty channels",
			testFunc: func(t *testing.T) {
				cfg := &declcfg.DeclarativeConfig{
					Channels: []declcfg.Channel{},
				}
				substitution := Substitute{Name: "quay.io/test/test-bundle:v1.0.0-alpha", Base: "test.v0.9.0"}
				template := createMockTemplate()
				ctx := context.Background()
				err := template.processSubstitution(ctx, cfg, substitution)
				require.Error(t, err)
				require.Contains(t, err.Error(), "unknown package")
			},
		},
		{
			name: "Error/channel with empty entries",
			testFunc: func(t *testing.T) {
				cfg := &declcfg.DeclarativeConfig{
					Packages: []declcfg.Package{
						{
							Schema:         "olm.package",
							Name:           "testoperator",
							DefaultChannel: "stable",
						},
					},
					Channels: []declcfg.Channel{
						{
							Schema:  "olm.channel",
							Name:    "stable",
							Package: "testoperator",
							Entries: []declcfg.ChannelEntry{},
						},
					},
				}
				substitution := Substitute{Name: "quay.io/test/test-bundle:v1.0.0-alpha", Base: "test.v0.9.0"}
				template := createMockTemplate()
				ctx := context.Background()
				err := template.processSubstitution(ctx, cfg, substitution)
				require.Error(t, err)
				require.Contains(t, err.Error(), "unknown package")
			},
		},
		{
			name: "Error/substitution with empty name",
			testFunc: func(t *testing.T) {
				cfg := createTestDeclarativeConfig()
				substitution := Substitute{Name: "", Base: "testoperator.v1.1.0"}
				template := createMockTemplate()
				ctx := context.Background()
				err := template.processSubstitution(ctx, cfg, substitution)
				require.Error(t, err)
				require.Contains(t, err.Error(), "substitution name cannot be empty")
				// Should not create any new entries with empty name
				require.Len(t, cfg.Channels[0].Entries, 3) // Original entries unchanged
			},
		},
		{
			name: "Error/substitution with empty base",
			testFunc: func(t *testing.T) {
				cfg := createTestDeclarativeConfig()
				substitution := Substitute{Name: "quay.io/test/testoperator-bundle:v1.2.0-alpha", Base: ""}
				template := createMockTemplate()
				ctx := context.Background()
				err := template.processSubstitution(ctx, cfg, substitution)
				require.Error(t, err)
				require.Contains(t, err.Error(), "substitution base cannot be empty")
				// Should not create any new entries with empty base
				require.Len(t, cfg.Channels[0].Entries, 3) // Original entries unchanged
			},
		},
		{
			name: "Error/substitution with same name and base",
			testFunc: func(t *testing.T) {
				cfg := createTestDeclarativeConfig()
				substitution := Substitute{Name: "quay.io/test/testoperator-bundle:v1.1.0-alpha", Base: "quay.io/test/testoperator-bundle:v1.1.0-alpha"}
				template := createMockTemplate()
				ctx := context.Background()
				err := template.processSubstitution(ctx, cfg, substitution)
				require.Error(t, err)
				require.Contains(t, err.Error(), "substitution name and base cannot be the same")
				// Should not create any new entries when name equals base
				require.Len(t, cfg.Channels[0].Entries, 3) // Original entries unchanged
			},
		},
		{
			name: "Error/template with malformed JSON in blob",
			testFunc: func(t *testing.T) {
				// Create a template with invalid JSON in the blob
				invalidMeta := &declcfg.Meta{
					Schema:  "olm.channel",
					Name:    "stable",
					Package: "testoperator",
					Blob:    json.RawMessage(`{"invalid": json, "missing": quote}`),
				}

				template := SubstitutesTemplateData{
					Schema:        "olm.template.substitutes",
					Entries:       []*declcfg.Meta{invalidMeta},
					Substitutions: []Substitute{},
				}

				_, err := json.Marshal(template)
				// The malformed JSON should cause an error during marshaling
				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid character")
			},
		},
		{
			name: "Success/template with nil context",
			testFunc: func(t *testing.T) {
				entries := []*declcfg.Meta{
					createValidTestPackageMeta("testoperator", "stable"),
					createValidTestChannelMeta("stable", "testoperator", []declcfg.ChannelEntry{
						{Name: "testoperator-v1.0.0-alpha"},
					}),
					createValidTestBundleMeta("testoperator-v1.0.0-alpha", "testoperator", "1.0.0", "alpha"),
				}

				templateData := SubstitutesTemplateData{
					Schema:        "olm.template.substitutes",
					Entries:       entries,
					Substitutions: []Substitute{},
				}

				templateJSON, err := json.Marshal(templateData)
				require.NoError(t, err)

				reader := strings.NewReader(string(templateJSON))
				templateInstance := template{}

				result, err := templateInstance.Render(context.TODO(), reader)
				require.NoError(t, err) // Context is not used in current implementation
				require.NotNil(t, result)
			},
		},
		{
			name: "Error/substitution with invalid declarative config - missing package",
			testFunc: func(t *testing.T) {
				// Create a config with a bundle that references a non-existent package
				cfg := &declcfg.DeclarativeConfig{
					Packages: []declcfg.Package{
						{
							Schema:         "olm.package",
							Name:           "nonexistent",
							DefaultChannel: "stable",
						},
					},
					Bundles: []declcfg.Bundle{
						{
							Name:    "testoperator.v1.1.0", // This is the substitution name we're testing
							Package: "nonexistent",         // This package exists but bundle name doesn't match
							Properties: []property.Property{
								{
									Type:  property.TypePackage,
									Value: json.RawMessage(`{"packageName":"nonexistent","version":"1.1.0"}`),
								},
							},
						},
					},
				}
				substitution := Substitute{Name: "quay.io/test/testoperator-bundle:v1.1.0-alpha", Base: "testoperator.v1.0.0"}
				template := createMockTemplate()
				ctx := context.Background()
				err := template.processSubstitution(ctx, cfg, substitution)
				require.Error(t, err)
				require.Contains(t, err.Error(), "not found in any channel entries")
			},
		},
		{
			name: "Error/substitution with invalid declarative config - bundle missing olm.package property",
			testFunc: func(t *testing.T) {
				// Create a config with a bundle that has no olm.package property
				cfg := &declcfg.DeclarativeConfig{
					Packages: []declcfg.Package{
						{
							Schema:         "olm.package",
							Name:           "testoperator",
							DefaultChannel: "stable",
						},
					},
					Bundles: []declcfg.Bundle{
						{
							Name:       "testoperator.v1.1.0", // This is the substitution name we're testing
							Package:    "testoperator",
							Properties: []property.Property{}, // No olm.package property
						},
					},
				}
				substitution := Substitute{Name: "quay.io/test/testoperator-bundle:v1.1.0-alpha", Base: "testoperator.v1.0.0"}
				template := createMockTemplate()
				ctx := context.Background()
				err := template.processSubstitution(ctx, cfg, substitution)
				require.Error(t, err)
				require.Contains(t, err.Error(), "must have exactly 1 \"olm.package\" property")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}
