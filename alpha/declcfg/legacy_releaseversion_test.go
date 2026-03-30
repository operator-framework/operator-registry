package declcfg

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/property"
)

func TestBundle_usesLegacyReleaseVersion(t *testing.T) {
	tests := []struct {
		name     string
		bundle   Bundle
		expected bool
	}{
		{
			name: "path 1: CsvJSON with substitutesFor annotation",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1-0.1234567890.p",
				Package: "messaging-server",
				CsvJSON: `{
					"apiVersion": "operators.coreos.com/v1alpha1",
					"kind": "ClusterServiceVersion",
					"metadata": {
						"name": "messaging-operator.v8.11.3-opr-1-0.1234567890.p",
						"annotations": {
							"olm.substitutesFor": "messaging-operator.v8.11.3-opr-1"
						}
					}
				}`,
			},
			expected: true,
		},
		{
			name: "path 1: CsvJSON without substitutesFor annotation",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1",
				Package: "messaging-server",
				CsvJSON: `{
					"apiVersion": "operators.coreos.com/v1alpha1",
					"kind": "ClusterServiceVersion",
					"metadata": {
						"name": "messaging-operator.v8.11.3-opr-1",
						"annotations": {
							"capabilities": "Seamless Upgrades"
						}
					}
				}`,
			},
			expected: false,
		},
		{
			name: "path 1: CsvJSON with empty substitutesFor annotation",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1",
				Package: "messaging-server",
				CsvJSON: `{
					"apiVersion": "operators.coreos.com/v1alpha1",
					"kind": "ClusterServiceVersion",
					"metadata": {
						"name": "messaging-operator.v8.11.3-opr-1",
						"annotations": {
							"olm.substitutesFor": ""
						}
					}
				}`,
			},
			expected: false,
		},
		{
			name: "path 1: CsvJSON with malformed JSON falls through to properties",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1",
				Package: "messaging-server",
				CsvJSON: `{not valid json`,
				Properties: []property.Property{
					{
						Type: property.TypeCSVMetadata,
						Value: mustMarshalJSON(t, property.CSVMetadata{
							Annotations: map[string]string{
								"olm.substitutesFor": "messaging-operator.v8.11.3-opr-1",
							},
						}),
					},
				},
			},
			expected: true,
		},
		{
			name: "path 2: olm.csv.metadata with substitutesFor annotation",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1-0.1234567890.p",
				Package: "messaging-server",
				Properties: []property.Property{
					{
						Type: property.TypeCSVMetadata,
						Value: mustMarshalJSON(t, property.CSVMetadata{
							Annotations: map[string]string{
								"alm-examples":       "[{\"apiVersion\":\"broker.example.io/v1beta1\"}]",
								"capabilities":       "Seamless Upgrades",
								"olm.substitutesFor": "messaging-operator.v8.11.3-opr-1",
								"categories":         "Streaming & Messaging",
							},
						}),
					},
				},
			},
			expected: true,
		},
		{
			name: "path 2: olm.csv.metadata without substitutesFor annotation",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1",
				Package: "messaging-server",
				Properties: []property.Property{
					{
						Type: property.TypeCSVMetadata,
						Value: mustMarshalJSON(t, property.CSVMetadata{
							Annotations: map[string]string{
								"capabilities": "Seamless Upgrades",
								"categories":   "Streaming & Messaging",
							},
						}),
					},
				},
			},
			expected: false,
		},
		{
			name: "path 2: olm.csv.metadata with nil annotations map",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1",
				Package: "messaging-server",
				Properties: []property.Property{
					{
						Type: property.TypeCSVMetadata,
						Value: mustMarshalJSON(t, property.CSVMetadata{
							Annotations: nil,
						}),
					},
				},
			},
			expected: false,
		},
		{
			name: "path 2: olm.csv.metadata with empty substitutesFor value",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1",
				Package: "messaging-server",
				Properties: []property.Property{
					{
						Type: property.TypeCSVMetadata,
						Value: mustMarshalJSON(t, property.CSVMetadata{
							Annotations: map[string]string{
								"olm.substitutesFor": "",
							},
						}),
					},
				},
			},
			expected: false,
		},
		{
			name: "path 3: olm.bundle.object with substitutesFor annotation",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1-0.1234567890.p",
				Package: "messaging-server",
				Properties: []property.Property{
					{
						Type: property.TypeBundleObject,
						Value: mustMarshalJSON(t, property.BundleObject{
							Data: []byte(`{
								"apiVersion": "operators.coreos.com/v1alpha1",
								"kind": "ClusterServiceVersion",
								"metadata": {
									"name": "messaging-operator.v8.11.3-opr-1-0.1234567890.p",
									"annotations": {
										"alm-examples": "[{\"apiVersion\":\"broker.example.io/v1beta1\"}]",
										"capabilities": "Seamless Upgrades",
										"olm.substitutesFor": "messaging-operator.v8.11.3-opr-1",
										"categories": "Streaming & Messaging"
									}
								}
							}`),
						}),
					},
				},
			},
			expected: true,
		},
		{
			name: "path 3: olm.bundle.object without substitutesFor annotation",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1",
				Package: "messaging-server",
				Properties: []property.Property{
					{
						Type: property.TypeBundleObject,
						Value: mustMarshalJSON(t, property.BundleObject{
							Data: []byte(`{
								"apiVersion": "operators.coreos.com/v1alpha1",
								"kind": "ClusterServiceVersion",
								"metadata": {
									"name": "messaging-operator.v8.11.3-opr-1",
									"annotations": {
										"capabilities": "Seamless Upgrades"
									}
								}
							}`),
						}),
					},
				},
			},
			expected: false,
		},
		{
			name: "path 3: olm.bundle.object with malformed CSV JSON",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1",
				Package: "messaging-server",
				Properties: []property.Property{
					{
						Type: property.TypeBundleObject,
						Value: mustMarshalJSON(t, property.BundleObject{
							Data: []byte(`{not valid json`),
						}),
					},
				},
			},
			expected: false,
		},
		{
			name: "no CSV data at all",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1",
				Package: "messaging-server",
				Properties: []property.Property{
					{
						Type: property.TypePackage,
						Value: mustMarshalJSON(t, property.Package{
							PackageName: "messaging-server",
							Version:     "8.11.3-opr-1",
						}),
					},
				},
			},
			expected: false,
		},
		{
			name: "multiple properties, substitutesFor in csv.metadata",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1-0.1234567890.p",
				Package: "messaging-server",
				Properties: []property.Property{
					{
						Type: property.TypePackage,
						Value: mustMarshalJSON(t, property.Package{
							PackageName: "messaging-server",
							Version:     "8.11.3-opr-1+0.1234567890.p",
						}),
					},
					{
						Type: property.TypeGVK,
						Value: mustMarshalJSON(t, property.GVK{
							Group:   "broker.example.io",
							Kind:    "ActiveBroker",
							Version: "v1beta1",
						}),
					},
					{
						Type: property.TypeCSVMetadata,
						Value: mustMarshalJSON(t, property.CSVMetadata{
							Annotations: map[string]string{
								"olm.substitutesFor": "messaging-operator.v8.11.3-opr-1",
							},
						}),
					},
				},
			},
			expected: true,
		},
		{
			name: "CsvJSON takes precedence over properties",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1",
				Package: "messaging-server",
				CsvJSON: `{
					"apiVersion": "operators.coreos.com/v1alpha1",
					"kind": "ClusterServiceVersion",
					"metadata": {
						"name": "messaging-operator.v8.11.3-opr-1",
						"annotations": {
							"olm.substitutesFor": "messaging-operator.v8.11.2-opr-1"
						}
					}
				}`,
				Properties: []property.Property{
					{
						Type: property.TypeCSVMetadata,
						Value: mustMarshalJSON(t, property.CSVMetadata{
							Annotations: map[string]string{
								"capabilities": "Basic Install",
							},
						}),
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.bundle.usesLegacyReleaseVersion()
			assert.Equal(t, tt.expected, result, "usesLegacyReleaseVersion() result mismatch")
		})
	}
}

// TestBundle_VersionRelease_LegacyReleaseInference tests the integration
// between usesLegacyReleaseVersion() and VersionRelease() to ensure
// build metadata is only converted to release when substitutesFor annotation exists.
func TestBundle_VersionRelease_LegacyReleaseInference(t *testing.T) {
	tests := []struct {
		name            string
		bundle          Bundle
		expectedVersion string
		expectedRelease string
		expectError     bool
	}{
		{
			name: "build metadata converted to release when substitutesFor present",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1-0.1234567890.p",
				Package: "messaging-server",
				Properties: []property.Property{
					{
						Type: property.TypePackage,
						Value: mustMarshalJSON(t, property.Package{
							PackageName: "messaging-server",
							Version:     "8.11.3-opr-1+0.1234567890.p",
							Release:     "", // no release in property
						}),
					},
					{
						Type: property.TypeCSVMetadata,
						Value: mustMarshalJSON(t, property.CSVMetadata{
							Annotations: map[string]string{
								"olm.substitutesFor": "messaging-operator.v8.11.3-opr-1",
							},
						}),
					},
				},
			},
			expectedVersion: "8.11.3-opr-1",
			expectedRelease: "0.1234567890.p",
			expectError:     false,
		},
		{
			name: "build metadata NOT converted when substitutesFor absent",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1",
				Package: "messaging-server",
				Properties: []property.Property{
					{
						Type: property.TypePackage,
						Value: mustMarshalJSON(t, property.Package{
							PackageName: "messaging-server",
							Version:     "8.11.3-opr-1+0.1234567890.p",
							Release:     "", // no release in property
						}),
					},
					{
						Type: property.TypeCSVMetadata,
						Value: mustMarshalJSON(t, property.CSVMetadata{
							Annotations: map[string]string{
								"capabilities": "Seamless Upgrades",
							},
						}),
					},
				},
			},
			expectedVersion: "8.11.3-opr-1+0.1234567890.p",
			expectedRelease: "",
			expectError:     false,
		},
		{
			name: "explicit release in property takes precedence over build metadata",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1-0.1234567890.p",
				Package: "messaging-server",
				Properties: []property.Property{
					{
						Type: property.TypePackage,
						Value: mustMarshalJSON(t, property.Package{
							PackageName: "messaging-server",
							Version:     "8.11.3-opr-1+0.9999999999.x",
							Release:     "0.1234567890.p", // explicit release
						}),
					},
					{
						Type: property.TypeCSVMetadata,
						Value: mustMarshalJSON(t, property.CSVMetadata{
							Annotations: map[string]string{
								"olm.substitutesFor": "messaging-operator.v8.11.3-opr-1",
							},
						}),
					},
				},
			},
			expectedVersion: "8.11.3-opr-1+0.9999999999.x",
			expectedRelease: "0.1234567890.p",
			expectError:     false,
		},
		{
			name: "no build metadata, no release, substitutesFor present",
			bundle: Bundle{
				Name:    "messaging-operator.v8.11.3-opr-1",
				Package: "messaging-server",
				Properties: []property.Property{
					{
						Type: property.TypePackage,
						Value: mustMarshalJSON(t, property.Package{
							PackageName: "messaging-server",
							Version:     "8.11.3-opr-1",
							Release:     "",
						}),
					},
					{
						Type: property.TypeCSVMetadata,
						Value: mustMarshalJSON(t, property.CSVMetadata{
							Annotations: map[string]string{
								"olm.substitutesFor": "messaging-operator.v8.11.2-opr-1",
							},
						}),
					},
				},
			},
			expectedVersion: "8.11.3-opr-1",
			expectedRelease: "",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vr, err := tt.bundle.VersionRelease()

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, vr)
			assert.Equal(t, tt.expectedVersion, vr.Version.String(), "version mismatch")
			assert.Equal(t, tt.expectedRelease, vr.Release.String(), "release mismatch")
		})
	}
}

// TestBundle_VersionRelease_DuplicatePackageProperty tests that duplicate
// olm.package properties are detected and return an error.
func TestBundle_VersionRelease_DuplicatePackageProperty(t *testing.T) {
	bundle := Bundle{
		Name:    "duplicate-package.v1.0.0",
		Package: "test-package",
		Properties: []property.Property{
			{
				Type: property.TypePackage,
				Value: mustMarshalJSON(t, property.Package{
					PackageName: "test-package",
					Version:     "1.0.0",
				}),
			},
			{
				Type: property.TypePackage,
				Value: mustMarshalJSON(t, property.Package{
					PackageName: "test-package",
					Version:     "1.0.0",
				}),
			},
		},
	}

	vr, err := bundle.VersionRelease()
	require.Error(t, err)
	assert.Nil(t, vr)
	assert.Contains(t, err.Error(), "must be exactly one property of type")
}

// TestBundle_VersionRelease_BuildMetadataConversion tests that build metadata
// is converted to release when substitutesFor annotation is present.
func TestBundle_VersionRelease_BuildMetadataConversion(t *testing.T) {
	tests := []struct {
		name                 string
		bundle               Bundle
		expectedVersion      string
		expectedRelease      string
		expectedBuildCleared bool
	}{
		{
			name: "build metadata converted when substitutesFor present",
			bundle: Bundle{
				Name:    "test-operator.v1.2.3-alpha-1",
				Package: "test-package",
				Properties: []property.Property{
					{
						Type: property.TypePackage,
						Value: mustMarshalJSON(t, property.Package{
							PackageName: "test-package",
							Version:     "1.2.3+alpha.1",
							Release:     "",
						}),
					},
					{
						Type: property.TypeCSVMetadata,
						Value: mustMarshalJSON(t, property.CSVMetadata{
							Annotations: map[string]string{
								"olm.substitutesFor": "test-operator.v1.2.2",
							},
						}),
					},
				},
			},
			expectedVersion:      "1.2.3",
			expectedRelease:      "alpha.1",
			expectedBuildCleared: true,
		},
		{
			name: "build metadata NOT converted when substitutesFor absent",
			bundle: Bundle{
				Name:    "test-operator.v1.2.3",
				Package: "test-package",
				Properties: []property.Property{
					{
						Type: property.TypePackage,
						Value: mustMarshalJSON(t, property.Package{
							PackageName: "test-package",
							Version:     "1.2.3+alpha.1",
							Release:     "",
						}),
					},
					{
						Type: property.TypeCSVMetadata,
						Value: mustMarshalJSON(t, property.CSVMetadata{
							Annotations: map[string]string{
								"capabilities": "Basic Install",
							},
						}),
					},
				},
			},
			expectedVersion:      "1.2.3+alpha.1",
			expectedRelease:      "",
			expectedBuildCleared: false,
		},
		{
			name: "build metadata with multiple segments converted",
			bundle: Bundle{
				Name:    "test-operator.v2.0.0-beta-2-1234567890",
				Package: "test-package",
				Properties: []property.Property{
					{
						Type: property.TypePackage,
						Value: mustMarshalJSON(t, property.Package{
							PackageName: "test-package",
							Version:     "2.0.0+beta.2.1234567890",
							Release:     "",
						}),
					},
					{
						Type: property.TypeBundleObject,
						Value: mustMarshalJSON(t, property.BundleObject{
							Data: []byte(`{
								"apiVersion": "operators.coreos.com/v1alpha1",
								"kind": "ClusterServiceVersion",
								"metadata": {
									"annotations": {
										"olm.substitutesFor": "test-operator.v1.9.9"
									}
								}
							}`),
						}),
					},
				},
			},
			expectedVersion:      "2.0.0",
			expectedRelease:      "beta.2.1234567890",
			expectedBuildCleared: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vr, err := tt.bundle.VersionRelease()

			require.NoError(t, err)
			require.NotNil(t, vr)
			assert.Equal(t, tt.expectedVersion, vr.Version.String(), "version mismatch")
			assert.Equal(t, tt.expectedRelease, vr.Release.String(), "release mismatch")

			if tt.expectedBuildCleared {
				assert.Nil(t, vr.Version.Build, "build metadata should be cleared")
			}
		})
	}
}

// mustMarshalJSON marshals v to JSON and panics on error.
// Used in test data setup to avoid error handling noise.
func mustMarshalJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	return data
}
