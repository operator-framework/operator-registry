package sqlite

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

// Scheme is the default instance of runtime.Scheme to which types in the Kubernetes API are already registered.
var Scheme = runtime.NewScheme()

// Codecs provides access to encoding and decoding for the scheme
var Codecs = serializer.NewCodecFactory(Scheme)

func DefaultYAMLDecoder() runtime.Decoder {
	return Codecs.UniversalDeserializer()
}

func init() {
	if err := v1alpha1.AddToScheme(Scheme); err != nil {
		panic(err)
	}

	if err := v1beta1.AddToScheme(Scheme); err != nil {
		panic(err)
	}
}

type SQLPopulator interface {
	Populate() error
}

type APIKey struct {
	group   string
	version string
	kind    string
}

// DirectoryLoader loads a directory of resources into the database
// files ending in `.crd.yaml` will be parsed as CRDs
// files ending in `.clusterserviceversion.yaml` will be parsed as CSVs
// files ending in `.package.yaml` will be parsed as Packages
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

	log.Info("loading Packages")
	if err := filepath.Walk(d.directory, d.LoadPackagesWalkFunc); err != nil {
		return err
	}

	log.Info("extracting provided API information")
	if err := d.store.AddProvidedApis(); err!= nil {
		return err
	}
	return nil
}

// LoadBundleWalkFunc walks the directory. When it sees a `.clusterserviceversion.yaml` file, it
// attempts to load the surrounding files in the same directory as a bundle, and stores them in the
// db for querying
func (d *DirectoryLoader) LoadBundleWalkFunc(path string, f os.FileInfo, err error) error {
	if f == nil {
		return fmt.Errorf("Not a valid file")
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

	if !strings.HasSuffix(path, ".clusterserviceversion.yaml") {
		log.Info("skipping non-csv file")
		return nil
	}

	log.Info("found csv, loading bundle")
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("unable to load CSV from file %s: %v", path, err)
	}
	csv := v1alpha1.ClusterServiceVersion{}
	if _, _, err = scheme.Codecs.UniversalDecoder().Decode(data, nil, &csv); err != nil {
		return fmt.Errorf("could not decode contents of file %s into CSV: %v", path, err)
	}

	bundleObjs, err := d.LoadBundle(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("error loading objs in dir: %s", err.Error())
	}

	if len(bundleObjs) == 0 {
		log.Warnf("no bundle objects found")
		return nil
	}

	providedAPIsInBundle, err := d.ProvidedAPIs(bundleObjs)
	if err != nil {
		return err
	}
	if err := d.AllProvidedAPIsInBundle(&csv, providedAPIsInBundle); err != nil {
		return err
	}

	return d.store.AddOperatorBundle(bundleObjs)
}

// LoadBundle takes the directory that a CSV is in and assumes the rest of the objects in that directory
// are part of the bundle.
func (d *DirectoryLoader) LoadBundle(dir string) ([]*unstructured.Unstructured, error) {
	log := logrus.WithFields(logrus.Fields{"dir": d.directory, "load": "bundle"})
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	objs := []*unstructured.Unstructured{}
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
		data, err := ioutil.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			return nil, fmt.Errorf("unable to load bundle file %s: %v", f.Name(), err)
		}

		obj := &unstructured.Unstructured{}
		if _, _, err = DefaultYAMLDecoder().Decode(data, nil, obj); err != nil {
			return nil, fmt.Errorf("could not decode contents of file %s into object: %v", f.Name(), err)
		}
		if obj != nil {
			objs = append(objs, obj)
		}

	}
	return objs, nil
}

func (d *DirectoryLoader) ProvidedAPIs(objs []*unstructured.Unstructured) (map[APIKey]struct{}, error) {
	provided := map[APIKey]struct{}{}
	for _, o := range objs {
		if o.GetObjectKind().GroupVersionKind().Kind == "CustomResourceDefinition" {
			crd := &apiextensions.CustomResourceDefinition{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.UnstructuredContent(), crd); err != nil {
				return nil, err
			}
			for _, v := range crd.Spec.Versions {
				provided[APIKey{group: crd.Spec.Group, version: v.Name, kind: crd.Spec.Names.Kind}] = struct{}{}
			}
			if crd.Spec.Version != "" {
				provided[APIKey{group: crd.Spec.Group, version: crd.Spec.Version, kind: crd.Spec.Names.Kind}] = struct{}{}
			}
		}

		//TODO: APIServiceDefinitions
	}
	return provided, nil
}

func (d *DirectoryLoader) AllProvidedAPIsInBundle(csv *v1alpha1.ClusterServiceVersion, bundleAPIs map[APIKey]struct{}) error {
	shouldExist := make(map[APIKey]struct{}, len(csv.Spec.CustomResourceDefinitions.Owned)+len(csv.Spec.APIServiceDefinitions.Owned))
	for _, crdDef := range csv.Spec.CustomResourceDefinitions.Owned {
		parts := strings.SplitAfterN(crdDef.Name, ".", 2)
		shouldExist[APIKey{parts[1], crdDef.Version, crdDef.Kind}] = struct{}{}
	}
	//TODO: APIServiceDefinitions
	for key, _ := range shouldExist {
		if _, ok := bundleAPIs[key]; !ok {
			return fmt.Errorf("couldn't find %v in bundle", key)
		}
	}
	return nil
}

func (d *DirectoryLoader) LoadPackagesWalkFunc(path string, f os.FileInfo, err error) error {
	log := logrus.WithFields(logrus.Fields{"dir": d.directory, "file": f.Name(), "load": "package"})
	if f == nil {
		return fmt.Errorf("Not a valid file")
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

	if !strings.HasSuffix(path, ".package.yaml") {
		log.Info("skipping non-package file")
		return nil
	}

	fileReader, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("unable to load package from file %s: %v", path, err)
	}

	decoder := yaml.NewYAMLOrJSONDecoder(fileReader, 30)
	manifest := registry.PackageManifest{}
	if err = decoder.Decode(&manifest); err != nil {
		return fmt.Errorf("could not decode contents of file %s into package: %v", path, err)
	}

	if err := d.store.AddPackageChannels(manifest); err != nil {
		return fmt.Errorf("error loading package into db: %s", err.Error())
	}

	return nil
}
