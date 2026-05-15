package cache

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

type packageIndex map[string]cPkg

func (pkgs packageIndex) ListPackages(_ context.Context) ([]string, error) {
	// nolint:prealloc
	var packages []string
	for pkgName := range pkgs {
		packages = append(packages, pkgName)
	}
	return packages, nil
}

func (pkgs packageIndex) GetPackage(_ context.Context, name string) (*registry.PackageManifest, error) {
	pkg, ok := pkgs[name]
	if !ok {
		return nil, fmt.Errorf("package %q not found", name)
	}

	// nolint:prealloc
	var channels []registry.PackageChannel
	for _, ch := range pkg.Channels {
		var deprecation *registry.Deprecation
		if ch.Deprecation != nil {
			deprecation = &registry.Deprecation{Message: *ch.Deprecation}
		}
		channels = append(channels, registry.PackageChannel{
			Name:           ch.Name,
			CurrentCSVName: ch.Head,
			Deprecation:    deprecation,
		})
	}
	sort.Slice(channels, func(i, j int) bool { return strings.Compare(channels[i].Name, channels[j].Name) < 0 })
	registryPackage := &registry.PackageManifest{
		PackageName:        pkg.Name,
		Channels:           channels,
		DefaultChannelName: pkg.DefaultChannel,
	}
	if pkg.Deprecation != nil {
		registryPackage.Deprecation = &registry.Deprecation{Message: *pkg.Deprecation}
	}
	return registryPackage, nil
}

func (pkgs packageIndex) GetChannelEntriesThatReplace(_ context.Context, name string) ([]*registry.ChannelEntry, error) {
	entries := make([]*registry.ChannelEntry, 0, len(pkgs))

	for _, pkg := range pkgs {
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				entries = append(entries, channelEntriesThatReplace(b, name)...)
			}
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no channel entries found that replace %s", name)
	}
	return entries, nil
}

type getBundleFunc func(context.Context, bundleKey) (*api.Bundle, error)

func (pkgs packageIndex) GetBundleForChannel(ctx context.Context, getBundle getBundleFunc, pkgName string, channelName string) (*api.Bundle, error) {
	pkg, ok := pkgs[pkgName]
	if !ok {
		return nil, fmt.Errorf("package %q not found", pkgName)
	}
	ch, ok := pkg.Channels[channelName]
	if !ok {
		return nil, fmt.Errorf("package %q, channel %q not found", pkgName, channelName)
	}
	return getBundle(ctx, bundleKey{pkg.Name, ch.Name, ch.Head})
}

func (pkgs packageIndex) GetBundleThatReplaces(ctx context.Context, getBundle getBundleFunc, name, pkgName, channelName string) (*api.Bundle, error) {
	pkg, ok := pkgs[pkgName]
	if !ok {
		return nil, fmt.Errorf("package %s not found", pkgName)
	}
	ch, ok := pkg.Channels[channelName]
	if !ok {
		return nil, fmt.Errorf("package %q, channel %q not found", pkgName, channelName)
	}

	// NOTE: iterating over a map is non-deterministic in Go, so if multiple bundles replace this one,
	//       the bundle returned by this function is also non-deterministic.
	for _, b := range ch.Bundles {
		if bundleReplaces(b, name) {
			return getBundle(ctx, bundleKey{pkg.Name, ch.Name, b.Name})
		}
	}
	return nil, fmt.Errorf("no entry found for package %q, channel %q", pkgName, channelName)
}

func (pkgs packageIndex) GetChannelEntriesThatProvide(ctx context.Context, getBundle getBundleFunc, group, version, kind string) ([]*registry.ChannelEntry, error) {
	var entries []*registry.ChannelEntry

	for _, pkg := range pkgs {
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				provides, err := doesBundleProvide(ctx, getBundle, b.Package, b.Channel, b.Name, group, version, kind)
				if err != nil {
					return nil, err
				}
				if provides {
					// TODO(joelanford): This may return invalid entries (i.e. where bundle
					//   `Replaces` isn't actually in channel `ChannelName`). Is that a bug?
					//   Don't worry about this. Not used anymore.

					entries = append(entries, pkgs.channelEntriesForBundle(b, true)...)
				}
			}
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no channel entries found that provide group:%q version:%q kind:%q", group, version, kind)
	}
	return entries, nil
}

// TODO(joelanford): Need to review the expected functionality of this function. This currently
//
//	only returns channel heads that provide the GVK (rather than searching down the graph if
//	parent bundles don't provide the API).
func (pkgs packageIndex) GetLatestChannelEntriesThatProvide(ctx context.Context, getBundle getBundleFunc, group, version, kind string) ([]*registry.ChannelEntry, error) {
	var entries []*registry.ChannelEntry

	for _, pkg := range pkgs {
		for _, ch := range pkg.Channels {
			b := ch.Bundles[ch.Head]
			provides, err := doesBundleProvide(ctx, getBundle, b.Package, b.Channel, b.Name, group, version, kind)
			if err != nil {
				return nil, err
			}
			if provides {
				entries = append(entries, pkgs.channelEntriesForBundle(b, false)...)
			}
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no channel entries found that provide group:%q version:%q kind:%q", group, version, kind)
	}
	return entries, nil
}

func (pkgs packageIndex) GetBundleThatProvides(ctx context.Context, c Cache, group, version, kind string) (*api.Bundle, error) {
	latestEntries, err := c.GetLatestChannelEntriesThatProvide(ctx, group, version, kind)
	if err != nil {
		return nil, err
	}

	// It's possible for multiple packages to provide an API, but this function is forced to choose one.
	// To do that deterministically, we'll pick the the bundle based on a lexicographical sort of its
	// package name.
	sort.Slice(latestEntries, func(i, j int) bool {
		return latestEntries[i].PackageName < latestEntries[j].PackageName
	})

	for _, entry := range latestEntries {
		pkg, ok := pkgs[entry.PackageName]
		if !ok {
			// This should never happen because the latest entries were
			// collected based on iterating over the packages in q.packageIndex.
			continue
		}
		if entry.ChannelName == pkg.DefaultChannel {
			return c.GetBundle(ctx, entry.PackageName, entry.ChannelName, entry.BundleName)
		}
	}
	return nil, fmt.Errorf("no entry found that provides group:%q version:%q kind:%q", group, version, kind)
}

type cPkg struct {
	Name           string        `json:"name"`
	Description    string        `json:"description"`
	Icon           *declcfg.Icon `json:"icon"`
	DefaultChannel string        `json:"defaultChannel"`
	Channels       map[string]cChannel
	Deprecation    *string `json:"deprecation,omitempty"`
}

type cChannel struct {
	Name        string
	Head        string
	Bundles     map[string]cBundle
	Deprecation *string `json:"deprecation,omitempty"`
}

type cBundle struct {
	Package  string   `json:"package"`
	Channel  string   `json:"channel"`
	Name     string   `json:"name"`
	Replaces string   `json:"replaces"`
	Skips    []string `json:"skips"`
}

func packagesFromDeclcfg(cfg declcfg.DeclarativeConfig) (map[string]cPkg, error) {
	pkgs := map[string]cPkg{}

	// First pass: create packages
	for _, p := range cfg.Packages {
		pkgs[p.Name] = cPkg{
			Name:           p.Name,
			Icon:           p.Icon,
			Description:    p.Description,
			DefaultChannel: p.DefaultChannel,
			Channels:       map[string]cChannel{},
			Deprecation:    nil,
		}
	}

	// Second pass: create channels and add bundles
	for _, ch := range cfg.Channels {
		pkg, ok := pkgs[ch.Package]
		if !ok {
			return nil, fmt.Errorf("channel %q references unknown package %q", ch.Name, ch.Package)
		}

		// Find the head of this channel
		head, err := findChannelHead(ch.Entries)
		if err != nil {
			return nil, fmt.Errorf("find head for channel %q in package %q: %v", ch.Name, ch.Package, err)
		}

		newCh := cChannel{
			Name:        ch.Name,
			Head:        head,
			Bundles:     map[string]cBundle{},
			Deprecation: nil,
		}

		for _, entry := range ch.Entries {
			newB := cBundle{
				Package:  ch.Package,
				Channel:  ch.Name,
				Name:     entry.Name,
				Replaces: entry.Replaces,
				Skips:    entry.Skips,
			}
			newCh.Bundles[entry.Name] = newB
		}

		pkg.Channels[ch.Name] = newCh
		pkgs[ch.Package] = pkg
	}

	// Third pass: apply deprecations
	for _, d := range cfg.Deprecations {
		pkg, ok := pkgs[d.Package]
		if !ok {
			return nil, fmt.Errorf("deprecation references unknown package %q", d.Package)
		}

		for _, entry := range d.Entries {
			switch entry.Reference.Schema {
			case declcfg.SchemaPackage:
				msg := entry.Message
				pkg.Deprecation = &msg
			case declcfg.SchemaChannel:
				ch, ok := pkg.Channels[entry.Reference.Name]
				if !ok {
					return nil, fmt.Errorf("deprecation references unknown channel %q in package %q", entry.Reference.Name, d.Package)
				}
				msg := entry.Message
				ch.Deprecation = &msg
				pkg.Channels[entry.Reference.Name] = ch
			}
		}
		pkgs[d.Package] = pkg
	}

	return pkgs, nil
}

// findChannelHead finds the head bundle of a channel by analyzing the replaces chain.
// The head is the bundle that is not replaced by any other bundle in the channel.
func findChannelHead(entries []declcfg.ChannelEntry) (string, error) {
	if len(entries) == 0 {
		return "", fmt.Errorf("channel has no entries")
	}

	// Build a map of bundles that are replaced
	replaced := make(map[string]bool)
	for _, entry := range entries {
		if entry.Replaces != "" {
			replaced[entry.Replaces] = true
		}
		for _, skip := range entry.Skips {
			replaced[skip] = true
		}
	}

	// Find bundles that are not replaced by anything
	var heads []string
	for _, entry := range entries {
		if !replaced[entry.Name] {
			heads = append(heads, entry.Name)
		}
	}

	if len(heads) == 0 {
		return "", fmt.Errorf("channel has circular replaces chain, no head found")
	}
	if len(heads) > 1 {
		return "", fmt.Errorf("channel has multiple heads: %v", heads)
	}

	return heads[0], nil
}

func bundleReplaces(b cBundle, name string) bool {
	if b.Replaces == name {
		return true
	}
	for _, s := range b.Skips {
		if s == name {
			return true
		}
	}
	return false
}

func channelEntriesThatReplace(b cBundle, name string) []*registry.ChannelEntry {
	var entries []*registry.ChannelEntry
	if b.Replaces == name {
		entries = append(entries, &registry.ChannelEntry{
			PackageName: b.Package,
			ChannelName: b.Channel,
			BundleName:  b.Name,
			Replaces:    b.Replaces,
		})
	}
	for _, s := range b.Skips {
		if s == name && s != b.Replaces {
			entries = append(entries, &registry.ChannelEntry{
				PackageName: b.Package,
				ChannelName: b.Channel,
				BundleName:  b.Name,
				Replaces:    b.Replaces,
			})
		}
	}
	return entries
}

func (pkgs packageIndex) channelEntriesForBundle(b cBundle, ignoreChannel bool) []*registry.ChannelEntry {
	entries := []*registry.ChannelEntry{{
		PackageName: b.Package,
		ChannelName: b.Channel,
		BundleName:  b.Name,
		Replaces:    b.Replaces,
	}}
	for _, s := range b.Skips {
		// Ignore skips that duplicate b.Replaces. Also, only add it if its
		// in the same channel as b (or we're ignoring channel presence).
		if _, inChannel := pkgs[b.Package].Channels[b.Channel].Bundles[s]; s != b.Replaces && (ignoreChannel || inChannel) {
			entries = append(entries, &registry.ChannelEntry{
				PackageName: b.Package,
				ChannelName: b.Channel,
				BundleName:  b.Name,
				Replaces:    s,
			})
		}
	}
	return entries
}
