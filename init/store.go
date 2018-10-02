package main

import (
	"database/sql"
	"fmt"

	"github.com/operator-framework/operator-lifecycle-manager/pkg/controller/registry"
	_ "github.com/mattn/go-sqlite3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Load interface {
	AddOperatorBundle(bundleObjs []*unstructured.Unstructured) error
	AddPackageChannels(manifest registry.PackageManifest) error
}

type Query interface {

}

type Store interface {
	Load
}

type SQLStore struct {
	db        *sql.DB
}

var _ Store = &SQLStore{}

func NewSQLLiteStore(outFilename string) (*SQLStore, error) {
	db, err := sql.Open("sqlite3", outFilename) // TODO: ?immutable=true
	if err != nil {
		return nil, err
	}

	createTable := `
	CREATE TABLE operatorbundle (
		name TEXT PRIMARY KEY,  
		csv TEXT UNIQUE, 
		bundle TEXT
	);
	CREATE TABLE package (
		name TEXT PRIMARY KEY,
		default_channel TEXT,
		FOREIGN KEY(default_channel) REFERENCES channel(name)
	);
	CREATE TABLE channel (
		name TEXT, 
		package_name TEXT, 
		operatorbundle_name TEXT,
		PRIMARY KEY(name, package_name),
		FOREIGN KEY(package_name) REFERENCES package(name),
		FOREIGN KEY(operatorbundle_name) REFERENCES operatorbundle(name)
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
	return &SQLStore{db}, nil
}

func (s *SQLStore)	AddOperatorBundle(bundleObjs []*unstructured.Unstructured) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("insert into operatorbundle(name, csv, bundle) values(?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	csvName, csvBytes, bundleBytes, err  := s.serializeBundle(bundleObjs)
	if err!=nil{
		return err
	}

	if _, err := stmt.Exec(csvName, csvBytes, bundleBytes); err!=nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLStore) AddPackageChannels(manifest registry.PackageManifest) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	addPackage, err := tx.Prepare("insert into package(name) values(?)")
	if err != nil {
		return err
	}
	defer addPackage.Close()

	addDefaultChannel, err := tx.Prepare("update package set default_channel = ? where name = ?")
	if err != nil {
		return err
	}
	defer addPackage.Close()

	if _, err := addPackage.Exec(manifest.PackageName); err!=nil {
		return err
	}

	addChannel, err := tx.Prepare("insert into channel(name, package_name, operatorbundle_name) values(?, ?, ?)")
	if err != nil {
		return err
	}
	defer addChannel.Close()

	for _, c := range manifest.Channels {
		if _, err := addChannel.Exec(c.Name, manifest.PackageName, c.CurrentCSVName); err!=nil {
			return err
		}
		if c.IsDefaultChannel(manifest) {
			if _, err := addDefaultChannel.Exec(c.Name, manifest.PackageName); err!=nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *SQLStore) Close() {
	s.db.Close()
}

func (s *SQLStore) serializeBundle(bundleObjs []*unstructured.Unstructured) (csvName string, csvBytes []byte, bundleBytes []byte, err error) {
	csvCount := 0
	for _, obj := range bundleObjs {
		objBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
		if err != nil {
			return "", nil, nil, err
		}
		bundleBytes = append(bundleBytes, objBytes...)


		if obj.GetObjectKind().GroupVersionKind().Kind == "ClusterServiceVersion" {
			csvName = obj.GetName()
			csvBytes, err = runtime.Encode(unstructured.UnstructuredJSONScheme, obj);
			if err != nil {
				return "", nil, nil, err
			}
			csvCount += 1
			if csvCount > 1 {
				return "", nil, nil, fmt.Errorf("two csvs found in one bundle")
			}
		}
	}

	return csvName, csvBytes, bundleBytes, nil
}
