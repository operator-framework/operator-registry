package bundle

import (
	"fmt"
	"io/ioutil"
	"os"
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
			nil,
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

func TestGenerateDockerfile(t *testing.T) {
	expected := `FROM scratch

LABEL operators.operatorframework.io.bundle.mediatype.v1=test1
LABEL operators.operatorframework.io.bundle.manifests.v1=test2
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=test4
LABEL operators.operatorframework.io.bundle.channels.v1=test5
COPY a/b/c /manifests/
COPY x/y/z /metadata/
`

	actual, err := GenerateDockerfile("test1", "test2", "metadata/", filepath.Join("a", "b", "c"), filepath.Join("x", "y", "z"), "./", "test4", "test5", "")
	require.NoError(t, err)
	require.Equal(t, expected, string(actual))
}

func TestCopyYamlOutput(t *testing.T) {
	testOutputDir, _ := ioutil.TempDir("./", "test-generate")
	defer os.RemoveAll(testOutputDir)

	testContent := []byte{0, 1, 0, 0}
	testManifestDir := "./testdata/generate/manifests"
	testWorkingDir := "./"
	testOverwrite := true

	resultManifestDir, resultMetadataDir, err := CopyYamlOutput(testContent, testManifestDir, testOutputDir, testWorkingDir, testOverwrite)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(testOutputDir, "manifests/"), resultManifestDir)
	require.Equal(t, filepath.Join(testOutputDir, "metadata/"), resultMetadataDir)

	outputAnnotationsFile := filepath.Join(testOutputDir, "metadata/", "annotations.yaml")
	annotationsBlob, err := ioutil.ReadFile(outputAnnotationsFile)
	require.NoError(t, err)
	require.Equal(t, testContent, annotationsBlob)

	csvFile := filepath.Join(testOutputDir, "manifests/", "prometheusoperator.0.14.0.clusterserviceversion.yaml")
	_, err = ioutil.ReadFile(csvFile)
	require.NoError(t, err)
}

func TestCopyYamlOutput_NoOutputDir(t *testing.T) {
	testContent := []byte{0, 1, 0, 0}
	testManifestDir := "./testdata/generate/manifests"
	testWorkingDir := "./"
	testOverwrite := true

	resultManifestDir, resultMetadataDir, err := CopyYamlOutput(testContent, testManifestDir, "", testWorkingDir, testOverwrite)
	require.NoError(t, err)
	require.Equal(t, testManifestDir, resultManifestDir)
	require.Equal(t, filepath.Join(filepath.Dir(testManifestDir), "metadata/"), resultMetadataDir)

	outputAnnotationsFile := filepath.Join(resultMetadataDir, "annotations.yaml")
	annotationsBlob, err := ioutil.ReadFile(outputAnnotationsFile)
	require.NoError(t, err)
	require.Equal(t, testContent, annotationsBlob)

	os.RemoveAll(filepath.Dir(outputAnnotationsFile))
}

func TestCopyYamlOutput_NestedCopy(t *testing.T) {
	testOutputDir, _ := ioutil.TempDir("./", "test-generate")
	defer os.RemoveAll(testOutputDir)

	testContent := []byte{0, 1, 0, 0}
	testManifestDir := "./testdata/generate/nested_manifests"
	testWorkingDir := "./"
	testOverwrite := true

	resultManifestDir, resultMetadataDir, err := CopyYamlOutput(testContent, testManifestDir, testOutputDir, testWorkingDir, testOverwrite)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(testOutputDir, "manifests/"), resultManifestDir)
	require.Equal(t, filepath.Join(testOutputDir, "metadata/"), resultMetadataDir)

	outputAnnotationsFile := filepath.Join(testOutputDir, "metadata/", "annotations.yaml")
	annotationsBlob, err := ioutil.ReadFile(outputAnnotationsFile)
	require.NoError(t, err)
	require.Equal(t, testContent, annotationsBlob)

	csvFile := filepath.Join(testOutputDir, "manifests/nested_manifests/", "prometheusoperator.0.14.0.clusterserviceversion.yaml")
	_, err = ioutil.ReadFile(csvFile)
	require.NoError(t, err)
}

func TestGenerateFunc(t *testing.T) {
	etcdPkgPath := "./testdata/etcd"
	outputPath := "./testdata/tmp_output"
	defer os.RemoveAll(outputPath)
	err := GenerateFunc(filepath.Join(etcdPkgPath, "0.6.1"), outputPath, "", "", "", true)
	require.NoError(t, err)
	os.Remove(filepath.Join("./", DockerFile))

	output := fmt.Sprintf("annotations:\n" +
		"  operators.operatorframework.io.bundle.channel.default.v1: alpha\n" +
		"  operators.operatorframework.io.bundle.channels.v1: beta\n" +
		"  operators.operatorframework.io.bundle.manifests.v1: manifests/\n" +
		"  operators.operatorframework.io.bundle.mediatype.v1: registry+v1\n" +
		"  operators.operatorframework.io.bundle.metadata.v1: metadata/\n" +
		"  operators.operatorframework.io.bundle.package.v1: etcd\n")
	outputAnnotationsFile := filepath.Join(outputPath, "metadata/", "annotations.yaml")
	annotationsBlob, err := ioutil.ReadFile(outputAnnotationsFile)
	require.NoError(t, err)
	require.EqualValues(t, output, string(annotationsBlob))
}
