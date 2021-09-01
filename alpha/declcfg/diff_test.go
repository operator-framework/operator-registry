package declcfg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/model"
	"github.com/operator-framework/operator-registry/alpha/property"
)

type deprecated struct{}

const deprecatedType = "olm.deprecated"

func init() {
	property.AddToScheme(deprecatedType, &deprecated{})
}

func TestDiffLatest(t *testing.T) {
	type spec struct {
		name      string
		g         *DiffGenerator
		oldCfg    DeclarativeConfig
		newCfg    DeclarativeConfig
		expCfg    DeclarativeConfig
		assertion require.ErrorAssertionFunc
	}

	specs := []spec{
		{
			name:   "NoDiff/Empty",
			oldCfg: DeclarativeConfig{},
			newCfg: DeclarativeConfig{},
			g:      &DiffGenerator{},
			expCfg: DeclarativeConfig{},
		},
		{
			name: "NoDiff/OneEqualBundle",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
				},
			},
			g:      &DiffGenerator{},
			expCfg: DeclarativeConfig{},
		},
		{
			name: "NoDiff/UnsortedBundleProps",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
				},
			},
			g:      &DiffGenerator{},
			expCfg: DeclarativeConfig{},
		},
		{
			name: "HasDiff/OneModifiedBundle",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("bar", ">=1.0.0"),
						},
					},
				},
			},
			g: &DiffGenerator{},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("bar", ">=1.0.0"),
						},
					},
				},
			},
		},
		{
			name: "HasDiff/ManyBundlesAndChannels",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "fast", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.2.0-alpha.0"},
						{Name: "foo.v0.2.0-alpha.1", Replaces: "foo.v0.2.0-alpha.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0-alpha.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.1",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0-alpha.1"),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
						{Name: "foo.v0.2.0", Skips: []string{"foo.v0.1.0"}},
					}},
					{Schema: schemaChannel, Name: "fast", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.2.0-alpha.0"},
						{Name: "foo.v0.2.0-alpha.1", Replaces: "foo.v0.2.0-alpha.0"},
					}},
					{Schema: schemaChannel, Name: "clusterwide", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0-clusterwide"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuild(&deprecated{}),
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0-alpha.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.1",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0-alpha.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0-clusterwide",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0-clusterwide"),
						},
					},
				},
			},
			g: &DiffGenerator{},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "clusterwide", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0-clusterwide"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
						{Name: "foo.v0.2.0", Skips: []string{"foo.v0.1.0"}},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuild(&deprecated{}),
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0-clusterwide",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0-clusterwide"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0"),
						},
					},
				},
			},
		},
		{
			name: "HasDiff/OldBundleUpdatedDependencyRange",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("etcd", ">=0.9.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
				},
			},
			g: &DiffGenerator{},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("etcd", ">=0.9.0"),
						},
					},
				},
			},
		},
		{
			name: "HasDiff/BundleNewDependencyRange",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("etcd", ">=0.9.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
				},
			},
			g: &DiffGenerator{},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("etcd", ">=0.9.0"),
						},
					},
				},
			},
		},
		{
			name: "HasDiff/NewBundleNewDependencyRange",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "clusterwide", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0-clusterwide"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0-clusterwide",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0-clusterwide"),
							property.MustBuildPackageRequired("etcd", ">=0.9.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
				},
			},
			g: &DiffGenerator{},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1"},
					}},
					{Schema: schemaChannel, Name: "clusterwide", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0-clusterwide"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0-clusterwide",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0-clusterwide"),
							property.MustBuildPackageRequired("etcd", ">=0.9.0"),
						},
					},
				},
			},
		},
		{
			name: "HasDiff/OneNewDependencyRange",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("etcd", ">=0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1"},
						{Name: "etcd.v0.9.2", Replaces: "etcd.v0.9.1"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("etcd", ">=0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.2"),
						},
					},
				},
			},
			g: &DiffGenerator{},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.2", Replaces: "etcd.v0.9.1"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.2"),
						},
					},
				},
			},
		},
		{
			name: "HasDiff/TwoDependencyRanges",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
						{Name: "foo.v0.2.0", Replaces: "foo.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1"},
						{Name: "etcd.v0.9.2", Replaces: "etcd.v0.9.1"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("etcd", ">=0.9.0 <0.9.2"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0"),
							property.MustBuildPackageRequired("etcd", ">=0.9.2"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.2"),
						},
					},
				},
			},
			g: &DiffGenerator{},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1"},
						{Name: "etcd.v0.9.2", Replaces: "etcd.v0.9.1"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
						{Name: "foo.v0.2.0", Replaces: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.2"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("etcd", ">=0.9.0 <0.9.2"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0"),
							property.MustBuildPackageRequired("etcd", ">=0.9.2"),
						},
					},
				},
			},
		},
		{
			name: "HasDiff/BundleNewDependencyGVK",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
				},
			},
			g: &DiffGenerator{},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
				},
			},
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			if s.assertion == nil {
				s.assertion = require.NoError
			}

			oldModel, err := ConvertToModel(s.oldCfg)
			require.NoError(t, err)

			newModel, err := ConvertToModel(s.newCfg)
			require.NoError(t, err)

			outputModel, err := s.g.Run(oldModel, newModel)
			s.assertion(t, err)

			outputCfg := ConvertFromModel(outputModel)
			require.Equal(t, s.expCfg, outputCfg)
		})
	}
}

func TestDiffHeadsOnly(t *testing.T) {
	type spec struct {
		name      string
		g         *DiffGenerator
		newCfg    DeclarativeConfig
		expCfg    DeclarativeConfig
		assertion require.ErrorAssertionFunc
	}

	specs := []spec{
		{
			name:   "NoDiff/Empty",
			newCfg: DeclarativeConfig{},
			g:      &DiffGenerator{},
			expCfg: DeclarativeConfig{},
		},
		{
			name: "HasDiff/OneBundle",
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
				},
			},
			g: &DiffGenerator{},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
				},
			},
		},
		{
			name: "HasDiff/Graph",
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.0"},
						{Name: "etcd.v0.9.1", Replaces: "etcd.v0.9.0"},
					}},
					{Schema: schemaChannel, Name: "clusterwide", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1-clusterwide"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
						{Name: "foo.v0.2.0", Replaces: "foo.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "alpha", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.2.0-alpha.0"},
						{Name: "foo.v0.2.0-alpha.1", Replaces: "foo.v0.2.0-alpha.0"},
						{Name: "foo.v0.2.0", Replaces: "foo.v0.2.0-alpha.1"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0-alpha.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.1",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0-alpha.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.0",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1-clusterwide",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1-clusterwide"),
						},
					},
				},
			},
			g: &DiffGenerator{},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "clusterwide", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1-clusterwide"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1", Replaces: "etcd.v0.9.0"},
					}},
					{Schema: schemaChannel, Name: "alpha", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.2.0", Replaces: "foo.v0.2.0-alpha.1"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.2.0", Replaces: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1-clusterwide",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1-clusterwide"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0"),
						},
					},
				},
			},
		},
		{
			// Testing SkipDependencies only really makes sense in heads-only mode,
			// since new dependencies are always added.
			name: "HasDiff/SkipDependencies",
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1"},
						{Name: "etcd.v0.9.2", Replaces: "etcd.v0.9.1"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackageRequired("etcd", "<=0.9.1"),
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.2"),
						},
					},
				},
			},
			g: &DiffGenerator{
				SkipDependencies: true,
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.2", Replaces: "etcd.v0.9.1"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.2"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("etcd", "<=0.9.1"),
						},
					},
				},
			},
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			if s.assertion == nil {
				s.assertion = require.NoError
			}

			newModel, err := ConvertToModel(s.newCfg)
			require.NoError(t, err)

			outputModel, err := s.g.Run(model.Model{}, newModel)
			s.assertion(t, err)

			outputCfg := ConvertFromModel(outputModel)
			require.Equal(t, s.expCfg, outputCfg)
		})
	}
}
