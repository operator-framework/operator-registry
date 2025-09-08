package template

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/operator-framework/operator-registry/alpha/template/api"
	"github.com/operator-framework/operator-registry/alpha/template/basic"
	"github.com/operator-framework/operator-registry/alpha/template/semver"
	"github.com/operator-framework/operator-registry/alpha/template/substitutes"
)

// Re-export api types for backward compatibility
type (
	BundleRenderer = api.BundleRenderer
	Template       = api.Template
	Factory        = api.Factory
)

type Registry interface {
	Register(factory Factory)
	GetSupportedTypes() []string
	HasType(templateType string) bool
	HasSchema(schema string) bool
	CreateTemplateBySchema(reader io.Reader, renderBundle BundleRenderer) (Template, io.Reader, error)
	CreateTemplateByType(templateType string, renderBundle BundleRenderer) (Template, error)
	GetSupportedSchemas() []string
	HelpText() string
}

// registry maintains a mapping of schema identifiers to template factories
type registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates a new registry and registers all built-in template factories.
func NewRegistry() Registry {
	r := &registry{
		factories: make(map[string]Factory),
	}
	r.Register(&basic.Factory{})
	r.Register(&semver.Factory{})
	r.Register(&substitutes.Factory{})
	return r
}

func (r *registry) HelpText() string {
	var help strings.Builder
	supportedTypes := r.GetSupportedTypes()
	help.WriteString("\n")
	tabber := tabwriter.NewWriter(&help, 0, 0, 1, ' ', 0)
	for _, item := range supportedTypes {
		fmt.Fprintf(tabber, " - %s\n", item)
	}
	tabber.Flush()
	return help.String()
}

// Register adds a template factory to the registry
func (r *registry) Register(factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[factory.Schema()] = factory
}

// CreateTemplateBySchema creates a template instance based on the schema found in the input
// and returns a reader that can be used to render the template. The returned reader includes
// both the data consumed during schema detection and the remaining unconsumed data.
func (r *registry) CreateTemplateBySchema(reader io.Reader, renderBundle BundleRenderer) (Template, io.Reader, error) {
	schema, replayReader, err := detectSchema(reader)
	if err != nil {
		return nil, nil, err
	}

	r.mu.RLock()
	factory, exists := r.factories[schema]
	defer r.mu.RUnlock()
	if !exists {
		return nil, nil, &UnknownSchemaError{Schema: schema}
	}

	return factory.CreateTemplate(renderBundle), replayReader, nil
}

func (r *registry) CreateTemplateByType(templateType string, renderBundle BundleRenderer) (Template, error) {
	r.mu.RLock()
	factory, exists := r.factories[templateType]
	defer r.mu.RUnlock()
	if !exists {
		return nil, &UnknownSchemaError{Schema: templateType}
	}

	return factory.CreateTemplate(renderBundle), nil
}

// GetSupportedSchemas returns all supported schema identifiers
func (r *registry) GetSupportedSchemas() []string {
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
func (r *registry) GetSupportedTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	types := make([]string, 0, len(r.factories))
	for schema := range r.factories {
		types = append(types, schema[strings.LastIndex(schema, ".")+1:])
	}
	slices.Sort(types)
	return types
}

func (r *registry) HasSchema(schema string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.factories[schema]
	return exists
}

func (r *registry) HasType(templateType string) bool {
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
