package substitutes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

type Template struct {
	RenderBundle func(context.Context, string) (*declcfg.DeclarativeConfig, error)
}

type Substitute struct {
	Name string `json:"name"` // the bundle image pullspec to substitute
	Base string `json:"base"` // the bundle name to substitute for
}

type SubstitutesForTemplate struct {
	Schema        string          `json:"schema"`
	Entries       []*declcfg.Meta `json:"entries"`
	Substitutions []Substitute    `json:"substitutions"`
}

const schema string = "olm.template.substitutes"

func parseSpec(reader io.Reader) (*SubstitutesForTemplate, error) {
	st := &SubstitutesForTemplate{}
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

func (t Template) Render(ctx context.Context, reader io.Reader) (*declcfg.DeclarativeConfig, error) {
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

// processSubstitution handles the complex logic for processing a single substitution
func (t Template) processSubstitution(ctx context.Context, cfg *declcfg.DeclarativeConfig, substitution Substitute) error {
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

	substituteCfg, err := t.RenderBundle(ctx, substitution.Name)
	if err != nil {
		return fmt.Errorf("failed to render bundle image reference %q: %v", substitution.Name, err)
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
