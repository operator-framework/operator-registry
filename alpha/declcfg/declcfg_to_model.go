package declcfg

import (
	"encoding/json"
	"fmt"

	"github.com/blang/semver"

	"github.com/operator-framework/operator-registry/alpha/model"
	"github.com/operator-framework/operator-registry/alpha/property"
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

			// Parse version from the package property, falling back to the CSV's spec.version field.
			var ver semver.Version
			for _, pkgProp := range props.Packages {
				if pkgProp.PackageName == mpkg.Name && pkgProp.Version != "" {
					if ver, err = semver.Parse(pkgProp.Version); err != nil {
						return nil, fmt.Errorf("error parsing bundle version: %v", err)
					}
					break
				}
			}
			if ver.Equals(semver.Version{}) {
				if ver, err = getCSVVersion([]byte(b.CsvJSON)); err != nil {
					return nil, fmt.Errorf("error reading bundle version from CSV: %v", err)
				}
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
				PropertiesP:   props,
				Version:       ver,
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

func getCSVVersion(csvJSON []byte) (semver.Version, error) {
	var tmp struct {
		Spec struct {
			Version semver.Version `json:"version"`
		} `json:"spec"`
	}
	err := json.Unmarshal(csvJSON, &tmp)
	return tmp.Spec.Version, err
}
