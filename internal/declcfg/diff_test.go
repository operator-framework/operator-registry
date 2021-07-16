package declcfg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/internal/model"
	"github.com/operator-framework/operator-registry/internal/property"
)

type deprecated struct{}

const deprecatedType = "olm.deprecated"

func init() {
	property.AddToScheme(deprecatedType, &deprecated{})
}

func TestDiffLatest(t *testing.T) {
	type spec struct {
		name      string
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
			expCfg: DeclarativeConfig{},
		},
		{
			name: "NoDiff/OneEqualBundle",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
						},
					},
				},
			},
			expCfg: DeclarativeConfig{},
		},
		{
			name: "NoDiff/UnsortedBundleProps",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
				},
			},
			expCfg: DeclarativeConfig{},
		},
		{
			name: "HasDiff/OneModifiedBundle",
			oldCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackageRequired("bar", ">=1.0.0"),
						},
					},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0-alpha.0"),
							property.MustBuildChannel("fast", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.1",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0-alpha.1"),
							property.MustBuildChannel("fast", "foo.v0.2.0-alpha.0"),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
							property.MustBuild(&deprecated{}),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0"),
							property.MustBuildChannel("stable", ""),
							property.MustBuildSkips("foo.v0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0-alpha.0"),
							property.MustBuildChannel("fast", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.1",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0-alpha.1"),
							property.MustBuildChannel("fast", "foo.v0.2.0-alpha.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0-clusterwide",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0-clusterwide"),
							property.MustBuildChannel("clusterwide", ""),
						},
					},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
							property.MustBuild(&deprecated{}),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0-clusterwide",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0-clusterwide"),
							property.MustBuildChannel("clusterwide", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0"),
							property.MustBuildChannel("stable", ""),
							property.MustBuildSkips("foo.v0.1.0"),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
							property.MustBuildChannel("stable", ""),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", ""),
						},
					},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", ""),
						},
					},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
							property.MustBuildChannel("stable", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0-clusterwide",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0-clusterwide"),
							property.MustBuildChannel("clusterwide", ""),
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
							property.MustBuildChannel("stable", ""),
						},
					},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
							property.MustBuildChannel("stable", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0-clusterwide",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0-clusterwide"),
							property.MustBuildChannel("clusterwide", ""),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", ""),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.2"),
							property.MustBuildChannel("stable", "etcd.v0.9.1"),
						},
					},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.2"),
							property.MustBuildChannel("stable", "etcd.v0.9.1"),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", "foo.v0.1.0"),
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
							property.MustBuildChannel("stable", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.2"),
							property.MustBuildChannel("stable", "etcd.v0.9.1"),
						},
					},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
							property.MustBuildChannel("stable", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.2"),
							property.MustBuildChannel("stable", "etcd.v0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", "foo.v0.1.0"),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
						},
					},
				},
			},
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
							property.MustBuildChannel("stable", ""),
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
						},
					},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
							property.MustBuildChannel("stable", ""),
							property.MustBuildGVK("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
							property.MustBuildGVKRequired("etcd.database.coreos.com", "v1beta2", "EtcdBackup"),
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

			outputModel, err := Diff(oldModel, newModel)
			s.assertion(t, err)

			outputCfg := ConvertFromModel(outputModel)
			require.Equal(t, s.expCfg, outputCfg)
		})
	}
}

func TestDiffHeadsOnly(t *testing.T) {
	type spec struct {
		name      string
		newCfg    DeclarativeConfig
		expCfg    DeclarativeConfig
		assertion require.ErrorAssertionFunc
	}

	specs := []spec{
		{
			name:   "NoDiff/Empty",
			newCfg: DeclarativeConfig{},
			expCfg: DeclarativeConfig{},
		},
		{
			name: "HasDiff/OneBundle",
			newCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
						},
					},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.1.0"),
							property.MustBuildChannel("stable", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0"),
							property.MustBuildChannel("alpha", "foo.v0.2.0-alpha.1"),
							property.MustBuildChannel("stable", "foo.v0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0-alpha.0"),
							property.MustBuildChannel("alpha", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.1",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0-alpha.1"),
							property.MustBuildChannel("alpha", "foo.v0.2.0-alpha.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
							property.MustBuildChannel("stable", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
							property.MustBuildChannel("stable", "etcd.v0.9.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1-clusterwide",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1-clusterwide"),
							property.MustBuildChannel("clusterwide", ""),
						},
					},
				},
			},
			expCfg: DeclarativeConfig{
				Packages: []Package{
					{Schema: schemaPackage, Name: "etcd", DefaultChannel: "stable"},
					{Schema: schemaPackage, Name: "foo", DefaultChannel: "stable"},
				},
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1"),
							property.MustBuildChannel("stable", "etcd.v0.9.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1-clusterwide",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildPackage("etcd", "0.9.1-clusterwide"),
							property.MustBuildChannel("clusterwide", ""),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildPackage("foo", "0.2.0"),
							property.MustBuildChannel("alpha", "foo.v0.2.0-alpha.1"),
							property.MustBuildChannel("stable", "foo.v0.1.0"),
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

			outputModel, err := Diff(model.Model{}, newModel)
			s.assertion(t, err)

			outputCfg := ConvertFromModel(outputModel)
			require.Equal(t, s.expCfg, outputCfg)
		})
	}
}
