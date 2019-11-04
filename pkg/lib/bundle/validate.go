package bundle

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const (
	FileStoreDir     = "bundle-"
	BundleTarFile    = "bundle.tar"
	ManifestJsonFile = "manifest.json"
)

type ManifestJson struct {
	Config string   `json:"Config"`
	Layers []string `json:"Layers"`
}

type LabelsJson struct {
	Config ConfigData `json:"config"`
}

type ConfigData struct {
	Labels map[string]string `json:"Labels"`
}

// ValidateFunc is used to validate bundle container image.
// Inputs:
// @imageTag: The image tag that is applied to the bundle image
// @imageBuilder: The image builder tool that is used to build container image
// (docker or podman)
func ValidateFunc(imageTag, imageBuilder string) error {
	dir, err := ioutil.TempDir("", FileStoreDir)
	log.Infof("Create a temp directory at %s", dir)
	if err != nil {
		return err
	}
	//defer os.RemoveAll(dir)

	log.Infof("Pulling bunde image %s", imageTag)
	pullCmd, err := FullImage(imageTag, imageBuilder)

	if err := ExecuteCommand(pullCmd); err != nil {
		return err
	}

	log.Infof("Saving bunde image into tarball %s", BundleTarFile)
	saveCmd, err := SaveImage(dir, imageTag, imageBuilder)
	if err := ExecuteCommand(saveCmd); err != nil {
		return err
	}

	log.Infof("Extracting tarball %s", BundleTarFile)
	err = UntarFile(dir, BundleTarFile)
	if err != nil {
		return err
	}

	log.Infof("Parsing manifest.json")
	file, err := ioutil.ReadFile(filepath.Join(dir, ManifestJsonFile))
	if err != nil {
		return err
	}
	_, layerFiles, err := ParseManifestJson(file)
	if err != nil {
		return err
	}

	// TODO: Verify image labels
	if len(layerFiles) < 1 {
		return fmt.Errorf("Expecting at least one layer in bundle image")
	}

	var manifestsFound, metadataFound bool
	var annotationsDir, manifestsDir string
	for _, item := range layerFiles {
		log.Infof("Extracting layer tarball %s", item)
		err = UntarFile(dir, item)
		if err != nil {
			return err
		}

		items, _ := ioutil.ReadDir(dir)
		for _, item := range items {
			if item.IsDir() {
				switch s := item.Name(); s {
				case strings.TrimSuffix(ManifestsDir, "/"):
					log.Info("Found manifests directory")
					manifestsFound = true
					manifestsDir = filepath.Join(dir, ManifestsDir)
				case strings.TrimSuffix(MetadataDir, "/"):
					log.Info("Found metadata directory")
					metadataFound = true
					annotationsDir = filepath.Join(dir, MetadataDir)
				}
			}
		}
	}

	if manifestsFound == false {
		return fmt.Errorf("Unable to locate manifests directory")
	} else if metadataFound == false {
		return fmt.Errorf("Unable to locate metadata directory")
	}

	log.Info("Getting mediaType info from manifests directory")
	mediaType, err := GetMediaType(manifestsDir)
	if err != nil {
		log.Error("Unable to determine manifests mediaType")
		return err
	}

	// Validate annotations.yaml
	file, err = ioutil.ReadFile(filepath.Join(annotationsDir, AnnotationsFile))
	if err != nil {
		log.Error("Unable to validate annotations.yaml")
		return err
	} else {
		if err = ValidateBundleAnnotations(mediaType, file); err != nil {
			return err
		}
	}

	log.Info("All validation tests have been completed successfully")

	return nil
}

func UntarFile(dir, tarName string) error {
	file, err := os.Open(filepath.Join(dir, tarName))
	if err != nil {
		return err
	}

	err = Untar(file, dir)
	if err != nil {
		return err
	}

	return nil
}

func FullImage(imageTag, imageBuilder string) (*exec.Cmd, error) {
	var args []string

	switch imageBuilder {
	case "docker", "podman":
		args = append(args, "pull", imageTag)
	default:
		return nil, fmt.Errorf("%s is not supported image builder", imageBuilder)
	}

	return exec.Command(imageBuilder, args...), nil
}

func SaveImage(directory, imageTag, imageBuilder string) (*exec.Cmd, error) {
	var args []string

	switch imageBuilder {
	case "docker", "podman":
		args = append(args, "save", "-o", filepath.Join(directory, BundleTarFile), imageTag)
	default:
		return nil, fmt.Errorf("%s is not supported image builder", imageBuilder)
	}

	return exec.Command(imageBuilder, args...), nil
}

func ParseManifestJson(file []byte) (string, []string, error) {
	var manifest = []*ManifestJson{}

	err := json.Unmarshal(file, &manifest)
	if err != nil {
		log.Errorf("Unable to parse manifest.json")
		return "", nil, err
	}

	return manifest[0].Config, manifest[0].Layers, nil
}

func ValidateBundleAnnotations(mediaType string, file []byte) error {
	var fileAnnotations AnnotationMetadata
	var invalid bool

	annotations := map[string]string{
		MediatypeLabel:      mediaType,
		ManifestsLabel:      ManifestsDir,
		MetadataLabel:       MetadataDir,
		PackageLabel:        "",
		ChannelsLabel:       "",
		ChannelDefaultLabel: "",
	}

	log.Info("Validating annotations.yaml")

	err := yaml.Unmarshal(file, &fileAnnotations)
	if err != nil {
		log.Error("Unable to parse annotations.yaml")
		return err
	}

	for label, item := range annotations {
		val, ok := fileAnnotations.Annotations[label]
		if ok {
			log.Infof(`Found annotation "%s" with value "%s"`, label, val)
		} else {
			log.Errorf(`Missing annotation "%s"`, label)
			invalid = true
		}

		switch label {
		case MediatypeLabel:
			if item != val {
				log.Errorf(`Expecting annotation "%s" to have value "%s" instead of "%s"`, label, item, val)
				invalid = true
			}
		case ManifestsLabel:
			if item != ManifestsDir {
				log.Errorf(`Expecting annotation "%s" to have value "%s" instead of "%s"`, label, ManifestsDir, val)
				invalid = true
			}
		case MetadataDir:
			if item != MetadataLabel {
				log.Errorf(`Expecting annotation "%s" to have value "%s" instead of "%s"`, label, MetadataDir, val)
				invalid = true
			}
		case ChannelsLabel, ChannelDefaultLabel:
			if val == "" {
				log.Errorf(`Expecting annotation "%s" to have non-empty value`, label)
				invalid = true
			} else {
				annotations[label] = val
			}
		}
	}

	_, err = ValidateChannelDefault(annotations[ChannelsLabel], annotations[ChannelDefaultLabel])
	if err != nil {
		log.Error(err.Error())
		invalid = true
	}

	if invalid {
		return fmt.Errorf("The annotations.yaml is invalid")
	}

	return nil
}
