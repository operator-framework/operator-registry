package api

import (
	"context"
	"io"

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
