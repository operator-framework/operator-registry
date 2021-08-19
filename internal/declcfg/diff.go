package declcfg

import (
	"reflect"
	"sort"
	"sync"

	"github.com/blang/semver"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/sirupsen/logrus"

	"github.com/operator-framework/operator-registry/internal/model"
	"github.com/operator-framework/operator-registry/internal/property"
)

// DiffGenerator configures how diffs are created via Run().
type DiffGenerator struct {
	Logger *logrus.Entry

	// SkipDependencies directs Run() to not include dependencies
	// of bundles included in the diff if true.
	SkipDependencies bool

	initOnce sync.Once
}

func (g *DiffGenerator) init() {
	g.initOnce.Do(func() {
		if g.Logger == nil {
			g.Logger = &logrus.Entry{}
		}
	})
}

// Run returns a Model containing everything in newModel not in oldModel,
// and all bundles that exist in oldModel but are different in newModel.
// If oldModel is empty, only channel heads in newModel's packages are
// added to the output Model. All dependencies not in oldModel are also added.
func (g *DiffGenerator) Run(oldModel, newModel model.Model) (model.Model, error) {
	g.init()

	// TODO(estroz): loading both oldModel and newModel into memory may
	// exceed process/hardware limits. Instead, store models on-disk then
	// load by package.

	outputModel := model.Model{}
	if len(oldModel) == 0 {
		// Heads-only mode.

		// Make shallow copies of packages and channels that are only
		// filled with channel heads.
		for _, newPkg := range newModel {
			outputPkg := copyPackageNoChannels(newPkg)
			outputModel[outputPkg.Name] = outputPkg
			for _, newCh := range newPkg.Channels {
				outputCh := copyChannelNoBundles(newCh, outputPkg)
				outputPkg.Channels[outputCh.Name] = outputCh
				head, err := newCh.Head()
				if err != nil {
					return nil, err
				}
				outputBundle := copyBundle(head, outputCh, outputPkg)
				outputModel.AddBundle(*outputBundle)
			}
		}
	} else {
		// Latest mode.

		// Copy newModel to create an output model by deletion,
		// which is more succinct than by addition and potentially
		// more memory efficient.
		for _, newPkg := range newModel {
			outputModel[newPkg.Name] = copyPackage(newPkg)
		}

		// NB(estroz): if a net-new package or channel is published,
		// this currently adds the entire package. I'm fairly sure
		// this behavior is ok because the next diff after a new
		// package is published still has only new data.
		for _, outputPkg := range outputModel {
			oldPkg, oldHasPkg := oldModel[outputPkg.Name]
			if !oldHasPkg {
				// outputPkg was already copied to outputModel above.
				continue
			}
			if err := diffPackages(oldPkg, outputPkg); err != nil {
				return nil, err
			}
			if len(outputPkg.Channels) == 0 {
				// Remove empty packages.
				delete(outputModel, outputPkg.Name)
			}
		}
	}

	if !g.SkipDependencies {
		// Add dependencies to outputModel not already present in oldModel.
		if err := addAllDependencies(newModel, oldModel, outputModel); err != nil {
			return nil, err
		}
	}

	// Default channel may not have been copied, so set it to the new default channel here.
	for _, outputPkg := range outputModel {
		outputHasDefault := false
		newPkg := newModel[outputPkg.Name]
		outputPkg.DefaultChannel, outputHasDefault = outputPkg.Channels[newPkg.DefaultChannel.Name]
		if !outputHasDefault {
			// Create a name-only channel since oldModel contains the channel already.
			outputPkg.DefaultChannel = copyChannelNoBundles(newPkg.DefaultChannel, outputPkg)
		}
	}

	return outputModel, nil
}

// diffPackages removes any bundles and channels from newPkg that
// are in oldPkg, but not those that differ in any way.
func diffPackages(oldPkg, newPkg *model.Package) error {
	for _, newCh := range newPkg.Channels {
		oldCh, oldHasCh := oldPkg.Channels[newCh.Name]
		if !oldHasCh {
			// newCh is assumed to have been copied to outputModel by the caller.
			continue
		}

		for _, newBundle := range newCh.Bundles {
			oldBundle, oldHasBundle := oldCh.Bundles[newBundle.Name]
			if !oldHasBundle {
				// newBundle is copied to outputModel by the caller if it is a channel head.
				continue
			}
			equal, err := bundlesEqual(oldBundle, newBundle)
			if err != nil {
				return err
			}
			if equal {
				delete(newCh.Bundles, newBundle.Name)
			}
		}
		if len(newCh.Bundles) == 0 {
			// Remove empty channels.
			delete(newPkg.Channels, newCh.Name)
		}
	}

	return nil
}

// bundlesEqual computes then compares the hashes of b1 and b2 for equality.
func bundlesEqual(b1, b2 *model.Bundle) (bool, error) {
	// Use a declarative config bundle type to avoid infinite recursion.
	dcBundle1 := convertFromModelBundle(b1)
	dcBundle2 := convertFromModelBundle(b2)

	hash1, err := hashstructure.Hash(dcBundle1, hashstructure.FormatV2, nil)
	if err != nil {
		return false, err
	}
	hash2, err := hashstructure.Hash(dcBundle2, hashstructure.FormatV2, nil)
	if err != nil {
		return false, err
	}
	// CsvJSON and Objects are ignored by Hash, so they must be compared separately.
	return hash1 == hash2 && b1.CsvJSON == b2.CsvJSON && reflect.DeepEqual(b1.Objects, b2.Objects), nil
}

func addAllDependencies(newModel, oldModel, outputModel model.Model) error {
	// Get every oldModel's bundle's dependencies, and their dependencies, etc. by BFS.
	allProvidingBundles := []*model.Bundle{}
	for curr := getBundles(outputModel); len(curr) != 0; {
		reqGVKs, reqPkgs, err := findDependencies(curr)
		if err != nil {
			return err
		}
		// Break early so the entire source model is not iterated through unnecessarily.
		if len(reqGVKs) == 0 && len(reqPkgs) == 0 {
			break
		}
		curr = nil
		// Get bundles that provide dependencies from newModel, which should have
		// the latest bundles of each dependency package.
		for _, pkg := range newModel {
			providingBundles := getBundlesThatProvide(pkg, reqGVKs, reqPkgs)
			curr = append(curr, providingBundles...)
			allProvidingBundles = append(allProvidingBundles, providingBundles...)
		}
	}

	// Add the diff between an oldModel dependency package and its new counterpart
	// or the entire package if oldModel does not have it.
	//
	// TODO(estroz): add bundles then fill in dependency upgrade graph
	// by selecting latest versions, as the EP specifies.
	dependencyPkgs := map[string]*model.Package{}
	for _, b := range allProvidingBundles {
		if _, copied := dependencyPkgs[b.Package.Name]; !copied {
			dependencyPkgs[b.Package.Name] = copyPackage(b.Package)
		}
	}
	for _, newDepPkg := range dependencyPkgs {
		// newDepPkg is a copy of a newModel pkg, so running diffPackages
		// on it and oldPkg, which may have some but not all bundles,
		// will produce a set of all bundles that outputModel doesn't have.
		// Otherwise, just add the whole package.
		if oldPkg, oldHasPkg := oldModel[newDepPkg.Name]; oldHasPkg {
			if err := diffPackages(oldPkg, newDepPkg); err != nil {
				return err
			}
			if len(newDepPkg.Channels) == 0 {
				// Skip empty packages.
				continue
			}
		}
		outputModel[newDepPkg.Name] = newDepPkg
	}

	return nil
}

// getBundles collects all bundles specified by m. Since each bundle
// references its package, their uniqueness property holds in a flat list.
func getBundles(m model.Model) (bundles []*model.Bundle) {
	for _, pkg := range m {
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				b := b
				bundles = append(bundles, b)
			}
		}
	}
	return bundles
}

// findDependencies finds all GVK and package dependencies and indexes them
// by the apropriate key for lookups.
func findDependencies(bundles []*model.Bundle) (map[property.GVK]struct{}, map[string][]semver.Range, error) {
	// Find all dependencies of bundles in the output model.
	reqGVKs := map[property.GVK]struct{}{}
	reqPkgs := map[string][]semver.Range{}
	for _, b := range bundles {

		for _, gvkReq := range b.PropertiesP.GVKsRequired {
			gvk := property.GVK{
				Group:   gvkReq.Group,
				Version: gvkReq.Version,
				Kind:    gvkReq.Kind,
			}
			reqGVKs[gvk] = struct{}{}
		}

		for _, pkgReq := range b.PropertiesP.PackagesRequired {
			var inRange semver.Range
			if pkgReq.VersionRange != "" {
				var err error
				if inRange, err = semver.ParseRange(pkgReq.VersionRange); err != nil {
					// Should never happen since model has been validated.
					return nil, nil, err
				}
			} else {
				// Any bundle in this package will satisfy a range-less package requirement.
				inRange = func(semver.Version) bool { return true }
			}
			reqPkgs[pkgReq.PackageName] = append(reqPkgs[pkgReq.PackageName], inRange)
		}
	}

	return reqGVKs, reqPkgs, nil
}

// getBundlesThatProvide returns the latest-version bundles in pkg that provide
// a GVK or version in reqGVKs or reqPkgs, respectively.
func getBundlesThatProvide(pkg *model.Package, reqGVKs map[property.GVK]struct{}, reqPkgs map[string][]semver.Range) (providingBundles []*model.Bundle) {
	// Pre-allocate the amount of space needed for all ranges
	// specified by requiring bundles.
	var bundlesByRange [][]*model.Bundle
	ranges, isPkgRequired := reqPkgs[pkg.Name]
	if isPkgRequired {
		bundlesByRange = make([][]*model.Bundle, len(ranges))
	}
	// Collect package bundles that provide a GVK or are in a range.
	bundlesProvidingGVK := make(map[property.GVK][]*model.Bundle)
	for _, ch := range pkg.Channels {
		for _, b := range ch.Bundles {
			b := b
			for _, gvk := range b.PropertiesP.GVKs {
				if _, hasGVK := reqGVKs[gvk]; hasGVK {
					bundlesProvidingGVK[gvk] = append(bundlesProvidingGVK[gvk], b)
				}
			}
			for i, inRange := range ranges {
				if inRange(b.Version) {
					bundlesByRange[i] = append(bundlesByRange[i], b)
				}
			}
		}
	}

	// Sort bundles providing a GVK by version and use the latest version.
	latestBundles := make(map[string]*model.Bundle)
	for gvk, bundles := range bundlesProvidingGVK {
		sort.Slice(bundles, func(i, j int) bool {
			return bundles[i].Version.LT(bundles[j].Version)
		})
		lb := bundles[len(bundles)-1]
		latestBundles[lb.Version.String()] = lb
		delete(reqGVKs, gvk)
	}

	// Sort bundles in a range by version and use the latest version.
	unsatisfiedRanges := []semver.Range{}
	for i, bundlesInRange := range bundlesByRange {
		if len(bundlesInRange) == 0 {
			unsatisfiedRanges = append(unsatisfiedRanges, ranges[i])
			continue
		}
		sort.Slice(bundlesInRange, func(i, j int) bool {
			return bundlesInRange[i].Version.LT(bundlesInRange[j].Version)
		})
		lb := bundlesInRange[len(bundlesInRange)-1]
		latestBundles[lb.Version.String()] = lb
	}
	if isPkgRequired && len(unsatisfiedRanges) == 0 {
		delete(reqPkgs, pkg.Name)
	}
	// TODO(estroz): handle missed ranges with logs.

	// Return deduplicated bundles that provide GVKs/versions.
	for _, b := range latestBundles {
		providingBundles = append(providingBundles, b)
	}
	return providingBundles
}

func convertFromModelBundle(b *model.Bundle) Bundle {
	return Bundle{
		Schema:        schemaBundle,
		Name:          b.Name,
		Package:       b.Package.Name,
		Image:         b.Image,
		RelatedImages: modelRelatedImagesToRelatedImages(b.RelatedImages),
		CsvJSON:       b.CsvJSON,
		Objects:       b.Objects,
		Properties:    b.Properties,
	}
}

func copyPackageNoChannels(in *model.Package) *model.Package {
	cp := &model.Package{
		Name:        in.Name,
		Description: in.Description,
		Channels:    make(map[string]*model.Channel, len(in.Channels)),
	}
	if in.Icon != nil {
		cp.Icon = &model.Icon{
			Data:      make([]byte, len(in.Icon.Data)),
			MediaType: in.Icon.MediaType,
		}
		copy(cp.Icon.Data, in.Icon.Data)
	}
	return cp
}

func copyPackage(in *model.Package) *model.Package {
	cp := copyPackageNoChannels(in)
	for _, ch := range in.Channels {
		cp.Channels[ch.Name] = copyChannel(ch, cp)
	}
	return cp
}

func copyChannelNoBundles(in *model.Channel, pkg *model.Package) *model.Channel {
	cp := &model.Channel{
		Name:    in.Name,
		Package: pkg,
		Bundles: make(map[string]*model.Bundle, len(in.Bundles)),
	}
	return cp
}

func copyChannel(in *model.Channel, pkg *model.Package) *model.Channel {
	cp := copyChannelNoBundles(in, pkg)
	for _, b := range in.Bundles {
		cp.Bundles[b.Name] = copyBundle(b, cp, pkg)
	}
	return cp
}

func copyBundle(in *model.Bundle, ch *model.Channel, pkg *model.Package) *model.Bundle {
	cp := &model.Bundle{
		Name:     in.Name,
		Channel:  ch,
		Package:  pkg,
		Image:    in.Image,
		Replaces: in.Replaces,
		Version:  semver.MustParse(in.Version.String()),
		CsvJSON:  in.CsvJSON,
	}
	cp.PropertiesP, _ = property.Parse(in.Properties)
	if len(in.Skips) != 0 {
		cp.Skips = make([]string, len(in.Skips))
		copy(cp.Skips, in.Skips)
	}
	if len(in.Properties) != 0 {
		cp.Properties = make([]property.Property, len(in.Properties))
		copy(cp.Properties, in.Properties)
	}
	if len(in.RelatedImages) != 0 {
		cp.RelatedImages = make([]model.RelatedImage, len(in.RelatedImages))
		copy(cp.RelatedImages, in.RelatedImages)
	}
	if len(in.Objects) != 0 {
		cp.Objects = make([]string, len(in.Objects))
		copy(cp.Objects, in.Objects)
	}
	return cp
}
