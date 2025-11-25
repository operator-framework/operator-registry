package template

import (
	"context"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

// BundleRenderer defines the function signature for rendering a string containing a bundle image/path/file into a DeclarativeConfig fragment
// It's provided as a discrete type to allow for easy mocking in tests as well as facilitating variable
// restrictions on reference types
type BundleRenderer func(context.Context, string) (*declcfg.DeclarativeConfig, error)

// Template defines the common interface for all template types
type Template interface {
	// RenderBundle renders a bundle image reference into a DeclarativeConfig fragment.
	// This function is used to render a single bundle image reference by a template instance,
	// and is provided to the template on construction.
	// This is typically used in the call to Render the template to DeclarativeConfig, and
	// needs to be configurable to handle different bundle image formats and configurations.
	RenderBundle(ctx context.Context, imageRef string) (*declcfg.DeclarativeConfig, error)
	// Render processes the raw template yaml/json input and returns an expanded DeclarativeConfig
	// in the case where expansion fails, it returns an error
	Render(ctx context.Context, reader io.Reader) (*declcfg.DeclarativeConfig, error)
	// Schema returns the schema identifier for this template type
	Schema() string
}

// Factory creates template instances based on schema
type Factory interface {
	// CreateTemplate creates a new template instance with the given RenderBundle function
	CreateTemplate(renderBundle BundleRenderer) Template
	// Schema returns the schema identifier this factory handles
	Schema() string
}

// templateRegistry maintains a mapping of schema identifiers to template factories
type templateRegistry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewTemplateRegistry creates a new template registry
func NewTemplateRegistry() *templateRegistry {
	return &templateRegistry{
		factories: make(map[string]Factory),
	}
}

// Register adds a template factory to the registry
func (r *templateRegistry) Register(factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[factory.Schema()] = factory
}

// CreateTemplateBySchema creates a template instance based on the schema found in the input
// and returns a reader that can be used to render the template. The returned reader includes
// both the data consumed during schema detection and the remaining unconsumed data.
func (r *templateRegistry) CreateTemplateBySchema(reader io.Reader, renderBundle BundleRenderer) (Template, io.Reader, error) {
	schema, replayReader, err := detectSchema(reader)
	if err != nil {
		return nil, nil, err
	}

	r.mu.RLock()
	factory, exists := r.factories[schema]
	r.mu.RUnlock()
	if !exists {
		return nil, nil, &UnknownSchemaError{Schema: schema}
	}

	return factory.CreateTemplate(renderBundle), replayReader, nil
}

func (r *templateRegistry) CreateTemplateByType(templateType string, renderBundle BundleRenderer) (Template, error) {
	r.mu.RLock()
	factory, exists := r.factories[templateType]
	r.mu.RUnlock()
	if !exists {
		return nil, &UnknownSchemaError{Schema: templateType}
	}

	return factory.CreateTemplate(renderBundle), nil
}

// GetSupportedSchemas returns all supported schema identifiers
func (r *templateRegistry) GetSupportedSchemas() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
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
func (r *templateRegistry) GetSupportedTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	types := make([]string, 0, len(r.factories))
	for schema := range r.factories {
		types = append(types, schema[strings.LastIndex(schema, ".")+1:])
	}
	slices.Sort(types)
	return types
}

func (r *templateRegistry) HasSchema(schema string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.factories[schema]
	return exists
}

func (r *templateRegistry) HasType(templateType string) bool {
	types := r.GetSupportedTypes()
	return slices.Contains(types, templateType)
}

// UnknownSchemaError is returned when a schema is not recognized
type UnknownSchemaError struct {
	Schema string
}

func (e *UnknownSchemaError) Error() string {
	return "unknown template schema: " + e.Schema
}
