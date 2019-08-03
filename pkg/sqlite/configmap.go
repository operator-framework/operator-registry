package sqlite

import (
	"encoding/json"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

const (
	ConfigMapCRDName     = "customResourceDefinitions"
	ConfigMapCSVName     = "clusterServiceVersions"
	ConfigMapPackageName = "packages"
)

// ConfigMapLoader loads a configmap of resources into the database
// entries under "customResourceDefinitions" will be parsed as CRDs
// entries under "clusterServiceVersions"  will be parsed as CSVs
// entries under "packages" will be parsed as Packages
type ConfigMapLoader struct {
	log           *logrus.Entry
	store         registry.Load
	configMapData map[string]string
	crds          map[registry.APIKey]*unstructured.Unstructured
}

var _ SQLPopulator = &ConfigMapLoader{}

// NewSQLLoaderForConfigMapData is useful when the operator manifest(s)
// originate from a different source than a configMap. For example, operator
// manifest(s) can be downloaded from a remote registry like quay.io.
func NewSQLLoaderForConfigMapData(logger *logrus.Entry, store registry.Load, configMapData map[string]string) *ConfigMapLoader {
	return &ConfigMapLoader{
		log:           logger,
		store:         store,
		configMapData: configMapData,
		crds:          map[registry.APIKey]*unstructured.Unstructured{},
	}
}

func NewSQLLoaderForConfigMap(store registry.Load, configMap v1.ConfigMap) *ConfigMapLoader {
	logger := logrus.WithFields(logrus.Fields{"configmap": configMap.GetName(), "ns": configMap.GetNamespace()})
	return &ConfigMapLoader{
		log:           logger,
		store:         store,
		configMapData: configMap.Data,
		crds:          map[registry.APIKey]*unstructured.Unstructured{},
	}
}

func (c *ConfigMapLoader) Populate() error {
	if err := c.populate(); err != nil {
		c.store.AddLoadError(newConfigMapLoadError(err))
	}

	return nil
}

func (c *ConfigMapLoader) populate() error {
	c.log.Info("loading CRDs")

	var errs []error
	// First load CRDs into memory; these will be added to the bundle that owns them
	if err := c.parseCRDs(); err != nil {
		errs = append(errs, err)
	}

	c.log.Info("loading Bundles")
	parsedCSVList, err := c.parseCSVs()
	if err != nil {
		errs = append(errs, err)
	}

	for _, csv := range parsedCSVList {
		c.log.WithField("csv", csv.GetName()).Debug("loading CSV")
		csvUnst, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&csv)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "error remarshaling csv %s", csv.GetName()))
			continue
		}

		bundle := registry.NewBundle(csv.GetName(), "", "", &unstructured.Unstructured{Object: csvUnst})
		ownedCRDs, _, err := csv.GetCustomResourceDefintions()
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, owned := range ownedCRDs {
			split := strings.SplitN(owned.Name, ".", 2)
			if len(split) < 2 {
				errs = append(errs, errors.Errorf("error parsing owned name %s", owned.Name))
				continue
			}

			gvk := registry.APIKey{Group: split[1], Version: owned.Version, Kind: owned.Kind, Plural: split[0]}
			crdUnst, ok := c.crds[gvk]
			if !ok {
				errs = append(errs, errors.Errorf("couldn't find owned crd %s with gvk %s in crd list", owned.Name, gvk))
				continue
			}

			bundle.Add(crdUnst)
		}

		if err := c.store.AddOperatorBundle(bundle); err != nil {
			errs = append(errs, err)
		}
	}

	c.log.Info("loading Packages")
	parsedPackageList, err := c.parsePackages()
	if err != nil {
		errs = append(errs, err)
	}

	for _, pkg := range parsedPackageList {
		c.log.WithField("package", pkg.PackageName).Debug("loading package")
		if err := c.store.AddPackageChannels(pkg); err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (c *ConfigMapLoader) parseCRDs() error {
	crdListYaml, ok := c.configMapData[ConfigMapCRDName]
	if !ok {
		return errors.Errorf("couldn't find expected key %s in configmap", ConfigMapCRDName)
	}

	crdListJSON, err := yaml.YAMLToJSON([]byte(crdListYaml))
	if err != nil {
		return errors.Wrap(err, "error loading crd list")
	}

	var parsedCRDList []v1beta1.CustomResourceDefinition
	if err := json.Unmarshal(crdListJSON, &parsedCRDList); err != nil {
		return errors.Wrap(err, "error parsing crd list")
	}

	var errs []error
	for _, crd := range parsedCRDList {
		if crd.Spec.Versions == nil && crd.Spec.Version != "" {
			crd.Spec.Versions = []v1beta1.CustomResourceDefinitionVersion{{Name: crd.Spec.Version, Served: true, Storage: true}}
		}
		for _, version := range crd.Spec.Versions {
			gvk := registry.APIKey{Group: crd.Spec.Group, Version: version.Name, Kind: crd.Spec.Names.Kind, Plural: crd.Spec.Names.Plural}
			c.log.WithField("gvk", gvk).Debug("loading CRD")
			if _, ok := c.crds[gvk]; ok {
				// Throw away duplicate CRDs; only keep the first one found
				errs = append(errs, errors.Errorf("crd %s duplicates claim to gvk %s", crd.GetName(), gvk))
				continue
			}
			crdUnst, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&crd)
			if err != nil {
				errs = append(errs, errors.Wrapf(err, "error remarshaling crd: %s", crd.GetName()))
				continue
			}
			c.crds[gvk] = &unstructured.Unstructured{Object: crdUnst}
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (c *ConfigMapLoader) parseCSVs() ([]registry.ClusterServiceVersion, error) {
	csvListYaml, ok := c.configMapData[ConfigMapCSVName]
	if !ok {
		return nil, errors.Errorf("couldn't find expected key %s in configmap", ConfigMapCSVName)
	}

	csvListJSON, err := yaml.YAMLToJSON([]byte(csvListYaml))
	if err != nil {
		return nil, errors.Wrap(err, "error loading csv list")
	}

	var parsedCSVList []registry.ClusterServiceVersion
	if err = json.Unmarshal(csvListJSON, &parsedCSVList); err != nil {
		return nil, errors.Wrap(err, "error parsing csv list")
	}

	return parsedCSVList, nil
}

func (c *ConfigMapLoader) parsePackages() ([]registry.PackageManifest, error) {
	packageListYaml, ok := c.configMapData[ConfigMapPackageName]
	if !ok {
		return nil, errors.Errorf("couldn't find expected key %s in configmap", ConfigMapPackageName)
	}

	packageListJSON, err := yaml.YAMLToJSON([]byte(packageListYaml))
	if err != nil {
		return nil, errors.Wrap(err, "error loading package list")
	}

	var parsedPackageList []registry.PackageManifest
	if err := json.Unmarshal(packageListJSON, &parsedPackageList); err != nil {
		return nil, errors.Wrap(err, "error parsing package list")
	}

	return parsedPackageList, nil
}
