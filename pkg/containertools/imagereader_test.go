package containertools_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/containertools/containertoolsfakes"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const (
	expectedFilePath = "testdata/expected_unpack"
)

func TestReadImageLayers(t *testing.T) {
	image := "quay.io/operator-framework/example"
	testOutputDir := "testdata/output"
	expectedFiles, err := getFiles(expectedFilePath)
	logger := logrus.NewEntry(logrus.New())

	mockCmd := containertoolsfakes.FakeCommandRunner{}
	mockCmd.PullReturns(nil)
	mockCmd.SaveReturns(nil)

	imageReader := containertools.ImageLayerReader{
		Cmd:    &mockCmd,
		Logger: logger,
	}

	tests := []struct {
		description    string
		savedBundleDir string
	}{
		{
			description:    "SavedWithDocker",
			savedBundleDir: "testdata/docker",
		},
		{
			description:    "SavedWithPodman",
			savedBundleDir: "testdata/podman",
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			err = imageReader.GetImageData(image, testOutputDir, containertools.WithWorkingDir(tt.savedBundleDir))
			require.NoError(t, err)

			actualFiles, err := getFiles(testOutputDir)
			require.NoError(t, err)
			require.Len(t, actualFiles, len(expectedFiles), "the number of expected and actual files don't match: expected: %v, actual: %v", expectedFiles, actualFiles)

			for _, file := range expectedFiles {
				expectedFilePath := filepath.Join(expectedFilePath, file)
				expectedFile, err := ioutil.ReadFile(expectedFilePath)
				require.NoError(t, err)

				actualFilePath := filepath.Join(testOutputDir, file)
				actualFile, err := ioutil.ReadFile(actualFilePath)
				require.NoError(t, err)

				require.Equal(t, string(expectedFile), string(actualFile))
			}

			require.NoError(t, os.RemoveAll(testOutputDir))
		})
	}

}

func TestReadImageLayers_PullError(t *testing.T) {
	image := "quay.io/operator-framework/example"
	testOutputDir := "testdata/output"

	logger := logrus.NewEntry(logrus.New())
	mockCmd := containertoolsfakes.FakeCommandRunner{}

	mockCmd.PullReturns(fmt.Errorf("Unable to pull image"))

	imageReader := containertools.ImageLayerReader{
		Cmd:    &mockCmd,
		Logger: logger,
	}

	err := imageReader.GetImageData(image, testOutputDir)
	require.Error(t, err)
}

func TestReadImageLayers_SaveError(t *testing.T) {
	image := "quay.io/operator-framework/example"
	testOutputDir := "testdata/output"

	logger := logrus.NewEntry(logrus.New())
	mockCmd := containertoolsfakes.FakeCommandRunner{}

	mockCmd.PullReturns(nil)
	mockCmd.SaveReturns(fmt.Errorf("Unable to save image"))

	imageReader := containertools.ImageLayerReader{
		Cmd:    &mockCmd,
		Logger: logger,
	}

	err := imageReader.GetImageData(image, testOutputDir)
	require.Error(t, err)
}

func getFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			fileName := strings.Replace(path, dir, "", -1)
			files = append(files, fileName)
		}
		return nil
	})

	return files, err
}
