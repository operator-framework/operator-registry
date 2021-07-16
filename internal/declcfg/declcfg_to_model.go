package declcfg

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/operator-framework/operator-registry/internal/model"
	"github.com/operator-framework/operator-registry/internal/property"
)

func ConvertToModel(cfg DeclarativeConfig) (model.Model, error) {
	mpkgs := model.Model{}
	defaultChannels := map[string]string{}
	for _, p := range cfg.Packages {
		mpkg := &model.Package{
			Name:        p.Name,
			Description: p.Description,
			Channels:    map[string]*model.Channel{},
		}
		if p.Icon != nil {
			mpkg.Icon = &model.Icon{
				Data:      p.Icon.Data,
				MediaType: p.Icon.MediaType,
			}
		}
		defaultChannels[p.Name] = p.DefaultChannel
		mpkgs[p.Name] = mpkg
	}

	channelControlledBundles := map[string]struct{}{}
	for _, ch := range cfg.Channels {
		mpkg, ok := mpkgs[ch.Package]
		if !ok {
			return nil, fmt.Errorf("unknown package %q for channel %q", ch.Package, ch.Name)
		}

		mch := &model.Channel{
			Name:    ch.Name,
			Package: mpkg,
		}
		mpkg.Channels[ch.Name] = mch

		defaultChannelName := defaultChannels[ch.Package]
		if ch.Name == defaultChannelName {
			mpkg.DefaultChannel = mch
		}

		if len(ch.Versions) == 0 {
			return nil, fmt.Errorf("package %q channel %q must contain at least one version", ch.Package, ch.Name)
		}
		head := ch.Versions[len(ch.Versions)-1]

		all := sets.NewString(ch.Versions...)
		tombstones := sets.NewString(ch.Tombstones...)
		if tombstones.Has(ch.Versions[len(ch.Versions)-1]) {
			return nil, fmt.Errorf("package %q channel %q head node %q must not be a tombstone", ch.Package, ch.Name, head)
		}

		vMap := map[string]*model.Bundle{}
		for i := range cfg.Bundles {
			version := cfg.Bundles[i].Version
			if cfg.Bundles[i].Package == ch.Package && all.Has(version) {
				vMap[version] = &model.Bundle{
					Package:       mpkg,
					Channel:       mch,
					Name:          cfg.Bundles[i].Name,
					Image:         cfg.Bundles[i].Image,
					Properties:    cfg.Bundles[i].Properties,
					RelatedImages: relatedImagesToModelRelatedImages(cfg.Bundles[i].RelatedImages),
					CsvJSON:       cfg.Bundles[i].CsvJSON,
					Objects:       cfg.Bundles[i].Objects,
				}
			}
		}

		nonTombstones := []string{}
		for _, v := range ch.Versions {
			b, ok := vMap[v]
			if !ok {
				return nil, fmt.Errorf("package %q channel %q version %q not found in bundles", ch.Package, ch.Name, v)
			}
			channelControlledBundles[v] = struct{}{}

			// Existing channel membership, replaces, and skips are ignored for
			// all bundles referenced in an olm.channel. olm.channel-defined
			// upgrade graphs supersede bundle-defined graphs.
			props := b.Properties[:0]
			for _, p := range b.Properties {
				if p.Type != property.TypeChannel && p.Type != property.TypeSkips {
					props = append(props, p)
				}
			}
			b.Properties = props

			if !tombstones.Has(v) {
				nonTombstones = append(nonTombstones, v)
			}
		}

		tail := vMap[nonTombstones[0]]
		tail.Properties = append(tail.Properties, property.MustBuildChannel(ch.Name, ""))
		if len(nonTombstones) > 1 {
			for i := 1; i < len(nonTombstones); i++ {
				to := vMap[nonTombstones[i]]
				from := vMap[nonTombstones[i-1]]
				to.Replaces = from.Name
				to.Properties = append(to.Properties, property.MustBuildChannel(ch.Name, from.Name))
			}
		}

		// Iterate through ch.Versions adding skips properties.
		// i is the index of the skip candidate. If vMap[i] is a tombstoned
		// bundle, vMap[i] will be skipped by vMap[j].
		// j is the index of the first non-tombstoned node after i.
		i, j := 0, 1
		for j < len(ch.Versions) {
			// First increment j until it reaches a non-tombstone
			for tombstones.Has(ch.Versions[j]) {
				break
			}
			// Next increment i one step at a time, checking at each index if
			// i is tombstoned. If it is add a skips to j, skipping i.
			for i < j {
				if tombstones.Has(ch.Versions[i]) {
					to := vMap[ch.Versions[j]]
					from := vMap[ch.Versions[i]]
					to.Skips = append(to.Skips, from.Name)
					to.Properties = append(to.Properties, property.MustBuildSkips(from.Name))
				}
				i += 1
			}
			// Once i had caught up to j, we'll move i and j to the next two nodes
			// after the current non-tombstone node and start the loop again if j
			// is still a valid index.
			i += 1
			j += 2
		}

		// Re-key vMap on names instead of versions, since that's what the model
		// expects.
		for k, v := range vMap {
			delete(vMap, k)
			vMap[v.Name] = v
		}
		mch.Bundles = vMap
	}

	// Any other bundles not mentioned by an olm.channel blob in its package
	// is added directly.
	for _, b := range cfg.Bundles {
		if _, ok := channelControlledBundles[b.Version]; ok {
			continue
		}
		defaultChannelName := defaultChannels[b.Package]
		if b.Package == "" {
			return nil, fmt.Errorf("package name must be set for bundle %q", b.Name)
		}
		mpkg, ok := mpkgs[b.Package]
		if !ok {
			return nil, fmt.Errorf("unknown package %q for bundle %q", b.Package, b.Name)
		}

		props, err := parseProperties(b.Properties)
		if err != nil {
			return nil, fmt.Errorf("parse properties for bundle %q: %v", b.Name, err)
		}

		if len(props.Packages) == 0 {
			return nil, fmt.Errorf("missing package property for bundle %q", b.Name)
		}

		if b.Package != props.Packages[0].PackageName {
			return nil, fmt.Errorf("package %q does not match %q property %q", b.Package, property.TypePackage, props.Packages[0].PackageName)
		}

		if len(props.Channels) == 0 {
			return nil, fmt.Errorf("bundle %q is missing channel information", b.Name)
		}

		for _, bundleChannel := range props.Channels {
			pkgChannel, ok := mpkg.Channels[bundleChannel.Name]
			if !ok {
				pkgChannel = &model.Channel{
					Package: mpkg,
					Name:    bundleChannel.Name,
					Bundles: map[string]*model.Bundle{},
				}
				if bundleChannel.Name == defaultChannelName {
					mpkg.DefaultChannel = pkgChannel
				}
				mpkg.Channels[bundleChannel.Name] = pkgChannel
			}
			pkgChannel.Bundles[b.Name] = &model.Bundle{
				Package:       mpkg,
				Channel:       pkgChannel,
				Name:          b.Name,
				Image:         b.Image,
				Replaces:      bundleChannel.Replaces,
				Skips:         skipsToStrings(props.Skips),
				Properties:    b.Properties,
				RelatedImages: relatedImagesToModelRelatedImages(b.RelatedImages),
				CsvJSON:       b.CsvJSON,
				Objects:       b.Objects,
			}
		}
	}

	for _, mpkg := range mpkgs {
		defaultChannelName := defaultChannels[mpkg.Name]
		if defaultChannelName != "" && mpkg.DefaultChannel == nil {
			dch := &model.Channel{
				Package: mpkg,
				Name:    defaultChannelName,
				Bundles: map[string]*model.Bundle{},
			}
			mpkg.DefaultChannel = dch
			mpkg.Channels[dch.Name] = dch
		}
	}

	if err := mpkgs.Validate(); err != nil {
		return nil, err
	}
	mpkgs.Normalize()
	return mpkgs, nil
}

func skipsToStrings(in []property.Skips) []string {
	var out []string
	for _, s := range in {
		out = append(out, string(s))
	}
	return out
}

func relatedImagesToModelRelatedImages(in []RelatedImage) []model.RelatedImage {
	var out []model.RelatedImage
	for _, p := range in {
		out = append(out, model.RelatedImage{
			Name:  p.Name,
			Image: p.Image,
		})
	}
	return out
}
