package registry

import (
	"context"
	"fmt"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type ValidatingPopulator struct {
	RegistryPopulator
	newImages []*Bundle
	overwrite bool
	querier   Query
}

func NewValidatingPopulator(imagesToAdd []*Bundle, querier Query, overwrite bool, populator RegistryPopulator) RegistryPopulator {
	return &ValidatingPopulator{
		RegistryPopulator: populator,
		newImages:         imagesToAdd,
		overwrite:         overwrite,
		querier:           querier,
	}
}

func (v *ValidatingPopulator) Populate(ctx context.Context) error {
	expectedBundles, err := expectedGraphBundles(ctx, v.newImages, v.querier, v.overwrite)
	if err != nil {
		return err
	}

	if err := v.RegistryPopulator.Populate(ctx); err != nil {
		return err
	}

	return CheckForBundles(ctx, v.querier, expectedBundles)
}

// expectedGraphBundles returns a set of package-channel-bundle tuples that MUST be present following an add.
/* opm index add drops bundles that replace a channel head, and since channel head selection heuristics
* choose the bundle with the greatest semver as the channel head, any bundle that replaces such a bundle
* will be dropped from the graph following an add.
* eg: 1.0.1 <- 1.0.1-new
*
* 1.0.1-new replaces 1.0.1 but will not be chosen as the channel head because of its non-empty pre-release version.
* expectedGraphBundles gives a set of bundles (old bundles from the graphLoader and the newly added set of bundles from
* imagesToAdd) that must be present following an add to ensure no bundle is dropped.
*
* Overwritten bundles will only be verified on the channels of the newly added version.
* Any inherited channels due to addition of a new bundle on its tail bundles may not be verified
* eg:  1.0.1 (alpha) <- [1.0.2 (alpha, stable)]
* When 1.0.2 in alpha and stable channels is added replacing 1.0.1, 1.0.1's presence will only be marked as expected on
* the alpha channel, not on the inherited stable channel.
 */
func expectedGraphBundles(ctx context.Context, imagesToAdd []*Bundle, querier Query, overwrite bool) (map[string]*Package, error) {
	expectedBundles := map[string]*Package{}
	for _, bundle := range imagesToAdd {
		version, err := bundle.Version()
		if err != nil {
			return nil, err
		}
		newBundleKey := BundleKey{
			BundlePath: bundle.BundleImage,
			Version:    version,
			CsvName:    bundle.Name,
		}
		var pkg *Package
		var ok bool

		if pkg, ok = expectedBundles[bundle.Package]; !ok {
			var err error
			channelEntries, err := querier.GetChannelEntriesFromPackage(ctx, bundle.Package)
			if err != nil {
				if err != ErrPackageNotInDatabase {
					return nil, err
				}

			}
			pkg = &Package{
				Name:     bundle.Package,
				Channels: map[string]Channel{},
			}
			for _, c := range channelEntries {
				if _, ok := pkg.Channels[c.ChannelName]; !ok {
					pkg.Channels[c.ChannelName] = Channel{
						Nodes: map[BundleKey]map[BundleKey]struct{}{},
					}
				}
				pkg.Channels[c.ChannelName].Nodes[BundleKey{BundlePath: c.BundlePath, Version: c.Version, CsvName: c.BundleName}] = nil
			}
		}

		for c, channelEntries := range pkg.Channels {
			for oldBundle := range channelEntries.Nodes {
				if oldBundle.CsvName == bundle.Name {
					if overwrite {
						delete(pkg.Channels[c].Nodes, oldBundle)
						if len(pkg.Channels[c].Nodes) == 0 {
							delete(pkg.Channels, c)
						}
					} else {
						return nil, BundleImageAlreadyAddedErr{ErrorString: fmt.Sprintf("Bundle %s already exists", bundle.BundleImage)}
					}
				}
			}
		}
		for _, c := range bundle.Channels {
			if _, ok := pkg.Channels[c]; !ok {
				pkg.Channels[c] = Channel{
					Nodes: map[BundleKey]map[BundleKey]struct{}{},
				}
			}
			// This can miss out on some channels, when a new bundle has channels that the one it replaces does not.
			// eg: When bundle A in channel A replaces bundle B in channel B is added, bundle B is also added to channel A
			// but it is only expected to be in channel B, presence in channel A will be ignored
			pkg.Channels[c].Nodes[newBundleKey] = nil
		}
		expectedBundles[bundle.Package] = pkg
	}
	return expectedBundles, nil
}

// replaces mode selects highest version as channel head and
// prunes any bundles in the upgrade chain after the channel head.
// CheckForBundles checks for the presence of all bundles after a replaces-mode add.
func CheckForBundles(ctx context.Context, q Query, required map[string]*Package) error {
	var errs []error
	for _, pkg := range required {
		channelEntries, err := q.GetChannelEntriesFromPackage(ctx, pkg.Name)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to verify added bundles for package %s: %v", pkg.Name, err))
		}

		channelBundles := map[string]map[BundleKey]struct{}{}
		for _, b := range channelEntries {
			if len(channelBundles[b.ChannelName]) == 0 {
				channelBundles[b.ChannelName] = map[BundleKey]struct{}{}
			}
			channelBundles[b.ChannelName][BundleKey{BundlePath: b.BundlePath, Version: b.Version, CsvName: b.BundleName}] = struct{}{}
		}

		for channel, missing := range pkg.Channels {
			// trace replaces chain for reachable bundles
			for b := range channelBundles[channel] {
				delete(missing.Nodes, b)
			}

			for bundle := range missing.Nodes {
				// check if bundle is deprecated. Bundles readded after deprecation should not be present in index and can be ignored.
				deprecated, err := isDeprecated(ctx, q, bundle)
				if err != nil {
					if _, ok := err.(BundleNotFoundErr); !ok {
						errs = append(errs, fmt.Errorf("could not validate pruned bundle %s (%s) as deprecated: %v", bundle.CsvName, bundle.BundlePath, err))
					}
				}
				if !deprecated {
					errs = append(errs, fmt.Errorf("added bundle %s pruned from package %s, channel %s: this may be due to incorrect channel head", bundle.BundlePath, pkg.Name, channel))
				}
			}
		}
	}
	return utilerrors.NewAggregate(errs)
}

func isDeprecated(ctx context.Context, q Query, bundle BundleKey) (bool, error) {
	b, err := q.GetBundle(ctx, bundle.CsvName, bundle.Version, bundle.BundlePath)
	if err != nil {
		return false, err
	}
	for _, prop := range b.Properties {
		if prop.Type == DeprecatedType {
			return true, nil
		}
	}
	return false, nil
}
