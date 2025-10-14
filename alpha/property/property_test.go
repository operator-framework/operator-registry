package property

import (
	"encoding/json"
	"testing"

	"github.com/blang/semver/v4"
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
				{Type: "otherType1", Value: json.RawMessage(`{"v":"otherValue1"}`)},
				{Type: "otherType2", Value: json.RawMessage(`["otherValue2"]`)},
			},
			expectProps: &Properties{
				Packages: []Package{
					{PackageName: "package1", Version: "0.1.0"},
					{PackageName: "package2", Version: "0.2.0"},
				},
				PackagesRequired: []PackageRequired{
					{PackageName: "package3", VersionRange: ">=1.0.0 <2.0.0-0"},
					{PackageName: "package4", VersionRange: ">=2.0.0 <3.0.0-0"},
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
			input:            &Package{PackageName: "name", Version: "0.1.0"},
			assertion:        require.NoError,
			expectedProperty: propPtr(MustBuildPackage("name", "0.1.0")),
		},
		{
			name:             "Success/Package-ReleaseVersion",
			input:            &Package{PackageName: "name", Version: "0.1.0", Release: &Release{Label: "alpha-whatsit", Version: semver.MustParse("1.1.0-bluefoot")}},
			assertion:        require.NoError,
			expectedProperty: propPtr(MustBuildPackageRelease("name", "0.1.0", "alpha-whatsit", "1.1.0-bluefoot")),
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
