package semver

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"slices"
	"sort"

	"github.com/blang/semver/v4"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
	"github.com/operator-framework/operator-registry/alpha/template"
)

// IO structs -- BEGIN
type semverTemplateBundleEntry struct {
	Image string `json:"image,omitempty"`
}

type semverTemplateChannelBundles struct {
	Bundles []semverTemplateBundleEntry `json:"bundles,omitempty"`
}

type SemverTemplateData struct {
	Schema                       string                       `json:"schema"`
	GenerateMajorChannels        bool                         `json:"generateMajorChannels,omitempty"`
	GenerateMinorChannels        bool                         `json:"generateMinorChannels,omitempty"`
	DefaultChannelTypePreference streamType                   `json:"defaultChannelTypePreference,omitempty"`
	Candidate                    semverTemplateChannelBundles `json:"candidate,omitempty"`
	Fast                         semverTemplateChannelBundles `json:"fast,omitempty"`
	Stable                       semverTemplateChannelBundles `json:"stable,omitempty"`

	pkg            string `json:"-"` // the derived package name
	defaultChannel string `json:"-"` // detected "most stable" channel head
}

// IO structs -- END

// SemverTemplate implements the common template interface
type SemverTemplate struct {
	renderBundle template.BundleRenderer
}

// NewTemplate creates a new semver template instance
func NewTemplate(renderBundle template.BundleRenderer) template.Template {
	return &SemverTemplate{
		renderBundle: renderBundle,
	}
}

// RenderBundle implements the template.Template interface
func (t *SemverTemplate) RenderBundle(ctx context.Context, image string) (*declcfg.DeclarativeConfig, error) {
	return t.renderBundle(ctx, image)
}

// Render implements the template.Template interface
func (t *SemverTemplate) Render(ctx context.Context, reader io.Reader) (*declcfg.DeclarativeConfig, error) {
	var out declcfg.DeclarativeConfig

	sv, err := readFile(reader)
	if err != nil {
		return nil, fmt.Errorf("render: unable to read file: %v", err)
	}

	// nolint:prealloc
	var cfgs []declcfg.DeclarativeConfig

	bundleDict := buildBundleList(*sv)
	for b := range bundleDict {
		c, err := t.RenderBundle(ctx, b)
		if err != nil {
			return nil, err
		}
		if len(c.Bundles) != 1 {
			return nil, fmt.Errorf("bundle reference %q resulted in %d bundles, expected 1", b, len(c.Bundles))
		}
		bundleDict[b] = c.Bundles[0].Image
		cfgs = append(cfgs, *c)
	}
	out = *combineConfigs(cfgs)

	if len(out.Bundles) == 0 {
		return nil, fmt.Errorf("render: no bundles specified or no bundles could be rendered")
	}

	channelBundleVersions, err := sv.getVersionsFromStandardChannels(&out, bundleDict)
	if err != nil {
		return nil, fmt.Errorf("render: unable to post-process bundle info: %v", err)
	}

	channels := sv.generateChannels(channelBundleVersions)
	out.Channels = channels
	out.Packages[0].DefaultChannel = sv.defaultChannel

	return &out, nil
}

// Schema implements the template.Template interface
func (t *SemverTemplate) Schema() string {
	return schema
}

// Factory implements the template.TemplateFactory interface
type Factory struct{}

// CreateTemplate implements the template.TemplateFactory interface
func (f *Factory) CreateTemplate(renderBundle template.BundleRenderer) template.Template {
	return NewTemplate(renderBundle)
}

// Schema implements the template.TemplateFactory interface
func (f *Factory) Schema() string {
	return schema
}

const schema string = "olm.semver"

// channel "archetypes", restricted in this iteration to just these
type channelArchetype string

const (
	candidateChannelArchetype channelArchetype = "candidate"
	fastChannelArchetype      channelArchetype = "fast"
	stableChannelArchetype    channelArchetype = "stable"
)

// mapping channel name --> stability, where higher values indicate greater stability
var channelPriorities = map[channelArchetype]int{candidateChannelArchetype: 0, fastChannelArchetype: 1, stableChannelArchetype: 2}

// sorting capability for a slice according to the assigned channelPriorities
type byChannelPriority []channelArchetype

func (b byChannelPriority) Len() int { return len(b) }
func (b byChannelPriority) Less(i, j int) bool {
	return channelPriorities[b[i]] < channelPriorities[b[j]]
}
func (b byChannelPriority) Swap(i, j int) { b[i], b[j] = b[j], b[i] }

type streamType string

const defaultStreamType streamType = ""
const minorStreamType streamType = "minor"
const majorStreamType streamType = "major"

// general preference for minor channels
var streamTypePriorities = map[streamType]int{minorStreamType: 2, majorStreamType: 1, defaultStreamType: 0}

// map of archetypes --> bundles --> bundle-version from the input file
type bundleVersions map[channelArchetype]map[string]semver.Version // e.g. srcv["stable"]["example-operator.v1.0.0"] = 1.0.0

// the "high-water channel" struct functions as a freely-rising indicator of the "most stable" channel head, so we can use that
// later as the package's defaultChannel attribute
type highwaterChannel struct {
	archetype channelArchetype
	kind      streamType
	version   semver.Version
	name      string
}

// entryTuple represents a channel entry with its associated metadata
type entryTuple struct {
	arch    channelArchetype
	kind    streamType
	parent  string
	name    string
	version semver.Version
	index   int
}

func buildBundleList(t SemverTemplateData) map[string]string {
	dict := make(map[string]string)
	for _, bl := range []semverTemplateChannelBundles{t.Candidate, t.Fast, t.Stable} {
		for _, b := range bl.Bundles {
			if _, ok := dict[b.Image]; !ok {
				dict[b.Image] = b.Image
			}
		}
	}
	return dict
}

func readFile(reader io.Reader) (*SemverTemplateData, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	sv := SemverTemplateData{}
	if err := yaml.UnmarshalStrict(data, &sv); err != nil {
		return nil, err
	}

	if sv.Schema != schema {
		return nil, fmt.Errorf("readFile: input file has unknown schema, should be %q", schema)
	}

	// if no generate option is selected, default to GenerateMinorChannels
	if !sv.GenerateMajorChannels && !sv.GenerateMinorChannels {
		sv.GenerateMinorChannels = true
	}

	// for default channel preference,
	// if un-set, default to align to the selected generate option
	// if set, error out if we mismatch the two
	switch sv.DefaultChannelTypePreference {
	case defaultStreamType:
		if sv.GenerateMinorChannels {
			sv.DefaultChannelTypePreference = minorStreamType
		} else if sv.GenerateMajorChannels {
			sv.DefaultChannelTypePreference = majorStreamType
		}
	case minorStreamType:
		if !sv.GenerateMinorChannels {
			return nil, fmt.Errorf("schema attribute mismatch: DefaultChannelTypePreference set to 'minor' doesn't make sense if not generating minor-version channels")
		}
	case majorStreamType:
		if !sv.GenerateMajorChannels {
			return nil, fmt.Errorf("schema attribute mismatch: DefaultChannelTypePreference set to 'major' doesn't make sense if not generating major-version channels")
		}
	default:
		return nil, fmt.Errorf("unknown DefaultChannelTypePreference: %q\nValid values are 'major' or 'minor'", sv.DefaultChannelTypePreference)
	}

	return &sv, nil
}

func (sv *SemverTemplateData) getVersionsFromStandardChannels(cfg *declcfg.DeclarativeConfig, bundleDict map[string]string) (*bundleVersions, error) {
	versions := bundleVersions{}

	bdm, err := sv.getVersionsFromChannel(sv.Candidate.Bundles, bundleDict, cfg)
	if err != nil {
		return nil, err
	}
	if err = validateVersions(&bdm); err != nil {
		return nil, err
	}
	versions[candidateChannelArchetype] = bdm

	bdm, err = sv.getVersionsFromChannel(sv.Fast.Bundles, bundleDict, cfg)
	if err != nil {
		return nil, err
	}
	if err = validateVersions(&bdm); err != nil {
		return nil, err
	}
	versions[fastChannelArchetype] = bdm

	bdm, err = sv.getVersionsFromChannel(sv.Stable.Bundles, bundleDict, cfg)
	if err != nil {
		return nil, err
	}
	if err = validateVersions(&bdm); err != nil {
		return nil, err
	}
	versions[stableChannelArchetype] = bdm

	return &versions, nil
}

func (sv *SemverTemplateData) getVersionsFromChannel(semverBundles []semverTemplateBundleEntry, bundleDict map[string]string, cfg *declcfg.DeclarativeConfig) (map[string]semver.Version, error) {
	entries := make(map[string]semver.Version)

	// we iterate over the channel bundles from the template, to:
	// - identify if any required bundles for the channel are missing/not rendered/otherwise unavailable
	// - maintain the channel-bundle relationship as we map from un-rendered semver template bundles to rendered bundles in `entries` which is accumulated by the caller
	//   in a per-channel structure to which we can safely refer when generating/linking channels
	for _, semverBundle := range semverBundles {
		// test if the bundle specified in the template is present in the successfully-rendered bundles
		index := 0
		for index < len(cfg.Bundles) {
			if cfg.Bundles[index].Image == bundleDict[semverBundle.Image] {
				break
			}
			index++
		}
		if index == len(cfg.Bundles) {
			return nil, fmt.Errorf("supplied bundle image name %q not found in rendered bundle images", semverBundle.Image)
		}
		b := cfg.Bundles[index]

		props, err := property.Parse(b.Properties)
		if err != nil {
			return nil, fmt.Errorf("parse properties for bundle %q: %v", b.Name, err)
		}
		if len(props.Packages) != 1 {
			return nil, fmt.Errorf("bundle %q has multiple %q properties, expected exactly 1", b.Name, property.TypePackage)
		}
		v, err := semver.Parse(props.Packages[0].Version)
		if err != nil {
			return nil, fmt.Errorf("bundle %q has invalid version %q: %v", b.Name, props.Packages[0].Version, err)
		}

		// package name detection
		if sv.pkg != "" {
			// if we have a known package name, then ensure all subsequent packages match
			if props.Packages[0].PackageName != sv.pkg {
				return nil, fmt.Errorf("bundle %q does not belong to this package: %q", props.Packages[0].PackageName, sv.pkg)
			}
		} else {
			// else cache the first
			p := newPackage(props.Packages[0].PackageName)
			cfg.Packages = append(cfg.Packages, *p)
			sv.pkg = props.Packages[0].PackageName
		}

		if _, ok := entries[b.Name]; ok {
			return nil, fmt.Errorf("duplicate bundle name %q", b.Name)
		}

		entries[b.Name] = v
	}

	return entries, nil
}

// generates an unlinked channel for each channel as per the input template config (major || minor), then link up the edges of the set of channels so that:
// - for minor version increase, the new edge replaces the previous
// - (for major channels) iterating to a new minor version channel (traversing between Y-streams) creates a 'replaces' edge between the predecessor and successor bundles
// - within the same minor version (Y-stream), the head of the channel should have a 'skips' encompassing all lesser Y.Z versions of the bundle enumerated in the template.
// along the way, uses a highwaterChannel marker to identify the "most stable" channel head to be used as the default channel for the generated package

func (sv *SemverTemplateData) generateChannels(semverChannels *bundleVersions) []declcfg.Channel {
	outChannels := []declcfg.Channel{}

	// sort the channel archetypes in ascending order so we can traverse the bundles in order of
	// their source channel's priority
	// nolint:prealloc
	var archetypesByPriority []channelArchetype
	for k := range channelPriorities {
		archetypesByPriority = append(archetypesByPriority, k)
	}
	sort.Sort(byChannelPriority(archetypesByPriority))

	// set to the least-priority channel
	hwc := highwaterChannel{archetype: archetypesByPriority[0], version: semver.Version{Major: 0, Minor: 0}}

	unlinkedChannels := make(map[string]*declcfg.Channel)
	unassociatedEdges := []entryTuple{}

	for _, archetype := range archetypesByPriority {
		bundles := (*semverChannels)[archetype]
		// skip channel if empty
		if len(bundles) == 0 {
			continue
		}

		// sort the bundle names according to their semver, so we can walk in ascending order
		bundleNamesByVersion := []string{}
		for b := range bundles {
			bundleNamesByVersion = append(bundleNamesByVersion, b)
		}
		sort.Slice(bundleNamesByVersion, func(i, j int) bool {
			return bundles[bundleNamesByVersion[i]].LT(bundles[bundleNamesByVersion[j]])
		})

		// for each bundle (by version):
		//   for each of Major/Minor setting (since they're independent)
		//     retrieve the existing channel object, or create a channel (by criteria major/minor) if one doesn't exist
		//     add a new edge entry based on the bundle name
		//     save the channel name --> channel archetype mapping
		//     test the channel object for 'more stable' than previous best
		for _, bundleName := range bundleNamesByVersion {
			// a dodge to avoid duplicating channel processing body; accumulate a map of the channels which need creating from the bundle
			// we need to associate by kind so we can partition the resulting entries
			channelNameKeys := make(map[streamType]string)
			if sv.GenerateMajorChannels {
				channelNameKeys[majorStreamType] = channelNameFromMajor(archetype, bundles[bundleName])
			}
			if sv.GenerateMinorChannels {
				channelNameKeys[minorStreamType] = channelNameFromMinor(archetype, bundles[bundleName])
			}

			for cKey, cName := range channelNameKeys {
				ch, ok := unlinkedChannels[cName]
				if !ok {
					ch = newChannel(sv.pkg, cName)

					unlinkedChannels[cName] = ch

					hwcCandidate := highwaterChannel{archetype: archetype, kind: cKey, version: bundles[bundleName], name: cName}
					if hwcCandidate.gt(&hwc, sv.DefaultChannelTypePreference) {
						hwc = hwcCandidate
					}
				}
				ch.Entries = append(ch.Entries, declcfg.ChannelEntry{Name: bundleName})
				unassociatedEdges = append(unassociatedEdges, entryTuple{arch: archetype, kind: cKey, parent: cName, name: bundleName, version: bundles[bundleName], index: len(ch.Entries) - 1})
			}
		}
	}

	// save off the name of the high-water-mark channel for the default for this package
	sv.defaultChannel = hwc.name

	outChannels = append(outChannels, sv.linkChannels(unlinkedChannels, unassociatedEdges)...)

	return outChannels
}

func (sv *SemverTemplateData) linkChannels(unlinkedChannels map[string]*declcfg.Channel, entries []entryTuple) []declcfg.Channel {
	channels := []declcfg.Channel{}

	// sort to force partitioning by archetype --> kind --> semver
	sort.Slice(entries, func(i, j int) bool {
		if channelPriorities[entries[i].arch] != channelPriorities[entries[j].arch] {
			return channelPriorities[entries[i].arch] < channelPriorities[entries[j].arch]
		}
		if streamTypePriorities[entries[i].kind] != streamTypePriorities[entries[j].kind] {
			return streamTypePriorities[entries[i].kind] < streamTypePriorities[entries[j].kind]
		}
		return entries[i].version.LT(entries[j].version)
	})

	prevZMax := ""
	var curSkips = sets.Set[string]{}

	// iterate over the entries, starting from the second
	// write any skips/replaces for the previous entry to the current entry
	// then accumulate the skips/replaces for the current entry to be used in subsequent iterations
	for index := 1; index < len(entries); index++ {
		prevTuple := entries[index-1]
		curTuple := entries[index]
		prevX := getMajorVersion(prevTuple.version)
		prevY := getMinorVersion(prevTuple.version)
		curX := getMajorVersion(curTuple.version)
		curY := getMinorVersion(curTuple.version)

		archChange := curTuple.arch != prevTuple.arch
		kindChange := curTuple.kind != prevTuple.kind
		xChange := !prevX.EQ(curX)
		yChange := !prevY.EQ(curY)

		if archChange || kindChange || xChange || yChange {
			// if we passed any kind of change besides Z, then we need to set skips/replaces for previous max-Z
			prevChannel := unlinkedChannels[prevTuple.parent]
			finalEntry := &prevChannel.Entries[prevTuple.index]
			finalEntry.Replaces = prevZMax
			skips := sets.List(curSkips.Difference(sets.New(finalEntry.Replaces)))
			if len(skips) > 0 {
				finalEntry.Skips = skips
			}
		}

		if archChange || kindChange || xChange {
			// we don't maintain skips/replaces over these transitions
			curSkips = sets.Set[string]{}
			prevZMax = ""
		} else {
			if yChange {
				prevZMax = prevTuple.name
			}
			curSkips.Insert(prevTuple.name)
		}
	}

	if len(entries) > 1 {
		// add edges for the last entry
		// note:  this is substantially similar to the main iteration, but there are some subtle differences since the main loop mode
		//  design is to write the edges and then accumulate new info for subsequent edges (and this is the last edge):
		// - we only need to watch for arch/kind/x change
		// - we don't need to accumulate skips/replaces, since we're not writing edges for subsequent entries
		lastTuple := entries[len(entries)-1]
		penultimateTuple := entries[len(entries)-2]
		prevX := getMajorVersion(penultimateTuple.version)
		curX := getMajorVersion(lastTuple.version)

		archChange := penultimateTuple.arch != lastTuple.arch
		kindChange := penultimateTuple.kind != lastTuple.kind
		xChange := !prevX.EQ(curX)
		// for arch / kind / x changes, we don't maintain skips/replaces
		if !archChange && !kindChange && !xChange {
			prevChannel := unlinkedChannels[lastTuple.parent]
			finalEntry := &prevChannel.Entries[lastTuple.index]
			finalEntry.Replaces = prevZMax
			skips := sets.List(curSkips.Difference(sets.New(finalEntry.Replaces)))
			if len(skips) > 0 {
				finalEntry.Skips = skips
			}
		}
	}

	for _, ch := range unlinkedChannels {
		channels = append(channels, *ch)
	}

	slices.SortFunc(channels, func(a, b declcfg.Channel) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return channels
}

func channelNameFromMinor(prefix channelArchetype, version semver.Version) string {
	return fmt.Sprintf("%s-v%d.%d", prefix, version.Major, version.Minor)
}

func channelNameFromMajor(prefix channelArchetype, version semver.Version) string {
	return fmt.Sprintf("%s-v%d", prefix, version.Major)
}

func newPackage(name string) *declcfg.Package {
	return &declcfg.Package{
		Schema:         "olm.package",
		Name:           name,
		DefaultChannel: "",
	}
}

func newChannel(pkgName string, chName string) *declcfg.Channel {
	return &declcfg.Channel{
		Schema:  "olm.channel",
		Name:    chName,
		Package: pkgName,
		Entries: []declcfg.ChannelEntry{},
	}
}

func combineConfigs(cfgs []declcfg.DeclarativeConfig) *declcfg.DeclarativeConfig {
	out := &declcfg.DeclarativeConfig{}
	for _, in := range cfgs {
		out.Merge(&in)
	}
	return out
}

func getMinorVersion(v semver.Version) semver.Version {
	return semver.Version{
		Major: v.Major,
		Minor: v.Minor,
	}
}

// nolint:unused
func getMajorVersion(v semver.Version) semver.Version {
	return semver.Version{
		Major: v.Major,
	}
}

func withoutBuildMetadataConflict(versions *map[string]semver.Version) error {
	errs := []error{}

	// using the stringified semver because the semver package generates deterministic representations,
	// and because the semver.Version contains slice fields which make it unsuitable as a map key
	//      stringified-semver.Version ==> incidence count
	seen := make(map[string]int)
	for b := range *versions {
		stripped := stripBuildMetadata((*versions)[b])
		if _, ok := seen[stripped]; !ok {
			seen[stripped] = 1
		} else {
			seen[stripped] = seen[stripped] + 1
			errs = append(errs, fmt.Errorf("bundle version %q cannot be compared to %q", (*versions)[b].String(), stripped))
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("encountered bundle versions which differ only by build metadata, which cannot be ordered: %v", errors.NewAggregate(errs))
	}

	return nil
}

func validateVersions(versions *map[string]semver.Version) error {
	// short-circuit if empty, since that is not an error
	if len(*versions) == 0 {
		return nil
	}
	return withoutBuildMetadataConflict(versions)
}

// strips out the build metadata from a semver.Version and then stringifies it to make it suitable for collision detection
func stripBuildMetadata(v semver.Version) string {
	v.Build = nil
	return v.String()
}

// prefer (in descending order of preference):
// - higher-rank archetype,
// - semver version,
// - a channel type matching the set preference, or
// - a 'better' (higher value) channel type
func (h *highwaterChannel) gt(ih *highwaterChannel, pref streamType) bool {
	if channelPriorities[h.archetype] != channelPriorities[ih.archetype] {
		return channelPriorities[h.archetype] > channelPriorities[ih.archetype]
	}
	if h.version.NE(ih.version) {
		return h.version.GT(ih.version)
	}
	if h.kind != ih.kind {
		if h.kind == pref {
			return true
		}
		if ih.kind == pref {
			return false
		}
		return h.kind.gt((*ih).kind)
	}
	return false
}

func (t streamType) gt(in streamType) bool {
	return streamTypePriorities[t] > streamTypePriorities[in]
}
