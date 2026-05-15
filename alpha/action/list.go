package action

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/pkg/image"
)

type ListPackages struct {
	IndexReference string
	Registry       image.Registry
}

func (l *ListPackages) Run(ctx context.Context) (*ListPackagesResult, error) {
	cfg, err := indexRefToDeclcfg(ctx, l.IndexReference, l.Registry)
	if err != nil {
		return nil, err
	}

	pkgs := cfg.Packages
	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i].Name < pkgs[j].Name
	})
	return &ListPackagesResult{Packages: pkgs, Channels: cfg.Channels, Bundles: cfg.Bundles}, nil
}

type ListPackagesResult struct {
	Packages []declcfg.Package
	Channels []declcfg.Channel
	Bundles  []declcfg.Bundle
}

func (r *ListPackagesResult) WriteColumns(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "NAME\tDISPLAY NAME\tDEFAULT CHANNEL"); err != nil {
		return err
	}
	for _, pkg := range r.Packages {
		displayName := getDisplayName(pkg, r.Channels, r.Bundles)
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", pkg.Name, displayName, pkg.DefaultChannel); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func getDisplayName(pkg declcfg.Package, channels []declcfg.Channel, bundles []declcfg.Bundle) string {
	if pkg.DefaultChannel == "" {
		return ""
	}

	// Find the default channel
	var defaultCh *declcfg.Channel
	for i, ch := range channels {
		if ch.Package == pkg.Name && ch.Name == pkg.DefaultChannel {
			defaultCh = &channels[i]
			break
		}
	}
	if defaultCh == nil {
		return ""
	}

	// Find the head bundle
	head, err := findChannelHead(defaultCh.Entries)
	if err != nil {
		return ""
	}

	// Find the bundle
	var headBundle *declcfg.Bundle
	for i, b := range bundles {
		if b.Package == pkg.Name && b.Name == head {
			headBundle = &bundles[i]
			break
		}
	}
	if headBundle == nil || headBundle.CsvJSON == "" {
		return ""
	}

	csv := v1alpha1.ClusterServiceVersion{}
	if err := json.Unmarshal([]byte(headBundle.CsvJSON), &csv); err != nil {
		return ""
	}
	return csv.Spec.DisplayName
}

// findChannelHead finds the head bundle of a channel by analyzing the replaces chain.
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

type ListChannels struct {
	IndexReference string
	PackageName    string
	Registry       image.Registry
}

func (l *ListChannels) Run(ctx context.Context) (*ListChannelsResult, error) {
	cfg, err := indexRefToDeclcfg(ctx, l.IndexReference, l.Registry)
	if err != nil {
		return nil, err
	}

	channels := filterChannelsByPackage(cfg.Channels, l.PackageName)
	if l.PackageName != "" && len(channels) == 0 {
		return nil, fmt.Errorf("package %q not found", l.PackageName)
	}

	sort.Slice(channels, func(i, j int) bool {
		if channels[i].Package != channels[j].Package {
			return channels[i].Package < channels[j].Package
		}
		return channels[i].Name < channels[j].Name
	})
	return &ListChannelsResult{Channels: channels}, nil
}

type ListChannelsResult struct {
	Channels []declcfg.Channel
}

func (r *ListChannelsResult) WriteColumns(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "PACKAGE\tCHANNEL\tHEAD"); err != nil {
		return err
	}
	for _, ch := range r.Channels {
		headStr := ""
		head, err := findChannelHead(ch.Entries)
		if err != nil {
			headStr = fmt.Sprintf("ERROR: %s", err)
		} else {
			headStr = head
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", ch.Package, ch.Name, headStr); err != nil {
			return err
		}
	}
	return tw.Flush()
}

type ListBundles struct {
	IndexReference string
	PackageName    string
	Registry       image.Registry
}

func (l *ListBundles) Run(ctx context.Context) (*ListBundlesResult, error) {
	cfg, err := indexRefToDeclcfg(ctx, l.IndexReference, l.Registry)
	if err != nil {
		return nil, err
	}

	bundles := filterBundlesByPackage(cfg.Bundles, cfg.Channels, l.PackageName)
	channels := filterChannelsByPackage(cfg.Channels, l.PackageName)

	if l.PackageName != "" && len(bundles) == 0 {
		return nil, fmt.Errorf("package %q not found", l.PackageName)
	}

	return &ListBundlesResult{Bundles: bundles, Channels: channels}, nil
}

type ListBundlesResult struct {
	Bundles  []declcfg.Bundle
	Channels []declcfg.Channel
}

func (r *ListBundlesResult) WriteColumns(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "PACKAGE\tCHANNEL\tBUNDLE\tREPLACES\tSKIPS\tSKIP RANGE\tIMAGE"); err != nil {
		return err
	}

	// Build a map of bundle -> channels containing it
	type bundleChannelEntry struct {
		channel   string
		replaces  string
		skips     []string
		skipRange string
	}
	bundleToChannels := make(map[string][]bundleChannelEntry)
	for _, ch := range r.Channels {
		for _, entry := range ch.Entries {
			key := ch.Package + "/" + entry.Name
			bundleToChannels[key] = append(bundleToChannels[key], bundleChannelEntry{
				channel:   ch.Name,
				replaces:  entry.Replaces,
				skips:     entry.Skips,
				skipRange: entry.SkipRange,
			})
		}
	}

	// Build list of bundle instances (one per channel)
	type bundleInstance struct {
		pkg       string
		channel   string
		name      string
		replaces  string
		skips     []string
		skipRange string
		image     string
	}
	var instances []bundleInstance

	for _, b := range r.Bundles {
		key := b.Package + "/" + b.Name
		channels := bundleToChannels[key]

		// Create one entry per channel
		for _, ch := range channels {
			instances = append(instances, bundleInstance{
				pkg:       b.Package,
				channel:   ch.channel,
				name:      b.Name,
				replaces:  ch.replaces,
				skips:     ch.skips,
				skipRange: ch.skipRange,
				image:     b.Image,
			})
		}
	}

	// Sort instances
	sort.Slice(instances, func(i, j int) bool {
		if instances[i].pkg != instances[j].pkg {
			return instances[i].pkg < instances[j].pkg
		}
		if instances[i].channel != instances[j].channel {
			return instances[i].channel < instances[j].channel
		}
		return instances[i].name < instances[j].name
	})

	// Write instances
	for _, inst := range instances {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", inst.pkg, inst.channel, inst.name, inst.replaces, strings.Join(inst.skips, ","), inst.skipRange, inst.image); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func indexRefToDeclcfg(ctx context.Context, ref string, reg image.Registry) (*declcfg.DeclarativeConfig, error) {
	render := Render{
		Refs:           []string{ref},
		AllowedRefMask: RefDCImage | RefDCDir,
		Registry:       reg,
	}
	cfg, err := render.Run(ctx)
	if err != nil {
		if errors.Is(err, ErrNotAllowed) {
			return nil, fmt.Errorf("cannot list non-index %q", ref)
		}
		return nil, err
	}

	return cfg, nil
}

func filterChannelsByPackage(channels []declcfg.Channel, packageName string) []declcfg.Channel {
	if packageName == "" {
		return channels
	}

	var result []declcfg.Channel
	for _, ch := range channels {
		if ch.Package == packageName {
			result = append(result, ch)
		}
	}
	return result
}

func filterBundlesByPackage(bundles []declcfg.Bundle, _ []declcfg.Channel, packageName string) []declcfg.Bundle {
	if packageName == "" {
		return bundles
	}

	var result []declcfg.Bundle
	for _, b := range bundles {
		if b.Package == packageName {
			result = append(result, b)
		}
	}
	return result
}
