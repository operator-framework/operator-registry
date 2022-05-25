package semver

import (
	"fmt"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/stretchr/testify/require"
)

func TestAddChannels(t *testing.T) {
	tests := []struct {
		name           string
		data           map[string][]string
		avoidSkipPatch bool
		out            []declcfg.Channel
	}{
		{
			name: "NoSkipPatch/No edges between successive major channels",
			data: map[string][]string{
				"a-v0": {"0.1.0", "0.1.1"},
				"a-v1": {"1.1.0", "1.2.1", "1.3.1"},
				"a-v2": {"2.1.0", "2.1.1", "2.3.1", "2.3.2"},
			},
			avoidSkipPatch: true,
			out: []declcfg.Channel{
				{
					Schema: "olm.channel",
					Name:   "a-v0",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v0.1.0", Replaces: ""},
						{Name: "a-v0.1.1", Replaces: "a-v0.1.0"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v1",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.1.0", Replaces: ""},
						{Name: "a-v1.2.1", Replaces: "a-v1.1.0"},
						{Name: "a-v1.3.1", Replaces: "a-v1.2.1"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v2",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.1.0", Replaces: ""},
						{Name: "a-v2.1.1", Replaces: "a-v2.1.0"},
						{Name: "a-v2.3.1", Replaces: "a-v2.1.1"},
						{Name: "a-v2.3.2", Replaces: "a-v2.3.1"},
					},
				},
			},
		},
		{
			name: "SkipPatch/No edges between successive major channels",
			data: map[string][]string{
				"a-v0": {"0.1.0", "0.1.1"},
				"a-v1": {"1.1.0", "1.2.1", "1.3.1"},
				"a-v2": {"2.1.0", "2.1.1", "2.3.1", "2.3.2"},
			},
			avoidSkipPatch: false,
			out: []declcfg.Channel{
				{
					Schema: "olm.channel",
					Name:   "a-v0",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v0.1.0", Replaces: ""},
						{Name: "a-v0.1.1", Replaces: "", Skips: []string{"a-v0.1.0"}},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v1",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.1.0", Replaces: ""},
						{Name: "a-v1.2.1", Replaces: "a-v1.1.0"},
						{Name: "a-v1.3.1", Replaces: "a-v1.2.1"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v2",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.1.0", Replaces: ""},
						{Name: "a-v2.1.1", Replaces: "", Skips: []string{"a-v2.1.0"}},
						{Name: "a-v2.3.1", Replaces: ""},
						{Name: "a-v2.3.2", Replaces: "a-v2.1.1", Skips: []string{"a-v2.3.1"}},
					},
				},
			},
		},
		{
			name: "NoSkipPatch/edges between minor channels",
			data: map[string][]string{
				"a-v0":   {"0.1.0", "0.1.1"},
				"a-v0.1": {"0.1.0", "0.1.1"},
				"a-v1":   {"1.1.0", "1.2.1", "1.3.1"},
				"a-v1.1": {"1.1.0"},
				"a-v1.2": {"1.2.1"},
				"a-v1.3": {"1.3.1"},
				"a-v2":   {"2.1.0", "2.1.1", "2.3.1", "2.3.2"},
				"a-v2.1": {"2.1.0", "2.1.1"},
				"a-v2.3": {"2.3.1", "2.3.2"},
				"a-v3":   {"3.1.0", "3.1.1"},
				"a-v3.1": {"3.1.0", "3.1.1"},
			},
			avoidSkipPatch: true,
			out: []declcfg.Channel{
				{
					Schema: "olm.channel",
					Name:   "a-v0",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v0.1.0", Replaces: ""},
						{Name: "a-v0.1.1", Replaces: "a-v0.1.0"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v0.1",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v0.1.0", Replaces: ""},
						{Name: "a-v0.1.1", Replaces: "a-v0.1.0"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v1",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.1.0", Replaces: ""},
						{Name: "a-v1.2.1", Replaces: "a-v1.1.0"},
						{Name: "a-v1.3.1", Replaces: "a-v1.2.1"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v1.1",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.1.0", Replaces: ""},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v1.2",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.2.1", Replaces: "a-v1.1.0"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v1.3",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.3.1", Replaces: "a-v1.2.1"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v2",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.1.0", Replaces: ""},
						{Name: "a-v2.1.1", Replaces: "a-v2.1.0"},
						{Name: "a-v2.3.1", Replaces: "a-v2.1.1"},
						{Name: "a-v2.3.2", Replaces: "a-v2.3.1"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v2.1",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.1.0", Replaces: ""},
						{Name: "a-v2.1.1", Replaces: "a-v2.1.0"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v2.3",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.3.1", Replaces: "a-v2.1.1"},
						{Name: "a-v2.3.2", Replaces: "a-v2.3.1"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v3",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v3.1.0", Replaces: ""},
						{Name: "a-v3.1.1", Replaces: "a-v3.1.0"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v3.1",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v3.1.0", Replaces: ""},
						{Name: "a-v3.1.1", Replaces: "a-v3.1.0"},
					},
				},
			},
		},
		{
			name: "SkipPatch/edges between minor channels",
			data: map[string][]string{
				"a-v0":   {"0.1.0", "0.1.1"},
				"a-v0.1": {"0.1.0", "0.1.1"},
				"a-v1":   {"1.1.0", "1.2.1", "1.3.1"},
				"a-v1.1": {"1.1.0"},
				"a-v1.2": {"1.2.1"},
				"a-v1.3": {"1.3.1"},
				"a-v2":   {"2.1.0", "2.1.1", "2.3.1", "2.3.2"},
				"a-v2.1": {"2.1.0", "2.1.1"},
				"a-v2.3": {"2.3.1", "2.3.2"},
				"a-v3":   {"3.1.0", "3.1.1"},
				"a-v3.1": {"3.1.0", "3.1.1"},
			},
			avoidSkipPatch: false,
			out: []declcfg.Channel{
				{
					Schema: "olm.channel",
					Name:   "a-v0",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v0.1.0", Replaces: ""},
						{Name: "a-v0.1.1", Replaces: "", Skips: []string{"a-v0.1.0"}},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v0.1",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v0.1.0", Replaces: ""},
						{Name: "a-v0.1.1", Replaces: "", Skips: []string{"a-v0.1.0"}},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v1",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.1.0", Replaces: ""},
						{Name: "a-v1.2.1", Replaces: "a-v1.1.0"},
						{Name: "a-v1.3.1", Replaces: "a-v1.2.1"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v1.1",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.1.0", Replaces: ""},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v1.2",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.2.1", Replaces: "a-v1.1.0"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v1.3",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.3.1", Replaces: "a-v1.2.1"},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v2",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.1.0", Replaces: ""},
						{Name: "a-v2.1.1", Replaces: "", Skips: []string{"a-v2.1.0"}},
						{Name: "a-v2.3.1", Replaces: ""},
						{Name: "a-v2.3.2", Replaces: "a-v2.1.1", Skips: []string{"a-v2.3.1"}},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v2.1",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.1.0", Replaces: ""},
						{Name: "a-v2.1.1", Replaces: "", Skips: []string{"a-v2.1.0"}},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v2.3",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.3.1", Replaces: ""},
						{Name: "a-v2.3.2", Replaces: "a-v2.1.1", Skips: []string{"a-v2.3.1"}},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v3",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v3.1.0", Replaces: ""},
						{Name: "a-v3.1.1", Replaces: "", Skips: []string{"a-v3.1.0"}},
					},
				},
				{
					Schema: "olm.channel",
					Name:   "a-v3.1",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v3.1.0", Replaces: ""},
						{Name: "a-v3.1.1", Replaces: "", Skips: []string{"a-v3.1.0"}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string][]*decomposedBundleEntry{}
			for c, e := range tt.data {
				data[c] = []*decomposedBundleEntry{}
				for _, v := range e {
					ver, err := semver.ParseTolerant(v)
					require.NoError(t, err, fmt.Errorf("error processing version %s for channel %s in test %s: %v", v, c, tt.name, err))
					data[c] = append(data[c], &decomposedBundleEntry{img: fmt.Sprintf("a-v%s", v), ver: ver})
				}
			}
			sv := &semverVeneer{AvoidSkipPatch: tt.avoidSkipPatch}
			require.ElementsMatch(t, tt.out, sv.addChannels(data, ""))
		})
	}
}
