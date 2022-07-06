package semver

import (
	"context"
	"fmt"
	"io/ioutil"
	"sort"

	"github.com/blang/semver/v4"
	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
	"github.com/operator-framework/operator-registry/pkg/image"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// data passed into this module externally
type Veneer struct {
	Ref string
	Reg image.Registry
}

// IO structs -- BEGIN
type semverVeneerBundleEntry struct {
	Image string `json:"image,omitempty"`
}

type candidateBundles struct {
	Bundles []semverVeneerBundleEntry `json:"bundles,omitempty"`
}
type fastBundles struct {
	Bundles []semverVeneerBundleEntry `json:"bundles,omitempty"`
}
type stableBundles struct {
	Bundles []semverVeneerBundleEntry `json:"bundles,omitempty"`
}

type semverVeneer struct {
	Schema                string           `json:"schema"`
	GenerateMajorChannels bool             `json:"generateMajorChannels,omitempty"`
	GenerateMinorChannels bool             `json:"generateMinorChannels,omitempty"`
	AvoidSkipPatch        bool             `json:"avoidSkipPatch,omitempty"`
	Candidate             candidateBundles `json:"candidate,omitempty"`
	Fast                  fastBundles      `json:"fast,omitempty"`
	Stable                stableBundles    `json:"stable,omitempty"`

	pkg            string `json:"-"` // the derived package name
	defaultChannel string `json:"-"` // detected "most stable" channel head
}

// IO structs -- END

// channel "kinds", restricted in this iteration to just these
const (
	candidateChannelName string = "candidate"
	fastChannelName      string = "fast"
	stableChannelName    string = "stable"
)

// mapping channel name --> stability, where higher values indicate greater stability
var channelPriorities = map[string]int{candidateChannelName: 0, fastChannelName: 1, stableChannelName: 2}

// sorting capability for a slice according to the assigned channelPriorities
type byChannelPriority []string

func (b byChannelPriority) Len() int { return len(b) }
func (b byChannelPriority) Less(i, j int) bool {
	return channelPriorities[b[i]] < channelPriorities[b[j]]
}
func (b byChannelPriority) Swap(i, j int) { b[i], b[j] = b[j], b[i] }

// map of channels : bundles : bundle-version
// channels --> bundles --> version
type semverRenderedChannelVersions map[string]map[string]semver.Version // e.g. d["stable-v1"]["example-operator/v1.0.0"] = 1.0.0

func (v Veneer) Render(ctx context.Context) (*declcfg.DeclarativeConfig, error) {
	var out declcfg.DeclarativeConfig

	sv, err := readFile(v.Ref)
	if err != nil {
		return nil, fmt.Errorf("semver-render: unable to read file: %v", err)
	}

	var cfgs []declcfg.DeclarativeConfig
	for _, b := range sv.Candidate.Bundles {
		r := action.Render{
			AllowedRefMask: action.RefBundleImage,
			Refs:           []string{b.Image},
			Registry:       v.Reg,
		}
		c, err := r.Run(ctx)
		if err != nil {
			return nil, err
		}
		cfgs = append(cfgs, *c)
	}
	out = *combineConfigs(cfgs)

	if len(out.Bundles) == 0 {
		return nil, fmt.Errorf("semver-render: no bundles specified or no bundles could be rendered")
	}

	channelBundleVersions, err := sv.getVersionsFromStandardChannels(&out)
	if err != nil {
		return nil, fmt.Errorf("semver-render: unable to post-process bundle info: %v", err)
	}

	channels := sv.generateChannels(channelBundleVersions)
	out.Channels = channels
	out.Packages[0].DefaultChannel = sv.defaultChannel

	return &out, nil
}

func readFile(ref string) (*semverVeneer, error) {
	data, err := ioutil.ReadFile(ref)
	if err != nil {
		return nil, err
	}

	// default behavior is to generate only minor channels and to use skips over replaces
	sv := semverVeneer{
		GenerateMajorChannels: false,
		GenerateMinorChannels: true,
		AvoidSkipPatch:        false,
	}
	if err := yaml.Unmarshal(data, &sv); err != nil {
		return nil, err
	}
	return &sv, nil
}

func (sv *semverVeneer) getVersionsFromStandardChannels(cfg *declcfg.DeclarativeConfig) (*semverRenderedChannelVersions, error) {
	versions := semverRenderedChannelVersions{}

	bdm, err := sv.getVersionsFromChannel(sv.Candidate.Bundles, cfg)
	if err != nil {
		return nil, err
	}
	if err = validateVersions(&bdm); err != nil {
		return nil, err
	}
	versions[candidateChannelName] = bdm

	bdm, err = sv.getVersionsFromChannel(sv.Fast.Bundles, cfg)
	if err != nil {
		return nil, err
	}
	if err = validateVersions(&bdm); err != nil {
		return nil, err
	}
	versions[fastChannelName] = bdm

	bdm, err = sv.getVersionsFromChannel(sv.Stable.Bundles, cfg)
	if err != nil {
		return nil, err
	}
	if err = validateVersions(&bdm); err != nil {
		return nil, err
	}
	versions[stableChannelName] = bdm

	return &versions, nil
}

func (sv *semverVeneer) getVersionsFromChannel(semverBundles []semverVeneerBundleEntry, cfg *declcfg.DeclarativeConfig) (map[string]semver.Version, error) {
	entries := make(map[string]semver.Version)

	// we iterate over the channel bundles from the veneer, to:
	// - identify if any required bundles for the channel are missing/not rendered/otherwise unavailable
	// - maintain the channel-bundle relationship as we map from un-rendered semver veneer bundles to rendered bundles in `entries` which is accumulated by the caller
	//   in a per-channel structure to which we can safely refer when generating/linking channels
	for _, semverBundle := range semverBundles {
		// test if the bundle specified in the veneer is present in the successfully-rendered bundles
		index := 0
		for index < len(cfg.Bundles) {
			if cfg.Bundles[index].Image == semverBundle.Image {
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

// the "high-water channel" struct functions as a freely-rising indicator of the "most stable" channel head, so we can use that
// later as the package's defaultChannel attribute
type highwaterChannel struct {
	kind    string
	version semver.Version
	name    string
}

func (h *highwaterChannel) gt(ih *highwaterChannel) bool {
	return (channelPriorities[h.kind] > channelPriorities[ih.kind]) || (h.version.GT(ih.version))
}

// generates an unlinked channel for each channel as per the input veneer config (major || minor), then link up the edges of the set of channels so that:
// - (for major channels) iterating to a new minor version channel (traversing between Y-streams) creates a 'replaces' edge between the predecessor and successor bundles
// - within the same minor version (Y-stream), the head of the channel should have a 'skips' encompassing all lesser minor versions of the bundle enumerated in the veneer.
// along the way, uses a highwaterChannel marker to identify the "most stable" channel head to be used as the default channel for the generated package
func (sv *semverVeneer) generateChannels(semverChannels *semverRenderedChannelVersions) []declcfg.Channel {
	outChannels := []declcfg.Channel{}

	// sort the channelkinds in ascending order so we can traverse the bundles in order of
	// their source channel's priority
	var keysByPriority []string
	for k, _ := range channelPriorities {
		keysByPriority = append(keysByPriority, k)
	}
	sort.Sort(byChannelPriority(keysByPriority))

	// set to the least-priority channel
	hwc := highwaterChannel{kind: keysByPriority[0], version: semver.Version{Major: 0, Minor: 0}}

	// mapping the generated channel name to the original semver name (i.e. channel kind), so we can do generated-channel-name --> original-semver-name --> version mapping, later
	channelMapping := map[string]string{}

	for _, k := range keysByPriority {
		bundles := (*semverChannels)[k]

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

		majors := map[string]*declcfg.Channel{}
		minors := map[string]*declcfg.Channel{}

		for _, b := range bundleNamesByVersion {
			if sv.GenerateMajorChannels {
				testChannelName := channelNameFromMajor(k, bundles[b])
				ch, ok := majors[testChannelName]
				if !ok {
					ch = newChannel(sv.pkg, testChannelName)
					majors[testChannelName] = ch
				}
				ch.Entries = append(ch.Entries, declcfg.ChannelEntry{Name: b})

				channelMapping[testChannelName] = k

				hwcCandidate := highwaterChannel{kind: k, version: bundles[b], name: testChannelName}
				if hwcCandidate.gt(&hwc) {
					hwc = hwcCandidate
				}
			}
			if sv.GenerateMinorChannels {
				testChannelName := channelNameFromMinor(k, bundles[b])
				ch, ok := minors[testChannelName]
				if !ok {
					ch = newChannel(sv.pkg, testChannelName)
					minors[testChannelName] = ch
				}
				ch.Entries = append(ch.Entries, declcfg.ChannelEntry{Name: b})

				channelMapping[testChannelName] = k

				hwcCandidate := highwaterChannel{kind: k, version: bundles[b], name: testChannelName}
				if hwcCandidate.gt(&hwc) {
					hwc = hwcCandidate
				}
			}
		}

		outChannels = append(outChannels, sv.linkChannels(majors, sv.pkg, semverChannels, &channelMapping)...)
		outChannels = append(outChannels, sv.linkChannels(minors, sv.pkg, semverChannels, &channelMapping)...)
	}

	// save off the name of the high-water-mark channel for the default for this package
	sv.defaultChannel = hwc.name

	return outChannels
}

// all channels that come to linkChannels MUST have the same prefix. This adds replaces edges of minor versions of the largest major version.
func (sv *semverVeneer) linkChannels(unlinkedChannels map[string]*declcfg.Channel, pkg string, semverChannels *semverRenderedChannelVersions, channelMapping *map[string]string) []declcfg.Channel {
	channels := []declcfg.Channel{}

	for channelName, channel := range unlinkedChannels {
		// sort the channel entries in ascending order, according to the corresponding bundle versions for the channelName stored in the semverRenderedChannelVersions
		// convenience function, to make this more clear
		versionLookup := func(generatedChannelName string, channelEdgeIndex int) semver.Version {
			channelKind := (*channelMapping)[generatedChannelName]
			bundleVersions := (*semverChannels)[channelKind]
			bundleName := channel.Entries[channelEdgeIndex].Name
			return bundleVersions[bundleName]
		}

		sort.Slice(channel.Entries, func(i, j int) bool {
			return versionLookup(channelName, i).LT(versionLookup(channelName, j))
		})

		// link up the edges according to config
		if sv.AvoidSkipPatch {
			for i := 1; i < len(channel.Entries); i++ {
				channel.Entries[i] = declcfg.ChannelEntry{
					Name:     channel.Entries[i].Name,
					Replaces: channel.Entries[i-1].Name,
				}
			}
		} else {
			curIndex := len(channel.Entries) - 1
			curMinor := getMinorVersion((*semverChannels)[(*channelMapping)[channelName]][channel.Entries[curIndex].Name])
			curSkips := sets.NewString()
			for i := len(channel.Entries) - 2; i >= 0; i-- {
				thisName := channel.Entries[i].Name
				thisMinor := getMinorVersion((*semverChannels)[(*channelMapping)[channelName]][thisName])
				if thisMinor.EQ(curMinor) {
					channel.Entries[i] = declcfg.ChannelEntry{Name: thisName}
					curSkips = curSkips.Insert(thisName)
				} else {
					channel.Entries[curIndex] = declcfg.ChannelEntry{
						Name:     channel.Entries[curIndex].Name,
						Replaces: thisName,
						Skips:    curSkips.List(),
					}
					curSkips = sets.NewString()
					curIndex = i
					curMinor = thisMinor
				}
			}
			channel.Entries[curIndex] = declcfg.ChannelEntry{
				Name:  channel.Entries[curIndex].Name,
				Skips: curSkips.List(),
			}
		}
		channels = append(channels, *channel)
	}
	return channels
}

func channelNameFromMinor(prefix string, version semver.Version) string {
	return fmt.Sprintf("%s-v%d.%d", prefix, version.Major, version.Minor)
}

func channelNameFromMajor(prefix string, version semver.Version) string {
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
		Name:    string(chName),
		Package: pkgName,
		Entries: []declcfg.ChannelEntry{},
	}
}

func combineConfigs(cfgs []declcfg.DeclarativeConfig) *declcfg.DeclarativeConfig {
	out := &declcfg.DeclarativeConfig{}
	for _, in := range cfgs {
		out.Packages = append(out.Packages, in.Packages...)
		out.Channels = append(out.Channels, in.Channels...)
		out.Bundles = append(out.Bundles, in.Bundles...)
		out.Others = append(out.Others, in.Others...)
	}
	return out
}

func getMinorVersion(v semver.Version) semver.Version {
	return semver.Version{
		Major: v.Major,
		Minor: v.Minor,
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
