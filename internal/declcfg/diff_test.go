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
			g:      &DiffGenerator{},
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
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("foo", "0.1.0"),
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
			g:      &DiffGenerator{},
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
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("foo", "0.1.0"),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
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
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("fast", ""),
							property.MustBuildPackage("foo", "0.2.0-alpha.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.1",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("fast", "foo.v0.2.0-alpha.0"),
							property.MustBuildPackage("foo", "0.2.0-alpha.1"),
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
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("foo", "0.2.0"),
							property.MustBuildSkips("foo.v0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("fast", ""),
							property.MustBuildPackage("foo", "0.2.0-alpha.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.1",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("fast", "foo.v0.2.0-alpha.0"),
							property.MustBuildPackage("foo", "0.2.0-alpha.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0-clusterwide",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("clusterwide", ""),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("clusterwide", ""),
							property.MustBuildPackage("foo", "0.1.0-clusterwide"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("foo", "0.2.0"),
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
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", ""),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", ""),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("foo", "0.1.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0-clusterwide",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("clusterwide", ""),
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
							property.MustBuildChannel("stable", ""),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0-clusterwide",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("clusterwide", ""),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", ""),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", "etcd.v0.9.1"),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", "etcd.v0.9.1"),
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
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", "foo.v0.1.0"),
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
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", "etcd.v0.9.1"),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", "etcd.v0.9.1"),
							property.MustBuildPackage("etcd", "0.9.2"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", "foo.v0.1.0"),
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
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", ""),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", ""),
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
			g: &DiffGenerator{},
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
							property.MustBuildChannel("stable", ""),
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
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("alpha", "foo.v0.2.0-alpha.1"),
							property.MustBuildChannel("stable", "foo.v0.1.0"),
							property.MustBuildPackage("foo", "0.2.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("alpha", ""),
							property.MustBuildPackage("foo", "0.2.0-alpha.0"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0-alpha.1",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("alpha", "foo.v0.2.0-alpha.0"),
							property.MustBuildPackage("foo", "0.2.0-alpha.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", "etcd.v0.9.0"),
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1-clusterwide",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("clusterwide", ""),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", "etcd.v0.9.0"),
							property.MustBuildPackage("etcd", "0.9.1"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.1-clusterwide",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("clusterwide", ""),
							property.MustBuildPackage("etcd", "0.9.1-clusterwide"),
						},
					},
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.2.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("alpha", "foo.v0.2.0-alpha.1"),
							property.MustBuildChannel("stable", "foo.v0.1.0"),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "foo.v0.1.0",
						Package: "foo",
						Image:   "reg/foo:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", ""),
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
							property.MustBuildChannel("stable", "etcd.v0.9.1"),
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
				Bundles: []Bundle{
					{
						Schema:  schemaBundle,
						Name:    "etcd.v0.9.2",
						Package: "etcd",
						Image:   "reg/etcd:latest",
						Properties: []property.Property{
							property.MustBuildChannel("stable", "etcd.v0.9.1"),
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
							property.MustBuildChannel("stable", ""),
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
