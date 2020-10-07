package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageGraphLoader(t *testing.T) {
	tests := []struct {
		dir            string
		fail           bool
		packageName    string
		defaultChannel string
		channel        map[string]Channel
	}{
		{
			dir:            "./testdata/validPackages/etcd",
			fail:           false,
			packageName:    "etcd",
			defaultChannel: "alpha",
			channel: map[string]Channel{
				"alpha": {Head: BundleKey{CsvName: "etcdoperator.v0.9.2"},
					Nodes: map[BundleKey]map[BundleKey]struct{}{
						BundleKey{CsvName: "etcdoperator.v0.6.1"}: {},
						BundleKey{CsvName: "etcdoperator.v0.9.0"}: {BundleKey{CsvName: "etcdoperator.v0.6.1"}: {}},
						BundleKey{CsvName: "etcdoperator.v0.9.1"}: {},
						BundleKey{CsvName: "etcdoperator.v0.9.2"}: {BundleKey{CsvName: "etcdoperator.v0.9.0"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.1"}: {}},
					}},

				"beta": {Head: BundleKey{CsvName: "etcdoperator.v0.9.0"},
					Nodes: map[BundleKey]map[BundleKey]struct{}{
						BundleKey{CsvName: "etcdoperator.v0.6.1"}: {},
						BundleKey{CsvName: "etcdoperator.v0.9.0"}: {BundleKey{CsvName: "etcdoperator.v0.6.1"}: {}},
					}},

				"stable": {Head: BundleKey{CsvName: "etcdoperator.v0.9.2"},
					Nodes: map[BundleKey]map[BundleKey]struct{}{
						BundleKey{CsvName: "etcdoperator.v0.6.1"}: {},
						BundleKey{CsvName: "etcdoperator.v0.9.0"}: {BundleKey{CsvName: "etcdoperator.v0.6.1"}: {}},
						BundleKey{CsvName: "etcdoperator.v0.9.1"}: {},
						BundleKey{CsvName: "etcdoperator.v0.9.2"}: {BundleKey{CsvName: "etcdoperator.v0.9.0"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.1"}: {}},
					}},
			},
		},
		{
			dir:            "./testdata/validPackages/prometheus/",
			fail:           false,
			packageName:    "prometheus",
			defaultChannel: "preview",
			channel: map[string]Channel{
				"preview": {Head: BundleKey{CsvName: "prometheusoperator.0.22.2"},
					Nodes: map[BundleKey]map[BundleKey]struct{}{
						BundleKey{CsvName: "prometheusoperator.0.14.0"}: {},
						BundleKey{CsvName: "prometheusoperator.0.15.0"}: {BundleKey{CsvName: "prometheusoperator.0.14.0"}: {}},
						BundleKey{CsvName: "prometheusoperator.0.22.2"}: {BundleKey{CsvName: "prometheusoperator.0.15.0"}: {}},
					}}},
		},
		{
			dir:  "testdata/invalidPackges/3scale-community-operator",
			fail: true,
		},
		{
			dir:            "./testdata/validPackages/aqua/",
			fail:           false,
			packageName:    "aqua",
			defaultChannel: "stable",
			channel: map[string]Channel{
				"stable": {Head: BundleKey{CsvName: "aqua-operator.v1.0.0"},
					Nodes: map[BundleKey]map[BundleKey]struct{}{
						BundleKey{CsvName: "aqua-operator.v1.0.0"}: {},
					}},
				"alpha": {Head: BundleKey{CsvName: "aqua-operator.v1.0.0"},
					Nodes: map[BundleKey]map[BundleKey]struct{}{
						BundleKey{CsvName: "aqua-operator.v1.0.0"}: {},
					}},
				"beta": {Head: BundleKey{CsvName: "aqua-operator.v0.0.1"},
					Nodes: map[BundleKey]map[BundleKey]struct{}{
						BundleKey{CsvName: "aqua-operator.v0.0.1"}: {},
					}},
			},
		},
	}

	for _, tt := range tests {
		t.Run("Loading Package Graph from "+tt.dir, func(t *testing.T) {
			pg, err := NewPackageGraphLoaderFromDir(tt.dir)
			require.NoError(t, err)

			p, err := pg.Generate()
			if tt.fail {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.EqualValues(t, tt.packageName, p.Name)
			assert.EqualValues(t, tt.defaultChannel, p.DefaultChannel)
			assert.EqualValues(t, tt.channel, p.Channels)
		})
	}
}
