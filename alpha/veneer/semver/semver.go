package semver

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"sort"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// data passed into this module externally
type Veneer struct {
	Ref string
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
	Stable                stableBundles    `json:"stable"`
}

// IO structs -- END

const (
	CandidateChannelName = "candidate"
	FastChannelName      = "fast"
	StableChannelName    = "stable"
)

// The order in which to choose a default channel for a package
// For the earliest prefix with any non-empty channels generated,
// the highest version with such a non-empty channel will be preferred as the default.
var safeDefaultChannelPrefixPriority = []string{StableChannelName, FastChannelName, CandidateChannelName}

type decomposedCatalog struct {
	pkg      string
	channels map[string][]*decomposedBundleEntry
}

type decomposedBundleEntry struct {
	img string
	ver semver.Version
}

func findit(s string, bundles []declcfg.Bundle) int {

	for index := range bundles {
		if bundles[index].Image == s {
			return index
		}
	}
	return len(bundles)
}

func decomposeBundle(bundle *semverVeneerBundleEntry, cfg *declcfg.DeclarativeConfig) (*decomposedBundleEntry, error) {
	// from inputs,
	// find the Bundle from the input name (bundle.Image in cfg.Bundles)
	// decompose the bundle into its relevant constituent elements
	// validate that:
	// 1 - the named bundle can be found in the []Bundle
	// 2 - there's only one olm.package property in the Bundle
	// 3 - the version is parse-able
	// 4 - the olm.package:packageName matches those already in the cfg.Packages
	// index := sort.Search(len(cfg.Bundles), func(i int) bool { return bundle.Image == cfg.Bundles[i].Image })
	index := findit(bundle.Image, cfg.Bundles)
	if index == len(cfg.Bundles) {
		return nil, fmt.Errorf("supplied bundle image name %q not found in rendered bundle images", bundle.Image)
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
	if len(cfg.Packages) > 0 {
		// if we have a known package name, then ensure all new packages match
		if props.Packages[0].PackageName != cfg.Packages[0].Name {
			return nil, fmt.Errorf("bundle %q does not belong to this package: %q", props.Packages[0].PackageName, cfg.Packages[0].Name)
		}
	} else {
		// if we don't currently have a package started in the catalog, cache the first
		p := newPackage(props.Packages[0].PackageName)
		cfg.Packages = append(cfg.Packages, *p)
	}
	return &decomposedBundleEntry{
		img: b.Name,
		ver: v,
	}, nil
}

func addBundlesToChannel(bundles []semverVeneerBundleEntry, cfg *declcfg.DeclarativeConfig) ([]*decomposedBundleEntry, error) {
	entries := []*decomposedBundleEntry{}
	for _, b := range bundles {
		// fmt.Printf("  <--> adding bundle %q\n", b.Image)

		e, err := decomposeBundle(&b, cfg)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (sv *semverVeneer) addBundlesToStandardChannels(cfg *declcfg.DeclarativeConfig) (*decomposedCatalog, error) {
	isvd := decomposedCatalog{channels: map[string][]*decomposedBundleEntry{}}

	bdm, err := addBundlesToChannel(sv.Candidate.Bundles, cfg)
	if err != nil {
		return nil, err
	}
	isvd.channels[CandidateChannelName] = bdm

	bdm, err = addBundlesToChannel(sv.Fast.Bundles, cfg)
	if err != nil {
		return nil, err
	}
	isvd.channels[FastChannelName] = bdm

	bdm, err = addBundlesToChannel(sv.Stable.Bundles, cfg)
	if err != nil {
		return nil, err
	}
	isvd.channels[StableChannelName] = bdm

	if len(cfg.Bundles) > 0 {
		isvd.pkg = cfg.Bundles[0].Package
	}

	return &isvd, nil
}

func ReadFile(ref string) (*semverVeneer, error) {
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

func (v Veneer) Render(ctx context.Context) (*declcfg.DeclarativeConfig, error) {
	var out declcfg.DeclarativeConfig
	// fmt.Printf("<--> Received config: skip(%t) major(%t) minor(%t) ref(%s)\n", v.SkipPatch, v.ChannelsMajor, v.ChannelsMinor, v.Ref)

	sv, err := ReadFile(v.Ref)
	if err != nil {
		log.Fatalf("semver-render: unable to read file: %v", err)
	}
	// fmt.Printf("Semver-Veneer parsed\n")
	// sv.write()

	var cfgs []declcfg.DeclarativeConfig
	for _, b := range sv.Candidate.Bundles {
		r := action.Render{
			AllowedRefMask: action.RefBundleImage,
			Refs:           []string{b.Image},
		}
		c, err := r.Run(ctx)
		if err != nil {
			return nil, err
		}
		cfgs = append(cfgs, *c)
	}
	out = *combineConfigs(cfgs)

	fmt.Printf("all bundles parsed\n")
	for _, b := range out.Bundles {
		fmt.Printf("{ bundle Name:%q Image:%q }\n", b.Name, b.Image)
	}

	semverCatalog, err := sv.addBundlesToStandardChannels(&out)
	if err != nil {
		log.Fatalf("semver-render: unable to post-process bundle info: %v", err)
	}
	// sv.write()
	// fmt.Printf("<--> <-->\n")
	// isvd.write()
	channels := sv.decomposeChannels(semverCatalog)
	out.Channels = channels

	choosePackageDefaultChannel(&out, safeDefaultChannelPrefixPriority)
	return &out, nil
}

func choosePackageDefaultChannel(dc *declcfg.DeclarativeConfig, defaultChannelPriority []string) {
	channelPrefixList := []string{}
	channelPrefixList = append(channelPrefixList, defaultChannelPriority...)
	sort.Slice(channelPrefixList, func(i, j int) bool {
		//longest prefix first.
		return len(channelPrefixList[i]) > len(channelPrefixList[j])
	})

	for i, p := range dc.Packages {
		prefixedChannels := map[string]string{}
		if len(p.DefaultChannel) > 0 {
			continue
		}

		for _, c := range dc.Channels {
			if c.Package != p.Name {
				continue
			}

			for _, pre := range channelPrefixList {
				version := versionFromChannelName(c.Name, pre)
				if version == nil {
					// channel doesn't have this prefix
					continue
				}

				if len(prefixedChannels[pre]) == 0 && len(c.Name) != 0 {
					prefixedChannels[pre] = c.Name
				} else {
					prefixedVersion := versionFromChannelName(prefixedChannels[pre], pre)
					if prefixedVersion.LT(*version) {
						prefixedChannels[pre] = c.Name
					} else if prefixedVersion.EQ(*version) && len(prefixedChannels[pre]) > len(c.Name) {
						// Prefer a major version channel over a minor version channel
						// stable-v1 > stable-v1.0
						prefixedChannels[pre] = c.Name
					}
				}
				// only the longest prefix must be matched
				break
			}
		}

		// first channel prefix in the priority list with the highest version.
		for _, pre := range defaultChannelPriority {
			if len(prefixedChannels[pre]) != 0 {
				dc.Packages[i].DefaultChannel = prefixedChannels[pre]
				break
			}
		}
	}
}

// all channels that come to addChannels MUST have the same prefix. This adds replaces edges of minor versions of the largest major version.
func (sv *semverVeneer) addChannels(data map[string][]*decomposedBundleEntry, pkg string) []declcfg.Channel {
	channels := []declcfg.Channel{}

	type channelParts struct {
		prefix  string
		version semver.Version
		index   int
	}
	channelNameParts := []channelParts{}

	for cname, bundles := range data {
		if len(bundles) == 0 {
			continue
		}
		c := newChannel(pkg, cname)
		sort.Slice(bundles, func(i, j int) bool {
			return bundles[i].ver.LT(bundles[j].ver)
		})

		currMinor := semver.Version{Major: bundles[0].ver.Major, Minor: bundles[0].ver.Minor}
		minorVersionStartIndex := 0
		var prevVersion string
		for i := range bundles {
			if sv.AvoidSkipPatch {
				c.Entries = append(c.Entries, declcfg.ChannelEntry{Name: bundles[i].img, Replaces: prevVersion})
				prevVersion = bundles[i].img
				if (i != len(bundles)-1) && currMinor.EQ(semver.Version{Major: bundles[i+1].ver.Major, Minor: bundles[i+1].ver.Minor}) {
					continue
				}
				if i == len(bundles)-1 {
					// we've already processed the entire bundle set, don't update any loop variables.
					break
				}
				prevVersion = bundles[i].img // The version that gets added to the replaces field of the bundle with the highest version among the next minor version
				currMinor = semver.Version{Major: bundles[i+1].ver.Major, Minor: bundles[i+1].ver.Minor}
				minorVersionStartIndex = i + 1 // The bundle from which we start adding skips entries
				continue
			}

			c.Entries = append(c.Entries, declcfg.ChannelEntry{Name: bundles[i].img})

			// if either the bundle processed is the last in the list or
			// the next bundle has a different minor version from the current bundle,
			// add skips entries to the current bundles.
			if (i != len(bundles)-1) && currMinor.EQ(semver.Version{Major: bundles[i+1].ver.Major, Minor: bundles[i+1].ver.Minor}) {
				continue
			}

			curSkips := sets.NewString()
			for j := minorVersionStartIndex; j < i; j++ {
				// previous n entries were all of the same minor.
				curSkips = curSkips.Insert(c.Entries[j].Name)
			}
			c.Entries[i].Replaces = prevVersion // this should be empty for minor version channels
			if curSkips.Len() > 0 {
				c.Entries[i].Skips = curSkips.List()
			}

			if i == len(bundles)-1 {
				// we've already processed the entire bundle set, don't update any loop variables.
				break
			}

			prevVersion = bundles[i].img // The version that gets added to the replaces field of the bundle with the highest version among the next minor version
			currMinor = semver.Version{Major: bundles[i+1].ver.Major, Minor: bundles[i+1].ver.Minor}
			minorVersionStartIndex = i + 1 // The bundle from which we start adding skips entries
		}

		//// add all (unlinked) entries to channel
		//for _, b := range bundles {
		//	c.Entries = append(c.Entries, declcfg.ChannelEntry{Name: b.img})
		//}
		//
		//// link up the edges according to config
		//if sv.AvoidSkipPatch {
		//	for i := 1; i < len(c.Entries); i++ {
		//		c.Entries[i] = declcfg.ChannelEntry{
		//			Name:     c.Entries[i].Name,
		//			Replaces: c.Entries[i-1].Name,
		//		}
		//	}
		//} else {
		//
		//	maxIndex := len(c.Entries) - 1
		//	curSkips := sets.NewString()
		//
		//	for i := 0; i < maxIndex; i++ {
		//		curSkips = curSkips.Insert(c.Entries[i].Name)
		//	}
		//	c.Entries[maxIndex] = declcfg.ChannelEntry{
		//		Name:  c.Entries[maxIndex].Name,
		//		Skips: curSkips.List(),
		//	}
		//}

		channelPrefix := prefixFromChannelName(cname, bundles[0].ver)
		channelVersion := versionFromChannelName(cname, channelPrefix)
		if c.Name == minorFromVersion(channelPrefix, *channelVersion) {
			channelNameParts = append(channelNameParts, channelParts{
				prefix:  channelPrefix,
				version: *channelVersion,
				index:   len(channels),
			})
		}

		channels = append(channels, *c)
	}

	// sort the channel ref map by their prefix + version. This should only contain indices of minor version channels
	sort.Slice(channelNameParts, func(i, j int) bool {
		if channelNameParts[i].prefix != channelNameParts[j].prefix {
			return channelNameParts[i].prefix < channelNameParts[j].prefix
		}
		return channelNameParts[i].version.LT(channelNameParts[j].version)
	})

	for i := range channelNameParts {
		if i == 0 {
			continue
		}
		if channelNameParts[i].version.Major != channelNameParts[i-1].version.Major {
			// No edges across different major versions
			continue
		}

		prevChanEntries := channels[channelNameParts[i-1].index].Entries
		prevChanLastEntry := channels[channelNameParts[i-1].index].Entries[len(prevChanEntries)-1]

		currChanEntries := channels[channelNameParts[i].index].Entries
		var currChanLastEntry *declcfg.ChannelEntry
		if sv.AvoidSkipPatch {
			// in no skipPatch mode, add the edge from previous channel minor version to first bundle (smallest version) in current minor version channel
			currChanLastEntry = &channels[channelNameParts[i].index].Entries[0]
		} else {
			// in skipPatch mode, the replaces edge from the previous channel minor version goes to the channel head (largest version)
			currChanLastEntry = &channels[channelNameParts[i].index].Entries[len(currChanEntries)-1]
		}

		if len(currChanLastEntry.Replaces) != 0 && currChanLastEntry.Replaces != prevChanLastEntry.Name {
			// This should never happen
			currChanLastEntry.Skips = append(currChanLastEntry.Skips, currChanLastEntry.Replaces)
		}
		currChanLastEntry.Replaces = prevChanLastEntry.Name
	}

	return channels
}

// generates a channel for each channel in the map, containing:
//   edges for each bundle with a predicted bundle name composed of
//   the bundle package + bundle version, with a "." delimiter, e.g.:
//   foo with version 0.1.0 ==> foo.0.1.0
// generates a bundle for each predicted bundle name
// for now, the name composition is fixed, but should be expanded to utilize user-supplied templates
func (sv *semverVeneer) decomposeChannels(catalog *decomposedCatalog) []declcfg.Channel {
	outChannels := []declcfg.Channel{}

	// fmt.Printf("<isvd.Schema> %s\n", data.Schema)
	// fmt.Printf("<isvd.Channels> %s\n", channels)
	for cname, blist := range catalog.channels {
		majors := map[string][]*decomposedBundleEntry{}
		minors := map[string][]*decomposedBundleEntry{}
		sort.Slice(blist, func(i, j int) bool {
			return blist[i].ver.LT(blist[j].ver)
		})

		var minorChannelNames []string
		for _, b := range blist {
			if sv.GenerateMajorChannels {
				testChannelName := majorFromVersion(cname, b.ver)
				if _, ok := majors[testChannelName]; !ok {
					majors[testChannelName] = []*decomposedBundleEntry{}
				}
				majors[testChannelName] = append(majors[testChannelName], b)
			}
			if sv.GenerateMinorChannels {
				testChannelName := minorFromVersion(cname, b.ver)
				if _, ok := minors[testChannelName]; !ok {
					minors[testChannelName] = []*decomposedBundleEntry{}
					minorChannelNames = append(minorChannelNames, testChannelName)
				}
				minors[testChannelName] = append(minors[testChannelName], b)
			}
		}

		outChannels = append(outChannels, sv.addChannels(majors, catalog.pkg)...)
		outChannels = append(outChannels, sv.addChannels(minors, catalog.pkg)...)
	}

	return outChannels
}

func minorFromVersion(prefix string, version semver.Version) string {
	return fmt.Sprintf("%s-v%d.%d", prefix, version.Major, version.Minor)
}

func majorFromVersion(prefix string, version semver.Version) string {
	return fmt.Sprintf("%s-v%d", prefix, version.Major)
}

func versionFromChannelName(channel, prefix string) *semver.Version {
	// assuming '<prefix>-v<version>' format:
	if !strings.HasPrefix(channel, fmt.Sprintf("%s-v", prefix)) {
		return nil
	}

	//edge case with weird names?
	version, err := semver.ParseTolerant(channel[len(prefix)+2:])
	if err != nil {
		// This probably shouldn't get swallowed here
		return nil
	}
	return &version
}

func prefixFromChannelName(channel string, version semver.Version) string {
	if suffix := fmt.Sprintf("-v%d.%d", version.Major, version.Minor); strings.HasSuffix(channel, suffix) {
		return channel[:len(channel)-len(suffix)]
	}
	if suffix := fmt.Sprintf("-v%d", version.Major); strings.HasSuffix(channel, suffix) {
		return channel[:len(channel)-len(suffix)]
	}
	// No matching prefix
	return ""
}

func newPackage(name string) *declcfg.Package {
	return &declcfg.Package{
		Schema:         "olm.package",
		Name:           name,
		DefaultChannel: "",
	}
}

func newChannel(pkgName, chName string) *declcfg.Channel {
	return &declcfg.Channel{
		Schema:  "olm.channel",
		Name:    chName,
		Package: pkgName,
	}
}

func newBundle(pkg, name, image string) *declcfg.Bundle {
	return &declcfg.Bundle{
		Schema:  "olm.bundle",
		Package: pkg,
		Image:   image,
		Name:    name,
	}
}

func (sv *semverVeneer) write() error {
	fmt.Printf("schema: %s\n", "olm.semver")

	fmt.Printf("generatemajorchannels: %t\n", sv.GenerateMajorChannels)
	fmt.Printf("generateminorchannels: %t\n", sv.GenerateMinorChannels)
	fmt.Printf("avoidSkipPatch: %t\n", sv.AvoidSkipPatch)

	fmt.Printf("candidate:\n")
	fmt.Printf("  bundles:\n")
	for _, b := range sv.Candidate.Bundles {
		fmt.Printf("  - image: %s\n", b.Image)
	}
	fmt.Printf("fast:\n")
	fmt.Printf("  bundles:\n")
	for _, b := range sv.Fast.Bundles {
		fmt.Printf("  - image: %s\n", b.Image)
	}

	fmt.Printf("stable:\n")
	fmt.Printf("  bundles:\n")
	for _, b := range sv.Stable.Bundles {
		fmt.Printf("  - image: %s\n", b.Image)
	}

	return nil
}

func (channels *decomposedCatalog) write() {
	for cname, bmap := range (*channels).channels {
		fmt.Printf("%s:\n", cname)
		fmt.Printf("  bundles:\n")
		for _, b := range bmap {
			fmt.Printf("  - image: %s:%s\n", b.img, b.ver)
		}
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
