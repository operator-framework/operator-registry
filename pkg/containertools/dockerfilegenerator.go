//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . DockerfileGenerator
package containertools

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/operator-framework/operator-registry/pkg/version"
)

const (
	defaultBinarySourceImage = "quay.io/operator-framework/upstream-opm-builder"
	DefaultDbLocation        = "/database/index.db"
	DbLocationLabel          = "operators.operatorframework.io.index.database.v1"
)

// DockerfileGenerator defines functions to generate index dockerfiles
type DockerfileGenerator interface {
	GenerateIndexDockerfile(string, string) string
}

// IndexDockerfileGenerator struct implementation of DockerfileGenerator interface
type IndexDockerfileGenerator struct {
	Logger *logrus.Entry
}

// NewDockerfileGenerator is a constructor that returns a DockerfileGenerator
func NewDockerfileGenerator(logger *logrus.Entry) DockerfileGenerator {
	return &IndexDockerfileGenerator{
		Logger: logger,
	}
}

// GenerateIndexDockerfile builds a string representation of a dockerfile to use when building
// an operator-registry index image
func (g *IndexDockerfileGenerator) GenerateIndexDockerfile(binarySourceImage, databasePath string) string {
	var dockerfile string

	g.Logger.Info("Generating dockerfile")

	if binarySourceImage == "" {
		imageTag := "latest"
		if version.GitCommit != "" {
			imageTag = version.GitCommit
		}

		binarySourceImage = fmt.Sprintf("%s:%s", defaultBinarySourceImage, imageTag)
	}

	// From
	dockerfile += fmt.Sprintf("FROM %s\n", binarySourceImage)

	// Labels
	dockerfile += fmt.Sprintf("LABEL %s=%s\n", DbLocationLabel, DefaultDbLocation)

	// Content
	dockerfile += fmt.Sprintf("ADD %s %s\n", databasePath, DefaultDbLocation)
	dockerfile += fmt.Sprintf("EXPOSE 50051\n")
	dockerfile += fmt.Sprintf("ENTRYPOINT [\"/bin/opm\"]\n")
	dockerfile += fmt.Sprintf("CMD [\"registry\", \"serve\", \"--database\", \"%s\"]\n", DefaultDbLocation)

	return dockerfile
}
