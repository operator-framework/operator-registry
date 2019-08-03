package sqlite

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

const ClusterServiceVersionKind = "ClusterServiceVersion"

type SQLPopulator interface {
	Populate() error
}

// DirectoryLoader loads a directory of resources into the database
type DirectoryLoader struct {
	store     registry.Load
	directory string
}

var _ SQLPopulator = &DirectoryLoader{}

func NewSQLLoaderForDirectory(store registry.Load, directory string) *DirectoryLoader {
	return &DirectoryLoader{
		store:     store,
		directory: directory,
	}
}

func (d *DirectoryLoader) Populate() error {
	log := logrus.WithField("dir", d.directory)

	log.Info("loading Bundles")
	if err := filepath.Walk(d.directory, d.LoadBundleWalkFunc); err != nil {
		return err
	}

	log.Info("loading Packages and Entries")
	if err := filepath.Walk(d.directory, d.LoadPackagesWalkFunc); err != nil {
		return err
	}

	return nil
}

// LoadBundleWalkFunc walks the directory. When it sees a `.clusterserviceversion.yaml` file, it
// attempts to load the surrounding files in the same directory as a bundle, and stores them in the
// db for querying
func (d *DirectoryLoader) LoadBundleWalkFunc(path string, f os.FileInfo, err error) error {
	if f == nil {
		d.store.AddLoadError(newDirectoryLoadError(errors.New("invalid file")))
		return nil
	}

	log := logrus.WithFields(logrus.Fields{"dir": d.directory, "file": f.Name(), "load": "bundles"})

	if f.IsDir() {
		if strings.HasPrefix(f.Name(), ".") {
			log.Info("skipping hidden directory")
			return filepath.SkipDir
		}
		log.Info("directory")
		return nil
	}

	if strings.HasPrefix(f.Name(), ".") {
		log.Info("skipping hidden file")
		return nil
	}

	if err := d.loadBundle(log, path); err != nil {
		d.store.AddLoadError(newDirectoryLoadError(err))
	}

	return nil
}

func (d *DirectoryLoader) loadBundle(log *logrus.Entry, path string) error {
	fileReader, err := os.Open(path)
	if err != nil {
		return errors.New("unable to load file")
	}

	decoder := yaml.NewYAMLOrJSONDecoder(fileReader, 30)
	csv := unstructured.Unstructured{}

	if err = decoder.Decode(&csv); err != nil {
		return nil
	}

	if csv.GetKind() != ClusterServiceVersionKind {
		return nil
	}

	log.Info("found csv, loading bundle")
	dir := filepath.Dir(path)
	bundle := &registry.Bundle{}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	// Collect bundle objects
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

		log.Info("loading bundle file")
		path := filepath.Join(dir, f.Name())
		fileReader, err := os.Open(path)
		if err != nil {
			errs = append(errs, errors.Errorf("unable to load file %s: %v", path, err))
			continue
		}
		decoder := yaml.NewYAMLOrJSONDecoder(fileReader, 30)
		obj := &unstructured.Unstructured{}

		if err = decoder.Decode(obj); err != nil {
			errs = append(errs, errors.Errorf("could not decode contents of file %s into file: %v", path, err))
			continue
		}

		// Don't include other CSVs in the bundle
		if obj.GetKind() == "ClusterServiceVersion" && obj.GetName() != csv.GetName() {
			continue
		}

		if obj.Object != nil {
			bundle.Add(obj)
		}
	}

	if bundle.Size() == 0 {
		// This should only happen when the bundle csv wasn't picked up again when collecting its objects.
		errs = append(errs, errors.Errorf("no bundle objects found for csv %s", csv.GetName()))
		return utilerrors.NewAggregate(errs)
	}

	if err := bundle.AllProvidedAPIsInBundle(); err != nil {
		errs = append(errs, err)
	}

	if err := d.store.AddOperatorBundle(bundle); err != nil {
		errs = append(errs, err)
	}

	return errors.Wrap(utilerrors.NewAggregate(errs), "error loading bundle into db")
}

func (d *DirectoryLoader) LoadPackagesWalkFunc(path string, f os.FileInfo, err error) error {
	log := logrus.WithFields(logrus.Fields{"dir": d.directory, "file": f.Name(), "load": "package"})
	if f == nil {
		d.store.AddLoadError(newDirectoryLoadError(errors.New("invalid file")))
		return nil
	}
	if f.IsDir() {
		if strings.HasPrefix(f.Name(), ".") {
			log.Info("skipping hidden directory")
			return filepath.SkipDir
		}
		log.Info("directory")
		return nil
	}

	if strings.HasPrefix(f.Name(), ".") {
		log.Info("skipping hidden file")
		return nil
	}

	if err := d.loadPackage(log, path); err != nil {
		d.store.AddLoadError(newDirectoryLoadError(err))
	}

	return nil
}

func (d *DirectoryLoader) loadPackage(log *logrus.Entry, path string) error {
	fileReader, err := os.Open(path)
	if err != nil {
		return errors.New("unable to load package from file")
	}

	decoder := yaml.NewYAMLOrJSONDecoder(fileReader, 30)
	manifest := registry.PackageManifest{}
	if err = decoder.Decode(&manifest); err != nil {
		return errors.Wrap(err, "could not decode contents of file into package")
	}
	if manifest.PackageName == "" {
		// return errors.New("empty package name encountered")
		return nil
	}

	return errors.Wrap(d.store.AddPackageChannels(manifest), "error loading package into db")
}
