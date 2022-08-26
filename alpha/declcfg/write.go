package declcfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/operator-framework/operator-registry/alpha/property"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"
)

// writes out the channel edges of the declarative config graph in a mermaid format capable of being pasted into
// mermaid renderers like github, mermaid.live, etc.
// output is sorted lexicographically by package name, and then by channel name
// if provided, minEdgeName will be used as the lower bound for edges in the output graph
//
// NB:  Output has wrapper comments stating the skipRange edge caveat in HTML comment format, which cannot be parsed by mermaid renderers.
//
//	This is deliberate, and intended as an explicit acknowledgement of the limitations, instead of requiring the user to notice the missing edges upon inspection.
//
// Example output:
// <!-- PLEASE NOTE:  skipRange edges are not currently displayed -->
// graph LR
//
//	  %% package "neuvector-certified-operator-rhmp"
//	  subgraph "neuvector-certified-operator-rhmp"
//	     %% channel "beta"
//	     subgraph neuvector-certified-operator-rhmp-beta["beta"]
//		      neuvector-certified-operator-rhmp-beta-neuvector-operator.v1.2.8["neuvector-operator.v1.2.8"]
//		      neuvector-certified-operator-rhmp-beta-neuvector-operator.v1.2.9["neuvector-operator.v1.2.9"]
//		      neuvector-certified-operator-rhmp-beta-neuvector-operator.v1.3.0["neuvector-operator.v1.3.0"]
//		      neuvector-certified-operator-rhmp-beta-neuvector-operator.v1.3.0["neuvector-operator.v1.3.0"]-- replaces --> neuvector-certified-operator-rhmp-beta-neuvector-operator.v1.2.8["neuvector-operator.v1.2.8"]
//		      neuvector-certified-operator-rhmp-beta-neuvector-operator.v1.3.0["neuvector-operator.v1.3.0"]-- skips --> neuvector-certified-operator-rhmp-beta-neuvector-operator.v1.2.9["neuvector-operator.v1.2.9"]
//	    end
//	  end
//
// end
// <!-- PLEASE NOTE:  skipRange edges are not currently displayed -->
func WriteMermaidChannels(cfg DeclarativeConfig, out io.Writer, minEdgeName string) error {
	pkgs := map[string]*strings.Builder{}

	sort.Slice(cfg.Channels, func(i, j int) bool {
		return cfg.Channels[i].Name < cfg.Channels[j].Name
	})

	versionMap, err := getBundleVersions(&cfg)
	if err != nil {
		return err
	}

	if _, ok := versionMap[minEdgeName]; !ok {
		if minEdgeName != "" {
			return fmt.Errorf("unknown minimum edge name: %q", minEdgeName)
		}
	}

	for _, c := range cfg.Channels {
		filteredChannel := filterChannel(&c, versionMap, minEdgeName)
		if filteredChannel != nil {
			pkgBuilder, ok := pkgs[c.Package]
			if !ok {
				pkgBuilder = &strings.Builder{}
				pkgs[c.Package] = pkgBuilder
			}

			channelID := fmt.Sprintf("%s-%s", filteredChannel.Package, filteredChannel.Name)
			pkgBuilder.WriteString(fmt.Sprintf("    %%%% channel %q\n", filteredChannel.Name))
			pkgBuilder.WriteString(fmt.Sprintf("    subgraph %s[%q]\n", channelID, filteredChannel.Name))

			for _, ce := range filteredChannel.Entries {
				if versionMap[ce.Name].GE(versionMap[minEdgeName]) {
					entryId := fmt.Sprintf("%s-%s", channelID, ce.Name)
					pkgBuilder.WriteString(fmt.Sprintf("      %s[%q]\n", entryId, ce.Name))

					// no support for SkipRange yet
					if len(ce.Replaces) > 0 {
						replacesId := fmt.Sprintf("%s-%s", channelID, ce.Replaces)
						pkgBuilder.WriteString(fmt.Sprintf("      %s[%q]-- %s --> %s[%q]\n", entryId, ce.Name, "replaces", replacesId, ce.Replaces))
					}
					if len(ce.Skips) > 0 {
						for _, s := range ce.Skips {
							skipsId := fmt.Sprintf("%s-%s", channelID, s)
							pkgBuilder.WriteString(fmt.Sprintf("      %s[%q]-- %s --> %s[%q]\n", entryId, ce.Name, "skips", skipsId, s))
						}
					}
				}
			}
			pkgBuilder.WriteString("    end\n")
		}
	}

	out.Write([]byte("<!-- PLEASE NOTE:  skipRange edges are not currently displayed -->\n"))
	out.Write([]byte("graph LR\n"))
	pkgNames := []string{}
	for pname, _ := range pkgs {
		pkgNames = append(pkgNames, pname)
	}
	sort.Slice(pkgNames, func(i, j int) bool {
		return pkgNames[i] < pkgNames[j]
	})
	for _, pkgName := range pkgNames {
		out.Write([]byte(fmt.Sprintf("  %%%% package %q\n", pkgName)))
		out.Write([]byte(fmt.Sprintf("  subgraph %q\n", pkgName)))
		out.Write([]byte(pkgs[pkgName].String()))
		out.Write([]byte("  end\n"))
	}
	out.Write([]byte("<!-- PLEASE NOTE:  skipRange edges are not currently displayed -->\n"))

	return nil
}

// filters the channel edges to include only those which are greater-than-or-equal to the edge named by startVersion
// returns a nil channel if all edges are filtered out
func filterChannel(c *Channel, versionMap map[string]semver.Version, minEdgeName string) *Channel {
	// short-circuit if no specified startVersion
	if minEdgeName == "" {
		return c
	}
	// convert the edge name to the version so we don't have to duplicate the lookup
	minVersion := versionMap[minEdgeName]

	out := &Channel{Name: c.Name, Package: c.Package, Properties: c.Properties, Entries: []ChannelEntry{}}
	for _, ce := range c.Entries {
		filteredCe := ChannelEntry{Name: ce.Name}
		// short-circuit to take the edge name (but no references to earlier versions)
		if ce.Name == minEdgeName {
			out.Entries = append(out.Entries, filteredCe)
			continue
		}
		// if len(ce.SkipRange) > 0 {
		// }
		if len(ce.Replaces) > 0 {
			if versionMap[ce.Replaces].GTE(minVersion) {
				filteredCe.Replaces = ce.Replaces
			}
		}
		if len(ce.Skips) > 0 {
			filteredSkips := []string{}
			for _, s := range ce.Skips {
				if versionMap[s].GTE(minVersion) {
					filteredSkips = append(filteredSkips, s)
				}
			}
			if len(filteredSkips) > 0 {
				filteredCe.Skips = filteredSkips
			}
		}
		if len(filteredCe.Replaces) > 0 || len(filteredCe.Skips) > 0 {
			out.Entries = append(out.Entries, filteredCe)
		}
	}

	if len(out.Entries) > 0 {
		return out
	} else {
		return nil
	}
}

func parseVersionProperty(b *Bundle) (*semver.Version, error) {
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

	return &v, nil
}

func getBundleVersions(cfg *DeclarativeConfig) (map[string]semver.Version, error) {
	entries := make(map[string]semver.Version)
	for index := range cfg.Bundles {
		if _, ok := entries[cfg.Bundles[index].Name]; !ok {
			ver, err := parseVersionProperty(&cfg.Bundles[index])
			if err != nil {
				return entries, err
			}
			entries[cfg.Bundles[index].Name] = *ver
		}
	}

	return entries, nil
}

func WriteJSON(cfg DeclarativeConfig, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)
	return writeToEncoder(cfg, enc)
}

func WriteYAML(cfg DeclarativeConfig, w io.Writer) error {
	enc := newYAMLEncoder(w)
	enc.SetEscapeHTML(false)
	return writeToEncoder(cfg, enc)
}

type yamlEncoder struct {
	w          io.Writer
	escapeHTML bool
}

func newYAMLEncoder(w io.Writer) *yamlEncoder {
	return &yamlEncoder{w, true}
}

func (e *yamlEncoder) SetEscapeHTML(on bool) {
	e.escapeHTML = on
}

func (e *yamlEncoder) Encode(v interface{}) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(e.escapeHTML)
	if err := enc.Encode(v); err != nil {
		return err
	}
	yamlData, err := yaml.JSONToYAML(buf.Bytes())
	if err != nil {
		return err
	}
	yamlData = append([]byte("---\n"), yamlData...)
	_, err = e.w.Write(yamlData)
	return err
}

type encoder interface {
	Encode(interface{}) error
}

func writeToEncoder(cfg DeclarativeConfig, enc encoder) error {
	pkgNames := sets.NewString()

	packagesByName := map[string][]Package{}
	for _, p := range cfg.Packages {
		pkgName := p.Name
		pkgNames.Insert(pkgName)
		packagesByName[pkgName] = append(packagesByName[pkgName], p)
	}
	channelsByPackage := map[string][]Channel{}
	for _, c := range cfg.Channels {
		pkgName := c.Package
		pkgNames.Insert(pkgName)
		channelsByPackage[pkgName] = append(channelsByPackage[pkgName], c)
	}
	bundlesByPackage := map[string][]Bundle{}
	for _, b := range cfg.Bundles {
		pkgName := b.Package
		pkgNames.Insert(pkgName)
		bundlesByPackage[pkgName] = append(bundlesByPackage[pkgName], b)
	}
	othersByPackage := map[string][]Meta{}
	for _, o := range cfg.Others {
		pkgName := o.Package
		pkgNames.Insert(pkgName)
		othersByPackage[pkgName] = append(othersByPackage[pkgName], o)
	}

	for _, pName := range pkgNames.List() {
		if len(pName) == 0 {
			continue
		}
		pkgs := packagesByName[pName]
		for _, p := range pkgs {
			if err := enc.Encode(p); err != nil {
				return err
			}
		}

		channels := channelsByPackage[pName]
		sort.Slice(channels, func(i, j int) bool {
			return channels[i].Name < channels[j].Name
		})
		for _, c := range channels {
			if err := enc.Encode(c); err != nil {
				return err
			}
		}

		bundles := bundlesByPackage[pName]
		sort.Slice(bundles, func(i, j int) bool {
			return bundles[i].Name < bundles[j].Name
		})
		for _, b := range bundles {
			if err := enc.Encode(b); err != nil {
				return err
			}
		}

		others := othersByPackage[pName]
		sort.SliceStable(others, func(i, j int) bool {
			return others[i].Schema < others[j].Schema
		})
		for _, o := range others {
			if err := enc.Encode(o); err != nil {
				return err
			}
		}
	}

	for _, o := range othersByPackage[""] {
		if err := enc.Encode(o); err != nil {
			return err
		}
	}
	return nil
}
