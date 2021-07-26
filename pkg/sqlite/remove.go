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
	CsvToRemove string
	CsvToSave   *string
}

var _ SQLRemover = &PackageRemover{}
var _ SQLOperatorCsvNamesRemover = &OperatorPackageVersionRemover{}

func NewSQLRemoverForPackages(store registry.Load, packages string) *PackageRemover {
	return &PackageRemover{
		store:    store,
		packages: packages,
	}
}

func NewSQLRemoverForOperatorCsvNames(store registry.Load, csvToRemove string, csvToSave *string) *OperatorPackageVersionRemover {
	return &OperatorPackageVersionRemover{
		store:       store,
		CsvToRemove: csvToRemove,
		CsvToSave:   csvToSave,
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
		"csv": d.CsvToRemove,
	}
	log := logrus.WithFields(fields)

	log.Infof("deleting package version %s", d.CsvToRemove)
	if d.CsvToSave != nil {
		log.Infof("replacing with %s as head in channel", *d.CsvToSave)
	}

	var errs []error

	if err := d.store.RemoveBundle(d.CsvToRemove, d.CsvToSave); err != nil {
		errs = append(errs, fmt.Errorf("error removing operator bundle %s: %s", d.CsvToRemove, err))
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
