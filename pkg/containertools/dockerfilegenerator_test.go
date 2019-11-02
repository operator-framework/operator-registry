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
	databaseFolder := "database"
	expectedDockerfile := `FROM quay.io/operator-framework/builder AS builder

FROM scratch
LABEL operators.operatorframework.io.index.database.v1=./index.db
COPY database ./
COPY --from=builder /build/bin/opm /opm
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe
EXPOSE 50051
ENTRYPOINT ["/opm"]
CMD ["registry", "serve", "--database", "index.db"]
`

	logger := logrus.NewEntry(logrus.New())

	dockerfileGenerator := containertools.IndexDockerfileGenerator{
		Logger: logger,
	}

	dockerfile := dockerfileGenerator.GenerateIndexDockerfile(binarySourceImage, databaseFolder)
	require.Equal(t, dockerfile, expectedDockerfile)
}

func TestGenerateDockerfile_EmptyBaseImage(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	databaseFolder := "database"
	expectedDockerfile := `FROM quay.io/operator-framework/upstream-registry-builder AS builder

FROM scratch
LABEL operators.operatorframework.io.index.database.v1=./index.db
COPY database ./
COPY --from=builder /build/bin/opm /opm
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe
EXPOSE 50051
ENTRYPOINT ["/opm"]
CMD ["registry", "serve", "--database", "index.db"]
`

	logger := logrus.NewEntry(logrus.New())

	dockerfileGenerator := containertools.IndexDockerfileGenerator{
		Logger: logger,
	}

	dockerfile := dockerfileGenerator.GenerateIndexDockerfile("", databaseFolder)
	require.Equal(t, dockerfile, expectedDockerfile)
}
