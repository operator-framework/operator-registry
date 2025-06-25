package main

import (
	"encoding/json"
	"fmt"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/declcfg/filter"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func main() {
	// Create filter metadata property
	searchMetadata := property.SearchMetadata{
		{Name: "Maturity", Type: "String", Value: "stable"},
		{Name: "Keywords", Type: "ListString", Value: []string{"database", "storage", "sql"}},
		{Name: "Features", Type: "MapStringBoolean", Value: map[string]bool{"backup": true, "monitoring": false}},
	}

	// Create a Meta object with properties
	properties := []property.Property{
		property.MustBuildSearchMetadata(searchMetadata),
		// ... other properties like packages, GVKs, etc.
	}

	// Create the Meta blob
	metaBlob := struct {
		Properties []property.Property `json:"properties,omitempty"`
	}{
		Properties: properties,
	}

	blobBytes, _ := json.Marshal(metaBlob)

	meta := declcfg.Meta{
		Schema: "olm.bundle",
		Name:   "my-operator.v1.0.0",
		Blob:   json.RawMessage(blobBytes),
	}

	// Create and use filter
	f := filter.New(filter.All). // All individual filter criteria must match
					HasAny("Maturity", "stable", "alpha"). // The "Maturity" filter must match "stable" or "alpha"
					HasAll("Keywords", "database", "sql")  // The "Keywords" filter must match "database" and "sql"

	matches, err := f.MatchMeta(meta)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Meta matches filter: %t\n", matches) // Output: Meta matches filter: true
}
