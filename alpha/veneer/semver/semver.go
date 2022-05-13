package semver

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"sort"

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

// isvd.Channels["stable"] --> quay.io/foo/foo-bundle[0.1.0]
type channelsDataMap map[string]bundlesDataMap  // channel-name --> bundlesDataMap
type bundlesDataMap map[string][]semver.Version // bundle-name --> []bundle-versions
// cname --> bname --> [v0, v1, v2, ...]

type decomposedBundleEntry struct {
	img string
	ver semver.Version
}

func findit(s string, bundles []declcfg.Bundle) int {

	for index, _ := range bundles {
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
		p := declcfg.Package{Name: props.Packages[0].PackageName}
		cfg.Packages = append(cfg.Packages, p)
	}
	return &decomposedBundleEntry{
		img: b.Name,
		ver: v,
	}, nil
}

func addBundlesToChannel(bundles []semverVeneerBundleEntry, cfg *declcfg.DeclarativeConfig) (*bundlesDataMap, error) {
	bdm := make(bundlesDataMap)
	for _, b := range bundles {
		fmt.Printf("  <--> adding bundle %q\n", b.Image)

		e, err := decomposeBundle(&b, cfg)
		if err != nil {
			return nil, err
		}
		bdm[e.img] = append(bdm[e.img], e.ver)
	}
	return &bdm, nil
}

func (sv *semverVeneer) addBundlesToStandardChannels(cfg *declcfg.DeclarativeConfig) (*channelsDataMap, error) {
	isvd := channelsDataMap{}

	bdm, err := addBundlesToChannel(sv.Candidate.Bundles, cfg)
	if err != nil {
		return nil, err
	}
	isvd[CandidateChannelName] = *bdm

	bdm, err = addBundlesToChannel(sv.Fast.Bundles, cfg)
	if err != nil {
		return nil, err
	}
	isvd[FastChannelName] = *bdm

	bdm, err = addBundlesToChannel(sv.Stable.Bundles, cfg)
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
	fmt.Printf("Semver-Veneer parsed\n")
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

	cdm, err := sv.addBundlesToStandardChannels(&out)
	if err != nil {
		log.Fatalf("semver-render: unable to post-process bundle info: %v", err)
	}
	// sv.write()
	// fmt.Printf("<--> <-->\n")
	// isvd.write()
	channels := sv.decomposeChannels(cdm)
	out.Channels = channels

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
func (sv *semverVeneer) decomposeChannels(channels *channelsDataMap) []declcfg.Channel {
	outChannels := []declcfg.Channel{}

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
			}

			outChannels = append(outChannels, sv.addChannels(majors, bpath)...)
			outChannels = append(outChannels, sv.addChannels(minors, bpath)...)
		}
	}

	return outChannels
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
