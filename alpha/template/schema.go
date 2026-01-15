package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/util/yaml"
)

// detectSchema reads the input, extracts the schema field, and returns a reader
// that includes the consumed data followed by the remaining stream data.
// This works when the input is stdin or a file (since stdin cannot be closed and reopened)
// and complies with the requirement that each supplied schema has a defined "schema" field,
// without attempting to load all input into memory.
func detectSchema(reader io.Reader) (string, io.Reader, error) {
	// Capture what's read during schema detection
	var capturedData bytes.Buffer
	teeReader := io.TeeReader(reader, &capturedData)

	// Read the input into a raw message
	rawDoc := json.RawMessage{}
	decoder := yaml.NewYAMLOrJSONDecoder(teeReader, 4096)
	err := decoder.Decode(&rawDoc)
	if err != nil {
		return "", nil, fmt.Errorf("decoding template input: %v", err)
	}

	// Parse the raw message to extract schema
	var schemaDoc struct {
		Schema string `json:"schema"`
	}
	err = json.Unmarshal(rawDoc, &schemaDoc)
	if err != nil {
		return "", nil, fmt.Errorf("unmarshalling template schema: %v", err)
	}

	if schemaDoc.Schema == "" {
		return "", nil, fmt.Errorf("template input missing required 'schema' field")
	}

	// Create a reader that combines the captured data with the remaining stream
	replayReader := io.MultiReader(&capturedData, reader)

	return schemaDoc.Schema, replayReader, nil
}
