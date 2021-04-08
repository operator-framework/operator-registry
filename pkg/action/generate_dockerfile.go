package action

import (
	"os"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/sirupsen/logrus"
)

type generator struct {
	logger *logrus.Entry
}

type GenerateDockerfileRequest struct {
	SourceImage string
	ConfigsDir  string
	File        *os.File
}

func NewGenerator(logger *logrus.Entry) generator {
	return generator{
		logger: logger,
	}
}

func (g generator) GenerateDockerfile(request GenerateDockerfileRequest) error {
	generator := containertools.NewConfigDockerFileGenerator(g.logger)
	content := generator.GenerateIndexDockerfile(request.SourceImage, request.ConfigsDir)

	_, err := request.File.WriteString(content)
	if err != nil {
		return err
	}
	return nil
}
