package main

import (
	"database/sql"
	"fmt"
	"github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"io/ioutil"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// Scheme is the default instance of runtime.Scheme to which types in the Kubernetes API are already registered.
var Scheme = runtime.NewScheme()

// Codecs provides access to encoding and decoding for the scheme
var Codecs = serializer.NewCodecFactory(Scheme)

// ParameterCodec handles versioning of objects that are converted to query parameters.
var ParameterCodec = runtime.NewParameterCodec(Scheme)

// DefaultJSONEncoder returns a default encoder for our scheme
func DefaultJSONEncoder() runtime.Encoder {
	return unstructured.JSONFallbackEncoder{Encoder: Codecs.LegacyCodec(Scheme.PrioritizedVersionsAllGroups()...)}
}

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

func NewSQLLiteDB(outFilename string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", outFilename) // TODO: ?immutable=true
	if err != nil {
		return nil, err
	}

	createTable := `
	CREATE TABLE operatorbundle (
		id   INTEGER PRIMARY KEY, 
		name TEXT UNIQUE,  
		csv TEXT UNIQUE, 
		bundle TEXT
	);
	CREATE TABLE package (
		id   INTEGER PRIMARY KEY, 
		name TEXT UNIQUE
	);
	CREATE TABLE channel (
        id INTEGER PRIMARY KEY,
		name TEXT UNIQUE, 
		package_id INTEGER, 
		operatorbundle_id INTEGER,
		FOREIGN KEY(package_id) REFERENCES package(id),
		FOREIGN KEY(operatorbundle_id) REFERENCES operatorbundle(id)
	);
	CREATE INDEX replaces ON operatorbundle(json_extract(csv, '$.spec.replaces'));
	`
	// what csv does this one replace?
	//	sqlquery := `
	//  SELECT DISTINCT json_extract(operatorbundle.csv, '$.spec.replaces')
	//  FROM operatorbundle,json_tree(operatorbundle.csv)
	//  WHERE operatorbundle.name IS "etcdoperator.v0.9.2"
	//`

	// what replaces this CSV?
	//sqlquery := `
	//SELECT DISTINCT operatorbundle.name
	//FROM operatorbundle,json_tree(operatorbundle.csv, '$.spec.replaces') WHERE json_tree.value = "etcdoperator.v0.9.0"
	//`

	// what apis does this csv provide?
	//sqlquery := `
	//SELECT DISTINCT json_extract(json_each.value, '$.name', '$.version', '$.kind')
	//FROM operatorbundle,json_each(operatorbundle.csv, '$.spec.customresourcedefinitions.owned')
	//WHERE operatorbundle.name IS "etcdoperator.v0.9.2"
	//`

	// what csvs provide this api?
	//sqlquery := `
	//SELECT DISTINCT operatorbundle.name
	//FROM operatorbundle,json_each(operatorbundle.csv, '$.spec.customresourcedefinitions.owned')
	//WHERE json_extract(json_each.value, '$.name') = "etcdclusters.etcd.database.coreos.com"
	//AND  json_extract(json_each.value, '$.version') =  "v1beta2"
	//AND json_extract(json_each.value, '$.kind') = "EtcdCluster"
	//`

	if _, err = db.Exec(createTable); err != nil {
		return nil, err
	}
	return db, nil
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
	db        *sql.DB
	directory string
	bundleInsert *sql.Stmt
	findCSVByName *sql.Stmt
	getReplacesName *sql.Stmt
	addReplacesRef *sql.Stmt
}

var _ SQLPopulator = &DirectoryLoader{}

func NewSQLLoaderForDirectory(db *sql.DB, directory string) *DirectoryLoader {
	return &DirectoryLoader{
		db:        db,
		directory: directory,
	}
}

func (d *DirectoryLoader) Populate() error {
	log := logrus.WithField("dir", d.directory)
	log.Info("loading CSVs")

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("insert into operatorbundle(name, csv, bundle) values(?, ?, ?)")
	if err != nil {
		return err
	}
	d.bundleInsert = stmt
	defer stmt.Close()

	if err := filepath.Walk(d.directory, d.LoadCSVsWalkFunc); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	//log.Info("loading Packages")
	//if err := filepath.Walk(d.directory, d.LoadPackagesWalkFunc); err != nil {
	//	return err
	//}
	return nil
}

func (d *DirectoryLoader) LoadCSVsWalkFunc(path string, f os.FileInfo, err error) error {
	if f == nil {
		return fmt.Errorf("Not a valid file")
	}

	log := logrus.WithFields(logrus.Fields{"dir": d.directory, "file": f.Name()})

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

	csvBytes, bundleBytes, err := d.BundleBytes(bundleObjs)
	if err != nil {
		return err
	}

	_, err = d.bundleInsert.Exec(csv.Name, csvBytes, bundleBytes)
	if err != nil {
		return err
	}

	return nil
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
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.UnstructuredContent(), crd); err!= nil {
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
	shouldExist := make(map[APIKey]struct{}, len(csv.Spec.CustomResourceDefinitions.Owned) + len(csv.Spec.APIServiceDefinitions.Owned))
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

func (d *DirectoryLoader) BundleBytes(bundleObjs []*unstructured.Unstructured) ([]byte, []byte, error) {
	var bundleBytes []byte
	var csvBytes []byte

	csvCount := 0
	for _, obj := range bundleObjs {
		objBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
		if err!= nil {
			return nil, nil, err
		}
		bundleBytes = append(bundleBytes, objBytes...)


		if obj.GetObjectKind().GroupVersionKind().Kind == "ClusterServiceVersion" {
			csvBytes, err = runtime.Encode(unstructured.UnstructuredJSONScheme, obj);
			if err != nil {
				return nil, nil, err
			}
			csvCount += 1
			if csvCount > 1 {
				return nil, nil, fmt.Errorf("two csvs found in one bundle")
			}
		}
	}

	return csvBytes, bundleBytes, nil
}

//func (d *DirectoryLoader) LoadPackagesWalkFunc(path string, f os.FileInfo, err error) error {
//	log.Debugf("Load Package     -- BEGIN %s", path)
//	if f == nil {
//		return fmt.Errorf("Not a valid file")
//	}
//	if f.IsDir() {
//		if strings.HasPrefix(f.Name(), ".") {
//			log.Debugf("Load Package     -- SKIPHIDDEN %s", path)
//			return filepath.SkipDir
//		}
//		log.Debugf("Load Package     -- ISDIR %s", path)
//		return nil
//	}
//	if strings.HasPrefix(f.Name(), ".") {
//		log.Debugf("Load Package     -- SKIPHIDDEN %s", path)
//		return nil
//	}
//	if strings.HasSuffix(path, ".package.yaml") {
//		pkg, err := LoadPackageFromFile(d.Catalog, path)
//		if err != nil {
//			log.Debugf("Load Package     -- ERROR %s", path)
//			return err
//		}
//		log.Debugf("Load Package     -- OK    %s", pkg.PackageName)
//	}
//	return nil
//}
