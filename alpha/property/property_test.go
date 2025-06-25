package property

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	type spec struct {
		name      string
		v         Property
		assertion require.ErrorAssertionFunc
	}

	specs := []spec{
		{
			name: "Success/Valid",
			v: Property{
				Type:  "custom.type",
				Value: json.RawMessage("{}"),
			},
			assertion: require.NoError,
		},
		{
			name: "Error/NoType",
			v: Property{
				Value: json.RawMessage(""),
			},
			assertion: require.Error,
		},
		{
			name: "Error/NoValue",
			v: Property{
				Type:  "custom.type",
				Value: nil,
			},
			assertion: require.Error,
		},
		{
			name: "Error/EmptyValue",
			v: Property{
				Type:  "custom.type",
				Value: json.RawMessage{},
			},
			assertion: require.Error,
		},
		{
			name: "Error/ValueNotJSON",
			v: Property{
				Type:  "custom.type",
				Value: json.RawMessage("{"),
			},
			assertion: require.Error,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			err := s.v.Validate()
			s.assertion(t, err)
		})
	}
}

func TestParse(t *testing.T) {
	type spec struct {
		name        string
		input       []Property
		expectProps *Properties
		assertion   assert.ErrorAssertionFunc
	}
	specs := []spec{
		{
			name: "Error/InvalidPackage",
			input: []Property{
				{Type: TypePackage, Value: json.RawMessage(`{`)},
			},
			assertion: assert.Error,
		},
		{
			name: "Error/InvalidPackageRequired",
			input: []Property{
				{Type: TypePackageRequired, Value: json.RawMessage(`{`)},
			},
			assertion: assert.Error,
		},
		{
			name: "Error/InvalidGVK",
			input: []Property{
				{Type: TypeGVK, Value: json.RawMessage(`{`)},
			},
			assertion: assert.Error,
		},
		{
			name: "Error/InvalidGVKRequired",
			input: []Property{
				{Type: TypeGVKRequired, Value: json.RawMessage(`{`)},
			},
			assertion: assert.Error,
		},
		{
			name: "Error/InvalidBundleObject",
			input: []Property{
				{Type: TypeBundleObject, Value: json.RawMessage(`{`)},
			},
			assertion: assert.Error,
		},
		{
			name: "Error/InvalidSearchMetadata",
			input: []Property{
				{Type: TypeSearchMetadata, Value: json.RawMessage(`{`)},
			},
			assertion: assert.Error,
		},
		{
			name: "Error/InvalidOther",
			input: []Property{
				{Type: "otherType1", Value: json.RawMessage(`{`)},
			},
			assertion: assert.Error,
		},
		{
			name: "Success/Valid",
			input: []Property{
				MustBuildPackage("package1", "0.1.0"),
				MustBuildPackage("package2", "0.2.0"),
				MustBuildPackageRequired("package3", ">=1.0.0 <2.0.0-0"),
				MustBuildPackageRequired("package4", ">=2.0.0 <3.0.0-0"),
				MustBuildGVK("group", "v1", "Kind1"),
				MustBuildGVK("group", "v1", "Kind2"),
				MustBuildGVKRequired("other", "v2", "Kind3"),
				MustBuildGVKRequired("other", "v2", "Kind4"),
				MustBuildBundleObject([]byte("testdata2")),
				MustBuildSearchMetadata(SearchMetadata{
					{Name: "Maturity", Type: "String", Value: "Stable"},
					{Name: "Keywords", Type: "ListString", Value: []string{"database", "nosql"}},
					{Name: "Features", Type: "MapStringBoolean", Value: map[string]bool{"Feature1": true, "Feature2": false}},
				}),
				{Type: "otherType1", Value: json.RawMessage(`{"v":"otherValue1"}`)},
				{Type: "otherType2", Value: json.RawMessage(`["otherValue2"]`)},
			},
			expectProps: &Properties{
				Packages: []Package{
					{"package1", "0.1.0"},
					{"package2", "0.2.0"},
				},
				PackagesRequired: []PackageRequired{
					{"package3", ">=1.0.0 <2.0.0-0"},
					{"package4", ">=2.0.0 <3.0.0-0"},
				},
				GVKs: []GVK{
					{"group", "Kind1", "v1"},
					{"group", "Kind2", "v1"},
				},
				GVKsRequired: []GVKRequired{
					{"other", "Kind3", "v2"},
					{"other", "Kind4", "v2"},
				},
				BundleObjects: []BundleObject{
					{Data: []byte("testdata2")},
				},
				SearchMetadatas: []SearchMetadata{
					{
						{Name: "Maturity", Type: "String", Value: "Stable"},
						{Name: "Keywords", Type: "ListString", Value: []interface{}{"database", "nosql"}},
						{Name: "Features", Type: "MapStringBoolean", Value: map[string]interface{}{"Feature1": true, "Feature2": false}},
					},
				},
				Others: []Property{
					{Type: "otherType1", Value: json.RawMessage(`{"v":"otherValue1"}`)},
					{Type: "otherType2", Value: json.RawMessage(`["otherValue2"]`)},
				},
			},
			assertion: assert.NoError,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			actual, err := Parse(s.input)
			s.assertion(t, err)
			assert.Equal(t, s.expectProps, actual)
		})
	}
}

func TestDeduplicate(t *testing.T) {
	type spec struct {
		name        string
		input       []Property
		expectProps []Property
	}
	specs := []spec{
		{
			name: "Identical",
			input: []Property{
				MustBuildPackage("package1", "0.1.0"),
				MustBuildGVK("group", "v1", "Kind"),
				MustBuildGVK("group", "v1", "Kind"),
				MustBuildGVK("group", "v1", "Kind"),
			},
			expectProps: []Property{
				MustBuildPackage("package1", "0.1.0"),
				MustBuildGVK("group", "v1", "Kind"),
			},
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			actual := Deduplicate(s.input)
			assert.Equal(t, s.expectProps, actual)
		})
	}
}

func TestBuild(t *testing.T) {
	type spec struct {
		name             string
		input            interface{}
		assertion        require.ErrorAssertionFunc
		expectedProperty *Property
	}
	specs := []spec{
		{
			name:             "Success/Package",
			input:            &Package{"name", "0.1.0"},
			assertion:        require.NoError,
			expectedProperty: propPtr(MustBuildPackage("name", "0.1.0")),
		},
		{
			name:             "Success/PackageRequired",
			input:            &PackageRequired{"name", ">=0.1.0"},
			assertion:        require.NoError,
			expectedProperty: propPtr(MustBuildPackageRequired("name", ">=0.1.0")),
		},
		{
			name:             "Success/GVK",
			input:            &GVK{"group", "Kind", "v1"},
			assertion:        require.NoError,
			expectedProperty: propPtr(MustBuildGVK("group", "v1", "Kind")),
		},
		{
			name:             "Success/GVKRequired",
			input:            &GVKRequired{"group", "Kind", "v1"},
			assertion:        require.NoError,
			expectedProperty: propPtr(MustBuildGVKRequired("group", "v1", "Kind")),
		},
		{
			name:             "Success/BundleObject",
			input:            &BundleObject{Data: []byte("test")},
			assertion:        require.NoError,
			expectedProperty: propPtr(MustBuildBundleObject([]byte("test"))),
		},
		{
			name: "Success/SearchMetadata",
			input: &SearchMetadata{
				{Name: "Maturity", Type: "String", Value: "Stable"},
				{Name: "Keywords", Type: "ListString", Value: []string{"database", "nosql"}},
				{Name: "Features", Type: "MapStringBoolean", Value: map[string]bool{"Feature1": true, "Feature2": false}},
			},
			assertion: require.NoError,
			expectedProperty: propPtr(MustBuildSearchMetadata(SearchMetadata{
				{Name: "Maturity", Type: "String", Value: "Stable"},
				{Name: "Keywords", Type: "ListString", Value: []string{"database", "nosql"}},
				{Name: "Features", Type: "MapStringBoolean", Value: map[string]bool{"Feature1": true, "Feature2": false}},
			})),
		},
		{
			name:             "Success/Property",
			input:            &Property{Type: "foo", Value: json.RawMessage(`"bar"`)},
			assertion:        require.NoError,
			expectedProperty: &Property{Type: "foo", Value: json.RawMessage(`"bar"`)},
		},
		{
			name:      "Error/InvalidProperty",
			input:     &Property{Type: "foo", Value: json.RawMessage(`{`)},
			assertion: require.Error,
		},
		{
			name:      "Error/NotAPointer",
			input:     Package{},
			assertion: require.Error,
		},
		{
			name:      "Error/NotRegisteredInScheme",
			input:     &struct{}{},
			assertion: require.Error,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			actual, err := Build(s.input)
			s.assertion(t, err)
			assert.Equal(t, s.expectedProperty, actual)
		})
	}
}

func TestMustBuild(t *testing.T) {
	assert.NotPanics(t, func() { MustBuild(&Package{}) })
	assert.Panics(t, func() { MustBuild(Package{}) })
}

func propPtr(in Property) *Property {
	return &in
}

func TestSearchMetadata(t *testing.T) {
	t.Run("Success/AllSupportedTypes", func(t *testing.T) {
		items := []SearchMetadataItem{
			{Name: "Maturity", Type: "String", Value: "Stable"},
			{Name: "Keywords", Type: "ListString", Value: []string{"database", "nosql", "operator"}},
			{Name: "Features", Type: "MapStringBoolean", Value: map[string]bool{"Feature1": true, "Feature2": false, "Feature3": true}},
		}

		prop := MustBuildSearchMetadata(SearchMetadata(items))
		require.Equal(t, TypeSearchMetadata, prop.Type)

		// Parse back and verify
		props, err := Parse([]Property{prop})
		require.NoError(t, err)
		require.Len(t, props.SearchMetadatas, 1)

		searchMetadata := props.SearchMetadatas[0]
		require.Len(t, searchMetadata, 3)

		// Verify each item exists (order might differ due to JSON marshaling)
		itemMap := make(map[string]SearchMetadataItem)
		for _, item := range searchMetadata {
			itemMap[item.Name] = item
		}

		// String type
		maturity, ok := itemMap["Maturity"]
		require.True(t, ok)
		assert.Equal(t, "String", maturity.Type)
		assert.Equal(t, "Stable", maturity.Value)

		// ListString type (JSON unmarshaling converts []string to []interface{})
		keywords, ok := itemMap["Keywords"]
		require.True(t, ok)
		assert.Equal(t, "ListString", keywords.Type)
		keywordsList, ok := keywords.Value.([]interface{})
		require.True(t, ok)
		assert.Len(t, keywordsList, 3)
		assert.Contains(t, keywordsList, "database")
		assert.Contains(t, keywordsList, "nosql")
		assert.Contains(t, keywordsList, "operator")

		// MapStringBoolean type (JSON unmarshaling converts map[string]bool to map[string]interface{})
		features, ok := itemMap["Features"]
		require.True(t, ok)
		assert.Equal(t, "MapStringBoolean", features.Type)
		featuresMap, ok := features.Value.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, true, featuresMap["Feature1"])
		assert.Equal(t, false, featuresMap["Feature2"])
		assert.Equal(t, true, featuresMap["Feature3"])
	})

	t.Run("Success/Build", func(t *testing.T) {
		searchMetadata := SearchMetadata{
			{Name: "TestField", Type: "String", Value: "TestValue"},
		}

		prop, err := Build(&searchMetadata)
		require.NoError(t, err)
		assert.Equal(t, TypeSearchMetadata, prop.Type)

		// Verify it can be parsed back
		props, err := Parse([]Property{*prop})
		require.NoError(t, err)
		require.Len(t, props.SearchMetadatas, 1)
		assert.Equal(t, "TestField", props.SearchMetadatas[0][0].Name)
		assert.Equal(t, "String", props.SearchMetadatas[0][0].Type)
		assert.Equal(t, "TestValue", props.SearchMetadatas[0][0].Value)
	})
}

func TestSearchMetadataValidation(t *testing.T) {
	createSearchMetadataProperty := func(items []SearchMetadataItem) Property {
		return MustBuildSearchMetadata(SearchMetadata(items))
	}

	t.Run("Success/ValidItems", func(t *testing.T) {
		prop := createSearchMetadataProperty([]SearchMetadataItem{
			{Name: "ValidString", Type: "String", Value: "valid"},
			{Name: "ValidListString", Type: "ListString", Value: []string{"item1", "item2"}},
			{Name: "ValidMapStringBoolean", Type: "MapStringBoolean", Value: map[string]bool{"key1": true, "key2": false}},
		})

		_, err := Parse([]Property{prop})
		require.NoError(t, err)
	})

	t.Run("Error/EmptyName", func(t *testing.T) {
		prop := createSearchMetadataProperty([]SearchMetadataItem{
			{Name: "", Type: "String", Value: "valid"},
		})

		_, err := Parse([]Property{prop})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name must be set")
	})

	t.Run("Error/EmptyType", func(t *testing.T) {
		prop := createSearchMetadataProperty([]SearchMetadataItem{
			{Name: "ValidName", Type: "", Value: "valid"},
		})

		_, err := Parse([]Property{prop})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type must be set")
	})

	t.Run("Error/NilStringValue", func(t *testing.T) {
		prop := createSearchMetadataProperty([]SearchMetadataItem{
			{Name: "ValidName", Type: "String", Value: nil},
		})

		_, err := Parse([]Property{prop})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "value must be set")
	})

	t.Run("Error/EmptyString", func(t *testing.T) {
		prop := createSearchMetadataProperty([]SearchMetadataItem{
			{Name: "EmptyString", Type: "String", Value: ""},
		})

		_, err := Parse([]Property{prop})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "string value must have length >= 1")
	})

	t.Run("Error/WrongTypeForString", func(t *testing.T) {
		prop := createSearchMetadataProperty([]SearchMetadataItem{
			{Name: "WrongType", Type: "String", Value: 123},
		})

		_, err := Parse([]Property{prop})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type is 'String' but value is not a string")
	})

	t.Run("Error/EmptyStringInList", func(t *testing.T) {
		prop := createSearchMetadataProperty([]SearchMetadataItem{
			{Name: "EmptyInList", Type: "ListString", Value: []string{"valid", "", "alsovalid"}},
		})

		_, err := Parse([]Property{prop})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ListString item[1] must have length >= 1")
	})

	t.Run("Error/NonStringInList", func(t *testing.T) {
		prop := createSearchMetadataProperty([]SearchMetadataItem{
			{Name: "NonStringInList", Type: "ListString", Value: []interface{}{"valid", 123, "alsovalid"}},
		})

		_, err := Parse([]Property{prop})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ListString item[1] is not a string")
	})

	t.Run("Error/EmptyKeyInMap", func(t *testing.T) {
		prop := createSearchMetadataProperty([]SearchMetadataItem{
			{Name: "EmptyKey", Type: "MapStringBoolean", Value: map[string]bool{"validkey": true, "": false}},
		})

		_, err := Parse([]Property{prop})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MapStringBoolean keys must have length >= 1")
	})

	t.Run("Error/NonBooleanInMap", func(t *testing.T) {
		prop := createSearchMetadataProperty([]SearchMetadataItem{
			{Name: "NonBooleanValue", Type: "MapStringBoolean", Value: map[string]interface{}{"key1": true, "key2": "false"}},
		})

		_, err := Parse([]Property{prop})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MapStringBoolean value for key 'key2' is not a boolean")
	})

	t.Run("Error/UnsupportedType", func(t *testing.T) {
		prop := createSearchMetadataProperty([]SearchMetadataItem{
			{Name: "UnsupportedType", Type: "UnknownType", Value: "value"},
		})

		_, err := Parse([]Property{prop})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported type: UnknownType")
	})

	t.Run("Error/DuplicateNames", func(t *testing.T) {
		prop := createSearchMetadataProperty([]SearchMetadataItem{
			{Name: "DuplicateName", Type: "String", Value: "value1"},
			{Name: "DuplicateName", Type: "ListString", Value: []string{"value2"}},
		})

		_, err := Parse([]Property{prop})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate name 'DuplicateName'")
	})
}

func TestParseOne(t *testing.T) {
	t.Run("Success/Package", func(t *testing.T) {
		prop := MustBuildPackage("test-package", "1.0.0")

		result, err := ParseOne[Package](prop)
		require.NoError(t, err)
		assert.Equal(t, "test-package", result.PackageName)
		assert.Equal(t, "1.0.0", result.Version)
	})

	t.Run("Success/PackageRequired", func(t *testing.T) {
		prop := MustBuildPackageRequired("test-package", ">=1.0.0")

		result, err := ParseOne[PackageRequired](prop)
		require.NoError(t, err)
		assert.Equal(t, "test-package", result.PackageName)
		assert.Equal(t, ">=1.0.0", result.VersionRange)
	})

	t.Run("Success/GVK", func(t *testing.T) {
		prop := MustBuildGVK("test.io", "v1", "TestKind")

		result, err := ParseOne[GVK](prop)
		require.NoError(t, err)
		assert.Equal(t, "test.io", result.Group)
		assert.Equal(t, "v1", result.Version)
		assert.Equal(t, "TestKind", result.Kind)
	})

	t.Run("Success/GVKRequired", func(t *testing.T) {
		prop := MustBuildGVKRequired("test.io", "v1", "TestKind")

		result, err := ParseOne[GVKRequired](prop)
		require.NoError(t, err)
		assert.Equal(t, "test.io", result.Group)
		assert.Equal(t, "v1", result.Version)
		assert.Equal(t, "TestKind", result.Kind)
	})

	t.Run("Success/BundleObject", func(t *testing.T) {
		testData := []byte("test bundle data")
		prop := MustBuildBundleObject(testData)

		result, err := ParseOne[BundleObject](prop)
		require.NoError(t, err)
		assert.Equal(t, testData, result.Data)
	})

	t.Run("Success/SearchMetadata", func(t *testing.T) {
		items := []SearchMetadataItem{
			{Name: "Category", Type: "String", Value: "Database"},
			{Name: "Keywords", Type: "ListString", Value: []string{"sql", "database"}},
			{Name: "Features", Type: "MapStringBoolean", Value: map[string]bool{"high-availability": true, "backup": false}},
		}
		prop := MustBuildSearchMetadata(SearchMetadata(items))

		result, err := ParseOne[SearchMetadata](prop)
		require.NoError(t, err)
		require.Len(t, result, 3)

		// Create a map for easier assertion
		itemMap := make(map[string]SearchMetadataItem)
		for _, item := range result {
			itemMap[item.Name] = item
		}

		category, ok := itemMap["Category"]
		require.True(t, ok)
		assert.Equal(t, "String", category.Type)
		assert.Equal(t, "Database", category.Value)
	})

	t.Run("Error/UnregisteredType", func(t *testing.T) {
		// Create a property with a custom unregistered type
		prop := Property{Type: "custom.unregistered", Value: json.RawMessage(`{}`)}

		type UnregisteredType struct {
			Field string `json:"field"`
		}

		_, err := ParseOne[UnregisteredType](prop)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not registered in the scheme")
	})

	t.Run("Error/TypeMismatch", func(t *testing.T) {
		// Try to parse a Package property as a GVK
		prop := MustBuildPackage("test-package", "1.0.0")

		_, err := ParseOne[GVK](prop)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "property type \"olm.package\" does not match expected type \"olm.gvk\"")
	})

	t.Run("Error/InvalidJSON", func(t *testing.T) {
		// Create a property with invalid JSON
		prop := Property{Type: TypePackage, Value: json.RawMessage(`{invalid json`)}

		_, err := ParseOne[Package](prop)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal property value")
	})

	t.Run("Error/InvalidSearchMetadata", func(t *testing.T) {
		// Create a SearchMetadata property with invalid item
		invalidItems := []SearchMetadataItem{
			{Name: "", Type: "String", Value: "invalid"}, // Empty name should fail validation
		}

		// Build the property manually to bypass MustBuildSearchMetadata validation
		jsonBytes, err := json.Marshal(invalidItems)
		require.NoError(t, err)

		prop := Property{Type: TypeSearchMetadata, Value: json.RawMessage(jsonBytes)}

		_, err = ParseOne[SearchMetadata](prop)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name must be set")
	})
}

func TestSearchMetadataItem_ExtractValue_String(t *testing.T) {
	tests := []struct {
		name    string
		item    SearchMetadataItem
		want    string
		wantErr bool
	}{
		{
			name: "valid string",
			item: SearchMetadataItem{
				Name:  "test",
				Type:  SearchMetadataTypeString,
				Value: "stable",
			},
			want:    "stable",
			wantErr: false,
		},
		{
			name: "not a string",
			item: SearchMetadataItem{
				Name:  "test",
				Type:  SearchMetadataTypeString,
				Value: 123,
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "empty string",
			item: SearchMetadataItem{
				Name:  "test",
				Type:  SearchMetadataTypeString,
				Value: "",
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.item.ExtractValue()
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if str, ok := got.(string); !ok || str != tt.want {
					t.Errorf("ExtractValue() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestSearchMetadataItem_ExtractValue_ListString(t *testing.T) {
	tests := []struct {
		name    string
		item    SearchMetadataItem
		want    []string
		wantErr bool
	}{
		{
			name: "valid string slice",
			item: SearchMetadataItem{
				Name:  "test",
				Type:  SearchMetadataTypeListString,
				Value: []string{"a", "b", "c"},
			},
			want:    []string{"a", "b", "c"},
			wantErr: false,
		},
		{
			name: "valid interface slice",
			item: SearchMetadataItem{
				Name:  "test",
				Type:  SearchMetadataTypeListString,
				Value: []interface{}{"a", "b", "c"},
			},
			want:    []string{"a", "b", "c"},
			wantErr: false,
		},
		{
			name: "empty string in slice",
			item: SearchMetadataItem{
				Name:  "test",
				Type:  SearchMetadataTypeListString,
				Value: []string{"a", "", "c"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "non-string in interface slice",
			item: SearchMetadataItem{
				Name:  "test",
				Type:  SearchMetadataTypeListString,
				Value: []interface{}{"a", 123, "c"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "not a slice",
			item: SearchMetadataItem{
				Name:  "test",
				Type:  SearchMetadataTypeListString,
				Value: "not a slice",
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.item.ExtractValue()
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if slice, ok := got.([]string); !ok || !reflect.DeepEqual(slice, tt.want) {
					t.Errorf("ExtractValue() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestSearchMetadataItem_ExtractValue_MapStringBoolean(t *testing.T) {
	tests := []struct {
		name    string
		item    SearchMetadataItem
		want    map[string]bool
		wantErr bool
	}{
		{
			name: "valid map[string]bool",
			item: SearchMetadataItem{
				Name:  "test",
				Type:  SearchMetadataTypeMapStringBoolean,
				Value: map[string]bool{"a": true, "b": false},
			},
			want:    map[string]bool{"a": true, "b": false},
			wantErr: false,
		},
		{
			name: "valid map[string]interface{}",
			item: SearchMetadataItem{
				Name:  "test",
				Type:  SearchMetadataTypeMapStringBoolean,
				Value: map[string]interface{}{"a": true, "b": false},
			},
			want:    map[string]bool{"a": true, "b": false},
			wantErr: false,
		},
		{
			name: "empty key",
			item: SearchMetadataItem{
				Name:  "test",
				Type:  SearchMetadataTypeMapStringBoolean,
				Value: map[string]bool{"": true, "b": false},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "non-boolean value in interface map",
			item: SearchMetadataItem{
				Name:  "test",
				Type:  SearchMetadataTypeMapStringBoolean,
				Value: map[string]interface{}{"a": true, "b": "not a bool"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "not a map",
			item: SearchMetadataItem{
				Name:  "test",
				Type:  SearchMetadataTypeMapStringBoolean,
				Value: "not a map",
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.item.ExtractValue()
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if m, ok := got.(map[string]bool); !ok || !reflect.DeepEqual(m, tt.want) {
					t.Errorf("ExtractValue() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestSearchMetadataItem_ExtractValue_UnsupportedType(t *testing.T) {
	item := SearchMetadataItem{
		Name:  "test",
		Type:  "unsupported",
		Value: "value",
	}

	_, err := item.ExtractValue()
	if err == nil {
		t.Error("ExtractValue() expected error for unsupported type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported type") {
		t.Errorf("ExtractValue() error = %v, want error containing 'unsupported type'", err)
	}
}
