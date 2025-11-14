package template

import (
	"encoding/json"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/util/yaml"
)

// detectSchema reads the input and extracts the schema field
func detectSchema(reader io.Reader) (string, error) {
	// Read the input into a raw message
	rawDoc := json.RawMessage{}
	decoder := yaml.NewYAMLOrJSONDecoder(reader, 4096)
	err := decoder.Decode(&rawDoc)
	if err != nil {
		return "", fmt.Errorf("decoding template input: %v", err)
	}

	// Parse the raw message to extract schema
	var schemaDoc struct {
		Schema string `json:"schema"`
	}
	err = json.Unmarshal(rawDoc, &schemaDoc)
	if err != nil {
		return "", fmt.Errorf("unmarshalling template schema: %v", err)
	}

	if schemaDoc.Schema == "" {
		return "", fmt.Errorf("template input missing required 'schema' field")
	}

	return schemaDoc.Schema, nil
}
