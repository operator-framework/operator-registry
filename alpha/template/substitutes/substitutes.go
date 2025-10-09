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
	Name string `json:"name"`
	Base string `json:"base"`
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

	// TODO: Implement the actual rendering logic using st.Entries and st.Substitutes
	_ = st
	return nil, nil
}
