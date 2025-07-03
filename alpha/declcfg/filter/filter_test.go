package filter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func TestNew(t *testing.T) {
	filter := New(All)
	assert.NotNil(t, filter)
	assert.NotNil(t, filter.matchFunc)
	assert.Empty(t, filter.criteria)
}

func TestFilter_HasAny(t *testing.T) {
	filter := New(All).HasAny("test", "value1", "value2")
	require.Len(t, filter.criteria, 1)
	assert.Equal(t, "test", filter.criteria[0].name)
	assert.Equal(t, []string{"value1", "value2"}, filter.criteria[0].values)
}

func TestFilter_HasAll(t *testing.T) {
	filter := New(All).HasAll("test", "value1", "value2")
	require.Len(t, filter.criteria, 1)
	assert.Equal(t, "test", filter.criteria[0].name)
	assert.Equal(t, []string{"value1", "value2"}, filter.criteria[0].values)
}

// Helper function to create a Meta with search metadata
func createMetaWithSearchMetadata(searchMetadata []property.SearchMetadataItem) declcfg.Meta {
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
		Blob: blobBytes,
	}
}

func TestAll(t *testing.T) {
	tests := []struct {
		name     string
		results  []Result
		expected bool
	}{
		{
			name: "all match",
			results: []Result{
				{Name: "test1", Matched: true},
				{Name: "test2", Matched: true},
			},
			expected: true,
		},
		{
			name: "some don't match",
			results: []Result{
				{Name: "test1", Matched: true},
				{Name: "test2", Matched: false},
			},
			expected: false,
		},
		{
			name: "none match",
			results: []Result{
				{Name: "test1", Matched: false},
				{Name: "test2", Matched: false},
			},
			expected: false,
		},
		{
			name:     "empty results",
			results:  []Result{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := All(tt.results)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAny(t *testing.T) {
	tests := []struct {
		name     string
		results  []Result
		expected bool
	}{
		{
			name: "all match",
			results: []Result{
				{Name: "test1", Matched: true},
				{Name: "test2", Matched: true},
			},
			expected: true,
		},
		{
			name: "some match",
			results: []Result{
				{Name: "test1", Matched: true},
				{Name: "test2", Matched: false},
			},
			expected: true,
		},
		{
			name: "none match",
			results: []Result{
				{Name: "test1", Matched: false},
				{Name: "test2", Matched: false},
			},
			expected: false,
		},
		{
			name:     "empty results",
			results:  []Result{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Any(tt.results)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchMeta(t *testing.T) {
	// Create test metas with different search metadata
	meta1 := createMetaWithSearchMetadata([]property.SearchMetadataItem{
		{Name: "maturity", Type: property.SearchMetadataTypeString, Value: "stable"},
		{Name: "keywords", Type: property.SearchMetadataTypeListString, Value: []string{"database", "storage", "backup"}},
		{Name: "features", Type: property.SearchMetadataTypeMapStringBoolean, Value: map[string]bool{"backup": true, "monitoring": false}},
	})

	meta2 := createMetaWithSearchMetadata([]property.SearchMetadataItem{
		{Name: "maturity", Type: property.SearchMetadataTypeString, Value: "alpha"},
		{Name: "keywords", Type: property.SearchMetadataTypeListString, Value: []string{"web", "http"}},
		{Name: "features", Type: property.SearchMetadataTypeMapStringBoolean, Value: map[string]bool{"ssl": true, "compression": false}},
	})

	meta3 := createMetaWithSearchMetadata([]property.SearchMetadataItem{
		{Name: "maturity", Type: property.SearchMetadataTypeString, Value: "stable"},
		{Name: "keywords", Type: property.SearchMetadataTypeListString, Value: []string{"database", "cache"}},
		{Name: "features", Type: property.SearchMetadataTypeMapStringBoolean, Value: map[string]bool{"backup": false, "monitoring": true}},
	})

	tests := []struct {
		name     string
		filter   *Filter
		meta     declcfg.Meta
		expected bool
	}{
		{
			name:     "filter by stable maturity - meta1 matches",
			filter:   New(All).HasAny("maturity", "stable"),
			meta:     meta1,
			expected: true,
		},
		{
			name:     "filter by stable maturity - meta2 doesn't match",
			filter:   New(All).HasAny("maturity", "stable"),
			meta:     meta2,
			expected: false,
		},
		{
			name:     "filter by stable maturity - meta3 matches",
			filter:   New(All).HasAny("maturity", "stable"),
			meta:     meta3,
			expected: true,
		},
		{
			name:     "filter by keywords containing database - meta1 matches",
			filter:   New(All).HasAny("keywords", "database"),
			meta:     meta1,
			expected: true,
		},
		{
			name:     "filter by keywords containing database - meta2 doesn't match",
			filter:   New(All).HasAny("keywords", "database"),
			meta:     meta2,
			expected: false,
		},
		{
			name:     "filter by features having backup key - meta1 matches",
			filter:   New(All).HasAny("features", "backup"),
			meta:     meta1,
			expected: true,
		},
		{
			name:     "filter by features having backup key - meta3 doesn't match",
			filter:   New(All).HasAny("features", "backup"),
			meta:     meta3,
			expected: false,
		},
		{
			name:     "filter by keywords having both database and storage - meta1 matches",
			filter:   New(All).HasAll("keywords", "database", "storage"),
			meta:     meta1,
			expected: true,
		},
		{
			name:     "filter by keywords having both database and storage - meta3 doesn't match",
			filter:   New(All).HasAll("keywords", "database", "storage"),
			meta:     meta3,
			expected: false,
		},
		{
			name: "multiple filters with All - stable AND database - meta1 matches",
			filter: New(All).
				HasAny("maturity", "stable").
				HasAny("keywords", "database"),
			meta:     meta1,
			expected: true,
		},
		{
			name: "multiple filters with Any - alpha OR monitoring key - meta2 matches",
			filter: New(Any).
				HasAny("maturity", "alpha").
				HasAny("features", "monitoring"),
			meta:     meta2,
			expected: true,
		},
		{
			name: "multiple filters with Any - alpha OR monitoring key - meta3 matches",
			filter: New(Any).
				HasAny("maturity", "alpha").
				HasAny("features", "monitoring"),
			meta:     meta3,
			expected: true,
		},
		{
			name:     "no matching criteria",
			filter:   New(All).HasAny("maturity", "beta"),
			meta:     meta1,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.filter.MatchMeta(tt.meta)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchMeta_NoSearchMetadata(t *testing.T) {
	// Create meta without search metadata
	props := []property.Property{
		property.MustBuildPackage("test-package", "1.0.0"),
	}

	type metaBlob struct {
		Properties []property.Property `json:"properties,omitempty"`
	}

	blob := metaBlob{Properties: props}
	blobBytes, _ := json.Marshal(blob)

	meta := declcfg.Meta{
		Blob: blobBytes,
	}

	filter := New(All).HasAny("maturity", "stable")

	result, err := filter.MatchMeta(meta)
	require.NoError(t, err)

	// Should return false since meta has no search metadata
	assert.False(t, result)
}

func TestMatchMeta_EmptyFilter(t *testing.T) {
	meta := createMetaWithSearchMetadata([]property.SearchMetadataItem{
		{Name: "maturity", Type: property.SearchMetadataTypeString, Value: "stable"},
	})

	// Test with empty filter (no criteria)
	filter := New(All)
	result, err := filter.MatchMeta(meta)
	require.NoError(t, err)
	assert.True(t, result) // Empty filter should match all metas
}

func TestChainedFilters(t *testing.T) {
	meta := createMetaWithSearchMetadata([]property.SearchMetadataItem{
		{Name: "maturity", Type: property.SearchMetadataTypeString, Value: "stable"},
		{Name: "keywords", Type: property.SearchMetadataTypeListString, Value: []string{"database", "storage", "backup"}},
		{Name: "features", Type: property.SearchMetadataTypeMapStringBoolean, Value: map[string]bool{"backup": true, "monitoring": false, "scaling": true}},
	})

	// Test method chaining
	filter := New(All).
		HasAny("maturity", "stable", "alpha").
		HasAll("keywords", "database", "storage").
		HasAny("features", "backup", "monitoring")

	result, err := filter.MatchMeta(meta)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestCrossTypeFiltering(t *testing.T) {
	// Test that the same filter name can work with different metadata types
	stringMeta := createMetaWithSearchMetadata([]property.SearchMetadataItem{
		{Name: "category", Type: property.SearchMetadataTypeString, Value: "database"},
	})

	listMeta := createMetaWithSearchMetadata([]property.SearchMetadataItem{
		{Name: "category", Type: property.SearchMetadataTypeListString, Value: []string{"database", "storage"}},
	})

	mapMeta := createMetaWithSearchMetadata([]property.SearchMetadataItem{
		{Name: "category", Type: property.SearchMetadataTypeMapStringBoolean, Value: map[string]bool{"database": true}},
	})

	// Filter should work across all types
	filter := New(All).HasAny("category", "database")

	stringResult, err := filter.MatchMeta(stringMeta)
	require.NoError(t, err)
	assert.True(t, stringResult)

	listResult, err := filter.MatchMeta(listMeta)
	require.NoError(t, err)
	assert.True(t, listResult)

	mapResult, err := filter.MatchMeta(mapMeta)
	require.NoError(t, err)
	assert.True(t, mapResult)
}

func TestCustomMatchFunction(t *testing.T) {
	// Test a custom match function that considers filter names
	customMatchFunc := func(results []Result) bool {
		// Custom logic: require maturity to match, but features are optional
		maturityMatched := false

		for _, result := range results {
			switch result.Name {
			case "maturity":
				maturityMatched = result.Matched
			case "features":
				// Features are optional, we don't need to track this
			}
		}

		// Must have maturity match, features are a bonus but not required
		return maturityMatched
	}

	meta1 := createMetaWithSearchMetadata([]property.SearchMetadataItem{
		{Name: "maturity", Type: property.SearchMetadataTypeString, Value: "stable"},
		{Name: "features", Type: property.SearchMetadataTypeMapStringBoolean, Value: map[string]bool{"backup": true}},
	})

	meta2 := createMetaWithSearchMetadata([]property.SearchMetadataItem{
		{Name: "maturity", Type: property.SearchMetadataTypeString, Value: "alpha"},
		{Name: "features", Type: property.SearchMetadataTypeMapStringBoolean, Value: map[string]bool{"backup": true}},
	})

	meta3 := createMetaWithSearchMetadata([]property.SearchMetadataItem{
		{Name: "maturity", Type: property.SearchMetadataTypeString, Value: "stable"},
		{Name: "features", Type: property.SearchMetadataTypeMapStringBoolean, Value: map[string]bool{"monitoring": true}},
	})

	// Create filter with custom match function
	filter := New(customMatchFunc).
		HasAny("maturity", "stable").
		HasAny("features", "nonexistent") // This won't match, but that's OK with our custom logic

	result1, err := filter.MatchMeta(meta1)
	require.NoError(t, err)
	assert.True(t, result1) // Should match (stable maturity)

	result2, err := filter.MatchMeta(meta2)
	require.NoError(t, err)
	assert.False(t, result2) // Should not match (alpha maturity)

	result3, err := filter.MatchMeta(meta3)
	require.NoError(t, err)
	assert.True(t, result3) // Should match (stable maturity)
}

func TestFilter_MatchMeta_NoMatchFunc(t *testing.T) {
	// Create a filter without a match function - this will cause a panic when matchFunc is called
	filter := &Filter{
		criteria: []criterion{
			{name: "test", values: []string{"value"}, matchFunc: anyValue},
		},
		matchFunc: nil, // Explicitly set to nil
	}

	// Create a meta with search metadata that will match
	searchMetadata := []property.SearchMetadataItem{
		{Name: "test", Type: property.SearchMetadataTypeString, Value: "value"},
	}
	props := []property.Property{
		property.MustBuildPackage("test-package", "1.0.0"),
		property.MustBuildSearchMetadata(searchMetadata),
	}
	blobData := map[string]interface{}{
		"schema":     "olm.bundle",
		"name":       "test-bundle",
		"package":    "test-package",
		"properties": props,
	}
	blob, err := json.Marshal(blobData)
	require.NoError(t, err)

	meta := declcfg.Meta{
		Schema:  "olm.bundle",
		Name:    "test-bundle",
		Package: "test-package",
		Blob:    blob,
	}

	// This should panic because matchFunc is nil
	assert.Panics(t, func() {
		_, _ = filter.MatchMeta(meta)
	})
}
