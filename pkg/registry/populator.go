package registry

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/pkg/image"
)

// DirectoryPopulator loads an unpacked operator bundle from a directory into the database.
type DirectoryPopulator struct {
	loader Load
	to     image.Reference
	from   string
}

func NewDirectoryPopulator(loader Load, to image.Reference, from string) *DirectoryPopulator {
	return &DirectoryPopulator{
		loader: loader,
		to:     to,
		from:   from,
	}
}

func (i *DirectoryPopulator) Populate() error {
	path := i.from
	manifests := filepath.Join(path, "manifests")
	metadata := filepath.Join(path, "metadata")
	// Get annotations file
	log := logrus.WithFields(logrus.Fields{"dir": i.from, "file": metadata, "load": "annotations"})
	files, err := ioutil.ReadDir(metadata)
	if err != nil {
		return fmt.Errorf("unable to read directory %s: %s", metadata, err)
	}

	// Look for the metadata and manifests sub-directories to find the annotations.yaml file that will inform how the
	// manifests of the bundle should be loaded into the database.
	annotationsFile := &AnnotationsFile{}
	for _, f := range files {
		fileReader, err := os.Open(filepath.Join(metadata, f.Name()))
		if err != nil {
			return fmt.Errorf("unable to read file %s: %s", f.Name(), err)
		}
		decoder := yaml.NewYAMLOrJSONDecoder(fileReader, 30)
		err = decoder.Decode(&annotationsFile)
		if err != nil || *annotationsFile == (AnnotationsFile{}) {
			continue
		} else {
			log.Info("found annotations file searching for csv")
		}
	}

	if *annotationsFile == (AnnotationsFile{}) {
		return fmt.Errorf("Could not find annotations.yaml file")
	}

	err = i.loadManifests(manifests, annotationsFile)
	if err != nil {
		return err
	}

	return nil
}

func (i *DirectoryPopulator) loadManifests(manifests string, annotationsFile *AnnotationsFile) error {
	log := logrus.WithFields(logrus.Fields{"dir": i.from, "file": manifests, "load": "bundle"})

	csv, err := i.findCSV(manifests)
	if err != nil {
		return err
	}

	if csv.Object == nil {
		return fmt.Errorf("csv is empty: %s", err)
	}

	log.Info("found csv, loading bundle")

	// TODO: Check channels against what's in the database vs in the bundle csv

	bundle, err := loadBundle(csv.GetName(), manifests)
	if err != nil {
		return fmt.Errorf("error loading objs in directory: %s", err)
	}

	if bundle == nil || bundle.Size() == 0 {
		return fmt.Errorf("no bundle objects found")
	}

	// set the bundleimage on the bundle
	bundle.BundleImage = i.to.String()

	if err := bundle.AllProvidedAPIsInBundle(); err != nil {
		return fmt.Errorf("error checking provided apis in bundle %s: %s", bundle.Name, err)
	}

	bcsv, err := bundle.ClusterServiceVersion()
	if err != nil {
		return fmt.Errorf("error getting csv from bundle %s: %s", bundle.Name, err)
	}

	packageManifest, err := translateAnnotationsIntoPackage(annotationsFile, bcsv)
	if err != nil {
		return fmt.Errorf("Could not translate annotations file into packageManifest %s", err)
	}

	if err := i.loadOperatorBundle(packageManifest, bundle); err != nil {
		return fmt.Errorf("Error adding package %s", err)
	}

	// Finally let's delete all the old bundles
	if err = i.loader.ClearNonDefaultBundles(packageManifest.PackageName); err != nil {
		return fmt.Errorf("Error deleting previous bundles: %s", err)
	}

	return nil
}

// loadBundle takes the directory that a CSV is in and assumes the rest of the objects in that directory
// are part of the bundle.
func loadBundle(csvName string, dir string) (*Bundle, error) {
	log := logrus.WithFields(logrus.Fields{"dir": dir, "load": "bundle"})
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var errs []error
	bundle := &Bundle{}
	for _, f := range files {
		log = log.WithField("file", f.Name())
		if f.IsDir() {
			log.Info("skipping directory")
			continue
		}

		if strings.HasPrefix(f.Name(), ".") {
			log.Info("skipping hidden file")
			continue
		}

		log.Info("loading bundle file")
		path := filepath.Join(dir, f.Name())
		fileReader, err := os.Open(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to load file %s: %s", path, err))
			continue
		}

		decoder := yaml.NewYAMLOrJSONDecoder(fileReader, 30)
		obj := &unstructured.Unstructured{}
		if err = decoder.Decode(obj); err != nil {
			logrus.WithError(err).Debugf("could not decode file contents for %s", path)
			continue
		}

		// Don't include other CSVs in the bundle
		if obj.GetKind() == "ClusterServiceVersion" && obj.GetName() != csvName {
			continue
		}

		if obj.Object != nil {
			bundle.Add(obj)
		}
	}

	return bundle, utilerrors.NewAggregate(errs)
}

// findCSV looks through the bundle directory to find a csv
func (i *DirectoryPopulator) findCSV(manifests string) (*unstructured.Unstructured, error) {
	log := logrus.WithFields(logrus.Fields{"dir": i.from, "find": "csv"})

	files, err := ioutil.ReadDir(manifests)
	if err != nil {
		return nil, fmt.Errorf("unable to read directory %s: %s", manifests, err)
	}

	var errs []error
	for _, f := range files {
		log = log.WithField("file", f.Name())
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

		if unst.GetKind() != clusterServiceVersionKind {
			continue
		}

		return unst, nil

	}

	errs = append(errs, fmt.Errorf("no csv found in bundle"))
	return nil, utilerrors.NewAggregate(errs)
}

// loadOperatorBundle adds the package information to the loader's store
func (i *DirectoryPopulator) loadOperatorBundle(manifest PackageManifest, bundle *Bundle) error {
	if manifest.PackageName == "" {
		return nil
	}

	if err := i.loader.AddBundlePackageChannels(manifest, bundle); err != nil {
		return fmt.Errorf("error loading bundle into db: %s", err)
	}

	return nil
}

// translateAnnotationsIntoPackage attempts to translate the channels.yaml file at the given path into a package.yaml
func translateAnnotationsIntoPackage(annotations *AnnotationsFile, csv *ClusterServiceVersion) (PackageManifest, error) {
	manifest := PackageManifest{}

	channels := []PackageChannel{}
	for _, ch := range annotations.GetChannels() {
		channels = append(channels,
			PackageChannel{
				Name:           ch,
				CurrentCSVName: csv.GetName(),
			})
	}

	manifest = PackageManifest{
		PackageName:        annotations.GetName(),
		DefaultChannelName: annotations.GetDefaultChannelName(),
		Channels:           channels,
	}

	return manifest, nil
}
