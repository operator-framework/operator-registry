package template

import (
	"context"
	"io"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

// BundleRenderer defines the function signature for rendering bundle images
type BundleRenderer func(context.Context, string) (*declcfg.DeclarativeConfig, error)

// Template defines the common interface for all template types
type Template interface {
	// RenderBundle renders a bundle image reference into a DeclarativeConfig
	RenderBundle(ctx context.Context, image string) (*declcfg.DeclarativeConfig, error)
	// Render processes the template input and returns a DeclarativeConfig
	Render(ctx context.Context, reader io.Reader) (*declcfg.DeclarativeConfig, error)
	// Schema returns the schema identifier for this template type
	Schema() string
}

// TemplateFactory creates template instances based on schema
type TemplateFactory interface {
	// CreateTemplate creates a new template instance with the given RenderBundle function
	CreateTemplate(renderBundle BundleRenderer) Template
	// Schema returns the schema identifier this factory handles
	Schema() string
}

// Registry maintains a mapping of schema identifiers to template factories
type Registry struct {
	factories map[string]TemplateFactory
}

// NewRegistry creates a new template registry
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]TemplateFactory),
	}
}

// Register adds a template factory to the registry
func (r *Registry) Register(factory TemplateFactory) {
	r.factories[factory.Schema()] = factory
}

// CreateTemplate creates a template instance based on the schema found in the input
func (r *Registry) CreateTemplate(reader io.Reader, renderBundle BundleRenderer) (Template, error) {
	schema, err := detectSchema(reader)
	if err != nil {
		return nil, err
	}

	factory, exists := r.factories[schema]
	if !exists {
		return nil, &UnknownSchemaError{Schema: schema}
	}

	return factory.CreateTemplate(renderBundle), nil
}

// GetSupportedSchemas returns all supported schema identifiers
func (r *Registry) GetSupportedSchemas() []string {
	schemas := make([]string, 0, len(r.factories))
	for schema := range r.factories {
		schemas = append(schemas, schema)
	}
	return schemas
}

// UnknownSchemaError is returned when a schema is not recognized
type UnknownSchemaError struct {
	Schema string
}

func (e *UnknownSchemaError) Error() string {
	return "unknown template schema: " + e.Schema
}
