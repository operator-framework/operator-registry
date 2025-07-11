package property

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"
)

type Property struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

func (p Property) Validate() error {
	if len(p.Type) == 0 {
		return errors.New("type must be set")
	}
	if len(p.Value) == 0 {
		return errors.New("value must be set")
	}
	var raw json.RawMessage
	if err := json.Unmarshal(p.Value, &raw); err != nil {
		return fmt.Errorf("value is not valid json: %v", err)
	}
	return nil
}

func (p Property) String() string {
	return fmt.Sprintf("type: %q, value: %q", p.Type, p.Value)
}

// ExtractValue extracts and validates the value from a SearchMetadataItem.
// It returns the properly typed value (string, []string, or map[string]bool) as an interface{}.
// The returned value is guaranteed to be valid according to the item's Type field.
func (item SearchMetadataItem) ExtractValue() (any, error) {
	switch item.Type {
	case SearchMetadataTypeString:
		return item.extractStringValue()
	case SearchMetadataTypeListString:
		return item.extractListStringValue()
	case SearchMetadataTypeMapStringBoolean:
		return item.extractMapStringBooleanValue()
	default:
		return nil, fmt.Errorf("unsupported type: %s", item.Type)
	}
}

// extractStringValue extracts and validates a string value from a SearchMetadataItem.
// This is an internal method used by ExtractValue.
func (item SearchMetadataItem) extractStringValue() (string, error) {
	str, ok := item.Value.(string)
	if !ok {
		return "", fmt.Errorf("type is 'String' but value is not a string: %T", item.Value)
	}
	if len(str) == 0 {
		return "", errors.New("string value must have length >= 1")
	}
	return str, nil
}

// extractListStringValue extracts and validates a []string value from a SearchMetadataItem.
// This is an internal method used by ExtractValue.
func (item SearchMetadataItem) extractListStringValue() ([]string, error) {
	switch v := item.Value.(type) {
	case []string:
		for i, str := range v {
			if len(str) == 0 {
				return nil, fmt.Errorf("ListString item[%d] must have length >= 1", i)
			}
		}
		return v, nil
	case []interface{}:
		result := make([]string, len(v))
		for i, val := range v {
			if str, ok := val.(string); !ok {
				return nil, fmt.Errorf("ListString item[%d] is not a string: %T", i, val)
			} else if len(str) == 0 {
				return nil, fmt.Errorf("ListString item[%d] must have length >= 1", i)
			} else {
				result[i] = str
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("type is 'ListString' but value is not a string list: %T", item.Value)
	}
}

// extractMapStringBooleanValue extracts and validates a map[string]bool value from a SearchMetadataItem.
// This is an internal method used by ExtractValue.
func (item SearchMetadataItem) extractMapStringBooleanValue() (map[string]bool, error) {
	switch v := item.Value.(type) {
	case map[string]bool:
		for key := range v {
			if len(key) == 0 {
				return nil, errors.New("MapStringBoolean keys must have length >= 1")
			}
		}
		return v, nil
	case map[string]interface{}:
		result := make(map[string]bool)
		for key, val := range v {
			if len(key) == 0 {
				return nil, errors.New("MapStringBoolean keys must have length >= 1")
			}
			if boolVal, ok := val.(bool); !ok {
				return nil, fmt.Errorf("MapStringBoolean value for key '%s' is not a boolean: %T", key, val)
			} else {
				result[key] = boolVal
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("type is 'MapStringBoolean' but value is not a string-to-boolean map: %T", item.Value)
	}
}

// validateSearchMetadataItem validates a single SearchMetadataItem.
// This is an internal helper function used during JSON unmarshaling.
func validateSearchMetadataItem(item SearchMetadataItem) error {
	if item.Name == "" {
		return errors.New("name must be set")
	}
	if item.Type == "" {
		return errors.New("type must be set")
	}
	if item.Value == nil {
		return errors.New("value must be set")
	}

	if _, err := item.ExtractValue(); err != nil {
		return err
	}
	return nil
}

type Package struct {
	PackageName string `json:"packageName"`
	Version     string `json:"version"`
}

// NOTICE: The Channel properties are for internal use only.
//
//	DO NOT use it for any public-facing functionalities.
//	This API is in alpha stage and it is subject to change.
type Channel struct {
	ChannelName string `json:"channelName"`
	//Priority    string `json:"priority"`
	Priority int `json:"priority"`
}

type PackageRequired struct {
	PackageName  string `json:"packageName"`
	VersionRange string `json:"versionRange"`
}

type GVK struct {
	Group   string `json:"group"`
	Kind    string `json:"kind"`
	Version string `json:"version"`
}

type GVKRequired struct {
	Group   string `json:"group"`
	Kind    string `json:"kind"`
	Version string `json:"version"`
}

type BundleObject struct {
	Data []byte `json:"data"`
}

type CSVMetadata struct {
	Annotations               map[string]string                  `json:"annotations,omitempty"`
	APIServiceDefinitions     v1alpha1.APIServiceDefinitions     `json:"apiServiceDefinitions,omitempty"`
	CustomResourceDefinitions v1alpha1.CustomResourceDefinitions `json:"crdDescriptions,omitempty"`
	Description               string                             `json:"description,omitempty"`
	DisplayName               string                             `json:"displayName,omitempty"`
	InstallModes              []v1alpha1.InstallMode             `json:"installModes,omitempty"`
	Keywords                  []string                           `json:"keywords,omitempty"`
	Labels                    map[string]string                  `json:"labels,omitempty"`
	Links                     []v1alpha1.AppLink                 `json:"links,omitempty"`
	Maintainers               []v1alpha1.Maintainer              `json:"maintainers,omitempty"`
	Maturity                  string                             `json:"maturity,omitempty"`
	MinKubeVersion            string                             `json:"minKubeVersion,omitempty"`
	NativeAPIs                []metav1.GroupVersionKind          `json:"nativeAPIs,omitempty"`
	Provider                  v1alpha1.AppLink                   `json:"provider,omitempty"`
}

// SearchMetadataItem represents a single search metadata item with a name, type, and value.
// Supported types are defined by the SearchMetadataType* constants.
type SearchMetadataItem struct {
	Name  string      `json:"name"`  // The name/key of the search metadata
	Type  string      `json:"type"`  // The type of the value (String, ListString, MapStringBoolean)
	Value interface{} `json:"value"` // The actual value, validated according to Type
}

// SearchMetadata represents a collection of search metadata items.
// It validates that all items are valid and that there are no duplicate names.
type SearchMetadata []SearchMetadataItem

// UnmarshalJSON implements custom JSON unmarshaling for SearchMetadata.
// It validates each item and ensures there are no duplicate names.
func (sm *SearchMetadata) UnmarshalJSON(data []byte) error {
	// First unmarshal into a slice of SearchMetadataItem
	var items []SearchMetadataItem
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// Validate each item and check for duplicate names
	namesSeen := make(map[string]bool)
	for i, item := range items {
		if err := validateSearchMetadataItem(item); err != nil {
			return fmt.Errorf("item[%d]: %v", i, err)
		}

		// Check for duplicate names
		if namesSeen[item.Name] {
			return fmt.Errorf("item[%d]: duplicate name '%s'", i, item.Name)
		}
		namesSeen[item.Name] = true
	}

	// Set the validated items
	*sm = SearchMetadata(items)
	return nil
}

type Properties struct {
	Packages         []Package         `hash:"set"`
	PackagesRequired []PackageRequired `hash:"set"`
	GVKs             []GVK             `hash:"set"`
	GVKsRequired     []GVKRequired     `hash:"set"`
	BundleObjects    []BundleObject    `hash:"set"`
	Channels         []Channel         `hash:"set"`
	CSVMetadatas     []CSVMetadata     `hash:"set"`
	SearchMetadatas  []SearchMetadata  `hash:"set"`

	Others []Property `hash:"set"`
}

const (
	TypePackage         = "olm.package"
	TypePackageRequired = "olm.package.required"
	TypeGVK             = "olm.gvk"
	TypeGVKRequired     = "olm.gvk.required"
	TypeBundleObject    = "olm.bundle.object"
	TypeCSVMetadata     = "olm.csv.metadata"
	TypeSearchMetadata  = "olm.search.metadata"
	TypeConstraint      = "olm.constraint"
	TypeChannel         = "olm.channel"
)

// Search metadata item type constants define the supported types for SearchMetadataItem values.
const (
	SearchMetadataTypeString           = "String"
	SearchMetadataTypeListString       = "ListString"
	SearchMetadataTypeMapStringBoolean = "MapStringBoolean"
)

// appendParsed is a generic helper function that parses a property and appends it to a slice.
// This is an internal helper used by the Parse function to reduce code duplication.
func appendParsed[T any](slice *[]T, prop Property) error {
	parsed, err := ParseOne[T](prop)
	if err != nil {
		return err
	}
	*slice = append(*slice, parsed)
	return nil
}

func Parse(in []Property) (*Properties, error) {
	var out Properties

	// Map of property types to their parsing functions that directly append to output slices
	parsers := map[string]func(Property) error{
		TypePackage:         func(p Property) error { return appendParsed(&out.Packages, p) },
		TypePackageRequired: func(p Property) error { return appendParsed(&out.PackagesRequired, p) },
		TypeGVK:             func(p Property) error { return appendParsed(&out.GVKs, p) },
		TypeGVKRequired:     func(p Property) error { return appendParsed(&out.GVKsRequired, p) },
		TypeBundleObject:    func(p Property) error { return appendParsed(&out.BundleObjects, p) },
		TypeCSVMetadata:     func(p Property) error { return appendParsed(&out.CSVMetadatas, p) },
		TypeSearchMetadata:  func(p Property) error { return appendParsed(&out.SearchMetadatas, p) },
		TypeChannel:         func(p Property) error { return appendParsed(&out.Channels, p) },
	}

	// Parse each property using the appropriate parser
	for i, prop := range in {
		if parser, exists := parsers[prop.Type]; exists {
			if err := parser(prop); err != nil {
				return nil, ParseError{Idx: i, Typ: prop.Type, Err: err}
			}
		} else {
			// For unknown types, use direct unmarshaling to preserve existing behavior
			var p json.RawMessage
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, ParseError{Idx: i, Typ: prop.Type, Err: err}
			}
			out.Others = append(out.Others, prop)
		}
	}

	return &out, nil
}

// ParseOne parses a single property into the specified type T.
// It validates that the property's Type field matches what the scheme expects for type T,
// ensuring type safety between the property metadata and the generic type parameter.
func ParseOne[T any](p Property) (T, error) {
	var zero T

	// Get the type of T
	targetType := reflect.TypeOf((*T)(nil)).Elem()

	// Check if T is a pointer type, if so get the element type
	if targetType.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
	}

	// Look up the expected property type for this Go type
	expectedPropertyType, ok := scheme[reflect.PointerTo(targetType)]
	if !ok {
		return zero, fmt.Errorf("type %s is not registered in the scheme", targetType)
	}

	// Verify the property type matches what we expect
	if p.Type != expectedPropertyType {
		return zero, fmt.Errorf("property type %q does not match expected type %q for %s", p.Type, expectedPropertyType, targetType)
	}

	// Unmarshal the property value into the target type
	// Any validation will happen automatically via custom UnmarshalJSON methods
	var result T
	if err := json.Unmarshal(p.Value, &result); err != nil {
		return zero, fmt.Errorf("failed to unmarshal property value: %v", err)
	}

	return result, nil
}

func Deduplicate(in []Property) []Property {
	type key struct {
		typ   string
		value string
	}

	props := map[key]Property{}
	// nolint:prealloc
	var out []Property
	for _, p := range in {
		k := key{p.Type, string(p.Value)}
		if _, ok := props[k]; ok {
			continue
		}
		props[k] = p
		out = append(out, p)
	}
	return out
}

func Build(p interface{}) (*Property, error) {
	var (
		typ string
		val interface{}
	)
	if prop, ok := p.(*Property); ok {
		typ = prop.Type
		val = prop.Value
	} else {
		t := reflect.TypeOf(p)
		if t.Kind() != reflect.Ptr {
			return nil, errors.New("input must be a pointer to a type")
		}
		typ, ok = scheme[t]
		if !ok {
			return nil, fmt.Errorf("%s not a known property type registered with the scheme", t)
		}
		val = p
	}
	d, err := jsonMarshal(val)
	if err != nil {
		return nil, err
	}

	return &Property{
		Type:  typ,
		Value: d,
	}, nil
}

func MustBuild(p interface{}) Property {
	prop, err := Build(p)
	if err != nil {
		panic(err)
	}
	return *prop
}

func jsonMarshal(p interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	dec := json.NewEncoder(buf)
	dec.SetEscapeHTML(false)
	err := dec.Encode(p)
	if err != nil {
		return nil, err
	}
	out := &bytes.Buffer{}
	if err := json.Compact(out, buf.Bytes()); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func MustBuildPackage(name, version string) Property {
	return MustBuild(&Package{PackageName: name, Version: version})
}
func MustBuildPackageRequired(name, versionRange string) Property {
	return MustBuild(&PackageRequired{name, versionRange})
}
func MustBuildGVK(group, version, kind string) Property {
	return MustBuild(&GVK{group, kind, version})
}
func MustBuildGVKRequired(group, version, kind string) Property {
	return MustBuild(&GVKRequired{group, kind, version})
}
func MustBuildBundleObject(data []byte) Property {
	return MustBuild(&BundleObject{Data: data})
}

func MustBuildCSVMetadata(csv v1alpha1.ClusterServiceVersion) Property {
	return MustBuild(&CSVMetadata{
		Annotations:               csv.GetAnnotations(),
		APIServiceDefinitions:     csv.Spec.APIServiceDefinitions,
		CustomResourceDefinitions: csv.Spec.CustomResourceDefinitions,
		Description:               csv.Spec.Description,
		DisplayName:               csv.Spec.DisplayName,
		InstallModes:              csv.Spec.InstallModes,
		Keywords:                  csv.Spec.Keywords,
		Labels:                    csv.GetLabels(),
		Links:                     csv.Spec.Links,
		Maintainers:               csv.Spec.Maintainers,
		Maturity:                  csv.Spec.Maturity,
		MinKubeVersion:            csv.Spec.MinKubeVersion,
		NativeAPIs:                csv.Spec.NativeAPIs,
		Provider:                  csv.Spec.Provider,
	})
}

// MustBuildSearchMetadata creates a search metadata property from a SearchMetadata.
// It panics if the items are invalid or if there are duplicate names.
func MustBuildSearchMetadata(searchMetadata SearchMetadata) Property {
	return MustBuild(&searchMetadata)
}

// NOTICE: The Channel properties are for internal use only.
//
//	DO NOT use it for any public-facing functionalities.
//	This API is in alpha stage and it is subject to change.
func MustBuildChannelPriority(name string, priority int) Property {
	return MustBuild(&Channel{ChannelName: name, Priority: priority})
}
