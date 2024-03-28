package filter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

func TestPackageFilter_KeepMeta(t *testing.T) {
	tests := []struct {
		name     string
		filter   declcfg.MetaFilter
		meta     *declcfg.Meta
		expected bool
	}{
		{
			name:     "NoFilter_Package",
			filter:   NewPackageFilter().(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaPackage, Name: "foo"},
			expected: false,
		},
		{
			name:     "NoFilter_Channel",
			filter:   NewPackageFilter().(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaChannel, Package: "foo"},
			expected: false,
		},
		{
			name:     "NoFilter_Bundle",
			filter:   NewPackageFilter().(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaBundle, Package: "foo"},
			expected: false,
		},
		{
			name:     "NoFilter_Deprecation",
			filter:   NewPackageFilter().(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaDeprecation, Package: "foo"},
			expected: false,
		},
		{
			name:     "NoFilter_Other",
			filter:   NewPackageFilter().(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: "other", Package: "foo"},
			expected: false,
		},
		{
			name:     "KeepFooBar_Package",
			filter:   NewPackageFilter("foo", "bar").(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaPackage, Name: "foo"},
			expected: true,
		},
		{
			name:     "KeepFooBar_Channel",
			filter:   NewPackageFilter("foo", "bar").(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaChannel, Package: "foo"},
			expected: true,
		},
		{
			name:     "KeepFooBar_Bundle",
			filter:   NewPackageFilter("foo", "bar").(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaBundle, Package: "foo"},
			expected: true,
		},
		{
			name:     "KeepFooBar_Deprecation",
			filter:   NewPackageFilter("foo", "bar").(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaDeprecation, Package: "foo"},
			expected: true,
		},
		{
			name:     "KeepFooBar_Other",
			filter:   NewPackageFilter("foo", "bar").(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: "other", Package: "foo"},
			expected: true,
		},
		{
			name:     "KeepBarBaz_Package",
			filter:   NewPackageFilter("bar", "baz").(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaPackage, Name: "foo"},
			expected: false,
		},
		{
			name:     "KeepBarBaz_Channel",
			filter:   NewPackageFilter("bar", "baz").(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaChannel, Package: "foo"},
			expected: false,
		},
		{
			name:     "KeepBarBaz_Bundle",
			filter:   NewPackageFilter("bar", "baz").(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaBundle, Package: "foo"},
			expected: false,
		},
		{
			name:     "KeepBarBaz_Deprecation",
			filter:   NewPackageFilter("bar", "baz").(declcfg.MetaFilter),
			meta:     &declcfg.Meta{Schema: declcfg.SchemaDeprecation, Package: "foo"},
			expected: false,
		},
		{
			name:     "KeepBarBaz_Other",
			filter:   NewPackageFilter("bar", "baz").(declcfg.MetaFilter),
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

func TestPackageFilter_FilterCatalog(t *testing.T) {
	tests := []struct {
		name     string
		filter   declcfg.CatalogFilter
		catalog  *declcfg.DeclarativeConfig
		expected *declcfg.DeclarativeConfig
	}{
		{
			name:    "EmptyFilter",
			filter:  NewPackageFilter(),
			catalog: testCatalog(),
			expected: &declcfg.DeclarativeConfig{
				Packages:     []declcfg.Package{},
				Channels:     []declcfg.Channel{},
				Bundles:      []declcfg.Bundle{},
				Deprecations: []declcfg.Deprecation{},
				Others:       []declcfg.Meta{},
			},
		},
		{
			name:     "NilCatalog",
			filter:   NewPackageFilter("foo"),
			catalog:  nil,
			expected: nil,
		},
		{
			name:    "KeepFooBar",
			filter:  NewPackageFilter("foo", "bar"),
			catalog: testCatalog(),
			expected: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{Name: "foo"},
					{Name: "bar"},
				},
				Channels: []declcfg.Channel{
					{Name: "ch1", Package: "foo"},
					{Name: "ch2", Package: "foo"},
					{Name: "ch3", Package: "bar"},
					{Name: "ch4", Package: "bar"},
				},
				Bundles: []declcfg.Bundle{
					{Name: "bundle1", Package: "foo"},
					{Name: "bundle2", Package: "foo"},
					{Name: "bundle3", Package: "bar"},
					{Name: "bundle4", Package: "bar"},
				},
				Deprecations: []declcfg.Deprecation{
					{Package: "foo"},
					{Package: "bar"},
				},
				Others: []declcfg.Meta{
					{Schema: "other", Package: "foo"},
					{Schema: "other", Package: "bar"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var inCatalog *declcfg.DeclarativeConfig = nil
			if tt.catalog != nil {
				inCatalog = &*tt.catalog
			}
			actual, err := tt.filter.FilterCatalog(context.Background(), inCatalog)
			require.NoError(t, err)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func testCatalog() *declcfg.DeclarativeConfig {
	return &declcfg.DeclarativeConfig{
		Packages: []declcfg.Package{
			{Name: "foo"},
			{Name: "bar"},
			{Name: "baz"},
		},
		Channels: []declcfg.Channel{
			{Name: "ch1", Package: "foo"},
			{Name: "ch2", Package: "foo"},
			{Name: "ch3", Package: "bar"},
			{Name: "ch4", Package: "bar"},
			{Name: "ch5", Package: "baz"},
			{Name: "foo", Package: "qux"},
			{Name: "bar", Package: "qux"},
			{Name: "baz", Package: "qux"},
		},
		Bundles: []declcfg.Bundle{
			{Name: "bundle1", Package: "foo"},
			{Name: "bundle2", Package: "foo"},
			{Name: "bundle3", Package: "bar"},
			{Name: "bundle4", Package: "bar"},
			{Name: "bundle5", Package: "baz"},
			{Name: "foo", Package: "qux"},
			{Name: "bar", Package: "qux"},
			{Name: "baz", Package: "qux"},
		},
		Deprecations: []declcfg.Deprecation{
			{Package: "foo"},
			{Package: "bar"},
			{Package: "baz"},
		},
		Others: []declcfg.Meta{
			{Schema: "other", Package: "foo"},
			{Schema: "other", Package: "bar"},
			{Schema: "other", Package: "baz"},
			{Schema: "other", Name: "foo", Package: "qux"},
			{Schema: "other", Name: "bar", Package: "qux"},
			{Schema: "other", Name: "baz", Package: "qux"},
		},
	}
}
