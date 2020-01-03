package bundle

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestGetMediaType(t *testing.T) {
	tests := []struct {
		directory string
		mediaType string
		errorMsg  string
	}{
		{
			"./testdata/get_mediatype/registry_v1_bundle",
			RegistryV1Type,
			"",
		},
		{
			"./testdata/get_mediatype/helm_bundle",
			HelmType,
			"",
		},
		{
			"./testdata/get_mediatype/plain_bundle",
			PlainType,
			"",
		},
		{
			"./testdata/get_mediatype/empty_bundle",
			"",
			fmt.Sprintf("The directory contains no files"),
		},
	}

	for _, item := range tests {
		manifestType, err := GetMediaType(item.directory)
		if item.errorMsg == "" {
			require.Equal(t, item.mediaType, manifestType)
		} else {
			require.Error(t, err)
		}
	}
}

func TestValidateChannelDefault(t *testing.T) {
	tests := []struct {
		channels       string
		channelDefault string
		result         string
		errorMsg       string
	}{
		{
			"test5,test6",
			"",
			"test5",
			"",
		},
		{
			"test5,test6",
			"test7",
			"test5",
			`The channel list "test5,test6" doesn't contain channelDefault "test7"`,
		},
		{
			",",
			"",
			"",
			`Invalid channels is provied: ,`,
		},
	}

	for _, item := range tests {
		output, err := ValidateChannelDefault(item.channels, item.channelDefault)
		if item.errorMsg == "" {
			require.Equal(t, item.result, output)
		} else {
			require.Equal(t, item.errorMsg, err.Error())
		}
	}
}

func TestValidateAnnotations(t *testing.T) {
	tests := []struct {
		existing []byte
		expected []byte
		err      error
	}{
		{
			buildTestAnnotations("annotations",
				map[string]string{
					"test1": "stable",
					"test2": "stable,beta",
				}),
			buildTestAnnotations("annotations",
				map[string]string{
					"test1": "stable",
					"test2": "stable,beta",
				}),
			nil,
		},
		{
			buildTestAnnotations("annotations",
				map[string]string{
					"test1": "stable",
					"test2": "stable,beta",
					"test3": "beta",
				}),
			buildTestAnnotations("annotations",
				map[string]string{
					"test1": "stable",
					"test2": "stable,beta",
				}),
			fmt.Errorf("Unmatched number of fields. Expected (2) vs existing (3)"),
		},
		{
			buildTestAnnotations("annotations",
				map[string]string{
					"test1": "stable",
					"test2": "stable",
				}),
			buildTestAnnotations("annotations",
				map[string]string{
					"test1": "stable",
					"test2": "stable,beta",
				}),
			fmt.Errorf(`Expect field "test2" to have value "stable,beta" instead of "stable"`),
		},
		{
			buildTestAnnotations("annotations",
				map[string]string{
					"test1": "stable",
					"test3": "stable",
				}),
			buildTestAnnotations("annotations",
				map[string]string{
					"test1": "stable",
					"test2": "stable,beta",
				}),
			fmt.Errorf("Missing field: test2"),
		},
		{
			[]byte("\t"),
			buildTestAnnotations("annotations",
				map[string]string{
					"test1": "stable",
					"test2": "stable,beta",
				}),
			fmt.Errorf("yaml: found character that cannot start any token"),
		},
		{
			buildTestAnnotations("annotations",
				map[string]string{
					"test1": "stable",
					"test2": "stable,beta",
				}),
			[]byte("\t"),
			fmt.Errorf("yaml: found character that cannot start any token"),
		},
	}

	for _, item := range tests {
		err := ValidateAnnotations(item.existing, item.expected)
		if item.err != nil {
			require.Equal(t, item.err.Error(), err.Error())
		} else {
			require.Nil(t, err)
		}
	}
}

func TestGenerateAnnotationsFunc(t *testing.T) {
	// Create test annotations struct
	testAnnotations := &AnnotationMetadata{
		Annotations: map[string]string{
			MediatypeLabel:      "test1",
			ManifestsLabel:      "test2",
			MetadataLabel:       "test3",
			PackageLabel:        "test4",
			ChannelsLabel:       "test5",
			ChannelDefaultLabel: "test5",
		},
	}
	// Create result annotations struct
	resultAnnotations := AnnotationMetadata{}
	data, err := GenerateAnnotations("test1", "test2", "test3", "test4", "test5", "test5")
	require.NoError(t, err)

	err = yaml.Unmarshal(data, &resultAnnotations)
	require.NoError(t, err)

	for key, value := range testAnnotations.Annotations {
		require.Equal(t, value, resultAnnotations.Annotations[key])
	}
}

func TestGenerateDockerfileFunc(t *testing.T) {
	output := fmt.Sprintf("FROM scratch\n\n"+
		"LABEL operators.operatorframework.io.bundle.mediatype.v1=test1\n"+
		"LABEL operators.operatorframework.io.bundle.manifests.v1=test2\n"+
		"LABEL operators.operatorframework.io.bundle.metadata.v1=%s\n"+
		"LABEL operators.operatorframework.io.bundle.package.v1=test4\n"+
		"LABEL operators.operatorframework.io.bundle.channels.v1=test5\n"+
		"LABEL operators.operatorframework.io.bundle.channel.default.v1=test5\n\n"+
		"COPY /*.yaml /manifests/\n"+
		"COPY %s/annotations.yaml /metadata/annotations.yaml\n", MetadataDir,
		filepath.Join("/", MetadataDir))

	content, err := GenerateDockerfile("test1", "test2", MetadataDir, "test4", "test5", "")
	require.NoError(t, err)
	require.Equal(t, output, string(content))
}
