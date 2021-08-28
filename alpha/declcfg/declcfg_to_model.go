package declcfg

import (
	"fmt"

	"github.com/blang/semver"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/operator-framework/operator-registry/alpha/model"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func ConvertToModel(cfg DeclarativeConfig) (model.Model, error) {
	mpkgs := model.Model{}
	defaultChannels := map[string]string{}
	for _, p := range cfg.Packages {
		if p.Name == "" {
			return nil, fmt.Errorf("config contains package with no name")
		}

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

	channelDefinedEdges := sets.NewString()
	channelDefinedEntries := map[string]sets.String{}
	for _, c := range cfg.Channels {
		mpkg, ok := mpkgs[c.Package]
		if !ok {
			return nil, fmt.Errorf("unknown package %q for channel %q", c.Package, c.Name)
		}

		if c.Name == "" {
			return nil, fmt.Errorf("package %q contains channel with no name", c.Package)
		}

		mch := &model.Channel{
			Package: mpkg,
			Name:    c.Name,
			Bundles: map[string]*model.Bundle{},
		}

		cde := sets.NewString()
		if c.Strategy.Legacy == nil {
			return nil, fmt.Errorf("package %q, channel %q has no defined strategy", c.Package, c.Name)
		}
		for _, entry := range c.Strategy.Legacy.Entries {
			if _, ok := mch.Bundles[entry.Name]; ok {
				return nil, fmt.Errorf("invalid package %q, channel %q: duplicate entry %q", c.Package, c.Name, entry.Name)
			}
			cde = cde.Insert(entry.Name)
			mch.Bundles[entry.Name] = &model.Bundle{
				Package:   mpkg,
				Channel:   mch,
				Name:      entry.Name,
				Replaces:  entry.Replaces,
				Skips:     entry.Skips,
				SkipRange: entry.SkipRange,
			}
		}
		channelDefinedEntries[c.Package] = cde
		channelDefinedEdges = channelDefinedEdges.Insert(c.Package)

		mpkg.Channels[c.Name] = mch

		defaultChannelName := defaultChannels[c.Package]
		if defaultChannelName == c.Name {
			mpkg.DefaultChannel = mch
		}
	}

	for _, b := range cfg.Bundles {
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

		if len(props.Packages) != 1 {
			return nil, fmt.Errorf("package %q bundle %q must have exactly 1 %q property, found %d", b.Package, b.Name, property.TypePackage, len(props.Packages))
		}

		if b.Package != props.Packages[0].PackageName {
			return nil, fmt.Errorf("package %q does not match %q property %q", b.Package, property.TypePackage, props.Packages[0].PackageName)
		}

		// Parse version from the package property.
		ver, err := semver.Parse(props.Packages[0].Version)
		if err != nil {
			return nil, fmt.Errorf("error parsing bundle version: %v", err)
		}

		if channelDefinedEdges.Has(b.Package) {
			if len(props.Channels) > 0 {
				return nil, fmt.Errorf("invalid package %q, bundle %q: cannot use %q properties with %q blobs", b.Package, b.Name, property.TypeChannel, schemaChannel)
			}
			if len(props.Skips) > 0 {
				return nil, fmt.Errorf("invalid package %q, bundle %q: cannot use %q properties with %q blobs", b.Package, b.Name, property.TypeSkips, schemaChannel)
			}
			if len(props.SkipRanges) > 0 {
				return nil, fmt.Errorf("invalid package %q, bundle %q: cannot use %q properties with %q blobs", b.Package, b.Name, property.TypeSkipRange, schemaChannel)
			}

			channelDefinedEntries[b.Package] = channelDefinedEntries[b.Package].Delete(b.Name)
			found := false
			for _, mch := range mpkg.Channels {
				if mb, ok := mch.Bundles[b.Name]; ok {
					found = true
					mb.Image = b.Image
					mb.Properties = b.Properties
					mb.RelatedImages = relatedImagesToModelRelatedImages(b.RelatedImages)
					mb.CsvJSON = b.CsvJSON
					mb.Objects = b.Objects
					mb.PropertiesP = props
					mb.Version = ver
				}
			}
			if !found {
				return nil, fmt.Errorf("package %q, bundle %q not found in any channel entries", b.Package, b.Name)
			}
		} else {
			if len(props.Channels) == 0 {
				return nil, fmt.Errorf("package %q bundle %q is missing channel information", b.Package, b.Name)
			}

			if len(props.SkipRanges) > 1 {
				return nil, fmt.Errorf("package %q bundle %q is invalid: multiple properties of type %q not allowed", b.Package, b.Name, property.TypeSkipRange)
			}

			skipRange := ""
			if len(props.SkipRanges) > 0 {
				skipRange = string(props.SkipRanges[0])
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
					SkipRange:     skipRange,
					Properties:    b.Properties,
					RelatedImages: relatedImagesToModelRelatedImages(b.RelatedImages),
					CsvJSON:       b.CsvJSON,
					Objects:       b.Objects,
					PropertiesP:   props,
					Version:       ver,
				}
			}
		}
	}

	for pkg, entries := range channelDefinedEntries {
		if entries.Len() > 0 {
			return nil, fmt.Errorf("no olm.bundle blobs found in package %q for olm.channel entries %s", pkg, entries.List())
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
