package action

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateDockerfile(t *testing.T) {
	type spec struct {
		name               string
		gen                GenerateDockerfile
		expectedDockerfile string
		expectedErr        string
	}

	specs := []spec{
		{
			name: "Fail/EmptyBaseImage",
			gen: GenerateDockerfile{
				IndexDir: "bar",
				ExtraLabels: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expectedErr: "base image is unset",
		},
		{
			name: "Fail/EmptyFromDir",
			gen: GenerateDockerfile{
				BaseImage: "foo",
				ExtraLabels: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expectedErr: "index directory is unset",
		},
		{
			name: "Success/WithoutExtraLabels",
			gen: GenerateDockerfile{
				BaseImage: "foo",
				IndexDir:  "bar",
			},
			expectedDockerfile: `# The base image is expected to contain
# /bin/opm (with a serve subcommand) and /bin/grpc_health_probe
FROM foo

# Configure the entrypoint and command
ENTRYPOINT ["/bin/opm"]
CMD ["serve", "/configs", "--cache-dir=/tmp/cache"]

# Copy declarative config root into image at /configs and pre-populate serve cache
ADD bar /configs
RUN ["/bin/opm", "serve", "/configs", "--cache-dir=/tmp/cache", "--cache-only"]

# Set DC-specific label for the location of the DC root directory
# in the image
LABEL operators.operatorframework.io.index.configs.v1=/configs
`,
		},
		{
			name: "Success/WithExtraLabels",
			gen: GenerateDockerfile{
				BaseImage: "foo",
				IndexDir:  "bar",
				ExtraLabels: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expectedDockerfile: `# The base image is expected to contain
# /bin/opm (with a serve subcommand) and /bin/grpc_health_probe
FROM foo

# Configure the entrypoint and command
ENTRYPOINT ["/bin/opm"]
CMD ["serve", "/configs", "--cache-dir=/tmp/cache"]

# Copy declarative config root into image at /configs and pre-populate serve cache
ADD bar /configs
RUN ["/bin/opm", "serve", "/configs", "--cache-dir=/tmp/cache", "--cache-only"]

# Set DC-specific label for the location of the DC root directory
# in the image
LABEL operators.operatorframework.io.index.configs.v1=/configs

# Set other custom labels
LABEL "key1"="value1"
LABEL "key2"="value2"
`,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			buf := bytes.Buffer{}
			s.gen.Writer = &buf
			err := s.gen.Run()
			if s.expectedErr != "" {
				require.EqualError(t, err, s.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, s.expectedDockerfile, buf.String())
			}
		})
	}
}
