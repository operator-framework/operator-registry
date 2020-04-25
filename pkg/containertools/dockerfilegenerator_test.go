package containertools_test

import (
	"testing"

	"github.com/operator-framework/operator-registry/pkg/containertools"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestGenerateDockerfile(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	binarySourceImage := "quay.io/operator-framework/builder"
	databasePath := "database/index.db"
	expectedDockerfile := `FROM quay.io/operator-framework/builder
LABEL operators.operatorframework.io.index.database.v1=/database/index.db
ADD database/index.db /database/index.db
EXPOSE 50051
ENTRYPOINT ["/bin/opm"]
CMD ["registry", "serve", "--database", "/database/index.db"]
`

	logger := logrus.NewEntry(logrus.New())

	dockerfileGenerator := containertools.IndexDockerfileGenerator{
		Logger: logger,
	}

	dockerfile := dockerfileGenerator.GenerateIndexDockerfile(binarySourceImage, databasePath)
	require.Equal(t, dockerfile, expectedDockerfile)
}

func TestGenerateDockerfile_EmptyBaseImage(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	databasePath := "database/index.db"
	expectedDockerfile := `FROM quay.io/operator-framework/upstream-opm-builder
LABEL operators.operatorframework.io.index.database.v1=/database/index.db
ADD database/index.db /database/index.db
EXPOSE 50051
ENTRYPOINT ["/bin/opm"]
CMD ["registry", "serve", "--database", "/database/index.db"]
`

	logger := logrus.NewEntry(logrus.New())

	dockerfileGenerator := containertools.IndexDockerfileGenerator{
		Logger: logger,
	}

	dockerfile := dockerfileGenerator.GenerateIndexDockerfile("", databasePath)
	require.Equal(t, dockerfile, expectedDockerfile)
}
