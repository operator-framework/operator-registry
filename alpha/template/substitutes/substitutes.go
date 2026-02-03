package substitutes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/template/api"
)

// Schema
const schema string = "olm.template.substitutes"

// Template types

// Substitute defines a replacement relationship between an existing bundle name and a superceding bundle image pullspec.
// Since registry+v0 graphs are bundle name based, this uses the name instead of a version.
type Substitute struct {
	Name string `json:"name"` // the bundle image pullspec to substitute
	Base string `json:"base"` // the bundle name to substitute for
}

// SubstitutesTemplateData represents a template for bundle substitutions.
// It contains the schema identifier, an input declarative config, and substitution mappings
// that define how bundles should be replaced in upgrade graphs.
type SubstitutesTemplateData struct {
	Schema        string          `json:"schema"`
	Entries       []*declcfg.Meta `json:"entries"`
	Substitutions []Substitute    `json:"substitutions"`
}

// template implements a catalog template to make the substitutesFor mechanics less error prone.
// It provides a customizable RenderBundle function that is used to produce declarative config for a supplied bundle.
type template struct {
	renderBundle api.BundleRenderer
}

func new(renderBundle api.BundleRenderer) api.Template {
	return &template{
		renderBundle: renderBundle,
	}
}

// Template functions

func (t *template) Schema() string {
	return schema
}

// Type returns the registration type for this template
func (t *template) Type() string {
	return api.TypeFromSchema(schema)
}

func (t *template) RenderBundle(ctx context.Context, bundleRef string) (*declcfg.DeclarativeConfig, error) {
	return t.renderBundle(ctx, bundleRef)
}

func (t *template) Render(ctx context.Context, reader io.Reader) (*declcfg.DeclarativeConfig, error) {
	st, err := parseSpec(reader)
	if err != nil {
		return nil, fmt.Errorf("render: unable to parse template: %v", err)
	}

	// Create DeclarativeConfig from template entries
	cfg, err := declcfg.LoadSlice(st.Entries)
	if err != nil {
		return nil, fmt.Errorf("render: unable to create declarative config from entries: %v", err)
	}

	_, err = declcfg.ConvertToModel(*cfg)
	if err != nil {
		return nil, fmt.Errorf("render: entries are not valid FBC: %v", err)
	}

	// Process each substitution
	for _, substitution := range st.Substitutions {
		err := t.processSubstitution(ctx, cfg, substitution)
		if err != nil {
			return nil, fmt.Errorf("render: error processing substitution %s->%s: %v", substitution.Base, substitution.Name, err)
		}
	}

	return cfg, nil
}

// Helper functions

func parseSpec(reader io.Reader) (*SubstitutesTemplateData, error) {
	st := &SubstitutesTemplateData{}
	stDoc := json.RawMessage{}
	stDecoder := yaml.NewYAMLOrJSONDecoder(reader, 4096)
	err := stDecoder.Decode(&stDoc)
	if err != nil {
		return nil, fmt.Errorf("decoding template schema: %v", err)
	}
	err = json.Unmarshal(stDoc, st)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling template: %v", err)
	}

	if st.Schema != schema {
		return nil, fmt.Errorf("template has unknown schema (%q), should be %q", st.Schema, schema)
	}

	return st, nil
}

// validateSubstitution validates the substitution references
func (t *template) validateSubstitution(ctx context.Context, cfg *declcfg.DeclarativeConfig, substitution Substitute) error {
	// Validate substitution fields - all are required
	if substitution.Name == "" {
		return fmt.Errorf("substitution name cannot be empty")
	}
	if substitution.Base == "" {
		return fmt.Errorf("substitution base cannot be empty")
	}
	if substitution.Name == substitution.Base {
		return fmt.Errorf("substitution name and base cannot be the same")
	}

	// determine the versions of the base and substitute bundles and ensure that
	// the composite version of the substitute bundle is greater than the composite version of the base bundle

	// 1. Render the pullspec represented by substitution.Name
	substituteCfg, err := t.renderBundle(ctx, substitution.Name)
	if err != nil {
		return fmt.Errorf("failed to render bundle image reference %q: %v", substitution.Name, err)
	}
	if substituteCfg == nil || len(substituteCfg.Bundles) == 0 {
		return fmt.Errorf("rendered bundle image reference %q contains no bundles", substitution.Name)
	}
	substituteBundle := &substituteCfg.Bundles[0]
	substituteCv, err := substituteBundle.CompositeVersion()
	if err != nil {
		return fmt.Errorf("failed to get composite version for substitute bundle %q: %v", substitution.Name, err)
	}

	// 2. Examine cfg to find the bundle which has matching name to substitution.Base
	var baseBundle *declcfg.Bundle
	for i := range cfg.Bundles {
		if cfg.Bundles[i].Name == substitution.Base {
			baseBundle = &cfg.Bundles[i]
			break
		}
	}
	if baseBundle == nil {
		return fmt.Errorf("base bundle %q does not exist in catalog", substitution.Base)
	}
	baseCv, err := baseBundle.CompositeVersion()
	if err != nil {
		return fmt.Errorf("failed to get composite version for base bundle %q: %v", substitution.Base, err)
	}

	// 3. Ensure that the base bundle composite version is less than the substitute bundle composite version
	if baseCv.Compare(substituteCv) >= 0 {
		return fmt.Errorf("base bundle %q is not less than substitute bundle %q", substitution.Base, substitution.Name)
	}

	return nil
}

// processSubstitution handles the complex logic for processing a single substitution
func (t *template) processSubstitution(ctx context.Context, cfg *declcfg.DeclarativeConfig, substitution Substitute) error {
	if err := t.validateSubstitution(ctx, cfg, substitution); err != nil {
		return err
	}

	substituteCfg, err := t.RenderBundle(ctx, substitution.Name)
	if err != nil {
		return fmt.Errorf("failed to render bundle image reference %q: %v", substitution.Name, err)
	}

	// normally, we'd rely RenderBundle to represent any failure via err, but since this is comes from input,
	// we need to perform more validation of the results here before processing them
	if substituteCfg == nil || len(substituteCfg.Bundles) == 0 {
		return fmt.Errorf("rendered bundle image reference %q contains no bundles", substitution.Name)
	}

	substituteBundle := &substituteCfg.Bundles[0]

	// Iterate over all channels
	for i := range cfg.Channels {
		channel := &cfg.Channels[i]

		// First pass: find entries that have substitution.base as their name
		// Only process original entries, not substitution entries (they have empty replaces after clearing)
		var entriesToSubstitute []int
		for j := range channel.Entries {
			entry := &channel.Entries[j]
			if entry.Name == substitution.Base {
				entriesToSubstitute = append(entriesToSubstitute, j)
			}
		}

		// Create new entries for each substitution (process in reverse order to avoid index issues)
		for i := len(entriesToSubstitute) - 1; i >= 0; i-- {
			entryIndex := entriesToSubstitute[i]
			// Create a new channel entry for substitution.name
			newEntry := declcfg.ChannelEntry{
				Name:      substituteBundle.Name,
				Replaces:  channel.Entries[entryIndex].Replaces,
				Skips:     channel.Entries[entryIndex].Skips,
				SkipRange: channel.Entries[entryIndex].SkipRange,
			}

			// Add skip relationship to substitution.base
			newEntry.Skips = append(newEntry.Skips, substitution.Base)

			// Add the new entry to the channel
			channel.Entries = append(channel.Entries, newEntry)

			// Clear the original entry's replaces/skips/skipRange since they moved to the new entry
			channel.Entries[entryIndex].Replaces = ""
			channel.Entries[entryIndex].Skips = nil
			channel.Entries[entryIndex].SkipRange = ""
		}

		// Second pass: update all references to substitution.base to point to substitution.name
		// Skip the newly created substitution entries (they are at the end)
		originalEntryCount := len(channel.Entries) - len(entriesToSubstitute)
		for j := 0; j < originalEntryCount; j++ {
			entry := &channel.Entries[j]

			// If this entry replaces substitution.base, update it to replace substitution.name
			if entry.Replaces == substitution.Base {
				entry.Replaces = substituteBundle.Name
				entry.Skips = append(entry.Skips, substitution.Base)
			} else if entry.Skips != nil && slices.Contains(entry.Skips, substitution.Base) {
				// If this entry skips substitution.base, update it to skip substitution.name
				// and remove substitution.base from the skips list
				entry.Skips = append(entry.Skips, substituteBundle.Name)
				entry.Skips = slices.DeleteFunc(entry.Skips, func(skip string) bool {
					return skip == substitution.Base
				})
			}
		}
	}

	// Add the substitute bundle to the config (only once)
	cfg.Bundles = append(cfg.Bundles, *substituteBundle)

	// now validate the resulting config
	_, err = declcfg.ConvertToModel(*cfg)
	if err != nil {
		return fmt.Errorf("resulting config is not valid FBC: %v", err)
	}

	return nil
}

// FromReader reads FBC from a reader and generates a SubstitutesForTemplate from it
func FromReader(r io.Reader) (*SubstitutesTemplateData, error) {
	var entries []*declcfg.Meta
	if err := declcfg.WalkMetasReader(r, func(meta *declcfg.Meta, err error) error {
		if err != nil {
			return err
		}

		entries = append(entries, meta)
		return nil
	}); err != nil {
		return nil, err
	}

	bt := &SubstitutesTemplateData{
		Schema:        schema,
		Entries:       entries,
		Substitutions: []Substitute{},
	}

	return bt, nil
}

// Factory types

type Factory struct{}

// Factory functions

func (f *Factory) CreateTemplate(renderBundle api.BundleRenderer) api.Template {
	return new(renderBundle)
}

func (f *Factory) Schema() string {
	return schema
}

// Type returns the registration type for this template
func (f *Factory) Type() string {
	return api.TypeFromSchema(schema)
}
