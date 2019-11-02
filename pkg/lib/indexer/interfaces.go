//go:generate counterfeiter indexer.go IndexAdder
//go:generate counterfeiter indexer.go IndexDeleter
package indexer

import (
	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/lib/registry"

	"github.com/sirupsen/logrus"
)

// IndexAdder allows the creation of index container images from scratch or
// based on previous index images
type IndexAdder interface {
	AddToIndex(AddToIndexRequest) error
}

// NewIndexAdder is a constructor that returns an IndexAdder
func NewIndexAdder(containerTool string, logger *logrus.Entry) IndexAdder {
	return ImageIndexer{
		DockerfileGenerator: containertools.NewDockerfileGenerator(containerTool, logger),
		CommandRunner: containertools.NewCommandRunner(containerTool, logger),
		LabelReader: containertools.NewLabelReader(containerTool, logger),
		RegistryAdder: registry.NewRegistryAdder(logger),
		ImageReader: containertools.NewImageReader(containerTool, logger),
		ContainerTool: containerTool,
		Logger: logger,
	}
}

// IndexDeleter takes indexes and deletes all references to an operator
// from them
type IndexDeleter interface {
	DeleteFromIndex(DeleteFromIndexRequest) error
}

// NewIndexDeleter is a constructor that returns an IndexDeleter
func NewIndexDeleter(containerTool string, logger *logrus.Entry) IndexDeleter {
	return ImageIndexer{
		DockerfileGenerator: containertools.NewDockerfileGenerator(containerTool, logger),
		CommandRunner: containertools.NewCommandRunner(containerTool, logger),
		LabelReader: containertools.NewLabelReader(containerTool, logger),
		RegistryDeleter: registry.NewRegistryDeleter(logger),
		ImageReader: containertools.NewImageReader(containerTool, logger),
		ContainerTool: containerTool,
		Logger: logger,
	}
}
