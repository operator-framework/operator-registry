package declcfg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/text/cases"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"

	"github.com/operator-framework/operator-registry/alpha/property"
	"github.com/operator-framework/operator-registry/pkg/prettyunmarshaler"
)

const (
	SchemaPackage     = "olm.package"
	SchemaChannel     = "olm.channel"
	SchemaBundle      = "olm.bundle"
	SchemaDeprecation = "olm.deprecations"

	SchemaPackageV2                 = "olm.package.v2"
	SchemaPackageV2Metadata         = "olm.package.v2.metadata"
	SchemaPackageV2Icon             = "olm.package.v2.icon"
	SchemaChannelV2                 = "olm.channel.v2"
	SchemaBundleV2                  = "olm.bundle.v2"
	SchemaBundleV2Metadata          = "olm.bundle.v2.metadata"
	SchemaBundleV2RelatedReferences = "olm.bundle.v2.relatedReferences"
)

type DeclarativeConfig struct {
	Packages []Package
	Channels []Channel
	Bundles  []Bundle

	PackageV2s                []PackageV2
	PackageV2Icons            []PackageV2Icon
	PackageV2Metadatas        []PackageV2Metadata
	ChannelV2s                []ChannelV2
	BundleV2s                 []BundleV2
	BundleV2Metadatas         []BundleV2Metadata
	BundleV2RelatedReferences []BundleV2RelatedReferences

	Deprecations []Deprecation
	Others       []Meta
}

type Package struct {
	Schema         string              `json:"schema"`
	Name           string              `json:"name"`
	DefaultChannel string              `json:"defaultChannel"`
	Icon           *Icon               `json:"icon,omitempty"`
	Description    string              `json:"description,omitempty"`
	Properties     []property.Property `json:"properties,omitempty" hash:"set"`
}

type Icon struct {
	Data      []byte `json:"base64data"`
	MediaType string `json:"mediatype"`
}

type Channel struct {
	Schema     string              `json:"schema"`
	Name       string              `json:"name"`
	Package    string              `json:"package"`
	Entries    []ChannelEntry      `json:"entries"`
	Properties []property.Property `json:"properties,omitempty" hash:"set"`
}

type ChannelEntry struct {
	Name      string   `json:"name"`
	Replaces  string   `json:"replaces,omitempty"`
	Skips     []string `json:"skips,omitempty"`
	SkipRange string   `json:"skipRange,omitempty"`
}

// Bundle specifies all metadata and data of a bundle object.
// Top-level fields are the source of truth, i.e. not CSV values.
//
// Notes:
//   - Any field slice type field or type containing a slice somewhere
//     where two types/fields are equal if their contents are equal regardless
//     of order must have a `hash:"set"` field tag for bundle comparison.
//   - Any fields that have a `json:"-"` tag must be included in the equality
//     evaluation in bundlesEqual().
type Bundle struct {
	Schema        string              `json:"schema"`
	Name          string              `json:"name,omitempty"`
	Package       string              `json:"package,omitempty"`
	Image         string              `json:"image"`
	Properties    []property.Property `json:"properties,omitempty" hash:"set"`
	RelatedImages []RelatedImage      `json:"relatedImages,omitempty" hash:"set"`

	// These fields are present so that we can continue serving
	// the GRPC API the way packageserver expects us to in a
	// backwards-compatible way. These are populated from
	// any `olm.bundle.object` properties.
	//
	// These fields will never be persisted in the bundle blob as
	// first class fields.
	CsvJSON string   `json:"-"`
	Objects []string `json:"-"`
}

type RelatedImage struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

type PackageV2 struct {
	Schema  string `json:"schema"`
	Package string `json:"package"`
}

type PackageV2Icon struct {
	Schema    string `json:"schema"`
	Package   string `json:"package"`
	Data      []byte `json:"data"`
	MediaType string `json:"mediaType"`
}

type PackageV2Metadata struct {
	Schema  string `json:"schema"`
	Package string `json:"package"`

	Annotations      map[string]string     `json:"annotations,omitempty"`
	DisplayName      string                `json:"displayName,omitempty"`
	ShortDescription string                `json:"shortDescription,omitempty"`
	LongDescription  string                `json:"longDescription,omitempty"`
	Keywords         []string              `json:"keywords,omitempty"`
	Links            []v1alpha1.AppLink    `json:"links,omitempty"`
	Provider         v1alpha1.AppLink      `json:"provider,omitempty"`
	Maintainers      []v1alpha1.Maintainer `json:"maintainers,omitempty"`
}

type ChannelV2 struct {
	Schema  string `json:"schema"`
	Package string `json:"package"`
	Name    string `json:"name"`

	Edges []UpgradeEdgeV2 `json:"edges"`
}

type UpgradeEdgeV2 struct {
	Version      string `json:"version"`
	UpgradesFrom string `json:"upgradesFrom"`
}

type BundleV2 struct {
	Schema  string `json:"schema"`
	Package string `json:"package"`
	Name    string `json:"name"`

	Version string `json:"version"`
	Release uint32 `json:"release"`

	Reference string `json:"ref"`

	Properties  map[string][]json.RawMessage `json:"properties,omitempty"`
	Constraints map[string][]json.RawMessage `json:"constraints,omitempty"`
}

type BundleV2Metadata struct {
	Schema  string `json:"schema"`
	Package string `json:"package"`
	Name    string `json:"name"`

	Annotations map[string]string `json:"annotations,omitempty"`
}

type BundleV2RelatedReferences struct {
	Schema  string `json:"schema"`
	Package string `json:"package"`
	Name    string `json:"name"`

	References []string `json:"references,omitempty"`
}

type Deprecation struct {
	Schema  string             `json:"schema"`
	Package string             `json:"package"`
	Entries []DeprecationEntry `json:"entries"`
}

type DeprecationEntry struct {
	Reference PackageScopedReference `json:"reference"`
	Message   string                 `json:"message"`
}

type PackageScopedReference struct {
	Schema string `json:"schema"`
	Name   string `json:"name,omitempty"`
}

type Meta struct {
	Schema  string
	Package string
	Name    string

	Blob json.RawMessage
}

func (m Meta) MarshalJSON() ([]byte, error) {
	return m.Blob, nil
}

func (m *Meta) UnmarshalJSON(blob []byte) error {
	blobMap := map[string]interface{}{}
	if err := json.Unmarshal(blob, &blobMap); err != nil {
		// TODO: unfortunately, there are libraries between here and the original caller
		//   that eat our error type and return a generic error, such that we lose the
		//   ability to errors.As to get this error on the other side. For now, just return
		//   a string error that includes the pretty printed message.
		return errors.New(prettyunmarshaler.NewJSONUnmarshalError(blob, err).Pretty())
	}

	// TODO: this function ensures we do not break backwards compatibility with
	//    the documented examples of FBC templates, which use upper camel case
	//    for JSON field names. We need to decide if we want to continue supporting
	//    case insensitive JSON field names, or if we want to enforce a specific
	//    case-sensitive key value for each field.
	if err := extractUniqueMetaKeys(blobMap, m); err != nil {
		return err
	}

	buf := bytes.Buffer{}
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(blobMap); err != nil {
		return err
	}
	m.Blob = buf.Bytes()
	return nil
}

// extractUniqueMetaKeys enables a case-insensitive key lookup for the schema, package, and name
// fields of the Meta struct. If the blobMap contains duplicate keys (that is, keys have the same folded value),
// an error is returned.
func extractUniqueMetaKeys(blobMap map[string]any, m *Meta) error {
	keySets := map[string]sets.Set[string]{}
	folder := cases.Fold()
	for key := range blobMap {
		foldKey := folder.String(key)
		if _, ok := keySets[foldKey]; !ok {
			keySets[foldKey] = sets.New[string]()
		}
		keySets[foldKey].Insert(key)
	}

	dupErrs := []error{}
	for foldedKey, keys := range keySets {
		if len(keys) != 1 {
			dupErrs = append(dupErrs, fmt.Errorf("duplicate keys for key %q: %v", foldedKey, sets.List(keys)))
		}
	}
	if len(dupErrs) > 0 {
		return utilerrors.NewAggregate(dupErrs)
	}

	metaMap := map[string]*string{
		folder.String("schema"):  &m.Schema,
		folder.String("package"): &m.Package,
		folder.String("name"):    &m.Name,
	}

	for foldedKey, ptr := range metaMap {
		// if the folded key doesn't exist in the key set derived from the blobMap, that means
		// the key doesn't exist in the blobMap, so we can skip it
		if _, ok := keySets[foldedKey]; !ok {
			continue
		}

		// reset key to the unfolded key, which we know is the one that appears in the blobMap
		key := keySets[foldedKey].UnsortedList()[0]
		if _, ok := blobMap[key]; !ok {
			continue
		}
		v, ok := blobMap[key].(string)
		if !ok {
			return fmt.Errorf("expected value for key %q to be a string, got %t: %v", key, blobMap[key], blobMap[key])
		}
		*ptr = v
	}
	return nil
}

func (destination *DeclarativeConfig) Merge(src *DeclarativeConfig) {
	destination.Packages = append(destination.Packages, src.Packages...)
	destination.Channels = append(destination.Channels, src.Channels...)
	destination.Bundles = append(destination.Bundles, src.Bundles...)

	destination.PackageV2s = append(destination.PackageV2s, src.PackageV2s...)
	destination.PackageV2Icons = append(destination.PackageV2Icons, src.PackageV2Icons...)
	destination.PackageV2Metadatas = append(destination.PackageV2Metadatas, src.PackageV2Metadatas...)
	destination.ChannelV2s = append(destination.ChannelV2s, src.ChannelV2s...)
	destination.BundleV2s = append(destination.BundleV2s, src.BundleV2s...)
	destination.BundleV2Metadatas = append(destination.BundleV2Metadatas, src.BundleV2Metadatas...)
	destination.BundleV2RelatedReferences = append(destination.BundleV2RelatedReferences, src.BundleV2RelatedReferences...)

	destination.Deprecations = append(destination.Deprecations, src.Deprecations...)

	destination.Others = append(destination.Others, src.Others...)

}
