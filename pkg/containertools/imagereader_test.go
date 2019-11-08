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

func TestReadImageLayersDocker(t *testing.T) {
	image := "quay.io/operator-framework/example"
	testWorkingDir := "testdata/docker"
	testOutputDir := "testdata/output"

	expectedFiles, err := helperGetExpectedFiles()

	logger := logrus.NewEntry(logrus.New())
	mockCmd := containertoolsfakes.FakeCommandRunner{}

	mockCmd.PullReturns(nil)
	mockCmd.SaveReturns(nil)

	imageReader := containertools.ImageLayerReader{
		Cmd:    &mockCmd,
		Logger: logger,
	}

	err = imageReader.GetImageData(image, testOutputDir, containertools.WithWorkingDir(testWorkingDir))
	require.NoError(t, err)

	for _, file := range expectedFiles {
		expectedFilePath := filepath.Join(expectedFilePath, file)
		expectedFile, err := ioutil.ReadFile(expectedFilePath)
		require.NoError(t, err)

		actualFilePath := filepath.Join(testOutputDir, file)
		actualFile, err := ioutil.ReadFile(actualFilePath)
		require.NoError(t, err)

		require.Equal(t, string(expectedFile), string(actualFile))
	}

	err = os.RemoveAll(testOutputDir)
	require.NoError(t, err)
}

func TestReadImageLayersPodman(t *testing.T) {
	image := "quay.io/operator-framework/example"
	testWorkingDir := "testdata/podman"
	testOutputDir := "testdata/output"

	expectedFiles, err := helperGetExpectedFiles()

	logger := logrus.NewEntry(logrus.New())
	mockCmd := containertoolsfakes.FakeCommandRunner{}

	mockCmd.PullReturns(nil)
	mockCmd.SaveReturns(nil)

	imageReader := containertools.ImageLayerReader{
		Cmd:    &mockCmd,
		Logger: logger,
	}

	err = imageReader.GetImageData(image, testOutputDir, containertools.WithWorkingDir(testWorkingDir))
	require.NoError(t, err)

	for _, file := range expectedFiles {
		expectedFilePath := filepath.Join(expectedFilePath, file)
		expectedFile, err := ioutil.ReadFile(expectedFilePath)
		require.NoError(t, err)

		actualFilePath := filepath.Join(testOutputDir, file)
		actualFile, err := ioutil.ReadFile(actualFilePath)
		require.NoError(t, err)

		require.Equal(t, string(expectedFile), string(actualFile))
	}

	err = os.RemoveAll(testOutputDir)
	require.NoError(t, err)
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

func helperGetExpectedFiles() ([]string, error) {
	var files []string

	err := filepath.Walk(expectedFilePath, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			fileName := strings.Replace(path, expectedFilePath, "", -1)
			files = append(files, fileName)
		}
		return nil
	})

	return files, err
}
