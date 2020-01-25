//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
package indexer

import (
	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/lib/registry"

	"github.com/sirupsen/logrus"
)

// IndexAdder allows the creation of index container images from scratch or
// based on previous index images
//counterfeiter:generate . IndexAdder
type IndexAdder interface {
	AddToIndex(AddToIndexRequest) error
}

// NewIndexAdder is a constructor that returns an IndexAdder
func NewIndexAdder(containerTool string, logger *logrus.Entry) IndexAdder {
	return ImageIndexer{
		DockerfileGenerator: containertools.NewDockerfileGenerator(containerTool, logger),
		CommandRunner:       containertools.NewCommandRunner(containerTool, logger),
		LabelReader:         containertools.NewLabelReader(containerTool, logger),
		RegistryAdder:       registry.NewRegistryAdder(logger),
		ImageReader:         containertools.NewImageReader(containerTool, logger),
		ContainerTool:       containerTool,
		Logger:              logger,
	}
}

// IndexDeleter takes indexes and deletes all references to an operator
// from them
//counterfeiter:generate . IndexDeleter
type IndexDeleter interface {
	DeleteFromIndex(DeleteFromIndexRequest) error
}

// NewIndexDeleter is a constructor that returns an IndexDeleter
func NewIndexDeleter(containerTool string, logger *logrus.Entry) IndexDeleter {
	return ImageIndexer{
		DockerfileGenerator: containertools.NewDockerfileGenerator(containerTool, logger),
		CommandRunner:       containertools.NewCommandRunner(containerTool, logger),
		LabelReader:         containertools.NewLabelReader(containerTool, logger),
		RegistryDeleter:     registry.NewRegistryDeleter(logger),
		ImageReader:         containertools.NewImageReader(containerTool, logger),
		ContainerTool:       containerTool,
		Logger:              logger,
	}
}

//counterfeiter:generate . IndexExporter
type IndexExporter interface {
	ExportFromIndex(ExportFromIndexRequest) error
}

// NewIndexExporter is a constructor that returns an IndexExporter
func NewIndexExporter(containerTool string, logger *logrus.Entry) IndexExporter {
	return ImageIndexer{
		DockerfileGenerator: containertools.NewDockerfileGenerator(containerTool, logger),
		CommandRunner:       containertools.NewCommandRunner(containerTool, logger),
		LabelReader:         containertools.NewLabelReader(containerTool, logger),
		ImageReader:         containertools.NewImageReader(containerTool, logger),
		ContainerTool:       containerTool,
		Logger:              logger,
	}
}
