package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

type SQLDeprecator interface {
	Deprecate() error
}

// BundleDeprecator removes bundles from the database
type BundleDeprecator struct {
	store   registry.Load
	querier registry.Query
	bundles []string
}

var _ SQLDeprecator = &BundleDeprecator{}

func NewSQLDeprecatorForBundles(store registry.Load, querier registry.Query, bundles []string) *BundleDeprecator {
	return &BundleDeprecator{
		store:   store,
		querier: querier,
		bundles: bundles,
	}
}

func (d *BundleDeprecator) Deprecate() error {
	log := logrus.WithField("bundles", d.bundles)

	log.Info("deprecating bundles")

	var errs []error
	type nameVersion struct {
		name    string
		version string
	}
	bundleInfo := make(map[string]nameVersion)
	// Check if all bundlepaths are valid
	for _, bundlePath := range d.bundles {
		name, version, err := d.querier.GetBundleNameAndVersionForImage(context.TODO(), bundlePath)
		if err != nil {
			errs = append(errs, fmt.Errorf("error deprecating bundle %s: %s", bundlePath, err))
		}
		bundleInfo[bundlePath] = nameVersion{name, version}
	}

	if len(errs) != 0 {
		return utilerrors.NewAggregate(errs)
	}

	for bundlePath, bundle := range bundleInfo {
		// verify that bundle is still present
		_, _, err := d.querier.GetBundleNameAndVersionForImage(context.TODO(), bundlePath)
		if err != nil {
			if !errors.Is(err, registry.ErrBundleImageNotInDatabase) {
				errs = append(errs, fmt.Errorf("error deprecating bundle %s: %s", bundlePath, err))
			}
			continue
		}
		if err := d.store.DeprecateBundle(bundle.name, bundle.version, bundlePath); err != nil {
			if !errors.Is(err, registry.ErrRemovingDefaultChannelDuringDeprecation) {
				return utilerrors.NewAggregate(append(errs, fmt.Errorf("error deprecating bundle %s: %s", bundlePath, err)))
			}
			errs = append(errs, fmt.Errorf("error deprecating bundle %s: %s", bundlePath, err))
		}
	}

	return utilerrors.NewAggregate(errs)
}
