package db

import (
	"fmt"
	"github.com/asdine/storm/v3"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

type StormLoader struct {
	db *storm.DB
}

var _ registry.Load = &StormLoader{}

func NewStormLoader(db *storm.DB) *StormLoader {
	return &StormLoader{db: db}
}

type OperatorBundle struct {
	Name       string `storm:"id"`
	Csv        []byte
	Bundle     []byte
	Skiprange  string
	Version    string
	Bundlepath string
}

func (s *StormLoader) AddOperatorBundle(bundle *registry.Bundle) error {
	csvName, bundleImage, csvBytes, bundleBytes, err := bundle.Serialize()
	if err != nil {
		return err
	}

	if csvName == "" {
		return fmt.Errorf("csv name not found")
	}

	version, err := bundle.Version()
	if err != nil {
		return err
	}
	skiprange, err := bundle.SkipRange()
	if err != nil {
		return err
	}

	return s.db.Save(&OperatorBundle{
		 Name: csvName,
		 Csv: csvBytes,
		 Bundle: bundleBytes,
		 Skiprange: skiprange,
		 Version: version,
		 Bundlepath: bundleImage,
	})
}

func (s *StormLoader) AddBundlePackageChannels(manifest registry.PackageManifest, bundle registry.Bundle) error {
	panic("implement me")
}

func (s *StormLoader) AddPackageChannels(manifest registry.PackageManifest) error {
	panic("implement me")
}

func (s *StormLoader) RmPackageName(packageName string) error {
	panic("implement me")
}

func (s *StormLoader) ClearNonDefaultBundles(packageName string) error {
	panic("implement me")
}
