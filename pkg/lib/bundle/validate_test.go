package bundle

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateBundleAnnotations(t *testing.T) {
	annotationsFilePath := "./testdata/annotations.yaml"
	file, err := ioutil.ReadFile(annotationsFilePath)
	require.NoError(t, err, "error reading from annotations.yaml")

	err = ValidateBundleAnnotations(RegistryV1Type, file)
	assert.NoError(t, err, "error validating annotations.yaml file")

	err = ValidateBundleAnnotations("", file)
	assert.Error(t, err, "expecting MediatypeLabel error when validating annotations.yaml file")
}

func TestParseManifestJson(t *testing.T) {
	manifestJsonFile := "./testdata/manifest.json"
	expectedConfig := "b7e63f6a13273f125c08ce9681e049d2946476f15bda3c556c93ffd3d3574bcd.json"
	expectedLayer := []string{"cba8efc9fb725f08c9956e87bfb84e92037b81264ba0a5a0633f546cb4f7d966/layer.tar",
		"e1f1237e89119d099ad19069292b6eef47f8f7676df706b600c3a945f0604bc8/layer.tar"}

	file, err := ioutil.ReadFile(manifestJsonFile)
	require.NoError(t, err, "error reading from manifest.json file")

	config, layers, err := ParseManifestJson(file)
	assert.NoError(t, err, "error parsing manifest.json file")
	assert.EqualValues(t, expectedConfig, config)
	assert.EqualValues(t, expectedLayer, layers)
}

func TestUntarFile(t *testing.T) {
	bundleDir := "./testdata/bundle"
	expectedBundleDir := "./testdata/expectedBundle"

	err := UntarFile(bundleDir, BundleTarFile)
	assert.NoError(t, err, "error untaring docker bundle")

	expectedFiles, err := ioutil.ReadDir(expectedBundleDir)
	require.NoError(t, err, "error reading from expected bundle directory")

	files, err := ioutil.ReadDir(bundleDir)
	require.NoError(t, err, "error reading from untared bundle directory")

	expectedFileMap := make(map[string]int64, len(expectedFiles))
	for _, file := range (expectedFiles) {
		expectedFileMap[file.Name()] = file.Size()
	}
	FileMap := make(map[string]int64, len(files))
	for _, file := range (files) {
		FileMap[file.Name()] = file.Size()
		if file.Name() != BundleTarFile{
			defer os.RemoveAll(filepath.Join(bundleDir,file.Name()))
		}
	}
	assert.EqualValues(t,expectedFileMap,FileMap)
}
