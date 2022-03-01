package declcfg

import (
	"testing"

	"github.com/blang/semver/v4"
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
		{
			name: "HasDiff/IncludePackage",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "bar", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{{Name: "foo.v0.1.0"}}},
					{Schema: schemaChannel, Name: "stable", Package: "bar", Entries: []ChannelEntry{{Name: "bar.v0.1.0"}}},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "bar.v0.1.0", Package: "bar", Image: "reg/bar:latest",
						Properties: []property.Property{property.MustBuildPackage("bar", "0.1.0")},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "bar", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{{Name: "foo.v0.1.0"}}},
					{Schema: schemaChannel, Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"}, {Name: "bar.v0.2.0", Replaces: "bar.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "bar.v0.1.0", Package: "bar", Image: "reg/bar:latest",
						Properties: []property.Property{property.MustBuildPackage("bar", "0.1.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "bar.v0.2.0", Package: "bar", Image: "reg/bar:latest",
						Properties: []property.Property{property.MustBuildPackage("bar", "0.2.0")},
					},
				},
			},
			g: &DiffGenerator{
				Includer: DiffIncluder{
					Packages: []DiffIncludePackage{{Name: "bar"}},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "bar", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.2.0", Replaces: "bar.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "bar.v0.2.0", Package: "bar", Image: "reg/bar:latest",
						Properties: []property.Property{property.MustBuildPackage("bar", "0.2.0")},
					},
				},
			},
		},
		{
			name: "HasDiff/IncludeChannel",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{{Name: "foo.v0.1.0"}}},
					{Schema: schemaChannel, Name: "alpha", Package: "foo", Entries: []ChannelEntry{{Name: "foo.v0.1.0-alpha.0"}}},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.0-alpha.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.0-alpha.0")},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "alpha"}, // Make sure the default channel is still updated.
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"}, {Name: "foo.v0.2.0", Replaces: "foo.v0.1.0"}},
					},
					{Schema: schemaChannel, Name: "alpha", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0-alpha.0"}, {Name: "foo.v0.2.0-alpha.0", Replaces: "foo.v0.1.0-alpha.0"}},
					},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.2.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.2.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.0-alpha.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.0-alpha.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.2.0-alpha.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.2.0-alpha.0")},
					},
				},
			},
			g: &DiffGenerator{
				Includer: DiffIncluder{
					Packages: []DiffIncludePackage{{Name: "foo", Channels: []DiffIncludeChannel{{Name: "stable"}}}},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "alpha"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.2.0", Replaces: "foo.v0.1.0"}},
					},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "foo.v0.2.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.2.0")},
					},
				},
			},
		},
		{
			name: "HasDiff/IncludeVersion",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"}, {Name: "foo.v0.2.0", Replaces: "foo.v0.1.0"}},
					},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.2.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.2.0")},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"}, {Name: "foo.v0.1.1", Replaces: "foo.v0.1.0"},
						{Name: "foo.v0.2.0", Replaces: "foo.v0.1.1"}, {Name: "foo.v0.3.0", Replaces: "foo.v0.2.0"}},
					},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.1", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.1")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.2.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.2.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.3.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.3.0")},
					},
				},
			},
			g: &DiffGenerator{
				Includer: DiffIncluder{
					Packages: []DiffIncludePackage{
						{Name: "foo", Channels: []DiffIncludeChannel{
							{Name: "stable", Versions: []semver.Version{{Major: 0, Minor: 2, Patch: 0}}}},
						}},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.3.0", Replaces: "foo.v0.2.0"}},
					},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "foo.v0.3.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.3.0")},
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
			name: "NoDiff/EmptyBundleWithInclude",
			newCfg: DeclarativeConfig{
				Packages: []Package{
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
				},
				Bundles: []Bundle{
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
			g: &DiffGenerator{
				IncludeAdditively: false,
				Includer: DiffIncluder{
					Packages: []DiffIncludePackage{
						{
							Name: "etcd",
							AllChannels: DiffIncludeChannel{
								Versions: []semver.Version{{Major: 0, Minor: 9, Patch: 2}},
							},
						},
					},
				},
			},
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
		{
			name: "HasDiff/SelectDependencies",
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "bar", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.0"},
						{Name: "etcd.v0.9.1", Replaces: "etcd.v0.9.0"},
						{Name: "etcd.v0.9.2", Replaces: "etcd.v0.9.1"},
						{Name: "etcd.v0.9.3", Replaces: "etcd.v0.9.2"},
						{Name: "etcd.v1.0.0", Replaces: "etcd.v0.9.3", Skips: []string{"etcd.v0.9.1", "etcd.v0.9.2", "etcd.v0.9.3"}},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackageRequired("etcd", "<0.9.2"),
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.0",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.0"),
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
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.3",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.3"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v1.0.0",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("etcd", "1.0.0"),
						},
					},
				},
			},
			g: &DiffGenerator{},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "bar", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1", Replaces: "etcd.v0.9.0"},
						{Name: "etcd.v0.9.2", Replaces: "etcd.v0.9.1"},
						{Name: "etcd.v0.9.3", Replaces: "etcd.v0.9.2"},
						{Name: "etcd.v1.0.0", Replaces: "etcd.v0.9.3", Skips: []string{"etcd.v0.9.1", "etcd.v0.9.2", "etcd.v0.9.3"}},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
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
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.3",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.3"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v1.0.0",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "1.0.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("etcd", "<0.9.2"),
						},
					},
				},
			},
		},
		{
			name: "HasDiff/SelectDependenciesInclude",
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "bar", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.0"},
						{Name: "etcd.v0.9.1", Replaces: "etcd.v0.9.0"},
						{Name: "etcd.v0.9.2", Replaces: "etcd.v0.9.1"},
						{Name: "etcd.v0.9.3", Replaces: "etcd.v0.9.2"},
						{Name: "etcd.v1.0.0", Replaces: "etcd.v0.9.3", Skips: []string{"etcd.v0.9.1", "etcd.v0.9.2", "etcd.v0.9.3"}},
					}},
					{Schema: schemaChannel, Name: "alpha", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.2", Replaces: "etcd.v0.9.1"},
						{Name: "etcd.v0.9.3", Replaces: "etcd.v0.9.2"},
						{Name: "etcd.v1.0.0", Replaces: "etcd.v0.9.3", Skips: []string{"etcd.v0.9.1", "etcd.v0.9.2", "etcd.v0.9.3"}},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackageRequired("etcd", "<0.9.2"),
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.0",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.0"),
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
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.3",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.3"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v1.0.0",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("etcd", "1.0.0"),
						},
					},
				},
			},
			g: &DiffGenerator{
				IncludeAdditively: false,
				Includer: DiffIncluder{
					Packages: []DiffIncludePackage{
						{
							Name:     "bar",
							Channels: []DiffIncludeChannel{{Name: "stable"}},
						},
					},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "bar", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v1.0.0", Replaces: "etcd.v0.9.3", Skips: []string{"etcd.v0.9.1", "etcd.v0.9.2", "etcd.v0.9.3"}},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v1.0.0",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "1.0.0"),
						},
					},
				},
			},
		},
		{
			name: "HasDiff/IncludeAdditive",
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "bar", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.0"},
						{Name: "etcd.v0.9.1", Replaces: "etcd.v0.9.0"},
						{Name: "etcd.v0.9.2", Replaces: "etcd.v0.9.1"},
						{Name: "etcd.v0.9.3", Replaces: "etcd.v0.9.2"},
						{Name: "etcd.v1.0.0", Replaces: "etcd.v0.9.3", Skips: []string{"etcd.v0.9.1", "etcd.v0.9.2", "etcd.v0.9.3"}},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackageRequired("etcd", "<0.9.2"),
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.0",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.0"),
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
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.3",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.3"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v1.0.0",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("etcd", "1.0.0"),
						},
					},
				},
			},
			g: &DiffGenerator{
				IncludeAdditively: true,
				Includer: DiffIncluder{
					Packages: []DiffIncludePackage{
						{
							Name: "etcd",
							Channels: []DiffIncludeChannel{{
								Name:     "stable",
								Versions: []semver.Version{{Major: 0, Minor: 9, Patch: 2}}},
							}},
						{
							Name:     "bar",
							Channels: []DiffIncludeChannel{{Name: "stable"}},
						},
					},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "bar", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.1", Replaces: "etcd.v0.9.0"},
						{Name: "etcd.v0.9.2", Replaces: "etcd.v0.9.1"},
						{Name: "etcd.v0.9.3", Replaces: "etcd.v0.9.2"},
						{Name: "etcd.v1.0.0", Replaces: "etcd.v0.9.3", Skips: []string{"etcd.v0.9.1", "etcd.v0.9.2", "etcd.v0.9.3"}},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
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
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.3",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.3"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v1.0.0",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "1.0.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildPackageRequired("etcd", "<0.9.2"),
						},
					},
				},
			},
		},
		{
			name: "HasDiff/IncludePackage",
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "bar", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{{Name: "foo.v0.1.0"}}},
					{Schema: schemaChannel, Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"}, {Name: "bar.v0.2.0", Replaces: "bar.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "bar.v0.1.0", Package: "bar", Image: "reg/bar:latest",
						Properties: []property.Property{property.MustBuildPackage("bar", "0.1.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "bar.v0.2.0", Package: "bar", Image: "reg/bar:latest",
						Properties: []property.Property{property.MustBuildPackage("bar", "0.2.0")},
					},
				},
			},
			g: &DiffGenerator{
				Includer: DiffIncluder{
					Packages: []DiffIncludePackage{{Name: "bar"}},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "bar", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"}, {Name: "bar.v0.2.0", Replaces: "bar.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "bar.v0.1.0", Package: "bar", Image: "reg/bar:latest",
						Properties: []property.Property{property.MustBuildPackage("bar", "0.1.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "bar.v0.2.0", Package: "bar", Image: "reg/bar:latest",
						Properties: []property.Property{property.MustBuildPackage("bar", "0.2.0")},
					},
				},
			},
		},
		{
			name: "HasDiff/IncludeChannel",
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "alpha"}, // Make sure the default channel is still updated.
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"}, {Name: "foo.v0.2.0", Replaces: "foo.v0.1.0"}},
					},
					{Schema: schemaChannel, Name: "alpha", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0-alpha.0"}, {Name: "foo.v0.2.0-alpha.0", Replaces: "foo.v0.1.0-alpha.0"}},
					},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.2.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.2.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.0-alpha.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.0-alpha.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.2.0-alpha.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.2.0-alpha.0")},
					},
				},
			},
			g: &DiffGenerator{
				Includer: DiffIncluder{
					Packages: []DiffIncludePackage{{Name: "foo", Channels: []DiffIncludeChannel{{Name: "stable"}}}},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "alpha"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"}, {Name: "foo.v0.2.0", Replaces: "foo.v0.1.0"}},
					},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.2.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.2.0")},
					},
				},
			},
		},
		{
			name: "HasDiff/IncludeVersion",
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"}, {Name: "foo.v0.1.1", Replaces: "foo.v0.1.0"},
						{Name: "foo.v0.2.0", Replaces: "foo.v0.1.1"}, {Name: "foo.v0.3.0", Replaces: "foo.v0.2.0"}},
					},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.1.1", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.1.1")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.2.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.2.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.3.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.3.0")},
					},
				},
			},
			g: &DiffGenerator{
				Includer: DiffIncluder{
					Packages: []DiffIncludePackage{
						{Name: "foo", Channels: []DiffIncludeChannel{
							{Name: "stable", Versions: []semver.Version{{Major: 0, Minor: 2, Patch: 0}}}},
						}},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.2.0", Replaces: "foo.v0.1.1"}, {Name: "foo.v0.3.0", Replaces: "foo.v0.2.0"}},
					},
				},
				Bundles: []Bundle{
					{
						Schema: schemaBundle,
						Name:   "foo.v0.2.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.2.0")},
					},
					{
						Schema: schemaBundle,
						Name:   "foo.v0.3.0", Package: "foo", Image: "reg/foo:latest",
						Properties: []property.Property{property.MustBuildPackage("foo", "0.3.0")},
					},
				},
			},
		},
		{
			name: "HasDiff/IncludeNonAdditive",
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "bar", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.0"},
						{Name: "etcd.v0.9.1", Replaces: "etcd.v0.9.0"},
						{Name: "etcd.v0.9.2", Replaces: "etcd.v0.9.1"},
						{Name: "etcd.v0.9.3", Replaces: "etcd.v0.9.2"},
						{Name: "etcd.v1.0.0", Replaces: "etcd.v0.9.3", Skips: []string{"etcd.v0.9.1", "etcd.v0.9.2", "etcd.v0.9.3"}},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackageRequired("etcd", "<0.9.2"),
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.0",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.0"),
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
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.3",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.3"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v1.0.0",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("etcd", "1.0.0"),
						},
					},
				},
			},
			g: &DiffGenerator{
				Includer: DiffIncluder{
					Packages: []DiffIncludePackage{
						{
							Name: "etcd",
							Channels: []DiffIncludeChannel{{
								Name:     "stable",
								Versions: []semver.Version{{Major: 0, Minor: 9, Patch: 3}}},
							}},
						{
							Name:     "bar",
							Channels: []DiffIncludeChannel{{Name: "stable"}},
						},
					},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "bar", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: schemaChannel, Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
					{Schema: schemaChannel, Name: "stable", Package: "etcd", Entries: []ChannelEntry{
						{Name: "etcd.v0.9.3", Replaces: "etcd.v0.9.2"},
						{Name: "etcd.v1.0.0", Replaces: "etcd.v0.9.3", Skips: []string{"etcd.v0.9.1", "etcd.v0.9.2", "etcd.v0.9.3"}},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.3",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "0.9.3"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v1.0.0",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildGVK("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
							property.MustBuildPackage("etcd", "1.0.0"),
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
