package basic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/template/api"
)

// Schema
const schema string = "olm.template.basic"

// Template types

type BasicTemplateData struct {
	Schema  string          `json:"schema"`
	Entries []*declcfg.Meta `json:"entries"`
}

type basicTemplate struct {
	renderBundle api.BundleRenderer
}

// new creates a new basic template instance
func new(renderBundle api.BundleRenderer) api.Template {
	return &basicTemplate{
		renderBundle: renderBundle,
	}
}

// Template functions

// RenderBundle expands the bundle image reference into a DeclarativeConfig fragment.
func (t *basicTemplate) RenderBundle(ctx context.Context, image string) (*declcfg.DeclarativeConfig, error) {
	return t.renderBundle(ctx, image)
}

// Render extracts the spec from the reader and converts it to a standalone DeclarativeConfig,
// expanding any bundle image references into full olm.bundle DeclarativeConfig
func (t *basicTemplate) Render(ctx context.Context, reader io.Reader) (*declcfg.DeclarativeConfig, error) {
	bt, err := parseSpec(reader)
	if err != nil {
		return nil, err
	}
	cfg, err := declcfg.LoadSlice(bt.Entries)
	if err != nil {
		return cfg, err
	}

	outb := cfg.Bundles[:0]
	for _, b := range cfg.Bundles {
		if !isBundleTemplate(&b) {
			return nil, fmt.Errorf("unexpected fields present in basic template bundle")
		}
		contributor, err := t.RenderBundle(ctx, b.Image)
		if err != nil {
			return nil, err
		}
		outb = append(outb, contributor.Bundles...)
	}

	cfg.Bundles = outb
	return cfg, nil
}

// Schema returns the schema identifier for this template type
func (t *basicTemplate) Schema() string {
	return schema
}

// Type returns the registration type for this template
func (t *basicTemplate) Type() string {
	return api.TypeFromSchema(schema)
}

// Helper functions

func parseSpec(reader io.Reader) (*BasicTemplateData, error) {
	bt := &BasicTemplateData{}
	btDoc := json.RawMessage{}
	btDecoder := yaml.NewYAMLOrJSONDecoder(reader, 4096)
	err := btDecoder.Decode(&btDoc)
	if err != nil {
		return nil, fmt.Errorf("decoding template schema: %v", err)
	}
	err = json.Unmarshal(btDoc, bt)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling template: %v", err)
	}

	if bt.Schema != schema {
		return nil, fmt.Errorf("template has unknown schema (%q), should be %q", bt.Schema, schema)
	}

	return bt, nil
}

// isBundleTemplate identifies a Bundle template source as having a Schema and Image defined
// but no Properties, RelatedImages or Package defined
func isBundleTemplate(b *declcfg.Bundle) bool {
	return b.Schema != "" && b.Image != "" && b.Package == "" && len(b.Properties) == 0 && len(b.RelatedImages) == 0
}

// FromReader reads FBC from a reader and generates a BasicTemplateData from it
func FromReader(r io.Reader) (*BasicTemplateData, error) {
	var entries []*declcfg.Meta
	if err := declcfg.WalkMetasReader(r, func(meta *declcfg.Meta, err error) error {
		if err != nil {
			return err
		}
		if meta.Schema == declcfg.SchemaBundle {
			var b declcfg.Bundle
			if err := json.Unmarshal(meta.Blob, &b); err != nil {
				return fmt.Errorf("parse bundle: %v", err)
			}
			b2 := declcfg.Bundle{
				Schema: b.Schema,
				Image:  b.Image,
			}
			meta.Blob, err = json.Marshal(b2)
			if err != nil {
				return fmt.Errorf("re-serialize bundle: %v", err)
			}
		}
		entries = append(entries, meta)
		return nil
	}); err != nil {
		return nil, err
	}

	bt := &BasicTemplateData{
		Schema:  schema,
		Entries: entries,
	}

	return bt, nil
}

// Factory types

// Factory represents the basic template factory
type Factory struct{}

// Factory functions

// CreateTemplate creates a new template instance with the given RenderBundle function
func (f *Factory) CreateTemplate(renderBundle api.BundleRenderer) api.Template {
	return new(renderBundle)
}

// Schema returns the schema supported by this factory
func (f *Factory) Schema() string {
	return schema
}

// Type returns the registration type for this template
func (f *Factory) Type() string {
	return api.TypeFromSchema(schema)
}
