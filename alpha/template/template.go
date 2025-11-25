package template

import (
	"context"
	"io"
	"slices"
	"strings"

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

// TemplateRegistry maintains a mapping of schema identifiers to template factories
type TemplateRegistry struct {
	factories map[string]TemplateFactory
}

// NewTemplateRegistry creates a new template registry
func NewTemplateRegistry() *TemplateRegistry {
	return &TemplateRegistry{
		factories: make(map[string]TemplateFactory),
	}
}

// Register adds a template factory to the registry
func (r *TemplateRegistry) Register(factory TemplateFactory) {
	r.factories[factory.Schema()] = factory
}

// CreateTemplateBySchema creates a template instance based on the schema found in the input
// and returns a reader that can be used to render the template. The returned reader includes
// both the data consumed during schema detection and the remaining unconsumed data.
func (r *TemplateRegistry) CreateTemplateBySchema(reader io.Reader, renderBundle BundleRenderer) (Template, io.Reader, error) {
	schema, replayReader, err := detectSchema(reader)
	if err != nil {
		return nil, nil, err
	}

	factory, exists := r.factories[schema]
	if !exists {
		return nil, nil, &UnknownSchemaError{Schema: schema}
	}

	return factory.CreateTemplate(renderBundle), replayReader, nil
}

func (r *TemplateRegistry) CreateTemplateByType(templateType string, renderBundle BundleRenderer) (Template, error) {
	factory, exists := r.factories[templateType]
	if !exists {
		return nil, &UnknownSchemaError{Schema: templateType}
	}

	return factory.CreateTemplate(renderBundle), nil
}

// GetSupportedSchemas returns all supported schema identifiers
func (r *TemplateRegistry) GetSupportedSchemas() []string {
	schemas := make([]string, 0, len(r.factories))
	for schema := range r.factories {
		schemas = append(schemas, schema)
	}
	slices.Sort(schemas)
	return schemas
}

// GetSupportedTypes returns all supported template types
// TODO: in future, might store the type separately from the schema
// right now it's just the last part of the schema string
func (r *TemplateRegistry) GetSupportedTypes() []string {
	types := make([]string, 0, len(r.factories))
	for schema := range r.factories {
		types = append(types, schema[strings.LastIndex(schema, ".")+1:])
	}
	slices.Sort(types)
	return types
}

func (r *TemplateRegistry) HasSchema(schema string) bool {
	_, exists := r.factories[schema]
	return exists
}

// UnknownSchemaError is returned when a schema is not recognized
type UnknownSchemaError struct {
	Schema string
}

func (e *UnknownSchemaError) Error() string {
	return "unknown template schema: " + e.Schema
}
