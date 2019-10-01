package bundle

import (
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"gopkg.in/yaml.v2"
)

const (
	defaultPermission = 0644
	registryV1Type    = "registry+v1"
	plainType         = "plain"
	helmType          = "helm"
	manifestsMetadata = "manifests+metadata"
	annotationsFile   = "annotations.yaml"
	dockerFile        = "Dockerfile"
	resourcesLabel    = "operators.operatorframework.io.bundle.resources"
	mediatypeLabel    = "operators.operatorframework.io.bundle.mediatype"
)

type AnnotationMetadata struct {
	Annotations AnnotationType `yaml:"annotations"`
}

type AnnotationType struct {
	Resources string `yaml:"operators.operatorframework.io.bundle.resources"`
	MediaType string `yaml:"operators.operatorframework.io.bundle.mediatype"`
}

func GenerateFunc(directory string) error {
	var mediaType string

	// Determine mediaType
	mediaType, err := GetMediaType(directory)
	if err != nil {
		return err
	}

	// Parent directory
	parentDir := path.Dir(path.Clean(directory))

	log.Info("Building annotations.yaml file")

	// Generate annotations.yaml
	content, err := GenerateAnnotations(manifestsMetadata, mediaType)
	if err != nil {
		return err
	}
	err = WriteFile(annotationsFile, parentDir, content)
	if err != nil {
		return err
	}

	log.Info("Building Dockerfile")

	// Generate Dockerfile
	content = GenerateDockerfile(manifestsMetadata, mediaType, directory)
	err = WriteFile(dockerFile, parentDir, content)
	if err != nil {
		return err
	}

	return nil
}

func GetMediaType(directory string) (string, error) {
	var files []string

	// Read all file names in directory
	items, _ := ioutil.ReadDir(directory)
	for _, item := range items {
		if item.IsDir() {
			continue
		} else {
			files = append(files, item.Name())
		}
	}

	if len(files) == 0 {
		return "", fmt.Errorf("The directory %s contains no files", directory)
	}

	// Validate the file names to determine media type
	for _, file := range files {
		if file == "Chart.yaml" {
			return helmType, nil
		} else if strings.HasSuffix(file, "clusterserviceversion.yaml") {
			return registryV1Type, nil
		} else {
			continue
		}
	}

	return plainType, nil
}

func GenerateAnnotations(resourcesType, mediaType string) ([]byte, error) {
	annotations := &AnnotationMetadata{
		Annotations: AnnotationType{
			Resources: resourcesType,
			MediaType: mediaType,
		},
	}

	afile, err := yaml.Marshal(annotations)
	if err != nil {
		return nil, err
	}

	return afile, nil
}

func GenerateDockerfile(resourcesType, mediaType, directory string) []byte {
	var fileContent string

	metadataDir := path.Dir(path.Clean(directory))

	// FROM
	fileContent += "FROM scratch\n\n"

	// LABEL
	fileContent += fmt.Sprintf("LABEL %s=%s\n", resourcesLabel, resourcesType)
	fileContent += fmt.Sprintf("LABEL %s=%s\n\n", mediatypeLabel, mediaType)

	// CONTENT
	fileContent += fmt.Sprintf("ADD %s %s\n", directory, "/manifests")
	fileContent += fmt.Sprintf("ADD %s/%s %s%s\n", metadataDir, annotationsFile, "/metadata/", annotationsFile)

	return []byte(fileContent)
}

// Write `fileName` file with `content` into a `directory`
// Note: Will overwrite the existing `fileName` file if it exists
func WriteFile(fileName, directory string, content []byte) error {
	err := ioutil.WriteFile(filepath.Join(directory, fileName), content, defaultPermission)
	if err != nil {
		return err
	}
	return nil
}
