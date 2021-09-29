package registry

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/operator-framework/operator-registry/pkg/image/execregistry"
	"github.com/operator-framework/operator-registry/pkg/lib/certs"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

type RegistryUpdater struct {
	Logger *logrus.Entry
}

type AddToRegistryRequest struct {
	Permissive    bool
	SkipTLS       bool
	CaFile        string
	InputDatabase string
	Bundles       []string
	Mode          registry.Mode
	ContainerTool containertools.ContainerTool
	Overwrite     bool
	EnableAlpha   bool
}

func (r RegistryUpdater) AddToRegistry(request AddToRegistryRequest) error {
	db, err := sqlite.Open(request.InputDatabase)
	if err != nil {
		return err
	}
	defer db.Close()

	dbLoader, err := sqlite.NewSQLLiteLoader(db, sqlite.WithEnableAlpha(request.EnableAlpha))
	if err != nil {
		return err
	}

	if err := dbLoader.Migrate(context.TODO()); err != nil {
		return err
	}

	graphLoader, err := sqlite.NewSQLGraphLoaderFromDB(db)
	if err != nil {
		return err
	}
	dbQuerier := sqlite.NewSQLLiteQuerierFromDb(db)

	// add custom ca certs to resolver

	var reg image.Registry
	var rerr error
	switch request.ContainerTool {
	case containertools.NoneTool:
		rootCAs, err := certs.RootCAs(request.CaFile)
		if err != nil {
			return fmt.Errorf("failed to get RootCAs: %v", err)
		}
		reg, rerr = containerdregistry.NewRegistry(containerdregistry.SkipTLS(request.SkipTLS), containerdregistry.WithRootCAs(rootCAs))
	case containertools.PodmanTool:
		fallthrough
	case containertools.DockerTool:
		reg, rerr = execregistry.NewRegistry(request.ContainerTool, r.Logger, containertools.SkipTLS(request.SkipTLS))
	}
	if rerr != nil {
		return rerr
	}
	defer func() {
		if err := reg.Destroy(); err != nil {
			r.Logger.WithError(err).Warn("error destroying local cache")
		}
	}()

	simpleRefs := make([]image.Reference, 0)
	for _, ref := range request.Bundles {
		simpleRefs = append(simpleRefs, image.SimpleReference(ref))
	}

	if err := populate(context.TODO(), dbLoader, graphLoader, dbQuerier, reg, simpleRefs, request.Mode, request.Overwrite); err != nil {
		r.Logger.Debugf("unable to populate database: %s", err)

		if !request.Permissive {
			r.Logger.WithError(err).Error("permissive mode disabled")
			return err
		}
		r.Logger.WithError(err).Warn("permissive mode enabled")
	}

	return nil
}

func unpackImage(ctx context.Context, reg image.Registry, ref image.Reference) (image.Reference, string, func(), error) {
	var errs []error
	workingDir, err := ioutil.TempDir("./", "bundle_tmp")
	if err != nil {
		errs = append(errs, err)
	}

	if err = reg.Pull(ctx, ref); err != nil {
		errs = append(errs, err)
	}

	if err = reg.Unpack(ctx, ref, workingDir); err != nil {
		errs = append(errs, err)
	}

	cleanup := func() {
		if err := os.RemoveAll(workingDir); err != nil {
			logrus.Error(err)
		}
	}

	if len(errs) > 0 {
		return nil, "", cleanup, utilerrors.NewAggregate(errs)
	}
	return ref, workingDir, cleanup, nil
}

func populate(ctx context.Context, loader registry.Load, graphLoader registry.GraphLoader, querier registry.Query, reg image.Registry, refs []image.Reference, mode registry.Mode, overwrite bool) error {
	unpackedImageMap := make(map[image.Reference]string, 0)
	overwrittenBundles := map[string][]string{}
	var imagesToAdd []*registry.Bundle
	for _, ref := range refs {
		to, from, cleanup, err := unpackImage(ctx, reg, ref)
		if err != nil {
			return err
		}
		unpackedImageMap[to] = from
		defer cleanup()

		img, err := registry.NewImageInput(to, from)
		if err != nil {
			return err
		}
		imagesToAdd = append(imagesToAdd, img.Bundle)

		if overwrite {
			overwritten, err := querier.GetBundlePathIfExists(ctx, img.Bundle.Name)
			if err != nil {
				if err == registry.ErrBundleImageNotInDatabase {
					continue
				}
				return err
			}
			if overwritten == "" {
				return fmt.Errorf("index add --overwrite-latest is only supported when using bundle images")
			}
			overwrittenBundles[img.Bundle.Package] = append(overwrittenBundles[img.Bundle.Package], img.Bundle.Name)
		}
	}

	expectedBundles, err := expectedGraphBundles(imagesToAdd, graphLoader, overwrite)
	if err != nil {

		return err

	}
	populator := registry.NewDirectoryPopulator(loader, graphLoader, querier, unpackedImageMap, overwrittenBundles)

	if err := populator.Populate(mode); err != nil {

		return err

	}
	return checkForBundles(ctx, querier.(*sqlite.SQLQuerier), graphLoader, expectedBundles)
}

type DeleteFromRegistryRequest struct {
	Permissive    bool
	InputDatabase string
	Packages      []string
}

func (r RegistryUpdater) DeleteFromRegistry(request DeleteFromRegistryRequest) error {
	db, err := sqlite.Open(request.InputDatabase)
	if err != nil {
		return err
	}
	defer db.Close()

	dbLoader, err := sqlite.NewDeprecationAwareLoader(db)
	if err != nil {
		return err
	}
	if err := dbLoader.Migrate(context.TODO()); err != nil {
		return err
	}

	for _, pkg := range request.Packages {
		remover := sqlite.NewSQLRemoverForPackages(dbLoader, pkg)
		if err := remover.Remove(); err != nil {
			err = fmt.Errorf("error deleting packages from database: %s", err)
			if !request.Permissive {
				logrus.WithError(err).Fatal("permissive mode disabled")
				return err
			}
			logrus.WithError(err).Warn("permissive mode enabled")
		}
	}

	// remove any stranded bundles from the database
	// TODO: This is unnecessary if the db schema can prevent this orphaned data from existing
	remover := sqlite.NewSQLStrandedBundleRemover(dbLoader)
	if err := remover.Remove(); err != nil {
		return fmt.Errorf("error removing stranded packages from database: %s", err)
	}

	return nil
}

type PruneStrandedFromRegistryRequest struct {
	InputDatabase string
}

func (r RegistryUpdater) PruneStrandedFromRegistry(request PruneStrandedFromRegistryRequest) error {
	db, err := sqlite.Open(request.InputDatabase)
	if err != nil {
		return err
	}
	defer db.Close()

	dbLoader, err := sqlite.NewSQLLiteLoader(db)
	if err != nil {
		return err
	}
	if err := dbLoader.Migrate(context.TODO()); err != nil {
		return err
	}

	remover := sqlite.NewSQLStrandedBundleRemover(dbLoader)
	if err := remover.Remove(); err != nil {
		return fmt.Errorf("error removing stranded packages from database: %s", err)
	}

	return nil
}

type PruneFromRegistryRequest struct {
	Permissive    bool
	InputDatabase string
	Packages      []string
}

func (r RegistryUpdater) PruneFromRegistry(request PruneFromRegistryRequest) error {
	db, err := sqlite.Open(request.InputDatabase)
	if err != nil {
		return err
	}
	defer db.Close()

	dbLoader, err := sqlite.NewDeprecationAwareLoader(db)
	if err != nil {
		return err
	}
	if err := dbLoader.Migrate(context.TODO()); err != nil {
		return err
	}

	// get all the packages
	lister := sqlite.NewSQLLiteQuerierFromDb(db)
	packages, err := lister.ListPackages(context.TODO())
	if err != nil {
		return err
	}

	// make it inexpensive to find packages
	pkgMap := make(map[string]bool)
	for _, pkg := range request.Packages {
		pkgMap[pkg] = true
	}

	// prune packages from registry
	for _, pkg := range packages {
		if _, found := pkgMap[pkg]; !found {
			remover := sqlite.NewSQLRemoverForPackages(dbLoader, pkg)
			if err := remover.Remove(); err != nil {
				err = fmt.Errorf("error deleting packages from database: %s", err)
				if !request.Permissive {
					logrus.WithError(err).Fatal("permissive mode disabled")
					return err
				}
				logrus.WithError(err).Warn("permissive mode enabled")
			}
		}
	}

	return nil
}

type DeprecateFromRegistryRequest struct {
	Permissive          bool
	InputDatabase       string
	Bundles             []string
	AllowPackageRemoval bool
}

func (r RegistryUpdater) DeprecateFromRegistry(request DeprecateFromRegistryRequest) error {
	db, err := sqlite.Open(request.InputDatabase)
	if err != nil {
		return err
	}
	defer db.Close()

	dbLoader, err := sqlite.NewDeprecationAwareLoader(db)
	if err != nil {
		return err
	}
	if err := dbLoader.Migrate(context.TODO()); err != nil {
		return fmt.Errorf("unable to migrate database: %s", err)
	}

	// Check if all bundlepaths are valid
	var toDeprecate []string

	dbQuerier := sqlite.NewSQLLiteQuerierFromDb(db)

	toDeprecate, _, err = checkForBundlePaths(dbQuerier, request.Bundles)
	if err != nil {
		if !request.Permissive {
			r.Logger.WithError(err).Error("permissive mode disabled")
			return err
		}
		r.Logger.WithError(err).Warn("permissive mode enabled")
	}

	deprecator := sqlite.NewSQLDeprecatorForBundles(dbLoader, toDeprecate)

	// Check for deprecation of head of default channel. If deprecation request includes heads of all other channels,
	// then remove the package entirely. Otherwise, deprecate provided bundles. This enables deprecating an entire package.
	// By default deprecating the head of default channel is not permitted.
	if request.AllowPackageRemoval {
		packageDeprecator := sqlite.NewSQLDeprecatorForBundlesAndPackages(deprecator, dbQuerier)
		if err := packageDeprecator.MaybeRemovePackages(); err != nil {
			r.Logger.Debugf("unable to deprecate package from database: %s", err)
			if !request.Permissive {
				r.Logger.WithError(err).Error("permissive mode disabled")
				return err
			}
			r.Logger.WithError(err).Warn("permissive mode enabled")
		}
	}

	// Any bundles associated with removed packages are now removed from the list of bundles to deprecate.
	if err := deprecator.Deprecate(); err != nil {
		r.Logger.Debugf("unable to deprecate bundles from database: %s", err)
		if !request.Permissive {
			r.Logger.WithError(err).Error("permissive mode disabled")
			return err
		}
		r.Logger.WithError(err).Warn("permissive mode enabled")
	}

	return nil
}

// checkForBundlePaths verifies presence of a list of bundle paths in the registry.
func checkForBundlePaths(querier registry.GRPCQuery, bundlePaths []string) ([]string, []string, error) {
	if len(bundlePaths) == 0 {
		return bundlePaths, nil, nil
	}

	registryBundles, err := querier.ListBundles(context.TODO())
	if err != nil {
		return bundlePaths, nil, err
	}

	if len(registryBundles) == 0 {
		return nil, bundlePaths, nil
	}

	registryBundlePaths := map[string]struct{}{}
	for _, b := range registryBundles {
		registryBundlePaths[b.BundlePath] = struct{}{}
	}

	var found, missing []string
	for _, b := range bundlePaths {
		if _, ok := registryBundlePaths[b]; ok {
			found = append(found, b)
			continue
		}
		missing = append(missing, b)
	}
	if len(missing) > 0 {
		return found, missing, fmt.Errorf("target bundlepaths for deprecation missing from registry: %v", missing)
	}
	return found, missing, nil
}

// replaces mode selects highest version as channel head and
// prunes any bundles in the upgrade chain after the channel head.
// check for the presence of all bundles after a replaces-mode add.
func checkForBundles(ctx context.Context, q *sqlite.SQLQuerier, g registry.GraphLoader, required map[string]*registry.Package) error {
	var errs []error
	for _, pkg := range required {
		graph, err := g.Generate(pkg.Name)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to verify added bundles for package %s: %v", pkg.Name, err))
			continue
		}

		for channel, missing := range pkg.Channels {
			// trace replaces chain for reachable bundles
			for next := []registry.BundleKey{graph.Channels[channel].Head}; len(next) > 0; next = next[1:] {
				delete(missing.Nodes, next[0])
				for edge := range graph.Channels[channel].Nodes[next[0]] {
					next = append(next, edge)
				}
			}

			for bundle := range missing.Nodes {
				// check if bundle is deprecated. Bundles readded after deprecation should not be present in index and can be ignored.
				deprecated, err := isDeprecated(ctx, q, bundle)
				if err != nil {
					errs = append(errs, fmt.Errorf("could not validate pruned bundle %s (%s) as deprecated: %v", bundle.CsvName, bundle.BundlePath, err))
				}
				if !deprecated {
					headSkips := []string{}
					for b := range graph.Channels[channel].Nodes[graph.Channels[channel].Head] {
						headSkips = append(headSkips, b.CsvName)
					}
					errs = append(errs, fmt.Errorf("add prunes bundle %s (%s, %s) from package %s, channel %s: this may be due to incorrect channel head (%s, skips/replaces %v)", bundle.CsvName, bundle.Version, bundle.BundlePath, pkg.Name, channel, graph.Channels[channel].Head.CsvName, headSkips))
				}
			}
		}
	}
	return utilerrors.NewAggregate(errs)
}

func isDeprecated(ctx context.Context, q *sqlite.SQLQuerier, bundle registry.BundleKey) (bool, error) {
	props, err := q.GetPropertiesForBundle(ctx, bundle.CsvName, bundle.Version, bundle.BundlePath)
	if err != nil {
		return false, err
	}
	for _, prop := range props {
		if prop.Type == registry.DeprecatedType {
			return true, nil
		}
	}
	return false, nil
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
* eg:  1.0.1 (alpha) <-[1.0.2 (alpha, stable)]
* When 1.0.2 in alpha and stable channels is added replacing 1.0.1, 1.0.1's presence will only be marked as expected on
* the alpha channel, not on the inherited stable channel.
 */
func expectedGraphBundles(imagesToAdd []*registry.Bundle, graphLoader registry.GraphLoader, overwrite bool) (map[string]*registry.Package, error) {
	expectedBundles := map[string]*registry.Package{}
	for _, bundle := range imagesToAdd {
		version, err := bundle.Version()
		if err != nil {
			return nil, err
		}
		newBundleKey := registry.BundleKey{
			BundlePath: bundle.BundleImage,
			Version:    version,
			CsvName:    bundle.Name,
		}
		var pkg *registry.Package
		var ok bool
		if pkg, ok = expectedBundles[bundle.Package]; !ok {
			var err error
			if pkg, err = graphLoader.Generate(bundle.Package); err != nil {
				if err != registry.ErrPackageNotInDatabase {
					return nil, err
				}
				pkg = &registry.Package{
					Name:     bundle.Package,
					Channels: map[string]registry.Channel{},
				}
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
						return nil, registry.BundleImageAlreadyAddedErr{ErrorString: fmt.Sprintf("Bundle %s already exists", bundle.BundleImage)}
					}
				}
			}
		}
		for _, c := range bundle.Channels {
			if _, ok := pkg.Channels[c]; !ok {
				pkg.Channels[c] = registry.Channel{
					Nodes: map[registry.BundleKey]map[registry.BundleKey]struct{}{},
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
