package registry

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
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
	for _, ref := range refs {
		to, from, cleanup, err := unpackImage(ctx, reg, ref)
		if err != nil {
			return err
		}
		unpackedImageMap[to] = from
		defer cleanup()
	}

	overwriteImageMap := make(map[string]map[image.Reference]string, 0)
	if overwrite {
		// find all bundles that are attempting to overwrite
		for to, from := range unpackedImageMap {
			img, err := registry.NewImageInput(to, from)
			if err != nil {
				return err
			}
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
			// get all bundle paths for that package - we will re-add these to regenerate the graph
			bundles, err := querier.GetBundlesForPackage(ctx, img.Bundle.Package)
			if err != nil {
				return err
			}
			type unpackedImage struct {
				to      image.Reference
				from    string
				cleanup func()
				err     error
			}
			unpacked := make(chan unpackedImage)
			for bundle := range bundles {
				// parallelize image pulls
				go func(bundle registry.BundleKey, img *registry.ImageInput) {
					if bundle.CsvName != img.Bundle.Name {
						to, from, cleanup, err := unpackImage(ctx, reg, image.SimpleReference(bundle.BundlePath))
						unpacked <- unpackedImage{to: to, from: from, cleanup: cleanup, err: err}
					} else {
						unpacked <- unpackedImage{to: to, from: from, cleanup: func() { return }, err: nil}
					}
				}(bundle, img)
			}
			if _, ok := overwriteImageMap[img.Bundle.Package]; !ok {
				overwriteImageMap[img.Bundle.Package] = make(map[image.Reference]string, 0)
			}
			for i := 0; i < len(bundles); i++ {
				unpack := <-unpacked
				if unpack.err != nil {
					return unpack.err
				}
				overwriteImageMap[img.Bundle.Package][unpack.to] = unpack.from
				if _, ok := unpackedImageMap[unpack.to]; ok {
					delete(unpackedImageMap, unpack.to)
				}
				defer unpack.cleanup()
			}
		}
	}

	populator := registry.NewDirectoryPopulator(loader, graphLoader, querier, unpackedImageMap, overwriteImageMap, overwrite)
	return populator.Populate(mode)
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

	dbLoader, err := sqlite.NewSQLLiteLoader(db)
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

	dbLoader, err := sqlite.NewSQLLiteLoader(db)
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

type PruneVersionFromRegistryRequest struct {
	Permissive      bool
	InputDatabase   string
	PackageVersions []string
}

func (r RegistryUpdater) PruneVersionFromRegistry(request PruneVersionFromRegistryRequest) error {
	// First we'll prune the packages
	// Create a map of the operator and versions we want to keep
	operatorVerMap := make(map[string][]string)
	for _, pkgVersion := range request.PackageVersions {
		split := strings.Split(pkgVersion, ":")
		operatorVerMap[split[0]] = append(operatorVerMap[split[0]], split[1])
	}

	// now we sort those lists of versions for later (might only contain one version each)
	for _, versionList := range operatorVerMap {
		sort.Slice(versionList, func(i, j int) bool {
			return semver.Compare(versionList[i], versionList[j]) < 0
		})
	}
	packageList := make([]string, 0, len(operatorVerMap))
	for operatorName := range operatorVerMap {
		packageList = append(packageList, operatorName)
	}

	logrus.Info(fmt.Sprintf("Keeping %s", packageList))

	prunePackageReq := PruneFromRegistryRequest{
		Permissive:    request.Permissive,
		InputDatabase: request.InputDatabase,
		Packages:      packageList,
	}
	r.PruneFromRegistry(prunePackageReq)

	// Now we go delete the versions we don't want
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

	// get all the packages
	lister := sqlite.NewSQLLiteQuerierFromDb(db)
	if err != nil {
		return err
	}

	// prune packages from registry
	for operatorName, versionList := range operatorVerMap {
		operatorBundleVersions := make(map[string]bool)
		for _, version := range versionList {
			operatorBundleVersions[version] = true
		}
		// bundlesForPackage, err := lister.GetBundlesForPackage(context.TODO(), operatorName)
		channelEntriesForPackage, err := lister.GetChannelEntriesFromPackage(context.TODO(), operatorName)
		if err != nil {
			return err
		}

		for _, channelEntryForPackage := range channelEntriesForPackage {
			// Find the newest of the package version for this channel (otherwise we lose everything if we delete)
			// the head bundle
			channel := channelEntryForPackage.ChannelName
			bundleToSave := findNewestVersionToSave(channelEntriesForPackage, operatorVerMap[channelEntryForPackage.PackageName], channel)
			if err != nil {
				return err
			}

			// Check our map to see if the bundle we found is in the list of bundles we want to keep
			if _, found := operatorBundleVersions[channelEntryForPackage.Version]; !found {
				// if not, then we delete that bundle
				remover := sqlite.NewSQLRemoverForOperatorCsvNames(dbLoader, channelEntryForPackage.BundleName, bundleToSave)
				if err := remover.Remove(); err != nil {
					err = fmt.Errorf("error deleting bundles by operator csv name from database: %s", err)
					if !request.Permissive {
						logrus.WithError(err).Fatal("permissive mode disabled")
						return err
					}
					logrus.WithError(err).Warn("permissive mode enabled")
				}
			}
		}
	}

	return nil
}

type DeprecateFromRegistryRequest struct {
	Permissive    bool
	InputDatabase string
	Bundles       []string
}

func findNewestVersionToSave(channelEntries []registry.ChannelEntryAnnotated, operatorVerList []string, channelName string) *string {
	// filter the channel entries for the specific channel name
	filteredChannelEntries := []registry.ChannelEntryAnnotated{}
	for _, channelEntryForPackage := range channelEntries {
		if channelEntryForPackage.ChannelName == channelName {
			filteredChannelEntries = append(filteredChannelEntries, channelEntryForPackage)
		}
	}

	sort.Slice(filteredChannelEntries, func(i, j int) bool {
		return semver.Compare(filteredChannelEntries[i].Version, filteredChannelEntries[j].Version) < 0
	})

	// Find all the versions that the user requested that are also in this channel
	filteredOperatorVerList := []string{}
	// this probably could be improved
	for _, operatorVer := range operatorVerList {
		for _, channelEntry := range filteredChannelEntries {
			if semver.Compare(operatorVer, channelEntry.Version) == 0 {
				filteredOperatorVerList = append(filteredOperatorVerList, operatorVer)
			}
		}
	}

	// if the list is empty, then we didn't find any that matched this channel
	if len(filteredOperatorVerList) == 0 {
		return nil
	}

	// now sort it to get the highest version we want to save for this channel
	sort.Slice(filteredOperatorVerList, func(i, j int) bool {
		return semver.Compare(filteredOperatorVerList[i], filteredOperatorVerList[j]) < 0
	})

	highestVersion := filteredOperatorVerList[len(filteredOperatorVerList)-1]

	for _, i := range filteredChannelEntries {
		if i.Version == highestVersion {
			return &i.BundleName
		}
	}

	// If we get here, there's no version we could find in this channel that we'd want to save
	return nil
}

func (r RegistryUpdater) DeprecateFromRegistry(request DeprecateFromRegistryRequest) error {
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
