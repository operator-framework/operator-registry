package template

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

// mockTemplate is a test implementation of the Template interface
type mockTemplate struct {
	schema       string
	renderBundle BundleRenderer
}

func (m *mockTemplate) RenderBundle(ctx context.Context, image string) (*declcfg.DeclarativeConfig, error) {
	if m.renderBundle != nil {
		return m.renderBundle(ctx, image)
	}
	return &declcfg.DeclarativeConfig{}, nil
}

func (m *mockTemplate) Render(ctx context.Context, reader io.Reader) (*declcfg.DeclarativeConfig, error) {
	return &declcfg.DeclarativeConfig{}, nil
}

func (m *mockTemplate) Schema() string {
	return m.schema
}

// mockFactory is a test implementation of the TemplateFactory interface
type mockFactory struct {
	schema string
}

func (f *mockFactory) CreateTemplate(renderBundle BundleRenderer) Template {
	return &mockTemplate{
		schema:       f.schema,
		renderBundle: renderBundle,
	}
}

func (f *mockFactory) Schema() string {
	return f.schema
}

func TestNewTemplateRegistry(t *testing.T) {
	registry := NewTemplateRegistry()

	require.NotNil(t, registry)
	require.NotNil(t, registry.factories)
	require.Empty(t, registry.factories)
}

func TestTemplateRegistry_Register(t *testing.T) {
	tests := []struct {
		name      string
		factories []Factory
		expected  []string
	}{
		{
			name:      "register single factory",
			factories: []Factory{&mockFactory{schema: "olm.semver"}},
			expected:  []string{"olm.semver"},
		},
		{
			name: "register multiple factories",
			factories: []Factory{
				&mockFactory{schema: "olm.semver"},
				&mockFactory{schema: "olm.basic"},
				&mockFactory{schema: "olm.composite"},
			},
			expected: []string{"olm.basic", "olm.composite", "olm.semver"},
		},
		{
			name: "register duplicate schema overwrites",
			factories: []Factory{
				&mockFactory{schema: "olm.semver"},
				&mockFactory{schema: "olm.semver"},
			},
			expected: []string{"olm.semver"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewTemplateRegistry()
			for _, factory := range tt.factories {
				registry.Register(factory)
			}

			schemas := registry.GetSupportedSchemas()
			require.Equal(t, tt.expected, schemas)
		})
	}
}

func TestTemplateRegistry_CreateTemplateByType(t *testing.T) {
	tests := []struct {
		name         string
		setupSchemas []string
		requestType  string
		expectError  bool
		expectedErr  string
	}{
		{
			name:         "create template for registered type",
			setupSchemas: []string{"olm.semver"},
			requestType:  "olm.semver",
			expectError:  false,
		},
		{
			name:         "create template for multiple registered types",
			setupSchemas: []string{"olm.semver", "olm.basic", "olm.composite"},
			requestType:  "olm.basic",
			expectError:  false,
		},
		{
			name:         "error on unregistered type",
			setupSchemas: []string{"olm.semver"},
			requestType:  "olm.unknown",
			expectError:  true,
			expectedErr:  "unknown template schema: olm.unknown",
		},
		{
			name:         "error on empty registry",
			setupSchemas: []string{},
			requestType:  "olm.semver",
			expectError:  true,
			expectedErr:  "unknown template schema: olm.semver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewTemplateRegistry()
			for _, schema := range tt.setupSchemas {
				registry.Register(&mockFactory{schema: schema})
			}

			template, err := registry.CreateTemplateByType(tt.requestType, nil)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, template)
				require.Contains(t, err.Error(), tt.expectedErr)

				var unknownSchemaErr *UnknownSchemaError
				require.ErrorAs(t, err, &unknownSchemaErr)
				require.Equal(t, tt.requestType, unknownSchemaErr.Schema)
			} else {
				require.NoError(t, err)
				require.NotNil(t, template)
				require.Equal(t, tt.requestType, template.Schema())
			}
		})
	}
}

func TestTemplateRegistry_CreateTemplateBySchema(t *testing.T) {
	tests := []struct {
		name           string
		setupSchemas   []string
		input          string
		expectError    bool
		expectedErr    string
		expectedSchema string
	}{
		{
			name:         "create template from valid YAML input",
			setupSchemas: []string{"olm.semver"},
			input: `schema: olm.semver
bundles:
  - image: example.com/bundle:v1.0.0`,
			expectError:    false,
			expectedSchema: "olm.semver",
		},
		{
			name:           "create template from valid JSON input",
			setupSchemas:   []string{"olm.semver"},
			input:          `{"schema": "olm.semver", "bundles": [{"image": "example.com/bundle:v1.0.0"}]}`,
			expectError:    false,
			expectedSchema: "olm.semver",
		},
		{
			name:         "error on unregistered schema",
			setupSchemas: []string{"olm.semver"},
			input:        `schema: olm.unknown`,
			expectError:  true,
			expectedErr:  "unknown template schema: olm.unknown",
		},
		{
			name:         "error on missing schema field",
			setupSchemas: []string{"olm.semver"},
			input:        `bundles: []`,
			expectError:  true,
			expectedErr:  "missing required 'schema' field",
		},
		{
			name:         "error on invalid YAML",
			setupSchemas: []string{"olm.semver"},
			input:        `schema: olm.semver\n\tinvalid: [unclosed`,
			expectError:  true,
			expectedErr:  "decoding template input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewTemplateRegistry()
			for _, schema := range tt.setupSchemas {
				registry.Register(&mockFactory{schema: schema})
			}

			reader := strings.NewReader(tt.input)
			template, replayReader, err := registry.CreateTemplateBySchema(reader, nil)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, template)
				require.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, template)
				require.NotNil(t, replayReader)
				require.Equal(t, tt.expectedSchema, template.Schema())

				// Verify replay reader contains original input
				replayedData, err := io.ReadAll(replayReader)
				require.NoError(t, err)
				require.Equal(t, tt.input, string(replayedData))
			}
		})
	}
}

func TestTemplateRegistry_GetSupportedSchemas(t *testing.T) {
	tests := []struct {
		name     string
		schemas  []string
		expected []string
	}{
		{
			name:     "empty registry",
			schemas:  []string{},
			expected: []string{},
		},
		{
			name:     "single schema",
			schemas:  []string{"olm.semver"},
			expected: []string{"olm.semver"},
		},
		{
			name:     "multiple schemas sorted alphabetically",
			schemas:  []string{"olm.semver", "olm.basic", "olm.composite"},
			expected: []string{"olm.basic", "olm.composite", "olm.semver"},
		},
		{
			name:     "schemas with different formats",
			schemas:  []string{"olm.template.semver", "olm.template.basic", "custom.schema"},
			expected: []string{"custom.schema", "olm.template.basic", "olm.template.semver"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewTemplateRegistry()
			for _, schema := range tt.schemas {
				registry.Register(&mockFactory{schema: schema})
			}

			schemas := registry.GetSupportedSchemas()
			require.Equal(t, tt.expected, schemas)
		})
	}
}

func TestTemplateRegistry_GetSupportedTypes(t *testing.T) {
	tests := []struct {
		name     string
		schemas  []string
		expected []string
	}{
		{
			name:     "empty registry",
			schemas:  []string{},
			expected: []string{},
		},
		{
			name:     "single schema extracts type",
			schemas:  []string{"olm.semver"},
			expected: []string{"semver"},
		},
		{
			name:     "multiple schemas extract types sorted",
			schemas:  []string{"olm.semver", "olm.basic", "olm.composite"},
			expected: []string{"basic", "composite", "semver"},
		},
		{
			name:     "schema without dots returns entire string",
			schemas:  []string{"simple"},
			expected: []string{"simple"},
		},
		{
			name:     "nested schema extracts last part",
			schemas:  []string{"olm.template.semver", "olm.template.basic"},
			expected: []string{"basic", "semver"},
		},
		{
			name:     "duplicate types from different schemas",
			schemas:  []string{"olm.semver", "custom.semver"},
			expected: []string{"semver", "semver"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewTemplateRegistry()
			for _, schema := range tt.schemas {
				registry.Register(&mockFactory{schema: schema})
			}

			types := registry.GetSupportedTypes()
			require.Equal(t, tt.expected, types)
		})
	}
}

func TestTemplateRegistry_HasSchema(t *testing.T) {
	tests := []struct {
		name              string
		registeredSchemas []string
		checkSchema       string
		expected          bool
	}{
		{
			name:              "schema exists",
			registeredSchemas: []string{"olm.semver"},
			checkSchema:       "olm.semver",
			expected:          true,
		},
		{
			name:              "schema does not exist",
			registeredSchemas: []string{"olm.semver"},
			checkSchema:       "olm.basic",
			expected:          false,
		},
		{
			name:              "empty registry",
			registeredSchemas: []string{},
			checkSchema:       "olm.semver",
			expected:          false,
		},
		{
			name:              "multiple schemas, check existing",
			registeredSchemas: []string{"olm.semver", "olm.basic", "olm.composite"},
			checkSchema:       "olm.basic",
			expected:          true,
		},
		{
			name:              "multiple schemas, check non-existing",
			registeredSchemas: []string{"olm.semver", "olm.basic"},
			checkSchema:       "olm.unknown",
			expected:          false,
		},
		{
			name:              "case sensitive check",
			registeredSchemas: []string{"olm.semver"},
			checkSchema:       "olm.Semver",
			expected:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewTemplateRegistry()
			for _, schema := range tt.registeredSchemas {
				registry.Register(&mockFactory{schema: schema})
			}

			result := registry.HasSchema(tt.checkSchema)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestUnknownSchemaError_Error(t *testing.T) {
	tests := []struct {
		name     string
		schema   string
		expected string
	}{
		{
			name:     "simple schema name",
			schema:   "olm.semver",
			expected: "unknown template schema: olm.semver",
		},
		{
			name:     "complex schema name",
			schema:   "custom.template.v1.0",
			expected: "unknown template schema: custom.template.v1.0",
		},
		{
			name:     "empty schema name",
			schema:   "",
			expected: "unknown template schema: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &UnknownSchemaError{Schema: tt.schema}
			require.Equal(t, tt.expected, err.Error())
		})
	}
}

// capturingRenderBundle creates a BundleRenderer that captures the image parameter
func capturingRenderBundle(captured *string) BundleRenderer {
	return func(_ context.Context, image string) (*declcfg.DeclarativeConfig, error) {
		*captured = image
		return &declcfg.DeclarativeConfig{}, nil
	}
}

func TestTemplateRegistry_RenderBundlePropagation(t *testing.T) {
	expectedImage := "example.com/bundle:v1.0.0"
	var capturedImage string

	mockRenderBundle := capturingRenderBundle(&capturedImage)

	tests := []struct {
		name   string
		method string
	}{
		{
			name:   "CreateTemplateByType propagates RenderBundle",
			method: "byType",
		},
		{
			name:   "CreateTemplateBySchema propagates RenderBundle",
			method: "bySchema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewTemplateRegistry()
			registry.Register(&mockFactory{schema: "olm.semver"})

			var template Template
			var err error

			if tt.method == "byType" {
				template, err = registry.CreateTemplateByType("olm.semver", mockRenderBundle)
				require.NoError(t, err)
			} else {
				input := `schema: olm.semver`
				reader := strings.NewReader(input)
				template, _, err = registry.CreateTemplateBySchema(reader, mockRenderBundle)
				require.NoError(t, err)
			}

			ctx := context.Background()
			_, err = template.RenderBundle(ctx, expectedImage)
			require.NoError(t, err)
			require.Equal(t, expectedImage, capturedImage)
		})
	}
}
