package declcfg

import (
	"testing"

	"github.com/operator-framework/operator-registry/alpha/property"
	"github.com/stretchr/testify/require"
)

func TestMergeDC(t *testing.T) {
	type spec struct {
		name      string
		mt        Merger
		dc, expDC *DeclarativeConfig
		expError  string
	}

	cases := []spec{
		{
			name:  "TwoWay/Empty",
			mt:    &TwoWayStrategy{},
			dc:    &DeclarativeConfig{},
			expDC: &DeclarativeConfig{},
		},
		{
			name: "TwoWay/NoMergeNeeded",
			mt:   &TwoWayStrategy{},
			dc: &DeclarativeConfig{
				Packages: []Package{
					{Schema: "olm.package", Name: "foo", DefaultChannel: "stable"},
					{Schema: "olm.package", Name: "bar", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: "olm.channel", Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
					{Schema: "olm.channel", Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
				},
			},
			expDC: &DeclarativeConfig{
				Packages: []Package{
					{Schema: "olm.package", Name: "bar", DefaultChannel: "stable"},
					{Schema: "olm.package", Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: "olm.channel", Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
					{Schema: "olm.channel", Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
					{
						Schema:  "olm.bundle",
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
			name: "TwoWay/MergePackagesChannelsBundles",
			mt:   &TwoWayStrategy{},
			dc: &DeclarativeConfig{
				Packages: []Package{
					{Schema: "olm.package", Name: "foo", DefaultChannel: "stable"},
					{Schema: "olm.package", Name: "foo", DefaultChannel: "alpha"},
					{Schema: "olm.package", Name: "bar", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: "olm.channel", Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
						{Name: "foo.v0.1.0", Replaces: "foo.v0.0.5"},
						{Name: "foo.v0.1.0", Skips: []string{"foo.v0.0.4"}},
					}},
					{Schema: "olm.channel", Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.1", Replaces: "foo.v1.0.0"},
					}},
					{Schema: "olm.channel", Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.0.5",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.0.5"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.1",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.1"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("foo.example.com", "v1", "Foo"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
				},
			},
			expDC: &DeclarativeConfig{
				Packages: []Package{
					{Schema: "olm.package", Name: "bar", DefaultChannel: "stable"},
					{Schema: "olm.package", Name: "foo", DefaultChannel: "alpha"},
				},
				Channels: []Channel{
					{Schema: "olm.channel", Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
					{Schema: "olm.channel", Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0", Replaces: "foo.v0.0.5", Skips: []string{"foo.v0.0.4"}},
						{Name: "foo.v0.1.1", Replaces: "foo.v1.0.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							// Can't merge properties since their keys are unknown.
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.0.5",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.0.5"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.1",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.1"),
						},
					},
				},
			},
		},
		{
			name:  "PreferLast/Empty",
			mt:    &PreferLastStrategy{},
			dc:    &DeclarativeConfig{},
			expDC: &DeclarativeConfig{},
		},
		{
			name: "PreferLast/NoMergeNeeded",
			mt:   &PreferLastStrategy{},
			dc: &DeclarativeConfig{
				Packages: []Package{
					{Schema: "olm.package", Name: "foo", DefaultChannel: "stable"},
					{Schema: "olm.package", Name: "bar", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: "olm.channel", Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
					{Schema: "olm.channel", Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
				},
			},
			expDC: &DeclarativeConfig{
				Packages: []Package{
					{Schema: "olm.package", Name: "bar", DefaultChannel: "stable"},
					{Schema: "olm.package", Name: "foo", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: "olm.channel", Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
					{Schema: "olm.channel", Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
					{
						Schema:  "olm.bundle",
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
			name: "PreferLast/MergePackagesChannelsBundles",
			mt:   &PreferLastStrategy{},
			dc: &DeclarativeConfig{
				Packages: []Package{
					{Schema: "olm.package", Name: "foo", DefaultChannel: "stable"},
					{Schema: "olm.package", Name: "foo", DefaultChannel: "alpha"},
					{Schema: "olm.package", Name: "bar", DefaultChannel: "stable"},
				},
				Channels: []Channel{
					{Schema: "olm.channel", Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.1.0"},
					}},
					{Schema: "olm.channel", Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.5.0"},
						{Name: "foo.v0.1.0", Replaces: "foo.v0.0.5"},
					}},
					{Schema: "olm.channel", Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.0.5",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.0.5"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("foo.example.com", "v1", "Foo"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
				},
			},
			expDC: &DeclarativeConfig{
				Packages: []Package{
					{Schema: "olm.package", Name: "bar", DefaultChannel: "stable"},
					{Schema: "olm.package", Name: "foo", DefaultChannel: "alpha"},
				},
				Channels: []Channel{
					{Schema: "olm.channel", Name: "stable", Package: "bar", Entries: []ChannelEntry{
						{Name: "bar.v0.1.0"},
					}},
					{Schema: "olm.channel", Name: "stable", Package: "foo", Entries: []ChannelEntry{
						{Name: "foo.v0.5.0"},
						{Name: "foo.v0.1.0", Replaces: "foo.v0.0.5"},
					}},
				},
				Bundles: []Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "bar.v0.1.0",
						Package: "bar",
						Image:   "reg/bar:latest",
						Properties: []property.Property{
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1", "EtcdBackup"),
							property.MustBuildPackage("bar", "0.1.0"),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.0.5",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.0.5"),
						},
					},
					{
						Schema:  "olm.bundle",
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
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.mt.MergeDC(c.dc)
			if c.expError != "" {
				require.EqualError(t, err, c.expError)
			} else {
				require.NoError(t, err)
				require.Equal(t, c.expDC, c.dc)
			}
		})
	}
}
