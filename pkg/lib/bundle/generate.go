package bundle

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kubectl/pkg/util/slice"
)

const (
	DefaultPermission   = 0644
	RegistryV1Type      = "registry+v1"
	PlainType           = "plain"
	HelmType            = "helm"
	AnnotationsFile     = "annotations.yaml"
	DockerFile          = "bundle.Dockerfile"
	ManifestsDir        = "manifests/"
	MetadataDir         = "metadata/"
	ManifestsLabel      = "operators.operatorframework.io.bundle.manifests.v1"
	MetadataLabel       = "operators.operatorframework.io.bundle.metadata.v1"
	MediatypeLabel      = "operators.operatorframework.io.bundle.mediatype.v1"
	PackageLabel        = "operators.operatorframework.io.bundle.package.v1"
	ChannelsLabel       = "operators.operatorframework.io.bundle.channels.v1"
	ChannelDefaultLabel = "operators.operatorframework.io.bundle.channel.default.v1"
)

type AnnotationMetadata struct {
	Annotations map[string]string `yaml:"annotations"`
}

func NewAnnotations(mediaType, manifests, metadata, packageName, channels, channelDefault string) *AnnotationMetadata {
	return &AnnotationMetadata{
		Annotations: map[string]string{
			MediatypeLabel:      mediaType,
			ManifestsLabel:      manifests,
			MetadataLabel:       metadata,
			PackageLabel:        packageName,
			ChannelsLabel:       channels,
			ChannelDefaultLabel: channelDefault,
		},
	}
}

func (a *AnnotationMetadata) IsComplete() bool {
	return a.Annotations[ManifestsLabel] != "" && a.Annotations[MetadataLabel] != "" && a.
		Annotations[ChannelsLabel] != "" && a.Annotations[PackageLabel] != "" && a.Annotations[MediatypeLabel] != ""
}

func (a *AnnotationMetadata) GetManifestsDirName() string {
	return a.Annotations[ManifestsLabel]
}

func (a *AnnotationMetadata) GetMetadataDirName() string {
	return a.Annotations[MetadataLabel]
}

func (a *AnnotationMetadata) GetChannels() string {
	return a.Annotations[ChannelsLabel]
}

func (a *AnnotationMetadata) GetPackageName() string {
	return a.Annotations[PackageLabel]
}

func (a *AnnotationMetadata) GetMediatype() string {
	return a.Annotations[MediatypeLabel]
}

func (a *AnnotationMetadata) GetDefaultChannel() string {
	return a.Annotations[ChannelDefaultLabel]
}

func (a *AnnotationMetadata) SetManifestsDirName(manifestsDirName string) {
	a.Annotations[ManifestsLabel] = manifestsDirName
}

func (a *AnnotationMetadata) SetMetadataDirName(metadataDirName string) {
	a.Annotations[MetadataLabel] = metadataDirName
}

func (a *AnnotationMetadata) SetChannels(channels string) {
	a.Annotations[ChannelsLabel] = channels
}

func (a *AnnotationMetadata) SetPackageName(packageName string) {
	a.Annotations[PackageLabel] = packageName
}

func (a *AnnotationMetadata) SetMediaType(mediatype string) {
	a.Annotations[MediatypeLabel] = mediatype
}

func (a *AnnotationMetadata) SetDefaultChannel(defaultChannel string) {
	a.Annotations[ChannelDefaultLabel] = defaultChannel
}

// GenerateFunc builds annotations.yaml with mediatype, manifests &
// metadata directories in bundle image, package name, channels and default
// channels information and then writes the file to `/metadata` directory.
// Inputs:
// @directory: The local directory where bundle manifests and metadata are located
// @outputDir: Optional generated path where the /manifests and /metadata directories are copied
// as they would appear on the bundle image
// @packageName: The name of the package that bundle image belongs to
// @channels: The list of channels that bundle image belongs to
// @channelDefault: The default channel for the bundle image
// @overwrite: Boolean flag to enable overwriting annotations.yaml locally if existed
func GenerateFunc(option ...BundleOption) error {
	bundleConfig := &BundleConfig{}
	bundleConfig.apply(option)
	if err := bundleConfig.complete(); err != nil {
		return err
	}

	// Determine mediaType
	mediaType, err := GetMediaType(bundleConfig.bundleDir)
	if err != nil {
		return err
	}

	// Get directory context for file output
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Channels and packageName are required fields where as default channel is automatically filled if unspecified
	// and that either of the required field is missing. We are interpreting the bundle information through
	// bundle directory embedded in the package folder.
	if bundleConfig.channels == "" || bundleConfig.packageName == "" {
		var notProvided []string
		if bundleConfig.channels == "" {
			notProvided = append(notProvided, "channels")
		}
		if bundleConfig.packageName == "" {
			notProvided = append(notProvided, "package name")
		}
		log.Infof("Bundle %s information not provided, inferring from parent package directory",
			strings.Join(notProvided, " and "))

		i, err := NewBundleDirInterperter(bundleConfig.bundleDir)
		if err != nil {
			return fmt.Errorf("please manually input channels and packageName, "+
				"error interpreting bundle from directory %s, %v", bundleConfig.bundleDir, err)
		}

		if bundleConfig.channels == "" {
			bundleConfig.channels = strings.Join(i.GetBundleChannels(), ",")
			if bundleConfig.channels == "" {
				return fmt.Errorf("error interpreting channels, please manually input channels instead")
			}
			log.Infof("Inferred channels: %s", bundleConfig.channels)
		}

		if bundleConfig.packageName == "" {
			bundleConfig.packageName = i.GetPackageName()
			log.Infof("Inferred package name: %s", bundleConfig.packageName)
		}

		if bundleConfig.channelDefault == "" {
			bundleConfig.channelDefault = i.GetDefaultChannel()
			if !containsString(strings.Split(bundleConfig.channels, ","), bundleConfig.channelDefault) {
				bundleConfig.channelDefault = ""
			}
			log.Infof("Inferred default channel: %s", bundleConfig.channelDefault)
		}
	}

	log.Info("Building annotations.yaml")

	// Generate annotations.yaml
	content, err := GenerateAnnotations(NewAnnotations(mediaType, ManifestsDir, MetadataDir, bundleConfig.packageName,
		bundleConfig.channels, bundleConfig.channelDefault))
	if err != nil {
		return err
	}

	// Push the output yaml content to the correct directory and conditionally copy the manifest dir
	outManifestDir, outMetadataDir, err := CopyYamlOutput(content, bundleConfig.bundleDir, bundleConfig.outputDir, bundleConfig.overwrite)
	if err != nil {
		return err
	}

	log.Info("Building Dockerfile")

	// Generate Dockerfile
	content, err = GenerateDockerfile(mediaType, ManifestsDir, MetadataDir, outManifestDir, outMetadataDir,
		workingDir, bundleConfig.packageName, bundleConfig.channels, bundleConfig.channelDefault)
	if err != nil {
		return err
	}

	_, err = os.Stat(filepath.Join(workingDir, DockerFile))
	if os.IsNotExist(err) || bundleConfig.overwrite {
		err = WriteFile(DockerFile, workingDir, content)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		log.Info("A bundle.Dockerfile already exists in current working directory")
	}

	return nil
}

// CopyYamlOutput takes the generated annotations yaml and writes it to disk.
// If an outputDir is specified, it will copy the input manifests
// It returns two strings. resultMetadata is the path to the output metadata/ folder.
// resultManifests is the path to the output manifests/ folder -- if no copy occured,
// it just returns the input manifestDir
func CopyYamlOutput(annotationsContent []byte, manifestDir, outputDir string, overwrite bool) (resultManifests, resultMetadata string, err error) {
	// First, determine the parent directory of the metadata and manifest directories
	copyDir := ""

	// If an output directory is not defined defined, generate metadata folder into the same parent dir as existing manifest dir
	if outputDir == "" {
		copyDir = filepath.Dir(manifestDir)
		resultManifests = manifestDir
	} else { // otherwise copy the manifests into $outputDir/manifests and create the annotations file in $outputDir/metadata
		copyDir = outputDir

		log.Info("Generating output manifests directory")

		resultManifests = filepath.Join(copyDir, "/manifests/")
		// copy the manifest directory into $pwd/manifests/
		err := copyManifestDir(manifestDir, resultManifests, overwrite, (manifestDir == copyDir),
			path.Clean(MetadataDir), path.Clean(ManifestsDir))
		if err != nil {
			return "", "", err
		}
	}

	// Now, generate the `metadata/` dir and write the annotations
	file, err := ioutil.ReadFile(filepath.Join(copyDir, MetadataDir, AnnotationsFile))
	if os.IsNotExist(err) || overwrite {
		writeDir := filepath.Join(copyDir, MetadataDir)
		err = WriteFile(AnnotationsFile, writeDir, annotationsContent)
		if err != nil {
			return "", "", err
		}
	} else if err != nil {
		return "", "", err
	} else {
		log.Info("An annotations.yaml already exists in directory")
		if err = ValidateAnnotations(file, annotationsContent); err != nil {
			return "", "", err
		}
	}

	resultMetadata = filepath.Join(copyDir, "metadata")

	return resultManifests, resultMetadata, nil
}

// GetMediaType determines mediatype from files (yaml) in given directory
// Currently able to detect helm chart, registry+v1 (CSV) and plain k8s resources
// such as CRD.
func GetMediaType(directory string) (string, error) {
	var files []string
	k8sFiles := make(map[string]*unstructured.Unstructured)

	// Read all file names in directory
	items, _ := ioutil.ReadDir(directory)
	for _, item := range items {
		if item.IsDir() {
			continue
		}

		files = append(files, item.Name())

		fileWithPath := filepath.Join(directory, item.Name())
		fileBlob, err := ioutil.ReadFile(fileWithPath)
		if err != nil {
			return "", fmt.Errorf("Unable to read file %s in bundle", fileWithPath)
		}

		dec := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(string(fileBlob)), 10)
		unst := &unstructured.Unstructured{}
		if err := dec.Decode(unst); err == nil {
			k8sFiles[item.Name()] = unst
		}
	}

	if len(files) == 0 {
		return "", fmt.Errorf("The directory %s contains no yaml files", directory)
	}

	// Validate if bundle is helm chart type
	if _, err := IsChartDir(directory); err == nil {
		return HelmType, nil
	}

	// Validate the files to determine media type
	for _, fileName := range files {
		// Check if one of the k8s files is a CSV
		if k8sFile, ok := k8sFiles[fileName]; ok {
			if k8sFile.GetObjectKind().GroupVersionKind().Kind == "ClusterServiceVersion" {
				return RegistryV1Type, nil
			}
		}
	}

	return PlainType, nil
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

// ValidateChannelDefault validates provided default channel to ensure it exists in
// provided channel list.
func ValidateChannelDefault(channels, channelDefault string) (string, error) {
	var chanDefault string
	var chanErr error
	channelList := strings.Split(channels, ",")

	if containsString(channelList, "") {
		return chanDefault, fmt.Errorf("invalid channels are provided: %s", channels)
	}

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
	}
	return chanDefault, chanErr
}

// GenerateAnnotations builds annotations.yaml with mediatype, manifests &
// metadata directories in bundle image, package name, channels and default
// channels information.
func GenerateAnnotations(annotations *AnnotationMetadata) ([]byte, error) {
	chanDefault, err := ValidateChannelDefault(annotations.GetChannels(), annotations.GetDefaultChannel())
	if err != nil {
		return nil, err
	}

	annotations.SetDefaultChannel(chanDefault)

	afile, err := yaml.Marshal(annotations)
	if err != nil {
		return nil, err
	}

	return afile, nil
}

func GetAnnotations(content []byte) (annotationsFile AnnotationMetadata, err error) {
	unmarshalErr := yaml.Unmarshal(content, &annotationsFile)
	if unmarshalErr != nil || annotationsFile.Annotations == nil {
		err = fmt.Errorf("failed to decode annotations file")
	}
	return
}

// GenerateDockerfile builds Dockerfile with mediatype, manifests &
// metadata directories in bundle image, package name, channels and default
// channels information in LABEL section.
func GenerateDockerfile(mediaType, manifests, metadata, copyManifestDir, copyMetadataDir, workingDir, packageName, channels, channelDefault string) ([]byte, error) {
	var fileContent string

	chanDefault, err := ValidateChannelDefault(channels, channelDefault)
	if err != nil {
		return nil, err
	}

	relativeManifestDirectory, err := filepath.Rel(workingDir, copyManifestDir)
	if err != nil {
		return nil, err
	}

	relativeMetadataDirectory, err := filepath.Rel(workingDir, copyMetadataDir)
	if err != nil {
		return nil, err
	}

	// FROM
	fileContent += "FROM scratch\n\n"

	// LABEL
	fileContent += fmt.Sprintf("LABEL %s=%s\n", MediatypeLabel, mediaType)
	fileContent += fmt.Sprintf("LABEL %s=%s\n", ManifestsLabel, manifests)
	fileContent += fmt.Sprintf("LABEL %s=%s\n", MetadataLabel, metadata)
	fileContent += fmt.Sprintf("LABEL %s=%s\n", PackageLabel, packageName)
	fileContent += fmt.Sprintf("LABEL %s=%s\n", ChannelsLabel, channels)
	fileContent += fmt.Sprintf("LABEL %s=%s\n\n", ChannelDefaultLabel, chanDefault)

	// CONTENT
	fileContent += fmt.Sprintf("COPY %s %s\n", relativeManifestDirectory, "/manifests/")
	fileContent += fmt.Sprintf("COPY %s %s\n", relativeMetadataDirectory, "/metadata/")

	return []byte(fileContent), nil
}

// Write `fileName` file with `content` into a `directory`
// Note: Will overwrite the existing `fileName` file if it exists
func WriteFile(fileName, directory string, content []byte) error {
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		err := os.MkdirAll(directory, os.ModePerm)
		if err != nil {
			return err
		}
	}

	err := ioutil.WriteFile(filepath.Join(directory, fileName), content, DefaultPermission)
	if err != nil {
		return err
	}
	return nil
}

// copy the contents of a potentially nested manifest dir into an output dir.
func copyManifestDir(from, to string, overwrite, deleteCopiedFiles bool, exceptForDirs ...string) error {
	fromFiles, err := ioutil.ReadDir(from)
	if err != nil {
		return err
	}

	if _, err := os.Stat(to); os.IsNotExist(err) {
		if err = os.MkdirAll(to, os.ModePerm); err != nil {
			return err
		}
	}

	for _, fromFile := range fromFiles {
		if fromFile.IsDir() {
			if slice.ContainsString(exceptForDirs, fromFile.Name(), nil) {
				continue
			}
			nestedTo := filepath.Join(to, filepath.Base(from))
			nestedFrom := filepath.Join(from, fromFile.Name())
			err = copyManifestDir(nestedFrom, nestedTo, overwrite, deleteCopiedFiles)
			if err != nil {
				return err
			}
			continue
		}

		fromFilePath := filepath.Join(from, fromFile.Name())
		contents, err := os.Open(fromFilePath)
		if err != nil {
			return err
		}
		defer func() {
			if err := contents.Close(); err != nil {
				log.Fatal(err)
			}
		}()

		toFilePath := filepath.Join(to, fromFile.Name())
		_, err = os.Stat(toFilePath)
		if err == nil && !overwrite {
			continue
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}

		toFile, err := os.Create(toFilePath)
		if err != nil {
			return err
		}
		defer func() {
			if err := toFile.Close(); err != nil {
				log.Fatal(err)
			}
		}()

		_, err = io.Copy(toFile, contents)
		if err != nil {
			return err
		}

		err = os.Chmod(toFilePath, fromFile.Mode())
		if err != nil {
			return err
		}

		if deleteCopiedFiles {
			err = os.Remove(fromFilePath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
