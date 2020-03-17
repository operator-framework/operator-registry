package sqlite

import (
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"path/filepath"
	"strings"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/registry"
	log "github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"io/ioutil"
	"os"
)

const bundleTempDirName = "bundle_tmp_"

// MultiImageLoader loads multiple bundle images into the database.
// It builds a graph between the new bundles and those already present in the database.
type MultiImageLoader struct {
	store         registry.Load
	images        bundleImages
	directories   map[string]string // maps images to directories on the filesystem - for unpacking
	bundles       map[string]string // maps bundle.Name to images- for setting bundle.BundleImage
	containerTool string
}

type bundleImages []string

func (b bundleImages) String() string {
	var output string
	for _, bundle := range b {
		output = fmt.Sprint(output, bundle, " ")
	}
	return output
}

var _ SQLPopulator = &MultiImageLoader{}

func NewSQLLoaderForMultiImage(store registry.Load, bundles []string, containerTool string) *MultiImageLoader {
	return &MultiImageLoader{
		store:         store,
		images:        bundles,
		containerTool: containerTool,
	}
}

func (m *MultiImageLoader) Populate() error {
	// get image data for each image down onto disk
	log.Info("populating multi-image")
	log.Info("images provided: ", m.images.String())

	for _, image := range m.images {
		logger := log.WithField("image", image)
		dirName := fmt.Sprint(bundleTempDirName, image)
		m.directories[image] = dirName

		workingDir, err := ioutil.TempDir("./", dirName)
		if err != nil {
			return err
		}
		// Pull the image and get the manifests by writing image data to disk
		reader := containertools.NewImageReader(m.containerTool, logger)
		err = reader.GetImageData(image, workingDir)
		if err != nil {
			return err
		}
	}

	// unpack bundles from disk and get all relevant data to build the graph
	// first get all annotation.yaml files from the directory
	log.Infof("unpacking bundles %s", m.images.String())
	errs := make([]error, 0)

	annotations, err := m.loadAnnotations()
	if err != nil {
		errs = append(errs, err)
	}

	// then get bundle data out by using the annotations
	bundles, err := m.loadBundles()
	if err != nil {
		errs = append(errs, err)
	}

	// get csvs from the bundles
	csvs, err := m.loadCSV(bundles)
	if err != nil {
		errs = append(errs, err)
	}

	// get packagemanifests
	packageManifests, err := m.loadPackageManifests(annotations, csvs)
	if err != nil {
		errs = append(errs, err)
	}

	// start analyzing data to build graph
	// if bundle images are unrelated - insert into DB
	// if bundle images are semver - insert into DB
	// TODO

	// cleanup bundles afterwards
	for _, image := range m.images {
		err := os.RemoveAll(m.directories[image])
		if err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

// LoadAnnotations walks the bundle directory for each directory. Looks for the metadata and manifests
// sub-directories to find the annotations.yaml file that will inform how the
// manifests of the bundle should be loaded into the database. It returns the annotation files themselves.
func (m *MultiImageLoader) loadAnnotations() ([]*registry.AnnotationsFile, error) {
	var annotations []*registry.AnnotationsFile

	for _, image := range m.images {
		path := m.directories[image]
		metadataPath := filepath.Join(path, "metadata")

		// Get annotations file
		logger := log.WithFields(log.Fields{"dir": path, "file": metadataPath, "load": "annotations"})
		files, err := ioutil.ReadDir(metadataPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read directory %s: %s", metadataPath, err)
		}

		annotationsFile := &registry.AnnotationsFile{}
		for _, f := range files {
			fileReader, err := os.Open(filepath.Join(metadataPath, f.Name()))
			if err != nil {
				return nil, fmt.Errorf("unable to read file %s: %s", f.Name(), err)
			}
			decoder := yaml.NewYAMLOrJSONDecoder(fileReader, 30)
			err = decoder.Decode(&annotationsFile)
			if err != nil || *annotationsFile == (registry.AnnotationsFile{}) {
				continue
			} else {
				logger.Info("found annotations file searching for csv")
			}
		}

		if *annotationsFile == (registry.AnnotationsFile{}) {
			return nil, fmt.Errorf("Could not find annotations.yaml file")
		}

		annotations = append(annotations, annotationsFile)
	}

	return annotations, nil
}

func (m *MultiImageLoader) loadBundles() ([]*registry.Bundle, error) {
	var bundles []*registry.Bundle

	for _, image := range m.images {
		//logger := log.WithFields(log.Fields{"dir": m.directories[image], "file": manifestsPath, "load": "bundle"})

		csv, err := m.findCSV(image)
		if err != nil {
			return nil, err
		}

		if csv.Object == nil {
			return nil, fmt.Errorf("csv is empty: %s", err)
		}

		log.Info("found csv, loading bundle")

		// TODO: Check channels against what's in the database vs in the bundle csv

		bundle, err := loadBundle(csv.GetName(), filepath.Join(m.directories[image], "manifests"))
		if err != nil {
			return nil, fmt.Errorf("error loading objs in directory: %s", err)
		}

		if bundle == nil || bundle.Size() == 0 {
			return nil, fmt.Errorf("no bundle objects found")
		}

		m.bundles[bundle.Name] = image
		bundles = append(bundles, bundle)
	}

	return bundles, nil
}

// findCSV looks through the bundle directory to find a csv
func (m *MultiImageLoader) findCSV(image string) (*unstructured.Unstructured, error) {
	path := m.directories[image]
	manifests := filepath.Join(path, "manifests")
	logger := log.WithFields(log.Fields{"dir": path, "find": "csv"})

	files, err := ioutil.ReadDir(manifests)
	if err != nil {
		return nil, fmt.Errorf("unable to read directory %s: %s", manifests, err)
	}

	var errs []error
	for _, f := range files {
		logger = logger.WithField("file", f.Name())
		if f.IsDir() {
			log.Info("skipping directory")
			continue
		}

		if strings.HasPrefix(f.Name(), ".") {
			log.Info("skipping hidden file")
			continue
		}

		path := filepath.Join(manifests, f.Name())
		fileReader, err := os.Open(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to read file %s: %s", path, err))
			continue
		}

		dec := yaml.NewYAMLOrJSONDecoder(fileReader, 30)
		unst := &unstructured.Unstructured{}
		if err := dec.Decode(unst); err != nil {
			continue
		}

		if unst.GetKind() != ClusterServiceVersionKind {
			continue
		}

		return unst, nil

	}

	errs = append(errs, fmt.Errorf("no csv found in bundle"))
	return nil, utilerrors.NewAggregate(errs)
}

func (m *MultiImageLoader) loadCSV(bundles []*registry.Bundle) ([]*registry.ClusterServiceVersion, error) {
	var csvs []*registry.ClusterServiceVersion

	for _, bundle := range bundles {
		// set the bundleimage on the bundle
		bundle.BundleImage = m.bundles[bundle.Name]

		if err := bundle.AllProvidedAPIsInBundle(); err != nil {
			return nil, fmt.Errorf("error checking provided apis in bundle %s: %s", bundle.Name, err)
		}

		bcsv, err := bundle.ClusterServiceVersion()
		if err != nil {
			return nil, fmt.Errorf("error getting csv from bundle %s: %s", bundle.Name, err)
		}

		csvs = append(csvs, bcsv)
	}

	return csvs, nil
}

// loadPackageManifests takes in annotations and csvs and returns package manifest files
func (m *MultiImageLoader) loadPackageManifests(annotations []*registry.AnnotationsFile, csvs []*registry.ClusterServiceVersion) ([]*registry.PackageManifest, error) {
	manifests := []*registry.PackageManifest{}

	// TODO this is probably not correct
	for _, annotation := range annotations {
		for _, csv := range csvs {
			channels := []registry.PackageChannel{}
			for _, ch := range annotation.GetChannels() {
				channels = append(channels,
					registry.PackageChannel{
						Name:           ch,
						CurrentCSVName: csv.GetName(),
					})
			}
			manifest := registry.PackageManifest{
				PackageName:        annotation.GetName(),
				DefaultChannelName: annotation.GetDefaultChannelName(),
				Channels:           channels,
			}
			manifests = append(manifests, &manifest)
		}
	}

	return manifests, nil
}
