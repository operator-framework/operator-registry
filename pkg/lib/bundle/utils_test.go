package bundle

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

const (
	testOperatorDir = "/test-operator"
	helmFile        = "Chart.yaml"
	csvFile         = "test.clusterserviceversion.yaml"
	crdFile         = "test.crd.yaml"
)

func setup(input string) {
	// Create test directory
	testDir := getTestDir()
	createDir(testDir)

	// Create test files in test directory
	createFiles(testDir, input)
}

func getTestDir() string {
	// Create test directory
	dir, _ := os.Getwd()
	testDir := filepath.Join(dir, testOperatorDir)
	return testDir
}

func cleanup() {
	// Remove test directory
	os.RemoveAll(getTestDir())
}

func createDir(dir string) {
	os.MkdirAll(dir, os.ModePerm)
}

func createFiles(dir, input string) {
	// Create test files in test directory
	switch input {
	case RegistryV1Type:
		file, _ := os.Create(filepath.Join(dir, csvFile))
		file.Close()
	case HelmType:
		file, _ := os.Create(filepath.Join(dir, helmFile))
		file.Close()
	case PlainType:
		file, _ := os.Create(filepath.Join(dir, crdFile))
		file.Close()
	default:
		break
	}
}

func buildTestAnnotations(key string, items map[string]string) []byte {
	temp := make(map[string]interface{})
	temp[key] = items
	output, _ := yaml.Marshal(temp)
	return output
}

func clearDir(dir string) {
	items, _ := ioutil.ReadDir(dir)

	for _, item := range items {
		if item.IsDir() {
			continue
		} else {
			os.Remove(filepath.Join(dir, item.Name()))
		}
	}
}
