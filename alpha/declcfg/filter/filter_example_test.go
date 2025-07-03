package filter_test

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/declcfg/filter"
	"github.com/operator-framework/operator-registry/alpha/property"
)

// Helper function to create a Meta with search metadata
func createMetaWithSearchMetadata(name string, searchMetadata []property.SearchMetadataItem) declcfg.Meta {
	props := []property.Property{
		property.MustBuildPackage("test-package", "1.0.0"),
		property.MustBuildSearchMetadata(searchMetadata),
	}

	type metaBlob struct {
		Properties []property.Property `json:"properties,omitempty"`
	}

	blob := metaBlob{Properties: props}
	blobBytes, _ := json.Marshal(blob)

	return declcfg.Meta{
		Name: name,
		Blob: blobBytes,
	}
}

func ExampleFilter_MatchMeta() {
	// Create some sample metas with search metadata
	meta1 := createMetaWithSearchMetadata("database-operator.v1.0.0", []property.SearchMetadataItem{
		{Name: "maturity", Type: property.SearchMetadataTypeString, Value: "stable"},
		{Name: "keywords", Type: property.SearchMetadataTypeListString, Value: []string{"database", "storage", "backup"}},
		{Name: "features", Type: property.SearchMetadataTypeMapStringBoolean, Value: map[string]bool{"backup": true, "monitoring": true}},
	})

	meta2 := createMetaWithSearchMetadata("web-server.v2.0.0", []property.SearchMetadataItem{
		{Name: "maturity", Type: property.SearchMetadataTypeString, Value: "alpha"},
		{Name: "keywords", Type: property.SearchMetadataTypeListString, Value: []string{"web", "http", "server"}},
		{Name: "features", Type: property.SearchMetadataTypeMapStringBoolean, Value: map[string]bool{"ssl": true, "compression": false}},
	})

	// Example 1: Filter by maturity level
	stableFilter := filter.New(filter.All).HasAny("maturity", "stable")

	matches1, err := stableFilter.MatchMeta(meta1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Meta1 matches stable filter: %t\n", matches1)

	matches2, err := stableFilter.MatchMeta(meta2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Meta2 matches stable filter: %t\n", matches2)

	// Example 2: Filter by keywords
	databaseFilter := filter.New(filter.All).HasAny("keywords", "database")

	dbMatches1, err := databaseFilter.MatchMeta(meta1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Meta1 matches database filter: %t\n", dbMatches1)

	dbMatches2, err := databaseFilter.MatchMeta(meta2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Meta2 matches database filter: %t\n", dbMatches2)

	// Example 3: Filter by features
	backupFeatureFilter := filter.New(filter.All).HasAny("features", "backup")

	backupMatches1, err := backupFeatureFilter.MatchMeta(meta1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Meta1 matches backup feature filter: %t\n", backupMatches1)

	backupMatches2, err := backupFeatureFilter.MatchMeta(meta2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Meta2 matches backup feature filter: %t\n", backupMatches2)

	// Example 4: Combined filters (All - must satisfy all conditions)
	combinedFilter := filter.New(filter.All).
		HasAny("maturity", "stable").
		HasAny("keywords", "database")

	combinedMatches1, err := combinedFilter.MatchMeta(meta1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Meta1 matches combined filter: %t\n", combinedMatches1)

	combinedMatches2, err := combinedFilter.MatchMeta(meta2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Meta2 matches combined filter: %t\n", combinedMatches2)

	// Example 5: Alternative filters (Any - satisfy any condition)
	anyFilter := filter.New(filter.Any).
		HasAny("maturity", "stable").
		HasAny("keywords", "web")

	anyMatches1, err := anyFilter.MatchMeta(meta1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Meta1 matches any filter: %t\n", anyMatches1)

	anyMatches2, err := anyFilter.MatchMeta(meta2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Meta2 matches any filter: %t\n", anyMatches2)

	// Output:
	// Meta1 matches stable filter: true
	// Meta2 matches stable filter: false
	// Meta1 matches database filter: true
	// Meta2 matches database filter: false
	// Meta1 matches backup feature filter: true
	// Meta2 matches backup feature filter: false
	// Meta1 matches combined filter: true
	// Meta2 matches combined filter: false
	// Meta1 matches any filter: true
	// Meta2 matches any filter: true
}

func ExampleFilter_MatchMeta_customMatchFunc() {
	// Create a meta with multiple search metadata
	meta := createMetaWithSearchMetadata("complex-operator.v1.0.0", []property.SearchMetadataItem{
		{Name: "maturity", Type: property.SearchMetadataTypeString, Value: "stable"},
		{Name: "keywords", Type: property.SearchMetadataTypeListString, Value: []string{"database", "storage"}},
		{Name: "features", Type: property.SearchMetadataTypeMapStringBoolean, Value: map[string]bool{"backup": false, "monitoring": true}},
	})

	// Custom match function that requires maturity to be stable but features are optional
	customMatchFunc := func(results []filter.Result) bool {
		maturityMatched := false
		for _, result := range results {
			if result.Name == "maturity" {
				maturityMatched = result.Matched
			}
		}
		return maturityMatched
	}

	// Create filter with custom logic
	f := filter.New(customMatchFunc).
		HasAny("maturity", "stable").
		HasAny("features", "nonexistent") // This won't match, but that's OK with our custom logic

	matches, err := f.MatchMeta(meta)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Custom filter matches: %t\n", matches)

	// Output:
	// Custom filter matches: true
}
