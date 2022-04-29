package declcfg

import (
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/operator-framework/operator-registry/alpha/model"
)

// DiffIncluder knows how to add packages, channels, and bundles
// from a source to a destination model.Model.
type DiffIncluder struct {
	// Packages to add.
	Packages []DiffIncludePackage
	Logger   *logrus.Entry
	// HeadsOnly is the mode that selects the head of the channels only.
	// This setting will be overridden by any versions or bundles in the channels.
	HeadsOnly bool
}

// DiffIncludePackage specifies a package, and optionally channels
// or a set of bundles from all channels (wrapped by a DiffIncludeChannel),
// to include.
type DiffIncludePackage struct {
	// Name of package.
	Name string
	// Channels in package.
	Channels []DiffIncludeChannel
	// AllChannels contains bundle versions in package.
	// Upgrade graphs from all channels in the named package containing a version
	// from this field are included.
	AllChannels DiffIncludeChannel
	// The semver range of bundle versions.
	// Package range setting is mutually exclusive with channel range/bundles/version
	// settings.
	Range semver.Range
	// HeadsOnly is the mode that selects the head of the channels only.
	// This setting will be overridden by any versions or bundles in the channels.
	HeadsOnly bool
}

// DiffIncludeChannel specifies a channel, and optionally bundles and bundle versions
// (or version range) to include.
type DiffIncludeChannel struct {
	// Name of channel.
	Name string
	// Versions of bundles.
	Versions []semver.Version
	// Bundles are bundle names to include.
	// Set this field only if the named bundle has no semantic version metadata.
	Bundles []string
	// The semver range of bundle versions.
	// Range setting is mutually exclusive with Versions and Bundles settings.
	Range semver.Range
}

func (dip DiffIncludePackage) Validate() error {
	var errs []error
	if dip.Name == "" {
		errs = append(errs, fmt.Errorf("missing package name"))
	}

	var isChannelSet bool
	for _, ch := range dip.Channels {
		isChannelSet = ch.isChannelSet()
		err := ch.Validate()
		if err != nil {
			errs = append(errs, err)
		}
	}

	isChannelSet = dip.AllChannels.isChannelSet()
	if isChannelSet && dip.Range != nil {
		errs = append(errs, fmt.Errorf("package range setting is mutually exclusive with channel versions/bundles/range settings"))
	}

	if len(errs) != 0 {
		return fmt.Errorf("invalid DiffIncludePackage config for package %q:\n%v", dip.Name, utilerrors.NewAggregate(errs))
	}
	return nil
}

// isChannelSet returns true if at least one of Range/Bundles/Versions is set
func (dic DiffIncludeChannel) isChannelSet() bool {
	return dic.Range != nil || len(dic.Versions) != 0 || len(dic.Bundles) != 0
}

func (dic DiffIncludeChannel) Validate() error {
	var errs []error
	if dic.Name == "" {
		errs = append(errs, fmt.Errorf("missing channel name"))
	}

	if dic.Range != nil && (len(dic.Versions) != 0 || len(dic.Bundles) != 0) {
		errs = append(errs, fmt.Errorf("Channel %q: range and versions/bundles are mutually exclusive", dic.Name))
	}

	if len(errs) != 0 {
		return fmt.Errorf("invalid DiffIncludeChannel config for channel %q:\n%v", dic.Name, utilerrors.NewAggregate(errs))
	}
	return nil
}

func (i DiffIncluder) Validate() error {
	var errs []error
	for _, pkg := range i.Packages {
		err := pkg.Validate()
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("invalid DiffIncluder config:\n%v", utilerrors.NewAggregate(errs))
	}
	return nil
}

// Run adds all packages and channels in DiffIncluder with matching names
// directly, and all versions plus their upgrade graphs to channel heads,
// from newModel to outputModel.
func (i DiffIncluder) Run(newModel, outputModel model.Model) error {
	var includeErrs []error
	if err := i.Validate(); err != nil {
		includeErrs = append(includeErrs, err)
		return fmt.Errorf("invalid DiffIncluder config:\n%v", utilerrors.NewAggregate(includeErrs))
	}

	for _, ipkg := range i.Packages {
		pkgLog := i.Logger.WithField("package", ipkg.Name)
		ipkg.HeadsOnly = i.HeadsOnly
		includeErrs = append(includeErrs, ipkg.includeNewInOutputModel(newModel, outputModel, pkgLog)...)
	}
	if len(includeErrs) != 0 {
		return fmt.Errorf("error including items:\n%v", utilerrors.NewAggregate(includeErrs))
	}
	return nil
}

// includeNewInOutputModel adds all packages, channels, and range (or versions/bundles)
// specified by ipkg that exist in newModel to outputModel. Any package, channel,
// or version in ipkg not satisfied by newModel is an error.
func (ipkg DiffIncludePackage) includeNewInOutputModel(newModel, outputModel model.Model, logger *logrus.Entry) (ierrs []error) {

	newPkg, newHasPkg := newModel[ipkg.Name]
	if !newHasPkg {
		ierrs = append(ierrs, fmt.Errorf("[package=%q] package does not exist in new model", ipkg.Name))
		return ierrs
	}
	pkgLog := logger.WithField("package", newPkg.Name)

	// No range, channels or versions were specified
	if len(ipkg.Channels) == 0 && len(ipkg.AllChannels.Versions) == 0 && len(ipkg.AllChannels.Bundles) == 0 && ipkg.Range == nil {
		// heads-only false, meaning "include the full package".
		if !ipkg.HeadsOnly {
			outputModel[ipkg.Name] = newPkg
			return nil
		}
		// heads-only true, get the head of every channel in the package
		for _, c := range newPkg.Channels {
			newCh := DiffIncludeChannel{
				Name: c.Name,
			}
			ipkg.Channels = append(ipkg.Channels, newCh)
		}
	}

	outputPkg := copyPackageNoChannels(newPkg)
	outputModel[outputPkg.Name] = outputPkg
	skipMissingBundleForChannels := map[string]bool{}
	if ipkg.Range != nil {
		if len(ipkg.Channels) != 0 {
			for _, ich := range ipkg.Channels {
				if ich.Range != nil {
					ierrs = append(ierrs, fmt.Errorf("[package=%q channel=%q] range setting is mutually exclusive between package and channel", newPkg.Name, ich.Name))
				}
			}
		} else {
			// Add package range setting to all existing channels if there is no
			// channel setting in the config
			for newChName := range newPkg.Channels {
				ipkg.Channels = append(ipkg.Channels, DiffIncludeChannel{
					Name:  newChName,
					Range: ipkg.Range,
				})
			}
		}
	} else {
		// Add all channels to ipkg.Channels if bundles or versions were specified to include across all channels.
		// skipMissingBundleForChannels's value for a channel will be true IFF at least one version is specified,
		// since some other channel may contain that version.
		if len(ipkg.AllChannels.Versions) != 0 || len(ipkg.AllChannels.Bundles) != 0 {
			for newChName := range newPkg.Channels {
				ipkg.Channels = append(ipkg.Channels, DiffIncludeChannel{
					Name:     newChName,
					Versions: ipkg.AllChannels.Versions,
					Bundles:  ipkg.AllChannels.Bundles,
				})
				skipMissingBundleForChannels[newChName] = true
			}
		}
	}

	for _, ich := range ipkg.Channels {
		newCh, pkgHasCh := newPkg.Channels[ich.Name]
		if !pkgHasCh {
			ierrs = append(ierrs, fmt.Errorf("[package=%q channel=%q] channel does not exist in new model", newPkg.Name, ich.Name))
			continue
		}
		chLog := pkgLog.WithField("channel", newCh.Name)

		var bundles []*model.Bundle
		var head *model.Bundle
		var err error
		// No versions have been specified, but heads-only set to true, get the channel head only.
		switch {
		case ipkg.HeadsOnly && len(ich.Versions) == 0 && len(ich.Bundles) == 0 && ich.Range == nil:
			head, err = newCh.Head()
			bundles = append(bundles, head)
		case ich.Range != nil:
			bundles, err = getBundlesForRange(newCh, ich.Range, chLog)
		default:
			bundles, err = getBundlesForVersions(newCh, ich.Versions, ich.Bundles, chLog, skipMissingBundleForChannels[newCh.Name])
		}

		if err != nil {
			ierrs = append(ierrs, fmt.Errorf("[package=%q channel=%q] %v", newPkg.Name, newCh.Name, err))
			continue
		}

		outputCh := copyChannelNoBundles(newCh, outputPkg)
		outputPkg.Channels[outputCh.Name] = outputCh
		for _, b := range bundles {
			tb := copyBundle(b, outputCh, outputPkg)
			outputCh.Bundles[tb.Name] = tb
		}
	}

	return ierrs
}

// getBundlesForVersions returns all bundles matching a version in vers
// and their upgrade graph(s) to ch.Head().
// If skipMissingBundles is true, bundle names and versions not satisfied by bundles in ch
// will not result in errors.
func getBundlesForVersions(ch *model.Channel, vers []semver.Version, names []string, logger *logrus.Entry, skipMissingBundles bool) (bundles []*model.Bundle, err error) {

	// Short circuit when no versions were specified, meaning "include the whole channel".
	if len(vers) == 0 {
		for _, b := range ch.Bundles {
			bundles = append(bundles, b)
		}
		return bundles, nil
	}

	// Add every bundle with a specified bundle name or directly satisfying a bundle version to bundles.
	versionsToInclude := make(map[string]struct{}, len(vers))
	for _, ver := range vers {
		versionsToInclude[ver.String()] = struct{}{}
	}
	namesToInclude := make(map[string]struct{}, len(vers))
	for _, name := range names {
		namesToInclude[name] = struct{}{}
	}
	for _, b := range ch.Bundles {
		_, includeVersionedBundle := versionsToInclude[b.Version.String()]
		_, includeNamedBundle := namesToInclude[b.Name]
		if includeVersionedBundle || includeNamedBundle {
			bundles = append(bundles, b)
		}
	}

	// Some version was not satisfied by this channel.
	if len(bundles) != len(versionsToInclude)+len(namesToInclude) && !skipMissingBundles {
		for _, b := range bundles {
			delete(versionsToInclude, b.Version.String())
			delete(namesToInclude, b.Name)
		}
		var verStrs, nameStrs []string
		for verStr := range versionsToInclude {
			verStrs = append(verStrs, verStr)
		}
		for nameStr := range namesToInclude {
			nameStrs = append(nameStrs, nameStr)
		}
		sb := strings.Builder{}
		if len(verStrs) != 0 {
			sb.WriteString(fmt.Sprintf("versions=%+q ", verStrs))
		}
		if len(nameStrs) != 0 {
			sb.WriteString(fmt.Sprintf("names=%+q", nameStrs))
		}
		return nil, fmt.Errorf("bundles do not exist in channel: %s", strings.TrimSpace(sb.String()))
	}

	bundles, err = fillUpgradeGraph(ch, bundles, logger)
	if err != nil {
		return nil, err
	}
	return bundles, nil
}

// getBundlesForRange returns all bundles matching the version range in vers
// If the range is nil, return all bundles in the channel
func getBundlesForRange(ch *model.Channel, vers semver.Range, logger *logrus.Entry) (bundles []*model.Bundle, err error) {
	// Short circuit when an empty range was specified, meaning "include the whole channel"
	if vers == nil {
		for _, b := range ch.Bundles {
			bundles = append(bundles, b)
		}
		return bundles, nil
	}

	for _, b := range ch.Bundles {
		v, err := semver.Parse(b.Version.String())
		if err != nil {
			return nil, fmt.Errorf("unable to parse bunble version: %s", err.Error())
		}
		if vers(v) {
			bundles = append(bundles, b)
		}
	}

	return bundles, nil
}

// fillUpgradeGraph fills in the upgrade graph between each bundle and head.
// Regardless of semver order, this step needs to be performed
// for each included bundle because there might be leaf nodes
// in the upgrade graph for a particular version not captured
// by any other fill due to skips not being honored here.
func fillUpgradeGraph(ch *model.Channel, bundles []*model.Bundle, logger *logrus.Entry) (bd []*model.Bundle, err error) {
	head, err := ch.Head()
	if err != nil {
		return nil, err
	}
	graph := makeUpgradeGraph(ch)
	bundleSet := map[string]*model.Bundle{}
	for _, ib := range bundles {
		if _, addedBundle := bundleSet[ib.Name]; addedBundle {
			// A prior graph traverse already included this bundle.
			continue
		}
		intersectingBundles, intersectionFound := findIntersectingBundles(ch, ib, head, graph)
		if !intersectionFound {
			logger.Debugf("channel head %q not reachable from bundle %q, adding without upgrade graph", head.Name, ib.Name)
			bundleSet[ib.Name] = ib
		}

		for _, rb := range intersectingBundles {
			bundleSet[rb.Name] = rb
		}
	}

	for _, b := range bundleSet {
		bundles = append(bundles, b)
	}
	return bundles, nil
}

// makeUpgradeGraph creates a DAG of bundles with map key Bundle.Replaces.
func makeUpgradeGraph(ch *model.Channel) map[string][]*model.Bundle {
	graph := map[string][]*model.Bundle{}
	for _, b := range ch.Bundles {
		if b.Replaces != "" {
			graph[b.Replaces] = append(graph[b.Replaces], b)
		}
	}
	return graph
}

// findIntersectingBundles finds the intersecting bundle of start and end in the
// replaces upgrade graph graph by traversing down to the lowest graph node,
// then returns every bundle higher than the intersection. It is possible
// to find no intersection; this should only happen when start and end
// are not part of the same upgrade graph.
// Output bundle order is not guaranteed.
// Precondition: start must be a bundle in ch.
// Precondition: end must be ch's head.
func findIntersectingBundles(ch *model.Channel, start, end *model.Bundle, graph map[string][]*model.Bundle) ([]*model.Bundle, bool) {
	// The intersecting set is equal to end if start is end.
	if start.Name == end.Name {
		return []*model.Bundle{end}, true
	}

	// Construct start's replaces chain for comparison against end's.
	startChain := map[string]*model.Bundle{start.Name: nil}
	for curr := start; curr != nil && curr.Replaces != ""; curr = ch.Bundles[curr.Replaces] {
		startChain[curr.Replaces] = curr
	}

	// Trace end's replaces chain until it intersects with start's, or the root is reached.
	var intersection string
	if _, inChain := startChain[end.Name]; inChain {
		intersection = end.Name
	} else {
		for curr := end; curr != nil && curr.Replaces != ""; curr = ch.Bundles[curr.Replaces] {
			if _, inChain := startChain[curr.Replaces]; inChain {
				intersection = curr.Replaces
				break
			}
		}
	}

	// No intersection is found, delegate behavior to caller.
	if intersection == "" {
		return nil, false
	}

	// Find all bundles that replace the intersection via BFS,
	// i.e. the set of bundles that fill the update graph between start and end.
	replacesIntersection := graph[intersection]
	replacesSet := map[string]*model.Bundle{}
	for _, b := range replacesIntersection {
		currName := ""
		for next := []*model.Bundle{b}; len(next) > 0; next = next[1:] {
			currName = next[0].Name
			if _, hasReplaces := replacesSet[currName]; !hasReplaces {
				replacers := graph[currName]
				next = append(next, replacers...)
				replacesSet[currName] = ch.Bundles[currName]
			}
		}
	}

	// Remove every bundle between start and intersection exclusively,
	// since these bundles must already exist in the destination channel.
	for rep := start; rep != nil && rep.Name != intersection; rep = ch.Bundles[rep.Replaces] {
		delete(replacesSet, rep.Name)
	}

	// Ensure both start and end are added to the output.
	replacesSet[start.Name] = start
	replacesSet[end.Name] = end
	var intersectingBundles []*model.Bundle
	for _, b := range replacesSet {
		intersectingBundles = append(intersectingBundles, b)
	}
	return intersectingBundles, true
}
