package property

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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

func TestFile_MarshalJSON(t *testing.T) {
	type spec struct {
		name      string
		file      File
		json      string
		assertion require.ErrorAssertionFunc
	}
	specs := []spec{
		{
			name:      "Success/Ref",
			file:      File{ref: "foo"},
			json:      `{"ref":"foo"}`,
			assertion: require.NoError,
		},
		{
			name:      "Success/Data",
			file:      File{data: []byte("foo")},
			json:      fmt.Sprintf(`{"data":%q}`, base64.StdEncoding.EncodeToString([]byte("foo"))),
			assertion: require.NoError,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			d, err := json.Marshal(s.file)
			s.assertion(t, err)
			assert.Equal(t, s.json, string(d))
		})
	}
}

func TestFile_UnmarshalJSON(t *testing.T) {
	type spec struct {
		name      string
		file      File
		json      string
		assertion require.ErrorAssertionFunc
	}
	specs := []spec{
		{
			name:      "Success/Ref",
			file:      File{ref: "foo"},
			json:      `{"ref":"foo"}`,
			assertion: require.NoError,
		},
		{
			name:      "Success/Data",
			file:      File{data: []byte("foo")},
			json:      fmt.Sprintf(`{"data":%q}`, base64.StdEncoding.EncodeToString([]byte("foo"))),
			assertion: require.NoError,
		},
		{
			name:      "Error/RefAndData",
			json:      fmt.Sprintf(`{"ref":"foo","data":%q}`, base64.StdEncoding.EncodeToString([]byte("bar"))),
			assertion: require.Error,
		},
		{
			name:      "Error/InvalidJSON",
			json:      `["ref","data"]`,
			assertion: require.Error,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			var actual File
			err := json.Unmarshal([]byte(s.json), &actual)
			s.assertion(t, err)
			assert.Equal(t, s.file, actual)
		})
	}
}

func TestFile_IsRef(t *testing.T) {
	assert.True(t, File{ref: "foo"}.IsRef())
	assert.False(t, File{data: []byte("bar")}.IsRef())
}

func TestFile_GetRef(t *testing.T) {
	assert.Equal(t, "foo", File{ref: "foo"}.GetRef())
	assert.Equal(t, "", File{data: []byte("bar")}.GetRef())
}

func TestFile_GetData(t *testing.T) {
	type spec struct {
		name       string
		createFile func(root string) error
		file       File
		assertion  assert.ErrorAssertionFunc
		expectData []byte
	}

	createFile := func(root string) error {
		dir := filepath.Join(root, "tmp")
		if err := os.MkdirAll(dir, 0777); err != nil {
			return err
		}
		return ioutil.WriteFile(filepath.Join(dir, "foo.txt"), []byte("bar"), 0666)
	}

	specs := []spec{
		{
			name:       "Success/NilData",
			file:       File{},
			assertion:  assert.NoError,
			expectData: nil,
		},
		{
			name:       "Success/WithData",
			file:       File{data: []byte("bar")},
			assertion:  assert.NoError,
			expectData: []byte("bar"),
		},
		{
			name:       "Success/WithRef",
			createFile: createFile,
			file:       File{ref: "tmp/foo.txt"},
			assertion:  assert.NoError,
			expectData: []byte("bar"),
		},
		{
			name:      "Error/WithRef/FileDoesNotExist",
			file:      File{ref: "non-existent.txt"},
			assertion: assert.Error,
		},
		{
			name:      "Error/WithRef/RefIsAbsolutePath",
			file:      File{ref: "/etc/hosts"},
			assertion: assert.Error,
		},
		{
			name:      "Error/WithRef/RefIsOutsideRoot",
			file:      File{ref: "../etc/hosts"},
			assertion: assert.Error,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			dir, err := ioutil.TempDir("", "operator-registry-test-file-")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			if s.createFile != nil {
				require.NoError(t, s.createFile(dir))
			}

			data, err := s.file.GetData(os.DirFS(dir), ".")
			s.assertion(t, err)
			assert.Equal(t, s.expectData, data)
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
			name: "Error/InvalidChannel",
			input: []Property{
				{Type: TypeChannel, Value: json.RawMessage(`{`)},
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
			name: "Error/InvalidSkips",
			input: []Property{
				{Type: TypeSkips, Value: json.RawMessage(`{`)},
			},
			assertion: assert.Error,
		},
		{
			name: "Error/InvalidSkipRange",
			input: []Property{
				{Type: TypeSkipRange, Value: json.RawMessage(`{`)},
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
				MustBuildChannel("testChannel1", ""),
				MustBuildChannel("testChannel2", "replaces2"),
				MustBuildGVK("group", "v1", "Kind1"),
				MustBuildGVK("group", "v1", "Kind2"),
				MustBuildGVKRequired("other", "v2", "Kind3"),
				MustBuildGVKRequired("other", "v2", "Kind4"),
				MustBuildSkips("package1.v0.0.1"),
				MustBuildSkips("package2.v0.1.1"),
				MustBuildSkipRange("<0.1.0-0"),
				MustBuildSkipRange("<0.2.0-0"),
				MustBuildBundleObjectRef("testref1"),
				MustBuildBundleObjectData([]byte("testdata2")),
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
				Channels: []Channel{
					{"testChannel1", ""},
					{"testChannel2", "replaces2"},
				},
				GVKs: []GVK{
					{"group", "Kind1", "v1"},
					{"group", "Kind2", "v1"},
				},
				GVKsRequired: []GVKRequired{
					{"other", "Kind3", "v2"},
					{"other", "Kind4", "v2"},
				},
				Skips: []Skips{
					"package1.v0.0.1",
					"package2.v0.1.1",
				},
				SkipRanges: []SkipRange{
					"<0.1.0-0",
					"<0.2.0-0",
				},
				BundleObjects: []BundleObject{
					{File: File{ref: "testref1"}},
					{File: File{data: []byte("testdata2")}},
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
				MustBuildChannel("channel", "replaces"),
				MustBuildChannel("channel", "replaces"),
				MustBuildGVK("group", "v1", "Kind"),
				MustBuildGVK("group", "v1", "Kind"),
				MustBuildGVK("group", "v1", "Kind"),
			},
			expectProps: []Property{
				MustBuildPackage("package1", "0.1.0"),
				MustBuildChannel("channel", "replaces"),
				MustBuildGVK("group", "v1", "Kind"),
			},
		},
		{
			name: "SameTypeDifferentValue",
			input: []Property{
				MustBuildPackage("package1", "0.1.0"),
				MustBuildChannel("channel", "replaces"),
				MustBuildChannel("channel", "replacesDifferent"),
				MustBuildGVK("group", "v1", "Kind"),
				MustBuildGVK("group", "v1", "Kind"),
				MustBuildGVK("group", "v1", "Kind"),
			},
			expectProps: []Property{
				MustBuildPackage("package1", "0.1.0"),
				MustBuildChannel("channel", "replaces"),
				MustBuildChannel("channel", "replacesDifferent"),
				MustBuildGVK("group", "v1", "Kind"),
			},
		},
		{
			name: "SameValueDifferentType",
			input: []Property{
				MustBuildPackage("package1", "0.1.0"),
				MustBuildChannel("channel", "replaces"),
				MustBuildChannel("channel", "replaces"),
				MustBuildGVK("group", "v1", "Kind"),
				MustBuildGVK("group", "v1", "Kind"),
				MustBuildGVK("group", "v1", "Kind"),
				MustBuildSkips("sameValue"),
				MustBuildSkipRange("sameValue"),
			},
			expectProps: []Property{
				MustBuildPackage("package1", "0.1.0"),
				MustBuildChannel("channel", "replaces"),
				MustBuildGVK("group", "v1", "Kind"),
				MustBuildSkips("sameValue"),
				MustBuildSkipRange("sameValue"),
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
			name:             "Success/Channel",
			input:            &Channel{"name", "replaces"},
			assertion:        require.NoError,
			expectedProperty: propPtr(MustBuildChannel("name", "replaces")),
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
			name:             "Success/Skips",
			input:            skipsPtr("test"),
			assertion:        require.NoError,
			expectedProperty: propPtr(MustBuildSkips("test")),
		},
		{
			name:             "Success/SkipRange",
			input:            skipRangePtr("test"),
			assertion:        require.NoError,
			expectedProperty: propPtr(MustBuildSkipRange("test")),
		},
		{
			name:             "Success/BundleObject",
			input:            &BundleObject{File: File{ref: "test"}},
			assertion:        require.NoError,
			expectedProperty: propPtr(MustBuildBundleObjectRef("test")),
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
	assert.NotPanics(t, func() { MustBuild(&Channel{}) })
	assert.Panics(t, func() { MustBuild(Channel{}) })
}

func propPtr(in Property) *Property {
	return &in
}
func skipsPtr(in Skips) *Skips {
	return &in
}
func skipRangePtr(in SkipRange) *SkipRange {
	return &in
}
