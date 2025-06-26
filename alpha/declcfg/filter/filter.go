// Package filter provides functionality for filtering File-Based Catalog metadata
// based on search metadata properties. It supports filtering by string, list, and map
// metadata types with flexible matching criteria and combination logic.
package filter

import (
	"encoding/json"
	"fmt"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

// Result represents the result of evaluating a single filter criterion
type Result struct {
	Name    string // The name of the filter criterion
	Matched bool   // Whether the criterion matched
}

// MatchFunc defines how multiple filter criteria should be combined
type MatchFunc func(results []Result) bool

// All returns true only if all criteria match (AND logic)
func All(results []Result) bool {
	for _, result := range results {
		if !result.Matched {
			return false
		}
	}
	return true
}

// Any returns true if any criteria matches (OR logic)
func Any(results []Result) bool {
	for _, result := range results {
		if result.Matched {
			return true
		}
	}
	return false
}

// ValueMatchFunc defines how values within a single criterion should be matched
type ValueMatchFunc func(metadataValues, filterValues []string) bool

// anyValue returns true if metadata contains any of the filter values.
// This is an internal value matching function used by HasAny criteria.
func anyValue(metadataValues, filterValues []string) bool {
	metadataSet := make(map[string]bool)
	for _, v := range metadataValues {
		metadataSet[v] = true
	}

	for _, filterValue := range filterValues {
		if metadataSet[filterValue] {
			return true
		}
	}
	return false
}

// allValues returns true if metadata contains all of the filter values.
// This is an internal value matching function used by HasAll criteria.
func allValues(metadataValues, filterValues []string) bool {
	metadataSet := make(map[string]bool)
	for _, v := range metadataValues {
		metadataSet[v] = true
	}

	for _, filterValue := range filterValues {
		if !metadataSet[filterValue] {
			return false
		}
	}
	return true
}

// Filter holds the configuration for filtering metadata based on filter criteria.
// It can be used to evaluate whether metadata objects match specified conditions.
type Filter struct {
	// criteria are the individual filter criteria
	criteria []criterion
	// matchFunc determines how multiple filter criteria should be combined
	matchFunc MatchFunc
}

// criterion represents a single filter criterion
type criterion struct {
	name      string
	values    []string
	matchFunc ValueMatchFunc
}

// New creates a new Filter with the specified match function.
// The match function determines how multiple filter criteria are combined (e.g., All, Any).
func New(matchFunc MatchFunc) *Filter {
	return &Filter{
		matchFunc: matchFunc,
	}
}

// HasAny adds a filter criterion that matches if the metadata contains any of the specified values.
// For string metadata, it checks if the value matches any of the provided values.
// For list metadata, it checks if any list element matches any of the provided values.
// For map metadata, it checks if any key with a true value matches any of the provided values.
func (f *Filter) HasAny(name string, values ...string) *Filter {
	f.criteria = append(f.criteria, criterion{
		name:      name,
		values:    values,
		matchFunc: anyValue,
	})
	return f
}

// HasAll adds a filter criterion that matches if the metadata contains all of the specified values.
// For string metadata, it checks if the value matches all of the provided values (typically used with a single value).
// For list metadata, it checks if the list contains all of the provided values.
// For map metadata, it checks if all of the provided values exist as keys with true values.
func (f *Filter) HasAll(name string, values ...string) *Filter {
	f.criteria = append(f.criteria, criterion{
		name:      name,
		values:    values,
		matchFunc: allValues,
	})
	return f
}

// matchSearchMetadata evaluates filter criteria against a single SearchMetadata instance.
// This is an internal helper method used by matchProperties.
func (f *Filter) matchSearchMetadata(searchMetadata property.SearchMetadata) (bool, error) {
	// Create a map of search metadata for quick lookup
	metadataMap := make(map[string]property.SearchMetadataItem)
	for _, item := range searchMetadata {
		metadataMap[item.Name] = item
	}

	// Evaluate each filter criterion
	results := make([]Result, 0, len(f.criteria))
	for _, filter := range f.criteria {
		metadata, exists := metadataMap[filter.name]

		// If the filter criterion is not defined in the search metadata, it doesn't match
		if !exists {
			results = append(results, Result{
				Name:    filter.name,
				Matched: false,
			})
			continue
		}

		criterionMatch, err := applyCriterion(metadata, filter)
		if err != nil {
			return false, err
		}

		results = append(results, Result{
			Name:    filter.name,
			Matched: criterionMatch,
		})
	}

	// Apply the match function to combine all criteria results
	return f.matchFunc(results), nil
}

// matchProperties evaluates whether the given properties match the filter criteria.
// This is an internal method used by MatchMeta.
func (f *Filter) matchProperties(properties []property.Property) (bool, error) {
	// If no filter criteria, everything matches
	if len(f.criteria) == 0 {
		return true, nil
	}

	var searchMetadatas []property.SearchMetadata
	for _, prop := range properties {
		if prop.Type == property.TypeSearchMetadata {
			sm, err := property.ParseOne[property.SearchMetadata](prop)
			if err != nil {
				return false, fmt.Errorf("failed to parse search metadata: %v", err)
			}
			searchMetadatas = append(searchMetadatas, sm)
		}
	}

	// If no search metadata, it doesn't match any filter
	if len(searchMetadatas) == 0 {
		return false, nil
	}

	if len(searchMetadatas) > 1 {
		return false, fmt.Errorf("multiple search metadata properties cannot be defined")
	}

	return f.matchSearchMetadata(searchMetadatas[0])
}

// MatchMeta evaluates whether the given Meta object matches the filter criteria.
// It extracts the properties from the Meta's blob and applies the configured filter criteria.
// Returns true if the metadata matches according to the configured match function, false otherwise.
func (f *Filter) MatchMeta(m declcfg.Meta) (bool, error) {
	// metaBlob represents the structure of a Meta blob for extracting properties
	type propertiesBlob struct {
		Properties []property.Property `json:"properties,omitempty"`
	}

	// Parse the blob to extract properties
	var blob propertiesBlob
	if err := json.Unmarshal(m.Blob, &blob); err != nil {
		return false, fmt.Errorf("failed to unmarshal meta blob: %v", err)
	}

	return f.matchProperties(blob.Properties)
}

// applyCriterion applies the filter criterion to the metadata based on the metadata's type.
// This is an internal helper function that handles the type-specific logic for matching.
func applyCriterion(metadata property.SearchMetadataItem, filter criterion) (bool, error) {
	metadataValue, err := metadata.ExtractValue()
	if err != nil {
		return false, err
	}
	values, err := metadataValueAsSlice(metadataValue)
	if err != nil {
		return false, err
	}
	return filter.matchFunc(values, filter.values), nil
}

// metadataValueAsSlice converts metadata values to a string slice for uniform processing.
// This is an internal helper function that normalizes different metadata types.
func metadataValueAsSlice(metadataValue any) ([]string, error) {
	switch v := metadataValue.(type) {
	case string:
		return []string{v}, nil
	case []string:
		return v, nil
	case map[string]bool:
		var keys []string
		for key, value := range v {
			if value {
				keys = append(keys, key)
			}
		}
		return keys, nil
	default:
		return nil, fmt.Errorf("unsupported metadata value type: %T", metadataValue)
	}
}
