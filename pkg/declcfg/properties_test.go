package declcfg

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/operator-framework/operator-registry/pkg/property"
)

func TestParseProperties(t *testing.T) {
	type spec struct {
		name          string
		properties    []property.Property
		expectErrType error
		expectProps   *property.Properties
	}

	specs := []spec{
		{
			name: "Error/InvalidChannel",
			properties: []property.Property{
				{Type: property.TypeChannel, Value: json.RawMessage(`""`)},
			},
			expectErrType: property.ParseError{},
		},
		{
			name: "Error/InvalidSkips",
			properties: []property.Property{
				{Type: property.TypeSkips, Value: json.RawMessage(`{}`)},
			},
			expectErrType: property.ParseError{},
		},
		{
			name: "Error/DuplicateChannels",
			properties: []property.Property{
				property.MustBuildChannel("alpha", "foo.v0.0.3"),
				property.MustBuildChannel("beta", "foo.v0.0.3"),
				property.MustBuildChannel("alpha", "foo.v0.0.4"),
			},
			expectErrType: propertyDuplicateError{},
		},
		{
			name: "Success/Valid",
			properties: []property.Property{
				property.MustBuildChannel("alpha", "foo.v0.0.3"),
				property.MustBuildChannel("beta", "foo.v0.0.4"),
				property.MustBuildSkips("foo.v0.0.1"),
				property.MustBuildSkips("foo.v0.0.2"),
			},
			expectProps: &property.Properties{
				Channels: []property.Channel{
					{Name: "alpha", Replaces: "foo.v0.0.3"},
					{Name: "beta", Replaces: "foo.v0.0.4"},
				},
				Skips: []property.Skips{"foo.v0.0.1", "foo.v0.0.2"},
			},
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			props, err := parseProperties(s.properties)
			if s.expectErrType != nil {
				assert.IsType(t, s.expectErrType, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, s.expectProps, props)
			}
		})
	}
}
