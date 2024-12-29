package bundle

import (
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
	_ = os.MkdirAll(dir, os.ModePerm)
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

func buildTestAnnotations(items map[string]string) []byte {
	temp := make(map[string]interface{})
	temp["annotations"] = items
	output, _ := yaml.Marshal(temp)
	return output
}
