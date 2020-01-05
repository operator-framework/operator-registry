package bundle

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	v "github.com/operator-framework/api/pkg/validation"
	v1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/operator-framework/operator-registry/pkg/containertools"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiValidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"

	y "github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const (
	csvKind                = "ClusterServiceVersion"
	crdKind                = "CustomResourceDefinition"
	secretKind             = "Secret"
	clusterRoleKind        = "ClusterRole"
	clusterRoleBindingKind = "ClusterRoleBinding"
	serviceAccountKind     = "ServiceAccount"
	serviceKind            = "Service"
	roleKind               = "Role"
	roleBindingKind        = "RoleBinding"
)

type Meta struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
}

// imageValidator is a struct implementation of the Indexer interface
type imageValidator struct {
	imageReader containertools.ImageReader
	logger      *log.Entry
}

// PullBundleImage shells out to a container tool and pulls a given image tag
// Then it unpacks the image layer filesystem contents and pushes the contents
// to a specified directory for further validation
func (i imageValidator) PullBundleImage(imageTag, directory string) error {
	i.logger.Debug("Pulling and unpacking container image")

	return i.imageReader.GetImageData(imageTag, directory)
}

// ValidateBundle takes a directory containing the contents of a bundle and validates
// the format and contents of that bundle for correctness
func (i imageValidator) ValidateBundle(directory string) error {
	var manifestsFound, metadataFound bool
	var annotationsDir, manifestsDir string
	var annotationErrors []error
	var formatErrors []error

	items, _ := ioutil.ReadDir(directory)
	for _, item := range items {
		if item.IsDir() {
			switch s := item.Name(); s {
			case strings.TrimSuffix(ManifestsDir, "/"):
				i.logger.Debug("Found manifests directory")
				manifestsFound = true
				manifestsDir = filepath.Join(directory, ManifestsDir)
			case strings.TrimSuffix(MetadataDir, "/"):
				i.logger.Debug("Found metadata directory")
				metadataFound = true
				annotationsDir = filepath.Join(directory, MetadataDir)
			}
		}
	}

	if manifestsFound == false {
		formatErrors = append(formatErrors, fmt.Errorf("Unable to locate manifests directory"))
	}
	if metadataFound == false {
		formatErrors = append(formatErrors, fmt.Errorf("Unable to locate metadata directory"))
	}

	// Break here if we can't even find the files
	if len(formatErrors) > 0 {
		return NewValidationError(annotationErrors, formatErrors)
	}

	i.logger.Debug("Getting mediaType info from manifests directory")
	mediaType, err := GetMediaType(manifestsDir)
	if err != nil {
		formatErrors = append(formatErrors, err)
	}

	// Validate bundle contents (only for registryv1 and plaintype type bundles)
	i.logger.Debug("Validating bundle contents")
	validationErrors := validateBundleContents(manifestsDir, mediaType, i.logger)
	if len(validationErrors) > 0 {
		for _, err := range validationErrors {
			formatErrors = append(formatErrors, err)
		}
	}

	// Validate annotations.yaml
	annotationsFile, err := ioutil.ReadFile(filepath.Join(annotationsDir, AnnotationsFile))
	if err != nil {
		fmtErr := fmt.Errorf("Unable to read annotations.yaml file: %s", err.Error())
		formatErrors = append(formatErrors, fmtErr)
		return NewValidationError(annotationErrors, formatErrors)
	}

	var fileAnnotations AnnotationMetadata

	annotations := map[string]string{
		MediatypeLabel:      mediaType,
		ManifestsLabel:      ManifestsDir,
		MetadataLabel:       MetadataDir,
		PackageLabel:        "",
		ChannelsLabel:       "",
		ChannelDefaultLabel: "",
	}

	i.logger.Debug("Validating annotations.yaml")

	err = yaml.Unmarshal(annotationsFile, &fileAnnotations)
	if err != nil {
		formatErrors = append(formatErrors, fmt.Errorf("Unable to parse annotations.yaml file"))
	}

	for label, item := range annotations {
		val, ok := fileAnnotations.Annotations[label]
		if ok {
			i.logger.Debugf(`Found annotation "%s" with value "%s"`, label, val)
		} else {
			aErr := fmt.Errorf("Missing annotation %q", label)
			annotationErrors = append(annotationErrors, aErr)
		}

		switch label {
		case MediatypeLabel:
			if item != val {
				aErr := fmt.Errorf("Expecting annotation %q to have value %q instead of %q", label, item, val)
				annotationErrors = append(annotationErrors, aErr)
			}
		case ManifestsLabel:
			if item != ManifestsDir {
				aErr := fmt.Errorf("Expecting annotation %q to have value %q instead of %q", label, ManifestsDir, val)
				annotationErrors = append(annotationErrors, aErr)
			}
		case MetadataDir:
			if item != MetadataLabel {
				aErr := fmt.Errorf("Expecting annotation %q to have value %q instead of %q", label, MetadataDir, val)
				annotationErrors = append(annotationErrors, aErr)
			}
		case ChannelsLabel, ChannelDefaultLabel:
			if val == "" {
				aErr := fmt.Errorf("Expecting annotation %q to have non-empty value", label)
				annotationErrors = append(annotationErrors, aErr)
			} else {
				annotations[label] = val
			}
		}
	}

	_, err = ValidateChannelDefault(annotations[ChannelsLabel], annotations[ChannelDefaultLabel])
	if err != nil {
		annotationErrors = append(annotationErrors, err)
	}

	if len(annotationErrors) > 0 || len(formatErrors) > 0 {
		return NewValidationError(annotationErrors, formatErrors)
	}

	return nil
}

// validateBundleContents confirms that the CSV and CRD files inside the bundle directory are valid
// and can be installed in a cluster. Other GVK types are confirmed as valid kube objects but are not
// explicitly validated currently.
func validateBundleContents(manifestDir string, mediaType string, logger *logrus.Entry) (errors []error) {
	var contentsErrors []error

	switch mediaType {
	case HelmType, PlainType:
		return contentsErrors
	}

	supportedTypes := map[string]string{
		csvKind:                "",
		crdKind:                "",
		secretKind:             "",
		clusterRoleKind:        "",
		clusterRoleBindingKind: "",
		serviceKind:            "",
		serviceAccountKind:     "",
		roleKind:               "",
		roleBindingKind:        "",
	}

	csvValidator := v.ClusterServiceVersionValidator
	crdValidator := v.CustomResourceDefinitionValidator

	// Read all files in manifests directory
	items, _ := ioutil.ReadDir(manifestDir)
	for _, item := range items {
		fileWithPath := filepath.Join(manifestDir, item.Name())
		data, err := ioutil.ReadFile(fileWithPath)
		if err != nil {
			contentsErrors = append(contentsErrors, fmt.Errorf("Unable to read file %s in supported types", fileWithPath))
			continue
		}

		dec := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 30)
		k8sFile := &unstructured.Unstructured{}
		if err := dec.Decode(k8sFile); err == nil {
			kind := k8sFile.GetObjectKind().GroupVersionKind().Kind
			logger.Debugf(`Validating file "%s" with type "%s"`, item.Name(), kind)
			// Verify if the object kind is supported for registryV1 format
			if _, ok := supportedTypes[kind]; !ok {
				contentsErrors = append(contentsErrors, fmt.Errorf("%s is not supported type for registryV1 bundle: %s", kind, fileWithPath))
			} else {
				if kind == csvKind {
					csv := &v1.ClusterServiceVersion{}
					err := runtime.DefaultUnstructuredConverter.FromUnstructured(k8sFile.Object, csv)
					if err != nil {
						contentsErrors = append(contentsErrors, err)
					}

					results := csvValidator.Validate(csv)
					if len(results) > 0 {
						for _, err := range results[0].Errors {
							contentsErrors = append(contentsErrors, err)
						}
					}
				} else if kind == crdKind {
					crd := &apiextensions.CustomResourceDefinition{}
					err := runtime.DefaultUnstructuredConverter.FromUnstructured(k8sFile.Object, crd)
					if err != nil {
						contentsErrors = append(contentsErrors, err)
					}

					results := crdValidator.Validate(crd)
					if len(results) > 0 {
						for _, err := range results[0].Errors {
							contentsErrors = append(contentsErrors, err)
						}
					}
				} else {
					err := validateKubectlable(data)
					if err != nil {
						contentsErrors = append(contentsErrors, err)
					}
				}
			}
		} else {
			contentsErrors = append(contentsErrors, err)
		}
	}

	return contentsErrors
}

// Validate if the file is kubecle-able
func validateKubectlable(fileBytes []byte) error {
	exampleFileBytesJson, err := y.YAMLToJSON(fileBytes)
	if err != nil {
		return err
	}

	parsedMeta := &Meta{}
	err = json.Unmarshal(exampleFileBytesJson, parsedMeta)
	if err != nil {
		return err
	}

	errs := apiValidation.ValidateObjectMeta(
		&parsedMeta.ObjectMeta,
		false,
		func(s string, prefix bool) []string {
			return nil
		},
		field.NewPath("metadata"),
	)

	if len(errs) > 0 {
		return fmt.Errorf("error validating object metadata: %s. %v", errs, parsedMeta)
	}

	return nil
}
