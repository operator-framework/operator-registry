package declcfg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/blang/semver/v4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/operator-framework/api/pkg/lib/version"
	"github.com/operator-framework/api/pkg/operators"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"

	"github.com/operator-framework/operator-registry/alpha/property"
	"github.com/operator-framework/operator-registry/pkg/api"
)

// ConvertBundleToAPIBundle converts a declcfg.Bundle to an api.Bundle.
// The pkg and channels parameters provide context needed for the conversion.
func ConvertBundleToAPIBundle(b Bundle, pkg Package, channels []Channel) (*api.Bundle, error) {
	props, err := parseProperties(b.Properties)
	if err != nil {
		return nil, fmt.Errorf("parse properties: %v", err)
	}

	csvJSON := generateCSVJSON(b, pkg, props)

	// Find which channel this bundle belongs to
	channelName := ""
	for _, ch := range channels {
		if ch.Package != b.Package {
			continue
		}
		for _, entry := range ch.Entries {
			if entry.Name == b.Name {
				channelName = ch.Name
				break
			}
		}
		if channelName != "" {
			break
		}
	}

	// Get replaces and skips from channel entry
	var replaces string
	var skips []string
	var skipRange string
	for _, ch := range channels {
		if ch.Package != b.Package {
			continue
		}
		for _, entry := range ch.Entries {
			if entry.Name == b.Name {
				replaces = entry.Replaces
				skips = entry.Skips
				skipRange = entry.SkipRange
				break
			}
		}
	}

	apiDeps, err := convertPropertiesToAPIDependencies(b.Properties)
	if err != nil {
		return nil, fmt.Errorf("convert properties to api dependencies: %v", err)
	}

	return &api.Bundle{
		CsvName:      b.Name,
		PackageName:  b.Package,
		ChannelName:  channelName,
		BundlePath:   b.Image,
		ProvidedApis: gvksProvidedtoAPIGVKs(props.GVKs),
		RequiredApis: gvksRequiredtoAPIGVKs(props.GVKsRequired),
		Version:      props.Packages[0].Version,
		SkipRange:    skipRange,
		Dependencies: apiDeps,
		Properties:   convertPropertiesToAPIProperties(b.Properties),
		Replaces:     replaces,
		Skips:        skips,
		CsvJson:      csvJSON,
		Object:       b.Objects,
	}, nil
}

func generateCSVJSON(b Bundle, pkg Package, props *property.Properties) string {
	if b.CsvJSON != "" || len(props.CSVMetadatas) != 1 {
		return b.CsvJSON
	}

	csv := buildCSV(b, pkg, props)
	csvData, err := json.Marshal(csv)
	if err != nil {
		return b.CsvJSON
	}
	csvJSON := string(csvData)
	if len(b.Objects) == 0 {
		b.Objects = []string{csvJSON}
	}
	return csvJSON
}

func buildCSV(b Bundle, pkg Package, props *property.Properties) *v1alpha1.ClusterServiceVersion {
	var icons []v1alpha1.Icon
	if pkg.Icon != nil {
		icons = []v1alpha1.Icon{{
			Data:      base64.StdEncoding.EncodeToString(pkg.Icon.Data),
			MediaType: pkg.Icon.MediaType,
		}}
	}

	csv := csvMetadataToCsv(props.CSVMetadatas[0])
	csv.Name = b.Name
	csv.Spec.Icon = icons
	csv.Spec.InstallStrategy = v1alpha1.NamedInstallStrategy{
		StrategyName: "deployment",
	}

	ver, err := semver.Parse(props.Packages[0].Version)
	if err == nil {
		csv.Spec.Version = version.OperatorVersion{Version: ver}
	}

	csv.Spec.RelatedImages = convertRelatedImagesToCSVRelatedImages(b.RelatedImages)
	if csv.Spec.Description == "" {
		csv.Spec.Description = pkg.Description
	}
	return &csv
}

func parseProperties(in []property.Property) (*property.Properties, error) {
	props, err := property.Parse(in)
	if err != nil {
		return nil, err
	}

	if len(props.Packages) != 1 {
		return nil, fmt.Errorf("expected exactly 1 property of type %q, found %d", property.TypePackage, len(props.Packages))
	}

	if len(props.CSVMetadatas) > 1 {
		return nil, fmt.Errorf("expected at most 1 property of type %q, found %d", property.TypeCSVMetadata, len(props.CSVMetadatas))
	}

	return props, nil
}

func csvMetadataToCsv(m property.CSVMetadata) v1alpha1.ClusterServiceVersion {
	return v1alpha1.ClusterServiceVersion{
		TypeMeta: metav1.TypeMeta{
			Kind:       operators.ClusterServiceVersionKind,
			APIVersion: v1alpha1.ClusterServiceVersionAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: m.Annotations,
			Labels:      m.Labels,
		},
		Spec: v1alpha1.ClusterServiceVersionSpec{
			APIServiceDefinitions:     m.APIServiceDefinitions,
			CustomResourceDefinitions: m.CustomResourceDefinitions,
			Description:               m.Description,
			DisplayName:               m.DisplayName,
			InstallModes:              m.InstallModes,
			Keywords:                  m.Keywords,
			Links:                     m.Links,
			Maintainers:               m.Maintainers,
			Maturity:                  m.Maturity,
			MinKubeVersion:            m.MinKubeVersion,
			NativeAPIs:                m.NativeAPIs,
			Provider:                  m.Provider,
		},
	}
}

func gvksProvidedtoAPIGVKs(in []property.GVK) []*api.GroupVersionKind {
	// nolint:prealloc
	var out []*api.GroupVersionKind
	for _, gvk := range in {
		out = append(out, &api.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		})
	}
	return out
}

func gvksRequiredtoAPIGVKs(in []property.GVKRequired) []*api.GroupVersionKind {
	// nolint:prealloc
	var out []*api.GroupVersionKind
	for _, gvk := range in {
		out = append(out, &api.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		})
	}
	return out
}

func convertPropertiesToAPIProperties(props []property.Property) []*api.Property {
	// nolint:prealloc
	var out []*api.Property
	for _, prop := range props {
		// NOTE: This is a special case filter to prevent problems with existing client implementations that
		//       project bundle properties into CSV annotations and store those CSVs in a size-constrained
		//       storage backend (e.g. etcd via kube-apiserver). If the bundle object property has data inlined
		//       in its `Data` field, this CSV annotation projection would cause the size of the on-cluster
		//       CSV to at least double, which is untenable since CSVs already have known issues running up
		//       against etcd size constraints.
		if prop.Type == property.TypeBundleObject || prop.Type == property.TypeCSVMetadata {
			continue
		}

		out = append(out, &api.Property{
			Type:  prop.Type,
			Value: string(prop.Value),
		})
	}
	return out
}

func convertPropertiesToAPIDependencies(props []property.Property) ([]*api.Dependency, error) {
	// nolint:prealloc
	var out []*api.Dependency
	for _, prop := range props {
		switch prop.Type {
		case property.TypeGVKRequired:
			out = append(out, &api.Dependency{
				Type:  property.TypeGVK,
				Value: string(prop.Value),
			})
		case property.TypePackageRequired:
			var v property.PackageRequired
			if err := json.Unmarshal(prop.Value, &v); err != nil {
				return nil, err
			}
			pkg := property.MustBuildPackage(v.PackageName, v.VersionRange)
			out = append(out, &api.Dependency{
				Type:  pkg.Type,
				Value: string(pkg.Value),
			})
		}
	}
	return out, nil
}

func convertRelatedImagesToCSVRelatedImages(in []RelatedImage) []v1alpha1.RelatedImage {
	// nolint:prealloc
	var out []v1alpha1.RelatedImage
	for _, ri := range in {
		out = append(out, v1alpha1.RelatedImage{
			Name:  ri.Name,
			Image: ri.Image,
		})
	}
	return out
}

// ConvertAPIBundleToBundle converts an api.Bundle to a declcfg.Bundle.
func ConvertAPIBundleToBundle(b *api.Bundle) (*Bundle, error) {
	bundleProps, err := convertAPIBundleToProperties(b)
	if err != nil {
		return nil, fmt.Errorf("convert properties: %v", err)
	}

	relatedImages, err := getRelatedImages(b.CsvJson)
	if err != nil {
		return nil, fmt.Errorf("get related images: %v", err)
	}

	return &Bundle{
		Schema:        SchemaBundle,
		Name:          b.CsvName,
		Package:       b.PackageName,
		Image:         b.BundlePath,
		Properties:    bundleProps,
		RelatedImages: relatedImages,
		CsvJSON:       b.CsvJson,
		Objects:       b.Object,
	}, nil
}

func convertAPIBundleToProperties(b *api.Bundle) ([]property.Property, error) {
	// nolint:prealloc
	var out []property.Property

	providedGVKs := map[property.GVK]struct{}{}
	requiredGVKs := map[property.GVKRequired]struct{}{}

	foundPackageProperty := false
	for i, p := range b.Properties {
		switch p.Type {
		case property.TypeGVK:
			var v api.GroupVersionKind
			if err := json.Unmarshal(json.RawMessage(p.Value), &v); err != nil {
				return nil, property.ParseError{Idx: i, Typ: p.Type, Err: err}
			}
			k := property.GVK{Group: v.Group, Kind: v.Kind, Version: v.Version}
			providedGVKs[k] = struct{}{}
		case property.TypePackage:
			foundPackageProperty = true
			out = append(out, property.Property{
				Type:  property.TypePackage,
				Value: json.RawMessage(p.Value),
			})
		default:
			out = append(out, property.Property{
				Type:  p.Type,
				Value: json.RawMessage(p.Value),
			})
		}
	}

	for i, p := range b.Dependencies {
		switch p.Type {
		case property.TypeGVK:
			var v api.GroupVersionKind
			if err := json.Unmarshal(json.RawMessage(p.Value), &v); err != nil {
				return nil, property.ParseError{Idx: i, Typ: p.Type, Err: err}
			}
			k := property.GVKRequired{Group: v.Group, Kind: v.Kind, Version: v.Version}
			requiredGVKs[k] = struct{}{}
		case property.TypePackage:
			var v property.Package
			if err := json.Unmarshal(json.RawMessage(p.Value), &v); err != nil {
				return nil, property.ParseError{Idx: i, Typ: p.Type, Err: err}
			}
			out = append(out, property.MustBuildPackageRequired(v.PackageName, v.Version))
		}
	}

	if !foundPackageProperty {
		out = append(out, property.MustBuildPackage(b.PackageName, b.Version))
	}

	for _, p := range b.ProvidedApis {
		k := property.GVK{Group: p.Group, Kind: p.Kind, Version: p.Version}
		if _, ok := providedGVKs[k]; !ok {
			providedGVKs[k] = struct{}{}
		}
	}
	for _, p := range b.RequiredApis {
		k := property.GVKRequired{Group: p.Group, Kind: p.Kind, Version: p.Version}
		if _, ok := requiredGVKs[k]; !ok {
			requiredGVKs[k] = struct{}{}
		}
	}

	for p := range providedGVKs {
		out = append(out, property.MustBuildGVK(p.Group, p.Version, p.Kind))
	}

	for p := range requiredGVKs {
		out = append(out, property.MustBuildGVKRequired(p.Group, p.Version, p.Kind))
	}

	for _, obj := range b.Object {
		out = append(out, property.MustBuildBundleObject([]byte(obj)))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return string(out[i].Value) < string(out[j].Value)
	})

	return out, nil
}

func getRelatedImages(csvJSON string) ([]RelatedImage, error) {
	if len(csvJSON) == 0 {
		return nil, nil
	}
	type csv struct {
		Spec struct {
			RelatedImages []struct {
				Name  string `json:"name"`
				Image string `json:"image"`
			} `json:"relatedImages"`
		} `json:"spec"`
	}
	c := csv{}
	if err := json.Unmarshal([]byte(csvJSON), &c); err != nil {
		return nil, fmt.Errorf("unmarshal csv: %v", err)
	}
	var relatedImages []RelatedImage
	for _, ri := range c.Spec.RelatedImages {
		relatedImages = append(relatedImages, RelatedImage{
			Name:  ri.Name,
			Image: ri.Image,
		})
	}
	return relatedImages, nil
}
