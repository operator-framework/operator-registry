package registry

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBundleGraphLoader(t *testing.T) {
	empty := &AnnotationsFile{}
	alpha := &AnnotationsFile{}
	alpha.Annotations.DefaultChannelName = "alpha"

	tests := []struct {
		name          string
		fail          bool
		graph         Package
		bundle        Bundle
		annotations   *AnnotationsFile
		expectedGraph *Package
		skipPatch     bool
	}{
		{
			name: "Add bundle to head of channels",
			fail: false,
			graph: Package{
				Name:           "etcd",
				DefaultChannel: "alpha",
				Channels: map[string]Channel{
					"alpha": {Head: BundleKey{CsvName: "etcdoperator.v0.9.2", Version: "0.9.2"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {}},
							BundleKey{CsvName: "etcdoperator.v0.9.2", Version: "0.9.2"}: {BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {},
								BundleKey{CsvName: "etcdoperator.v0.9.1"}: {}},
						}},

					"beta": {Head: BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {}},
						}},

					"stable": {Head: BundleKey{CsvName: "etcdoperator.v0.9.2", Version: "0.9.2"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {}},
							BundleKey{CsvName: "etcdoperator.v0.9.2", Version: "0.9.2"}: {BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {},
								BundleKey{CsvName: "etcdoperator.v0.9.1"}: {}},
						}},
				},
			},
			bundle: Bundle{
				Name:    "etcdoperator.v0.9.3",
				Package: "etcd",
				csv: &ClusterServiceVersion{
					Spec: json.RawMessage(`
							{
							"version": "0.9.3"
							}`),
				},
				Channels: []string{"alpha", "stable"},
			},
			expectedGraph: &Package{
				Name:           "etcd",
				DefaultChannel: "alpha",
				Channels: map[string]Channel{
					"alpha": {Head: BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {}},
							BundleKey{CsvName: "etcdoperator.v0.9.2", Version: "0.9.2"}: {BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {},
								BundleKey{CsvName: "etcdoperator.v0.9.1"}: {}},
							BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"}: {BundleKey{CsvName: "etcdoperator.v0.9.2", Version: "0.9.2"}: {}},
						}},

					"beta": {Head: BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {}},
						}},

					"stable": {Head: BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {}},
							BundleKey{CsvName: "etcdoperator.v0.9.2", Version: "0.9.2"}: {BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {},
								BundleKey{CsvName: "etcdoperator.v0.9.1"}: {}},
							BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"}: {BundleKey{CsvName: "etcdoperator.v0.9.2", Version: "0.9.2"}: {}},
						}},
				},
			},
			annotations: empty,
		},
		{
			name: "Add a bundle already in the graph, expect an error",
			fail: true,
			graph: Package{
				Name:           "etcd",
				DefaultChannel: "beta",
				Channels: map[string]Channel{
					"beta": {Head: BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
						}},
				},
			},
			bundle: Bundle{
				Name:    "etcdoperator.v0.6.1",
				Package: "etcd",
				csv: &ClusterServiceVersion{
					Spec: json.RawMessage(`
						{
						"version": "0.6.1"
						}`),
				},
				Channels: []string{"beta"},
			},
			annotations: empty,
		},
		{
			name: "Add a bundle behind the head of a channel",
			fail: false,
			graph: Package{
				Name:           "etcd",
				DefaultChannel: "beta",
				Channels: map[string]Channel{
					"beta": {Head: BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"}: {},
						}},
				},
			},
			bundle: Bundle{
				Name:    "etcdoperator.v0.6.1",
				Package: "etcd",
				csv: &ClusterServiceVersion{
					Spec: json.RawMessage(`
						{
						"version": "0.6.1"
						}`),
				},
				Channels: []string{"beta"},
			},
			expectedGraph: &Package{
				Name:           "etcd",
				DefaultChannel: "beta",
				Channels: map[string]Channel{
					"beta": {Head: BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"}: {BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {}},
						}},
				},
			},
			annotations: empty,
		},
		{
			name: "Add a bundle to a new channel",
			fail: false,
			graph: Package{
				Name:           "etcd",
				DefaultChannel: "beta",
				Channels: map[string]Channel{
					"beta": {Head: BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
						}},
				},
			},
			bundle: Bundle{
				Name:    "etcdoperator.v0.9.3",
				Package: "etcd",
				csv: &ClusterServiceVersion{
					Spec: json.RawMessage(`
						{
						"version": "0.9.3"
						}`),
				},
				Channels: []string{"alpha"},
			},
			expectedGraph: &Package{
				Name:           "etcd",
				DefaultChannel: "alpha",
				Channels: map[string]Channel{
					"beta": {Head: BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
						}},
					"alpha": {Head: BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"}: nil,
						}},
				},
			},
			annotations: alpha,
		},
		{
			name:  "Add a bundle to an empty graph",
			fail:  false,
			graph: Package{},
			bundle: Bundle{
				Name:    "etcdoperator.v0.9.3",
				Package: "etcd",
				csv: &ClusterServiceVersion{
					Spec: json.RawMessage(`
						{
						"version": "0.9.3"
						}`),
				},
				Channels: []string{"alpha"},
			},
			expectedGraph: &Package{
				Name:           "etcd",
				DefaultChannel: "alpha",
				Channels: map[string]Channel{
					"alpha": {Head: BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"}: nil,
						}},
				},
			},
			annotations: alpha,
		},
		{
			name: "Add a bundle in skippatch mode",
			fail: false,
			graph: Package{
				Name:           "etcd",
				DefaultChannel: "alpha",
				Channels: map[string]Channel{
					"alpha": {Head: BundleKey{CsvName: "etcdoperator.v0.9.2", Version: "0.9.2"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {}},
							BundleKey{CsvName: "etcdoperator.v0.9.2", Version: "0.9.2"}: {BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {},
								BundleKey{CsvName: "etcdoperator.v0.9.1"}: {}},
						}},

					"beta": {Head: BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {}},
						}},

					"stable": {Head: BundleKey{CsvName: "etcdoperator.v0.9.2", Version: "0.9.2"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {}},
							BundleKey{CsvName: "etcdoperator.v0.9.2", Version: "0.9.2"}: {BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {},
								BundleKey{CsvName: "etcdoperator.v0.9.1"}: {}},
						}},
				},
			},
			bundle: Bundle{
				Name:    "etcdoperator.v0.9.3",
				Package: "etcd",
				csv: &ClusterServiceVersion{
					Spec: json.RawMessage(`
							{
							"version": "0.9.3"
							}`),
				},
				Channels: []string{"alpha", "stable"},
			},
			skipPatch: true,
			expectedGraph: &Package{
				Name:           "etcd",
				DefaultChannel: "alpha",
				Channels: map[string]Channel{
					"alpha": {Head: BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"}: {BundleKey{CsvName: "etcdoperator.v0.9.2", Version: "0.9.2"}: {},
								BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {},
							},
						}},

					"beta": {Head: BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {}},
						}},

					"stable": {Head: BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"},
						Nodes: map[BundleKey]map[BundleKey]struct{}{
							BundleKey{CsvName: "etcdoperator.v0.6.1", Version: "0.6.1"}: {},
							BundleKey{CsvName: "etcdoperator.v0.9.3", Version: "0.9.3"}: {BundleKey{CsvName: "etcdoperator.v0.9.2", Version: "0.9.2"}: {},
								BundleKey{CsvName: "etcdoperator.v0.9.0", Version: "0.9.0"}: {},
							},
						}},
				},
			},
			annotations: empty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graphLoader := BundleGraphLoader{}

			newGraph, err := graphLoader.AddBundleToGraph(&tt.bundle, &tt.graph, tt.annotations, tt.skipPatch)
			if tt.fail {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.EqualValues(t, tt.expectedGraph.Name, newGraph.Name)
			assert.EqualValues(t, tt.expectedGraph, newGraph)
		})
	}
}
