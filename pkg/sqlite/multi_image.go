package sqlite

import "github.com/operator-framework/operator-registry/pkg/registry"

// MultiImageLoader loads multiple bundle images into the database.
// It builds a graph between the new bundles and those already present in the database.
type MultiImageLoader struct {
	store         registry.Load
	images        []string
	directory     string
	containerTool string
}

var _ SQLPopulator = &MultiImageLoader{}

func NewSQLLoaderForMultiImage(store registry.Load, bundles []string, containerTool string) *MultiImageLoader {
	return &MultiImageLoader{
		store:         store,
		images:        bundles,
		containerTool: containerTool,
	}
}

func (m *MultiImageLoader) Populate() error {
	return nil
}
