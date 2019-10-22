package bundle

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"gopkg.in/yaml.v2"
)

const (
	defaultPermission   = 0644
	registryV1Type      = "registry+v1"
	plainType           = "plain"
	helmType            = "helm"
	annotationsFile     = "annotations.yaml"
	dockerFile          = "Dockerfile"
	manifestsDir        = "manifests/"
	metadataDir         = "metadata/"
	manifestsLabel      = "operators.operatorframework.io.bundle.manifests.v1"
	metadataLabel       = "operators.operatorframework.io.bundle.metadata.v1"
	mediatypeLabel      = "operators.operatorframework.io.bundle.mediatype.v1"
	packageLabel        = "operators.operatorframework.io.bundle.package.v1"
	channelsLabel       = "operators.operatorframework.io.bundle.channels.v1"
	channelDefaultLabel = "operators.operatorframework.io.bundle.channel.default.v1"
)

type AnnotationMetadata struct {
	Annotations map[string]string `yaml:"annotations"`
}

// GenerateFunc builds annotations.yaml with mediatype, manifests &
// metadata directories in bundle image, package name, channels and default
// channels information and then writes the file to `/metadata` directory.
func GenerateFunc(directory, packageName, channels, channelDefault string, overwrite bool) error {
	var mediaType string

	// Determine mediaType
	mediaType, err := GetMediaType(directory)
	if err != nil {
		return err
	}

	log.Info("Building annotations.yaml")

	// Generate annotations.yaml
	content, err := GenerateAnnotations(mediaType, manifestsDir, metadataDir, packageName, channels, channelDefault)
	if err != nil {
		return err
	}

	file, err := ioutil.ReadFile(filepath.Join(directory, metadataDir, annotationsFile))
	if os.IsNotExist(err) || overwrite {
		err = WriteFile(annotationsFile, filepath.Join(directory, metadataDir), content)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		log.Info("An annotations.yaml already exists in directory")
		if err = ValidateAnnotations(file, content); err != nil {
			return err
		}
	}

	log.Info("Building Dockerfile")

	// Generate Dockerfile
	content, err = GenerateDockerfile(directory, mediaType, manifestsDir, metadataDir, packageName, channels, channelDefault)
	if err != nil {
		return err
	}

	err = WriteFile(dockerFile, directory, content)
	if err != nil {
		return err
	}

	return nil
}

// GenerateFunc determines mediatype from files (yaml) in given directory
// Currently able to detect helm chart, registry+v1 (CSV) and plain k8s resources
// such as CRD.
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

// ValidateAnnotations validates existing annotations.yaml against generated
// annotations.yaml to ensure existing annotations.yaml contains expected values.
func ValidateAnnotations(existing, expected []byte) error {
	var fileAnnotations AnnotationMetadata
	var expectedAnnotations AnnotationMetadata

	log.Info("Validating existing annotations.yaml")

	err := yaml.Unmarshal(existing, &fileAnnotations)
	if err != nil {
		log.Errorf("Unable to parse existing annotations.yaml")
		return err
	}

	err = yaml.Unmarshal(expected, &expectedAnnotations)
	if err != nil {
		log.Errorf("Unable to parse expected annotations.yaml")
		return err
	}

	if len(fileAnnotations.Annotations) != len(expectedAnnotations.Annotations) {
		return fmt.Errorf("Unmatched number of fields. Expected (%d) vs existing (%d)",
			len(expectedAnnotations.Annotations), len(fileAnnotations.Annotations))
	}

	for label, item := range expectedAnnotations.Annotations {
		value, ok := fileAnnotations.Annotations[label]
		if ok == false {
			return fmt.Errorf("Missing field: %s", label)
		}

		if item != value {
			return fmt.Errorf(`Expect field "%s" to have value "%s" instead of "%s"`,
				label, item, value)
		}
	}

	return nil
}

// ValidateAnnotations validates provided default channel to ensure it exists in
// provided channel list.
func ValidateChannelDefault(channels, channelDefault string) (string, error) {
	var chanDefault string
	var chanErr error
	channelList := strings.Split(channels, ",")

	if channelDefault != "" {
		for _, channel := range channelList {
			if channel == channelDefault {
				chanDefault = channelDefault
				break
			}
		}
		if chanDefault == "" {
			chanDefault = channelList[0]
			chanErr = fmt.Errorf(`The channel list "%s" doesn't contain channelDefault "%s"`, channels, channelDefault)
		}
	} else {
		chanDefault = channelList[0]
	}

	if chanDefault != "" {
		return chanDefault, chanErr
	} else {
		return chanDefault, fmt.Errorf("Invalid channels is provied: %s", channels)
	}
}

// GenerateAnnotations builds annotations.yaml with mediatype, manifests &
// metadata directories in bundle image, package name, channels and default
// channels information.
func GenerateAnnotations(mediaType, manifests, metadata, packageName, channels, channelDefault string) ([]byte, error) {
	annotations := &AnnotationMetadata{
		Annotations: map[string]string{
			mediatypeLabel:      mediaType,
			manifestsLabel:      manifests,
			metadataLabel:       metadata,
			packageLabel:        packageName,
			channelsLabel:       channels,
			channelDefaultLabel: channelDefault,
		},
	}

	chanDefault, err := ValidateChannelDefault(channels, channelDefault)
	if err != nil {
		return nil, err
	}

	annotations.Annotations[channelDefaultLabel] = chanDefault

	afile, err := yaml.Marshal(annotations)
	if err != nil {
		return nil, err
	}

	return afile, nil
}

// GenerateDockerfile builds Dockerfile with mediatype, manifests &
// metadata directories in bundle image, package name, channels and default
// channels information in LABEL section.
func GenerateDockerfile(directory, mediaType, manifests, metadata, packageName, channels, channelDefault string) ([]byte, error) {
	var fileContent string

	chanDefault, err := ValidateChannelDefault(channels, channelDefault)
	if err != nil {
		return nil, err
	}

	// FROM
	fileContent += "FROM scratch\n\n"

	// LABEL
	fileContent += fmt.Sprintf("LABEL %s=%s\n", mediatypeLabel, mediaType)
	fileContent += fmt.Sprintf("LABEL %s=%s\n", manifestsLabel, manifests)
	fileContent += fmt.Sprintf("LABEL %s=%s\n", metadataLabel, metadata)
	fileContent += fmt.Sprintf("LABEL %s=%s\n", packageLabel, packageName)
	fileContent += fmt.Sprintf("LABEL %s=%s\n", channelsLabel, channels)
	fileContent += fmt.Sprintf("LABEL %s=%s\n\n", channelDefaultLabel, chanDefault)

	// CONTENT
	fileContent += fmt.Sprintf("ADD %s %s\n", filepath.Join(directory, "*.yaml"), "/manifests")
	fileContent += fmt.Sprintf("ADD %s %s%s\n", filepath.Join(directory, metadata, annotationsFile), "/metadata/", annotationsFile)

	return []byte(fileContent), nil
}

// Write `fileName` file with `content` into a `directory`
// Note: Will overwrite the existing `fileName` file if it exists
func WriteFile(fileName, directory string, content []byte) error {
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		os.Mkdir(directory, os.ModePerm)
	}

	err := ioutil.WriteFile(filepath.Join(directory, fileName), content, defaultPermission)
	if err != nil {
		return err
	}
	return nil
}
