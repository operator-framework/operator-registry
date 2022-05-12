package semver

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
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

// isvd.Channels["stable"] --> quay.io/foo/foo-bundle[0.1.0]
type channelsDataMap map[string]bundlesDataMap  // channel-name --> bundlesDataMap
type bundlesDataMap map[string][]semver.Version // bundle-name --> []bundle-versions
// cname --> bname --> [v0, v1, v2, ...]

type tokenizedBundleEntry struct {
	path    string
	version semver.Version
}

// splits an image line into its identifying image path and version, e.g.
// quay.io/foo/foo-bundle:0.2.1 ==> {
//	path:   quay.io/foo/foo-bundle,
//  version: 0.2.1
// }
// specifically decomposes to retain the bundle origin to differentiate in
// case there are multiple operators of the same name but different origins,
// e.g.:
// "quay.io/foo/foo-bundle"
// "docker.io/foo/foo-bundle"
func newTokenizedBundleEntry(s string) (*tokenizedBundleEntry, error) {
	splits := strings.Split(s, ":")
	path := splits[0]
	verstring := splits[1]
	ver, err := semver.ParseTolerant(verstring)
	if err != nil {
		return nil, err
	}

	return &tokenizedBundleEntry{
		path:    path,
		version: ver,
	}, nil
}

func addBundlesToChannel(bundles []semverVeneerBundleEntry) (*bundlesDataMap, error) {
	bdm := make(bundlesDataMap)
	for _, b := range bundles {
		// fmt.Printf("  <--> adding %s bundle: %s\n", b.Image)
		e, err := newTokenizedBundleEntry(b.Image)
		if err != nil {
			return nil, err
		}
		bdm[e.path] = append(bdm[e.path], e.version)
	}
	return &bdm, nil
}

func (sv *semverVeneer) addBundlesToStandardChannels() (*channelsDataMap, error) {
	isvd := channelsDataMap{}

	bdm, err := addBundlesToChannel(sv.Candidate.Bundles)
	if err != nil {
		return nil, err
	}
	isvd[CandidateChannelName] = *bdm

	bdm, err = addBundlesToChannel(sv.Fast.Bundles)
	if err != nil {
		return nil, err
	}
	isvd[FastChannelName] = *bdm

	bdm, err = addBundlesToChannel(sv.Stable.Bundles)
	if err != nil {
		return nil, err
	}
	isvd[StableChannelName] = *bdm

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
	fmt.Printf("Semver-Veneer parsed:\n")
	sv.write()

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

	cdm, err := sv.addBundlesToStandardChannels()
	if err != nil {
		log.Fatalf("semver-render: unable to post-process bundle info: %v", err)
	}
	// sv.write()
	// fmt.Printf("<--> <-->\n")
	// isvd.write()
	channels, bundles := sv.decomposeChannelsAndBundles(cdm)
	out.Channels = channels

	// render the nascent bundles and accumulate them
	for _, b := range bundles {
		r := action.Render{
			AllowedRefMask: action.RefBundleImage,
			Refs:           []string{b.Image},
		}
		contributor, err := r.Run(ctx)
		if err != nil {
			return nil, err
		}
		out.Bundles = append(out.Bundles, contributor.Bundles...)
	}

	return &out, nil
}

func (sv *semverVeneer) addChannels(data map[string][]semver.Version, bpath string) []declcfg.Channel {
	channels := []declcfg.Channel{}
	for cvername, versions := range data {
		c := newChannel(bpath, cvername)
		// add all (unlinked) entries to channel
		bpkg := path.Base(bpath)
		for _, ver := range versions {
			bname := bpkg + "." + ver.String()
			c.Entries = append(c.Entries, declcfg.ChannelEntry{Name: bname})
		}

		// link up the edges according to config
		if sv.AvoidSkipPatch {
			for i := 1; i < len(c.Entries); i++ {
				c.Entries[i] = declcfg.ChannelEntry{
					Name:     c.Entries[i].Name,
					Replaces: c.Entries[i-1].Name,
				}
			}
		} else {
			maxIndex := len(c.Entries) - 1
			curSkips := sets.NewString()
			for i := 0; i < maxIndex; i++ {
				curSkips = curSkips.Insert(c.Entries[i].Name)
			}
			c.Entries[maxIndex] = declcfg.ChannelEntry{
				Name:  c.Entries[maxIndex].Name,
				Skips: curSkips.List(),
			}
		}

		channels = append(channels, *c)
	}
	return channels
}

// generates a channel for each channel in the map, containing:
//   edges for each bundle with a predicted bundle name composed of
//   the bundle package + bundle version, with a "." delimiter, e.g.:
//   foo with version 0.1.0 ==> foo.0.1.0
// generates a bundle for each predicted bundle name
// for now, the name composition is fixed, but should be expanded to utilize user-supplied templates
func (sv *semverVeneer) decomposeChannelsAndBundles(channels *channelsDataMap) ([]declcfg.Channel, []declcfg.Bundle) {
	outChannels := []declcfg.Channel{}
	outBundles := []declcfg.Bundle{}

	// fmt.Printf("<isvd.Schema> %s\n", data.Schema)
	// fmt.Printf("<isvd.Channels> %s\n", channels)
	for cname, bmap := range *channels {
		for bpath, bver := range bmap {
			sort.Slice(bver, func(i, j int) bool {
				return bver[i].LT(bver[j])
			})

			majors := make(map[string][]semver.Version, len(bver))
			minors := make(map[string][]semver.Version, len(bver))

			for _, ver := range bver {
				if sv.GenerateMajorChannels {
					testChannelName := cname + "-" + getMajorVersion(ver).String()
					if _, ok := majors[testChannelName]; !ok {
						majors[testChannelName] = []semver.Version{ver}
						// fmt.Printf("Adding new major channel: %s\n", testChannelName)
						// fmt.Printf("Adding new major channel contributor: %s to channel: %s\n", ver.String(), testChannelName)
					} else {
						majors[testChannelName] = append(majors[testChannelName], ver)
						// fmt.Printf("Adding new major channel contributor: %s to channel: %s\n", ver.String(), testChannelName)
					}
				}
				if sv.GenerateMinorChannels {
					testChannelName := cname + "-" + getMinorVersion(ver).String()
					if _, ok := minors[testChannelName]; !ok {
						minors[testChannelName] = []semver.Version{ver}
						// fmt.Printf("Adding new minor channel: %s\n", testChannelName)
						// fmt.Printf("Adding new minor channel contributor: %s to channel: %s\n", ver.String(), testChannelName)
					} else {
						minors[testChannelName] = append(minors[testChannelName], ver)
						// fmt.Printf("Adding new minor channel contributor: %s to channel: %s\n", ver.String(), testChannelName)
					}
				}
				bpkg := path.Base(bpath)
				bimg := bpath + ":" + ver.String()
				bname := bpkg + "." + ver.String()
				outBundles = append(outBundles, *newBundle(bpkg, bname, bimg))
			}

			outChannels = append(outChannels, sv.addChannels(majors, bpath)...)
			outChannels = append(outChannels, sv.addChannels(minors, bpath)...)
		}
	}

	return outChannels, outBundles
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

func getMinorVersion(v semver.Version) semver.Version {
	return semver.Version{
		Major: v.Major,
		Minor: v.Minor,
	}
}

func getMajorVersion(v semver.Version) semver.Version {
	return semver.Version{
		Major: v.Major,
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

func (channels *channelsDataMap) write() {
	for cname, bmap := range *channels {
		fmt.Printf("%s:\n", cname)
		fmt.Printf("  bundles:\n")
		for bname, bvers := range bmap {
			for _, elem := range bvers {
				fmt.Printf("  - image: %s:%s\n", bname, elem.String())
			}
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
