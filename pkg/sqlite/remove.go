package sqlite

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

type SQLRemover interface {
	Remove() error
}

type SQLOperatorCsvNamesRemover interface {
	Remove() error
}

// PackageRemover removes a package from the database
type PackageRemover struct {
	store    registry.Load
	packages string
}

type OperatorPackageVersionRemover struct {
	store       registry.Load
	packageName string
	version     string
}

var _ SQLRemover = &PackageRemover{}
var _ SQLOperatorCsvNamesRemover = &OperatorPackageVersionRemover{}

func NewSQLRemoverForPackages(store registry.Load, packages string) *PackageRemover {
	return &PackageRemover{
		store:    store,
		packages: packages,
	}
}

func NewSQLRemoverForOperatorCsvNames(store registry.Load, packageName string, version string) *OperatorPackageVersionRemover {
	return &OperatorPackageVersionRemover{
		store:       store,
		packageName: packageName,
		version:     version,
	}
}

func (d *PackageRemover) Remove() error {
	log := logrus.WithField("pkg", d.packages)

	log.Info("deleting packages")

	var errs []error
	packages := sanitizePackageList(strings.Split(d.packages, ","))
	log.Infof("packages: %s", packages)

	for _, pkg := range packages {
		if err := d.store.RemovePackage(pkg); err != nil {
			errs = append(errs, fmt.Errorf("error removing operator package %s: %s", pkg, err))
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (d *OperatorPackageVersionRemover) Remove() error {
	fields := logrus.Fields{
		"package": d.packageName,
		"version": d.version,
	}
	log := logrus.WithFields(fields)

	log.Info("deleting packages")

	var errs []error
	// packages := sanitizePackageList(strings.Split(d.operatorVersions, ","))
	log.Infof("operator package and version: %s %s", d.packageName, d.version)

	if err := d.store.RemoveBundleByVersion(d.packageName, d.version); err != nil {
		errs = append(errs, fmt.Errorf("error removing operator bundle %s %s: %s", d.packageName, d.version, err))
	}

	return utilerrors.NewAggregate(errs)
}

// sanitizePackageList sanitizes the set of package(s) specified. It removes
// duplicates and ignores empty string.
func sanitizePackageList(in []string) []string {
	out := make([]string, 0)

	inMap := map[string]bool{}
	for _, item := range in {
		if _, ok := inMap[item]; ok || item == "" {
			continue
		}

		inMap[item] = true
		out = append(out, item)
	}

	return out
}
