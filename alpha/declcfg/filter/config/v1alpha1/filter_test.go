package v1alpha1

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func TestFilter_KeepMeta(t *testing.T) {
	tests := []struct {
		name     string
		filter   declcfg.MetaFilter
		meta     *declcfg.Meta
		expected bool
	}{
		{
			name:     "NoFilter_Package",
			filter:   NewFilter(FilterConfiguration{}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaPackage, Name: "foo"},
			expected: false,
		},
		{
			name:     "NoFilter_Channel",
			filter:   NewFilter(FilterConfiguration{}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaChannel, Package: "foo"},
			expected: false,
		},
		{
			name:     "NoFilter_Bundle",
			filter:   NewFilter(FilterConfiguration{}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaBundle, Package: "foo"},
			expected: false,
		},
		{
			name:     "NoFilter_Deprecation",
			filter:   NewFilter(FilterConfiguration{}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaDeprecation, Package: "foo"},
			expected: false,
		},
		{
			name:     "NoFilter_Other",
			filter:   NewFilter(FilterConfiguration{}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: "other", Package: "foo"},
			expected: false,
		},
		{
			name:     "KeepFooBar_Package",
			filter:   NewFilter(FilterConfiguration{Packages: []Package{{Name: "foo"}, {Name: "bar"}}}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaPackage, Name: "foo"},
			expected: true,
		},
		{
			name:     "KeepFooBar_Channel",
			filter:   NewFilter(FilterConfiguration{Packages: []Package{{Name: "foo"}, {Name: "bar"}}}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaChannel, Package: "foo"},
			expected: true,
		},
		{
			name:     "KeepFooBar_Bundle",
			filter:   NewFilter(FilterConfiguration{Packages: []Package{{Name: "foo"}, {Name: "bar"}}}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaBundle, Package: "foo"},
			expected: true,
		},
		{
			name:     "KeepFooBar_Deprecation",
			filter:   NewFilter(FilterConfiguration{Packages: []Package{{Name: "foo"}, {Name: "bar"}}}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaDeprecation, Package: "foo"},
			expected: true,
		},
		{
			name:     "KeepFooBar_Other",
			filter:   NewFilter(FilterConfiguration{Packages: []Package{{Name: "foo"}, {Name: "bar"}}}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: "other", Package: "foo"},
			expected: true,
		},
		{
			name:     "KeepBarBaz_Package",
			filter:   NewFilter(FilterConfiguration{Packages: []Package{{Name: "bar"}, {Name: "baz"}}}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaPackage, Name: "foo"},
			expected: false,
		},
		{
			name:     "KeepBarBaz_Channel",
			filter:   NewFilter(FilterConfiguration{Packages: []Package{{Name: "bar"}, {Name: "baz"}}}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaChannel, Package: "foo"},
			expected: false,
		},
		{
			name:     "KeepBarBaz_Bundle",
			filter:   NewFilter(FilterConfiguration{Packages: []Package{{Name: "bar"}, {Name: "baz"}}}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaBundle, Package: "foo"},
			expected: false,
		},
		{
			name:     "KeepBarBaz_Deprecation",
			filter:   NewFilter(FilterConfiguration{Packages: []Package{{Name: "bar"}, {Name: "baz"}}}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaDeprecation, Package: "foo"},
			expected: false,
		},
		{
			name:     "KeepBarBaz_Other",
			filter:   NewFilter(FilterConfiguration{Packages: []Package{{Name: "bar"}, {Name: "baz"}}}).(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: "other", Package: "foo"},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.filter.KeepMeta(tt.meta)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestFilter_FilterCatalog(t *testing.T) {
	type testCase struct {
		name      string
		config    FilterConfiguration
		in        *declcfg.DeclarativeConfig
		assertion func(*testing.T, *declcfg.DeclarativeConfig, error)
	}
	testCases := []testCase{
		{
			name:   "empty config, nil fbc",
			config: FilterConfiguration{},
			in:     nil,
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Nil(t, actual)
				assert.NoError(t, err)
			},
		},
		{
			name:   "empty config, empty fbc",
			config: FilterConfiguration{},
			in:     &declcfg.DeclarativeConfig{},
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Equal(t, &declcfg.DeclarativeConfig{}, actual)
				assert.NoError(t, err)
			},
		},
		{
			name:   "empty config",
			config: FilterConfiguration{},
			in: &declcfg.DeclarativeConfig{
				Packages:     []declcfg.Package{{Name: "pkg1"}, {Name: "pkg2"}},
				Channels:     []declcfg.Channel{{Name: "ch", Package: "pkg1"}, {Name: "ch", Package: "pkg2"}},
				Bundles:      []declcfg.Bundle{{Name: "b", Package: "pkg1"}, {Name: "b", Package: "pkg2"}},
				Deprecations: []declcfg.Deprecation{{Package: "pkg1"}, {Package: "pkg2"}},
				Others:       []declcfg.Meta{{Name: "global"}},
			},
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Equal(t, &declcfg.DeclarativeConfig{
					Packages:     []declcfg.Package{},
					Channels:     []declcfg.Channel{},
					Bundles:      []declcfg.Bundle{},
					Deprecations: []declcfg.Deprecation{},
					Others:       []declcfg.Meta{{Name: "global"}},
				}, actual)
				assert.NoError(t, err)
			},
		},
		{
			name:   "keep one package",
			config: FilterConfiguration{Packages: []Package{{Name: "pkg1"}}},
			in: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{{Name: "pkg1"}, {Name: "pkg2"}, {Name: "pkg3"}},
				Channels: []declcfg.Channel{
					{Name: "ch1", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b1"}}},
					{Name: "ch2", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b2"}}},
					{Name: "ch", Package: "pkg2"},
					{Name: "ch", Package: "pkg3"},
				},
				Bundles: []declcfg.Bundle{
					{Name: "b1", Package: "pkg1", Properties: propertiesForBundle("pkg1", "1.0.0")},
					{Name: "b2", Package: "pkg1", Properties: propertiesForBundle("pkg1", "2.0.0")},
					{Name: "b3", Package: "pkg3", Properties: propertiesForBundle("pkg3", "3.0.0")},
				},
				Deprecations: []declcfg.Deprecation{{Package: "pkg1"}, {Package: "pkg2"}, {Package: "pkg3"}},
				Others:       []declcfg.Meta{{Name: "global"}},
			},
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Equal(t, &declcfg.DeclarativeConfig{
					Packages: []declcfg.Package{{Name: "pkg1"}},
					Channels: []declcfg.Channel{
						{Name: "ch1", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b1"}}},
						{Name: "ch2", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b2"}}},
					},
					Bundles: []declcfg.Bundle{
						{Name: "b1", Package: "pkg1", Properties: propertiesForBundle("pkg1", "1.0.0")},
						{Name: "b2", Package: "pkg1", Properties: propertiesForBundle("pkg1", "2.0.0")},
					},
					Deprecations: []declcfg.Deprecation{{Package: "pkg1"}},
					Others:       []declcfg.Meta{{Name: "global"}},
				}, actual)
				assert.NoError(t, err)
			},
		},
		{
			name:   "keep two packages",
			config: FilterConfiguration{Packages: []Package{{Name: "pkg1"}, {Name: "pkg3"}}},
			in: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{{Name: "pkg1"}, {Name: "pkg2"}, {Name: "pkg3"}},
				Channels: []declcfg.Channel{
					{Name: "ch", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b1"}}},
					{Name: "ch", Package: "pkg2"},
					{Name: "ch", Package: "pkg3", Entries: []declcfg.ChannelEntry{{Name: "b3"}}},
				},
				Bundles: []declcfg.Bundle{
					{Name: "b1", Package: "pkg1", Properties: propertiesForBundle("pkg1", "1.0.0")},
					{Name: "b2", Package: "pkg2", Properties: propertiesForBundle("pkg2", "2.0.0")},
					{Name: "b3", Package: "pkg3", Properties: propertiesForBundle("pkg3", "3.0.0")},
				},
				Deprecations: []declcfg.Deprecation{{Package: "pkg1"}, {Package: "pkg2"}, {Package: "pkg3"}},
				Others:       []declcfg.Meta{{Name: "global"}},
			},
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Equal(t, &declcfg.DeclarativeConfig{
					Packages: []declcfg.Package{{Name: "pkg1"}, {Name: "pkg3"}},
					Channels: []declcfg.Channel{
						{Name: "ch", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b1"}}},
						{Name: "ch", Package: "pkg3", Entries: []declcfg.ChannelEntry{{Name: "b3"}}},
					},
					Bundles: []declcfg.Bundle{
						{Name: "b1", Package: "pkg1", Properties: propertiesForBundle("pkg1", "1.0.0")},
						{Name: "b3", Package: "pkg3", Properties: propertiesForBundle("pkg3", "3.0.0")},
					},
					Deprecations: []declcfg.Deprecation{{Package: "pkg1"}, {Package: "pkg3"}},
					Others:       []declcfg.Meta{{Name: "global"}},
				}, actual)
				assert.NoError(t, err)
			},
		},
		{
			name:   "keep one package, one channel",
			config: FilterConfiguration{Packages: []Package{{Name: "pkg1", Channels: []Channel{{Name: "ch1"}}}}},
			in: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{{Name: "pkg1"}, {Name: "pkg2"}},
				Channels: []declcfg.Channel{
					{Name: "ch1", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b1"}}},
					{Name: "ch2", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b2"}}}},
				Bundles: []declcfg.Bundle{
					{Name: "b1", Package: "pkg1", Properties: propertiesForBundle("pkg1", "1.0.0")},
					{Name: "b2", Package: "pkg1", Properties: propertiesForBundle("pkg1", "2.0.0")},
				},
				Deprecations: []declcfg.Deprecation{{Package: "pkg1"}, {Package: "pkg2"}, {Package: "pkg3"}},
				Others:       []declcfg.Meta{{Name: "global"}},
			},
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Equal(t, &declcfg.DeclarativeConfig{
					Packages: []declcfg.Package{{Name: "pkg1"}},
					Channels: []declcfg.Channel{
						{Name: "ch1", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b1"}}},
					},
					Bundles: []declcfg.Bundle{
						{Name: "b1", Package: "pkg1", Properties: propertiesForBundle("pkg1", "1.0.0")},
					},
					Deprecations: []declcfg.Deprecation{{Package: "pkg1"}},
					Others:       []declcfg.Meta{{Name: "global"}},
				}, actual)
				assert.NoError(t, err)
			},
		},
		{
			name: "keep one package, one full channel, one version filtered channel",
			config: FilterConfiguration{Packages: []Package{
				{
					Name: "pkg1",
					Channels: []Channel{
						{Name: "ch1"},
						{Name: "ch2", VersionRange: ">=4.0.0 <8.0.0"},
					},
				}}},
			in: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{{Name: "pkg1"}, {Name: "pkg2"}},
				Channels: []declcfg.Channel{
					{Name: "ch1", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b2", Replaces: "b1", Skips: []string{"b0"}}, {Name: "b1"}}},
					{Name: "ch2", Package: "pkg1", Entries: []declcfg.ChannelEntry{
						{Name: "b10", Replaces: "b9"},
						{Name: "b9", Replaces: "b8"},
						{Name: "b8", Replaces: "b6", Skips: []string{"b7"}},
						{Name: "b7"},
						{Name: "b6", Replaces: "b5"},
						{Name: "b5", Replaces: "b4"},
						{Name: "b4", Replaces: "b3"},
						{Name: "b3"},
					}},
					{Name: "ch3", Package: "pkg2", Entries: []declcfg.ChannelEntry{{Name: "b12", Replaces: "b11"}, {Name: "b11"}}},
				},
				Bundles: []declcfg.Bundle{
					// Pkg1 bundles
					{Name: "b1", Package: "pkg1", Properties: propertiesForBundle("pkg1", "0.1.0")},
					{Name: "b2", Package: "pkg1", Properties: propertiesForBundle("pkg1", "0.2.0")},
					{Name: "b3", Package: "pkg1", Properties: propertiesForBundle("pkg1", "3.0.0")},
					{Name: "b4", Package: "pkg1", Properties: propertiesForBundle("pkg1", "4.0.0")},
					{Name: "b5", Package: "pkg1", Properties: propertiesForBundle("pkg1", "5.0.0")},
					{Name: "b6", Package: "pkg1", Properties: propertiesForBundle("pkg1", "6.0.0")},
					{Name: "b7", Package: "pkg1", Properties: propertiesForBundle("pkg1", "7.0.0")},
					{Name: "b8", Package: "pkg1", Properties: propertiesForBundle("pkg1", "8.0.0")},
					{Name: "b9", Package: "pkg1", Properties: propertiesForBundle("pkg1", "9.0.0")},
					{Name: "b10", Package: "pkg1", Properties: propertiesForBundle("pkg1", "10.0.0")},
				},
				Deprecations: []declcfg.Deprecation{{Package: "pkg1"}, {Package: "pkg2"}, {Package: "pkg3"}},
				Others:       []declcfg.Meta{{Name: "global"}},
			},
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Equal(t, &declcfg.DeclarativeConfig{
					Packages: []declcfg.Package{{Name: "pkg1"}},
					Channels: []declcfg.Channel{
						{Name: "ch1", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b2", Replaces: "b1", Skips: []string{"b0"}}, {Name: "b1"}}},
						{Name: "ch2", Package: "pkg1", Entries: []declcfg.ChannelEntry{
							{Name: "b8", Replaces: "b6", Skips: []string{"b7"}},
							{Name: "b7"},
							{Name: "b6", Replaces: "b5"},
							{Name: "b5", Replaces: "b4"},
							{Name: "b4", Replaces: "b3"},
						}},
					},
					Bundles: []declcfg.Bundle{
						// Pkg1 bundles
						{Name: "b1", Package: "pkg1", Properties: propertiesForBundle("pkg1", "0.1.0")},
						{Name: "b2", Package: "pkg1", Properties: propertiesForBundle("pkg1", "0.2.0")},
						{Name: "b4", Package: "pkg1", Properties: propertiesForBundle("pkg1", "4.0.0")},
						{Name: "b5", Package: "pkg1", Properties: propertiesForBundle("pkg1", "5.0.0")},
						{Name: "b6", Package: "pkg1", Properties: propertiesForBundle("pkg1", "6.0.0")},
						{Name: "b7", Package: "pkg1", Properties: propertiesForBundle("pkg1", "7.0.0")},
						{Name: "b8", Package: "pkg1", Properties: propertiesForBundle("pkg1", "8.0.0")},
					},
					Deprecations: []declcfg.Deprecation{{Package: "pkg1"}},
					Others:       []declcfg.Meta{{Name: "global"}},
				}, actual)
				assert.NoError(t, err)
			},
		},
		{
			name: "invalid version range",
			config: FilterConfiguration{Packages: []Package{
				{Name: "pkg1", Channels: []Channel{{Name: "ch1", VersionRange: "something-isnt-right"}}},
			}},
			in: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{{Name: "pkg1"}},
				Channels: []declcfg.Channel{{Name: "ch1", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b1"}}}},
				Bundles:  []declcfg.Bundle{{Name: "b1", Package: "pkg1", Properties: propertiesForBundle("pkg1", "1.0.0")}},
			},
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, "error parsing version range")
			},
		},
		{
			name: "invalid fbc channel",
			config: FilterConfiguration{Packages: []Package{
				{Name: "pkg1", Channels: []Channel{{Name: "ch1", VersionRange: ">=1.0.0 <2.0.0"}}},
			}},
			in: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{{Name: "pkg1"}},
				Channels: []declcfg.Channel{{Name: "ch1", Package: "pkg1", Entries: []declcfg.ChannelEntry{
					{Name: "b1", Replaces: "b0"},
					{Name: "b0", Replaces: "b1"},
				}}},
			},
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, "no channel heads found")
			},
		},
		{
			name: "range excludes all channel entries",
			config: FilterConfiguration{Packages: []Package{
				{Name: "pkg1", Channels: []Channel{{Name: "ch1", VersionRange: ">100.0.0"}}},
			}},
			in: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{{Name: "pkg1"}},
				Channels: []declcfg.Channel{{Name: "ch1", Package: "pkg1", Entries: []declcfg.ChannelEntry{
					{Name: "b1", Replaces: "b0"},
					{Name: "b0"},
				}}},
			},
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, "empty channel")
			},
		},
		{
			name: "FBC default channel specified, configuration default channel unspecified, channel remains",
			config: FilterConfiguration{Packages: []Package{
				{Name: "pkg1"},
			}},
			in: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{{Name: "pkg1", DefaultChannel: "ch1"}},
				Channels: []declcfg.Channel{
					{Name: "ch1", Package: "pkg1"},
					{Name: "ch2", Package: "pkg1"},
				},
			},
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Equal(t, &declcfg.DeclarativeConfig{
					Packages: []declcfg.Package{{Name: "pkg1", DefaultChannel: "ch1"}},
					Channels: []declcfg.Channel{
						{Name: "ch1", Package: "pkg1"},
						{Name: "ch2", Package: "pkg1"},
					},
				}, actual)
				assert.NoError(t, err)
			},
		},
		{
			name: "FBC default channel specified, configuration default channel unspecified, channel removed",
			config: FilterConfiguration{Packages: []Package{
				{Name: "pkg1", Channels: []Channel{{Name: "ch2"}}},
			}},
			in: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{{Name: "pkg1", DefaultChannel: "ch1"}},
				Channels: []declcfg.Channel{
					{Name: "ch1", Package: "pkg1"},
					{Name: "ch2", Package: "pkg1"},
				},
			},
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, `invalid default channel configuration for package "pkg1": the default channel "ch1" was filtered out, a new default channel must be configured for this package`)
			},
		},
		{
			name: "Configuration default channel specified, channel remains",
			config: FilterConfiguration{Packages: []Package{
				{Name: "pkg1", DefaultChannel: "ch2", Channels: []Channel{{Name: "ch2"}}},
			}},
			in: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{{Name: "pkg1", DefaultChannel: "ch1"}},
				Channels: []declcfg.Channel{
					{Name: "ch1", Package: "pkg1"},
					{Name: "ch2", Package: "pkg1"},
				},
			},
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Equal(t, &declcfg.DeclarativeConfig{
					Packages: []declcfg.Package{{Name: "pkg1", DefaultChannel: "ch2"}},
					Channels: []declcfg.Channel{
						{Name: "ch2", Package: "pkg1"},
					},
				}, actual)
				assert.NoError(t, err)
			},
		},
		{
			name: "Configuration default channel specified, channel removed",
			config: FilterConfiguration{Packages: []Package{
				{Name: "pkg1", DefaultChannel: "ch2", Channels: []Channel{{Name: "ch1"}}},
			}},
			in: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{{Name: "pkg1", DefaultChannel: "ch1"}},
				Channels: []declcfg.Channel{
					{Name: "ch1", Package: "pkg1"},
					{Name: "ch2", Package: "pkg1"},
				},
			},
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, `invalid default channel configuration for package "pkg1": specified default channel override "ch2" does not exist in the filtered output`)
			},
		},
		{
			name: "deprecation entries are filtered",
			config: FilterConfiguration{Packages: []Package{{
				Name:     "pkg1",
				Channels: []Channel{{Name: "ch1"}},
			}}},
			in: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{{Name: "pkg1"}},
				Channels: []declcfg.Channel{
					{Name: "ch1", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b2", Replaces: "b1", Skips: []string{"b0"}}, {Name: "b1"}}},
					{Name: "ch2", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b5", Replaces: "b4", Skips: []string{"b3"}}, {Name: "b4"}}},
				},
				Bundles: []declcfg.Bundle{
					// Pkg1 bundles
					{Name: "b1", Package: "pkg1", Properties: propertiesForBundle("pkg1", "0.1.0")},
					{Name: "b2", Package: "pkg1", Properties: propertiesForBundle("pkg1", "0.2.0")},
					{Name: "b3", Package: "pkg1", Properties: propertiesForBundle("pkg1", "3.0.0")},
					{Name: "b4", Package: "pkg1", Properties: propertiesForBundle("pkg1", "4.0.0")},
					{Name: "b5", Package: "pkg1", Properties: propertiesForBundle("pkg1", "5.0.0")},
				},
				Deprecations: []declcfg.Deprecation{{
					Package: "pkg1",
					Entries: []declcfg.DeprecationEntry{
						{Reference: declcfg.PackageScopedReference{Schema: declcfg.SchemaPackage}},
						{Reference: declcfg.PackageScopedReference{Schema: declcfg.SchemaChannel, Name: "ch1"}},
						{Reference: declcfg.PackageScopedReference{Schema: declcfg.SchemaChannel, Name: "ch2"}},
						{Reference: declcfg.PackageScopedReference{Schema: declcfg.SchemaBundle, Name: "b1"}},
						{Reference: declcfg.PackageScopedReference{Schema: declcfg.SchemaBundle, Name: "b4"}},
					},
				}},
				Others: []declcfg.Meta{{Name: "global"}},
			},
			assertion: func(t *testing.T, actual *declcfg.DeclarativeConfig, err error) {
				assert.Equal(t, &declcfg.DeclarativeConfig{
					Packages: []declcfg.Package{{Name: "pkg1"}},
					Channels: []declcfg.Channel{
						{Name: "ch1", Package: "pkg1", Entries: []declcfg.ChannelEntry{{Name: "b2", Replaces: "b1", Skips: []string{"b0"}}, {Name: "b1"}}},
					},
					Bundles: []declcfg.Bundle{
						// Pkg1 bundles
						{Name: "b1", Package: "pkg1", Properties: propertiesForBundle("pkg1", "0.1.0")},
						{Name: "b2", Package: "pkg1", Properties: propertiesForBundle("pkg1", "0.2.0")},
					},
					Deprecations: []declcfg.Deprecation{{
						Package: "pkg1",
						Entries: []declcfg.DeprecationEntry{
							{Reference: declcfg.PackageScopedReference{Schema: declcfg.SchemaPackage}},
							{Reference: declcfg.PackageScopedReference{Schema: declcfg.SchemaChannel, Name: "ch1"}},
							{Reference: declcfg.PackageScopedReference{Schema: declcfg.SchemaBundle, Name: "b1"}},
						},
					}},
					Others: []declcfg.Meta{{Name: "global"}},
				}, actual)
				assert.NoError(t, err)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if strings.HasPrefix(tc.name, "TODO") {
				t.Skip("TODO")
				return
			}
			f := NewFilter(tc.config)
			out, err := f.FilterCatalog(context.Background(), tc.in)
			tc.assertion(t, out, err)
		})
	}
}

func TestFilter_FilterCatalog_WithLogger(t *testing.T) {
	logOutput := &bytes.Buffer{}
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true, DisableQuote: true})
	log.SetOutput(logOutput)
	withLogger := WithLogger(logrus.NewEntry(log))
	f := NewFilter(FilterConfiguration{Packages: []Package{
		{Name: "pkg", Channels: []Channel{{Name: "ch", VersionRange: ">=1.0.0 <2.0.0"}}},
	}}, withLogger)

	out, err := f.FilterCatalog(context.Background(), &declcfg.DeclarativeConfig{
		Packages: []declcfg.Package{{Name: "pkg"}},
		Channels: []declcfg.Channel{{Name: "ch", Package: "pkg", Entries: []declcfg.ChannelEntry{
			{Name: "b2", Skips: []string{"b1"}},
			{Name: "b1"},
		}}},
		Bundles: []declcfg.Bundle{
			{Name: "b1", Package: "pkg", Properties: propertiesForBundle("pkg", "1.0.0")},
			{Name: "b2", Package: "pkg", Properties: propertiesForBundle("pkg", "2.0.0")},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, &declcfg.DeclarativeConfig{
		Packages: []declcfg.Package{{Name: "pkg"}},
		Channels: []declcfg.Channel{{Name: "ch", Package: "pkg", Entries: []declcfg.ChannelEntry{
			{Name: "b2", Skips: []string{"b1"}},
			{Name: "b1"},
		}}},
		Bundles: []declcfg.Bundle{
			{Name: "b1", Package: "pkg", Properties: propertiesForBundle("pkg", "1.0.0")},
			{Name: "b2", Package: "pkg", Properties: propertiesForBundle("pkg", "2.0.0")},
		},
	}, out)
	assert.Contains(t, logOutput.String(), `including bundle "b2" with version "2.0.0"`)
}

func propertiesForBundle(pkg, version string) []property.Property {
	return []property.Property{
		{Type: property.TypePackage, Value: []byte(fmt.Sprintf(`{"packageName": %q, "version": %q}`, pkg, version))},
	}
}
