package synthesize

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
)

var CsvInferenceFileNotFoundError = errors.New("a file that contains non-inferable metadata could not be found")

const (
	SynthesizedCSVFileName string = "olm.clusterserviceversion.yaml"
	DefaultPermission             = 0644
)

// GenerateCSV generates a CSV for the bundle based on the resource within the bundle directory.
func GenerateCSV(bundleDir string) error {
	bundleDir, err := filepath.Abs(bundleDir)
	if err != nil {
		return err
	}

	// find annotations.yaml file in the bundle dir.
	var annotations bundle.AnnotationMetadata
	var annotationsFilePath string
	if err := filepath.Walk(bundleDir, func(path string, info os.FileInfo, err error) error {
		if annotations.IsComplete() {
			return nil
		}

		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		annotationsFilePath = path
		content, err := ioutil.ReadFile(annotationsFilePath)
		if err != nil {
			return err
		}

		// tries to parse the content as annotations else go to the next file.
		annotations, _ = bundle.GetAnnotations(content)

		return nil
	}); err != nil {
		return err
	}

	if !annotations.IsComplete() {
		return fmt.Errorf("failed to find annotations YAML file in the bundle")
	}

	s, err := SynthesizeCsvFromDirectories(filepath.Join(bundleDir, annotations.GetManifestsDirName()),
		filepath.Join(bundleDir, annotations.GetMetadataDirName()))
	if err != nil {
		return err
	}

	csv, err := s.GenerateCSV()
	if err != nil {
		return err
	}

	// generate CSV file to the outputDir.
	manifestsDir := path.Join(bundleDir, annotations.GetManifestsDirName())
	if _, err := os.Stat(manifestsDir); os.IsNotExist(err) {
		err = os.MkdirAll(manifestsDir, os.ModePerm)
		if err != nil {
			return err
		}
	}

	filePath := path.Join(manifestsDir, SynthesizedCSVFileName)

	err = WriteStructToFile(csv, filePath)
	if err != nil {
		return fmt.Errorf("error writing CSV file, %v", err)
	}

	if s.existingCSVPath != "" {
		log.Warnf("Overwriting an existing ClusterServiceVersion in the bundle at %s", s.existingCSVPath)
		if s.existingCSVPath != filePath {
			os.Remove(s.existingCSVPath)
		}
	}

	// Update media type after successfully creating the CSV
	err = os.Remove(annotationsFilePath)
	if err != nil {
		return fmt.Errorf("error deleting annotations YAML file %s, %v", annotationsFilePath, err)
	}

	annotations.SetMediaType(bundle.RegistryV1Type)
	annotationsContent, err := bundle.GenerateAnnotations(&annotations)
	err = bundle.WriteFile(bundle.AnnotationsFile, path.Join(bundleDir, annotations.GetMetadataDirName()),
		annotationsContent)
	if err != nil {
		return err
	}

	// Validate the bundle directory
	logger := log.NewEntry(log.New())
	log.SetLevel(log.DebugLevel)

	bundleValidator := bundle.NewBundleValidator(logger)
	if err := bundleValidator.ValidateBundleFormat(bundleDir); err != nil {
		return err
	}

	if err := bundleValidator.ValidateBundleContent(manifestsDir); err != nil {
		return err
	}

	return nil
}

type MarshalFunc func(interface{}) ([]byte, error)

// WriteStructToFile writes a struct's content to a YAML file.
// This strategy is borrowed from SDK to marshall an object with m and removes runtime-managed fields:
// 'status', 'creationTimestamp'
func WriteStructToFile(obj runtime.Object, filePath string) error {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return err
	}
	deleteKeys := []string{"status", "creationTimestamp"}
	for _, dk := range deleteKeys {
		deleteKeyFromUnstructured(u, dk)
	}

	bytes, err := yaml.Marshal(u)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filePath, bytes, DefaultPermission); err != nil {
		return fmt.Errorf("error writing bundle file: %v", err)
	}

	return nil
}

func deleteKeyFromUnstructured(u map[string]interface{}, key string) {
	if _, ok := u[key]; ok {
		delete(u, key)
		return
	}

	for _, v := range u {
		switch t := v.(type) {
		case map[string]interface{}:
			deleteKeyFromUnstructured(t, key)
		case []interface{}:
			for _, ti := range t {
				if m, ok := ti.(map[string]interface{}); ok {
					deleteKeyFromUnstructured(m, key)
				}
			}
		}
	}
}
