package semver

import (
	"testing"

	"github.com/blang/semver/v4"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
	"github.com/stretchr/testify/require"
)

func TestLinkChannels(t *testing.T) {
	// type semverRenderedChannelVersions map[string]map[string]semver.Version // e.g. d["stable-v1"]["example-operator/v1.0.0"] = 1.0.0
	channelOperatorVersions := semverRenderedChannelVersions{
		"stable": {
			"a-v0.1.0": semver.MustParse("0.1.0"),
			"a-v0.1.1": semver.MustParse("0.1.1"),
			"a-v1.1.0": semver.MustParse("1.1.0"),
			"a-v1.2.1": semver.MustParse("1.2.1"),
			"a-v1.3.1": semver.MustParse("1.3.1"),
			"a-v2.1.0": semver.MustParse("2.1.0"),
			"a-v2.1.1": semver.MustParse("2.1.1"),
			"a-v2.3.1": semver.MustParse("2.3.1"),
			"a-v2.3.2": semver.MustParse("2.3.2"),
		},
	}
	// map[string]string
	channelNameToKind := map[string]string{
		"stable-v0":   "stable",
		"stable-v1":   "stable",
		"stable-v2":   "stable",
		"stable-v0.1": "stable",
		"stable-v1.1": "stable",
		"stable-v1.2": "stable",
		"stable-v1.3": "stable",
		"stable-v2.1": "stable",
		"stable-v2.3": "stable",
	}

	majorGeneratedUnlinkedChannels := map[string]*declcfg.Channel{
		"stable-v0": {
			Schema:  "olm.channel",
			Name:    "stable-v0",
			Package: "a",
			Entries: []declcfg.ChannelEntry{
				{Name: "a-v0.1.0"},
				{Name: "a-v0.1.1"},
			},
		},
		"stable-v1": {
			Schema:  "olm.channel",
			Name:    "stable-v1",
			Package: "a",
			Entries: []declcfg.ChannelEntry{
				{Name: "a-v1.1.0"},
				{Name: "a-v1.2.1"},
				{Name: "a-v1.3.1"},
			},
		},
		"stable-v2": {
			Schema:  "olm.channel",
			Name:    "stable-v2",
			Package: "a",
			Entries: []declcfg.ChannelEntry{
				{Name: "a-v2.1.0"},
				{Name: "a-v2.1.1"},
				{Name: "a-v2.3.1"},
				{Name: "a-v2.3.2"},
			},
		},
	}

	minorGeneratedUnlinkedChannels := map[string]*declcfg.Channel{
		"stable-v0.1": {
			Schema:  "olm.channel",
			Name:    "stable-v0.1",
			Package: "a",
			Entries: []declcfg.ChannelEntry{
				{Name: "a-v0.1.0"},
				{Name: "a-v0.1.1"},
			},
		},
		"stable-v1.1": {
			Schema:  "olm.channel",
			Name:    "stable-v1.1",
			Package: "a",
			Entries: []declcfg.ChannelEntry{
				{Name: "a-v1.1.0"},
			},
		},
		"stable-v1.2": {
			Schema:  "olm.channel",
			Name:    "stable-v1.2",
			Package: "a",
			Entries: []declcfg.ChannelEntry{
				{Name: "a-v1.2.1"},
			},
		},
		"stable-v1.3": {
			Schema:  "olm.channel",
			Name:    "stable-v1.3",
			Package: "a",
			Entries: []declcfg.ChannelEntry{
				{Name: "a-v1.3.1"},
			},
		},
		"stable-v2.1": {
			Schema:  "olm.channel",
			Name:    "stable-v2.1",
			Package: "a",
			Entries: []declcfg.ChannelEntry{
				{Name: "a-v2.1.0"},
				{Name: "a-v2.1.1"},
			},
		},
		"stable-v2.3": {
			Schema:  "olm.channel",
			Name:    "stable-v2.3",
			Package: "a",
			Entries: []declcfg.ChannelEntry{
				{Name: "a-v2.3.1"},
				{Name: "a-v2.3.2"},
			},
		},
		"stable-v3.1": {
			Schema:  "olm.channel",
			Name:    "stable-v3.1",
			Package: "a",
			Entries: []declcfg.ChannelEntry{
				{Name: "a-v3.1.0"},
				{Name: "a-v3.1.1"},
			},
		},
	}

	tests := []struct {
		name                  string
		unlinkedChannels      map[string]*declcfg.Channel
		avoidSkipPatch        bool
		generateMinorChannels bool
		generateMajorChannels bool
		out                   []declcfg.Channel
	}{
		{
			name:                  "NoSkipPatch/No edges between successive major channels",
			unlinkedChannels:      majorGeneratedUnlinkedChannels,
			avoidSkipPatch:        true,
			generateMinorChannels: false,
			generateMajorChannels: true,
			out: []declcfg.Channel{
				{
					Schema:  "olm.channel",
					Name:    "stable-v0",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v0.1.0", Replaces: ""},
						{Name: "a-v0.1.1", Replaces: "a-v0.1.0"},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v1",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.1.0", Replaces: ""},
						{Name: "a-v1.2.1", Replaces: "a-v1.1.0"},
						{Name: "a-v1.3.1", Replaces: "a-v1.2.1"},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v2",
					Package: "a",
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
			name:                  "SkipPatch/No edges between successive major channels",
			unlinkedChannels:      majorGeneratedUnlinkedChannels,
			avoidSkipPatch:        false,
			generateMinorChannels: false,
			generateMajorChannels: true,
			out: []declcfg.Channel{
				{
					Schema:  "olm.channel",
					Name:    "stable-v0",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v0.1.0", Replaces: ""},
						{Name: "a-v0.1.1", Replaces: "", Skips: []string{"a-v0.1.0"}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v1",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.1.0", Replaces: "", Skips: []string{}},
						{Name: "a-v1.2.1", Replaces: "a-v1.1.0", Skips: []string{}},
						{Name: "a-v1.3.1", Replaces: "a-v1.2.1", Skips: []string{}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v2",
					Package: "a",
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
			name:                  "NoSkipPatch/No edges between minor channels",
			unlinkedChannels:      minorGeneratedUnlinkedChannels,
			avoidSkipPatch:        true,
			generateMinorChannels: true,
			generateMajorChannels: false,
			out: []declcfg.Channel{
				{
					Schema:  "olm.channel",
					Name:    "stable-v0.1",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v0.1.0", Replaces: ""},
						{Name: "a-v0.1.1", Replaces: "a-v0.1.0"},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v1.1",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.1.0", Replaces: ""},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v1.2",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.2.1", Replaces: ""},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v1.3",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.3.1", Replaces: ""},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v2.1",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.1.0", Replaces: ""},
						{Name: "a-v2.1.1", Replaces: "a-v2.1.0"},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v2.3",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.3.1", Replaces: ""},
						{Name: "a-v2.3.2", Replaces: "a-v2.3.1"},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v3.1",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v3.1.0", Replaces: ""},
						{Name: "a-v3.1.1", Replaces: "a-v3.1.0"},
					},
				},
			},
		},
		{
			name:                  "SkipPatch/No edges between minor channels",
			unlinkedChannels:      minorGeneratedUnlinkedChannels,
			avoidSkipPatch:        false,
			generateMinorChannels: true,
			generateMajorChannels: false,
			out: []declcfg.Channel{
				{
					Schema:  "olm.channel",
					Name:    "stable-v0.1",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v0.1.0", Replaces: ""},
						{Name: "a-v0.1.1", Replaces: "", Skips: []string{"a-v0.1.0"}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v1.1",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.1.0", Replaces: "", Skips: []string{}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v1.2",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.2.1", Replaces: "", Skips: []string{}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v1.3",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.3.1", Replaces: "", Skips: []string{}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v2.1",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.1.0", Replaces: ""},
						{Name: "a-v2.1.1", Replaces: "", Skips: []string{"a-v2.1.0"}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v2.3",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.3.1", Replaces: ""},
						{Name: "a-v2.3.2", Replaces: "", Skips: []string{"a-v2.3.1"}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v3.1",
					Package: "a",
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
			// map[string]*declcfg.Channel
			unlinkedChannels := map[string]*declcfg.Channel{}
			for c, e := range tt.unlinkedChannels {
				unlinkedChannels[c] = e
			}
			sv := &semverVeneer{AvoidSkipPatch: tt.avoidSkipPatch, GenerateMajorChannels: tt.generateMajorChannels, GenerateMinorChannels: tt.generateMinorChannels}
			require.ElementsMatch(t, tt.out, sv.linkChannels(unlinkedChannels, "a", &channelOperatorVersions, &channelNameToKind))
		})
	}
}

func TestGenerateChannels(t *testing.T) {
	// type semverRenderedChannelVersions map[string]map[string]semver.Version // e.g. d["stable-v1"]["example-operator/v1.0.0"] = 1.0.0
	channelOperatorVersions := semverRenderedChannelVersions{
		"stable": {
			"a-v0.1.0":       semver.MustParse("0.1.0"),
			"a-v0.1.1":       semver.MustParse("0.1.1"),
			"a-v1.1.0":       semver.MustParse("1.1.0"),
			"a-v1.2.1":       semver.MustParse("1.2.1"),
			"a-v1.3.1":       semver.MustParse("1.3.1"),
			"a-v2.1.0":       semver.MustParse("2.1.0"),
			"a-v1.3.1-beta":  semver.MustParse(("1.3.1-beta")),
			"a-v2.1.1":       semver.MustParse("2.1.1"),
			"a-v2.3.1":       semver.MustParse("2.3.1"),
			"a-v2.3.2":       semver.MustParse("2.3.2"),
			"a-v3.1.0":       semver.MustParse("3.1.0"),
			"a-v3.1.1":       semver.MustParse("3.1.1"),
			"a-v1.3.1-alpha": semver.MustParse("1.3.1-alpha"),
			"a-v1.4.1":       semver.MustParse("1.4.1"),
			"a-v1.4.1-beta1": semver.MustParse("1.4.1-beta1"),
			"a-v1.4.1-beta2": semver.MustParse("1.4.1-beta2"),
		},
	}

	tests := []struct {
		name                  string
		avoidSkipPatch        bool
		generateMinorChannels bool
		generateMajorChannels bool
		out                   []declcfg.Channel
	}{
		{
			name:                  "SkipPatch/No edges between minor channels",
			avoidSkipPatch:        false,
			generateMinorChannels: true,
			generateMajorChannels: false,
			out: []declcfg.Channel{
				{
					Schema:  "olm.channel",
					Name:    "stable-v0.1",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v0.1.0", Replaces: ""},
						{Name: "a-v0.1.1", Replaces: "", Skips: []string{"a-v0.1.0"}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v1.1",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.1.0", Replaces: "", Skips: []string{}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v1.2",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.2.1", Replaces: "", Skips: []string{}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v1.3",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.3.1-alpha", Replaces: ""},
						{Name: "a-v1.3.1-beta", Replaces: ""},
						{Name: "a-v1.3.1", Replaces: "", Skips: []string{"a-v1.3.1-alpha", "a-v1.3.1-beta"}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v1.4",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.4.1-beta1", Replaces: ""},
						{Name: "a-v1.4.1-beta2", Replaces: ""},
						{Name: "a-v1.4.1", Replaces: "", Skips: []string{"a-v1.4.1-beta1", "a-v1.4.1-beta2"}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v2.1",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.1.0", Replaces: ""},
						{Name: "a-v2.1.1", Replaces: "", Skips: []string{"a-v2.1.0"}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v2.3",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.3.1", Replaces: ""},
						{Name: "a-v2.3.2", Replaces: "", Skips: []string{"a-v2.3.1"}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v3.1",
					Package: "a",
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
			sv := &semverVeneer{AvoidSkipPatch: tt.avoidSkipPatch, GenerateMajorChannels: tt.generateMajorChannels, GenerateMinorChannels: tt.generateMinorChannels, pkg: "a"}
			require.ElementsMatch(t, tt.out, sv.generateChannels(&channelOperatorVersions))
		})
	}
}

func TestGetVersionsFromStandardChannel(t *testing.T) {
	tests := []struct {
		name        string
		sv          semverVeneer
		outVersions semverRenderedChannelVersions
		dc          declcfg.DeclarativeConfig
	}{
		{
			name: "sunny day case",
			sv: semverVeneer{
				Stable: stableBundles{
					[]semverVeneerBundleEntry{
						{Image: "repo/origin/a-v0.1.0"},
						{Image: "repo/origin/a-v0.1.1"},
						{Image: "repo/origin/a-v1.1.0"},
						{Image: "repo/origin/a-v1.2.1"},
						{Image: "repo/origin/a-v1.3.1"},
						{Image: "repo/origin/a-v2.1.0"},
						{Image: "repo/origin/a-v2.1.1"},
						{Image: "repo/origin/a-v2.3.1"},
						{Image: "repo/origin/a-v2.3.2"},
						{Image: "repo/origin/a-v1.3.1-alpha"},
					},
				},
			},
			outVersions: semverRenderedChannelVersions{
				"candidate": map[string]semver.Version{},
				"fast":      map[string]semver.Version{},
				"stable": {
					"a-v0.1.0":       semver.MustParse("0.1.0"),
					"a-v0.1.1":       semver.MustParse("0.1.1"),
					"a-v1.1.0":       semver.MustParse("1.1.0"),
					"a-v1.2.1":       semver.MustParse("1.2.1"),
					"a-v1.3.1":       semver.MustParse("1.3.1"),
					"a-v2.1.0":       semver.MustParse("2.1.0"),
					"a-v2.1.1":       semver.MustParse("2.1.1"),
					"a-v2.3.1":       semver.MustParse("2.3.1"),
					"a-v2.3.2":       semver.MustParse("2.3.2"),
					"a-v1.3.1-alpha": semver.MustParse("1.3.1-alpha"),
				},
			},
			dc: declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema: "olm.package",
						Name:   "a",
					},
				},
				Bundles: []declcfg.Bundle{
					{Schema: "olm.bundle", Image: "repo/origin/a-v0.1.0", Name: "a-v0.1.0", Properties: []property.Property{property.MustBuildPackage("a", "0.1.0")}},
					{Schema: "olm.bundle", Image: "repo/origin/a-v0.1.1", Name: "a-v0.1.1", Properties: []property.Property{property.MustBuildPackage("a", "0.1.1")}},
					{Schema: "olm.bundle", Image: "repo/origin/a-v1.1.0", Name: "a-v1.1.0", Properties: []property.Property{property.MustBuildPackage("a", "1.1.0")}},
					{Schema: "olm.bundle", Image: "repo/origin/a-v1.2.1", Name: "a-v1.2.1", Properties: []property.Property{property.MustBuildPackage("a", "1.2.1")}},
					{Schema: "olm.bundle", Image: "repo/origin/a-v1.3.1-alpha", Name: "a-v1.3.1-alpha", Properties: []property.Property{property.MustBuildPackage("a", "1.3.1-alpha")}},
					{Schema: "olm.bundle", Image: "repo/origin/a-v1.3.1", Name: "a-v1.3.1", Properties: []property.Property{property.MustBuildPackage("a", "1.3.1")}},
					{Schema: "olm.bundle", Image: "repo/origin/a-v2.1.0", Name: "a-v2.1.0", Properties: []property.Property{property.MustBuildPackage("a", "2.1.0")}},
					{Schema: "olm.bundle", Image: "repo/origin/a-v2.1.1", Name: "a-v2.1.1", Properties: []property.Property{property.MustBuildPackage("a", "2.1.1")}},
					{Schema: "olm.bundle", Image: "repo/origin/a-v2.3.1", Name: "a-v2.3.1", Properties: []property.Property{property.MustBuildPackage("a", "2.3.1")}},
					{Schema: "olm.bundle", Image: "repo/origin/a-v2.3.2", Name: "a-v2.3.2", Properties: []property.Property{property.MustBuildPackage("a", "2.3.2")}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iosv := tt.sv
			versions, err := iosv.getVersionsFromStandardChannels(&tt.dc)
			require.NoError(t, err)
			require.EqualValues(t, tt.outVersions, *versions)
			require.EqualValues(t, "a", iosv.pkg) // verify that we learned the package name and stashed it in the receiver
		})
	}

}

func TestBailOnVersionBuildMetadata(t *testing.T) {
	sv := semverVeneer{
		Stable: stableBundles{
			[]semverVeneerBundleEntry{
				{Image: "repo/origin/a-v0.1.0"},
				{Image: "repo/origin/a-v0.1.1"},
				{Image: "repo/origin/a-v1.1.0"},
				{Image: "repo/origin/a-v1.2.1"},
				{Image: "repo/origin/a-v1.3.1"},
				{Image: "repo/origin/a-v2.1.0"},
				{Image: "repo/origin/a-v2.1.1"},
				{Image: "repo/origin/a-v2.3.1"},
				{Image: "repo/origin/a-v2.3.2"},
				{Image: "repo/origin/a-v1.3.1-alpha"},
				{Image: "repo/origin/a-v1.3.1-alpha+2001Jan21"},
				{Image: "repo/origin/a-v1.3.1-alpha+2003May06"},
			},
		},
	}

	dc := declcfg.DeclarativeConfig{
		Packages: []declcfg.Package{
			{
				Schema: "olm.package",
				Name:   "a",
			},
		},
		Bundles: []declcfg.Bundle{
			{Schema: "olm.bundle", Image: "repo/origin/a-v0.1.0", Name: "a-v0.1.0", Properties: []property.Property{property.MustBuildPackage("a", "0.1.0")}},
			{Schema: "olm.bundle", Image: "repo/origin/a-v0.1.1", Name: "a-v0.1.1", Properties: []property.Property{property.MustBuildPackage("a", "0.1.1")}},
			{Schema: "olm.bundle", Image: "repo/origin/a-v1.1.0", Name: "a-v1.1.0", Properties: []property.Property{property.MustBuildPackage("a", "1.1.0")}},
			{Schema: "olm.bundle", Image: "repo/origin/a-v1.2.1", Name: "a-v1.2.1", Properties: []property.Property{property.MustBuildPackage("a", "1.2.1")}},
			{Schema: "olm.bundle", Image: "repo/origin/a-v1.3.1-alpha", Name: "a-v1.3.1-alpha", Properties: []property.Property{property.MustBuildPackage("a", "1.3.1-alpha")}},
			{Schema: "olm.bundle", Image: "repo/origin/a-v1.3.1-alpha+2001Jan21", Name: "a-v1.3.1-alpha+2001Jan21", Properties: []property.Property{property.MustBuildPackage("a", "1.3.1-alpha+2001Jan21")}},
			{Schema: "olm.bundle", Image: "repo/origin/a-v1.3.1", Name: "a-v1.3.1", Properties: []property.Property{property.MustBuildPackage("a", "1.3.1")}},
			{Schema: "olm.bundle", Image: "repo/origin/a-v2.1.0", Name: "a-v2.1.0", Properties: []property.Property{property.MustBuildPackage("a", "2.1.0")}},
			{Schema: "olm.bundle", Image: "repo/origin/a-v2.1.1", Name: "a-v2.1.1", Properties: []property.Property{property.MustBuildPackage("a", "2.1.1")}},
			{Schema: "olm.bundle", Image: "repo/origin/a-v2.3.1", Name: "a-v2.3.1", Properties: []property.Property{property.MustBuildPackage("a", "2.3.1")}},
			{Schema: "olm.bundle", Image: "repo/origin/a-v2.3.2", Name: "a-v2.3.2", Properties: []property.Property{property.MustBuildPackage("a", "2.3.2")}},
			{Schema: "olm.bundle", Image: "repo/origin/a-v1.3.1-alpha+2003May06", Name: "a-v1.3.1-alpha+2003May06", Properties: []property.Property{property.MustBuildPackage("a", "1.3.1-alpha+2003May06")}},
		},
	}

	t.Run("Abort on unorderable build metadata version data", func(t *testing.T) {
		_, err := sv.getVersionsFromStandardChannels(&dc)
		require.Error(t, err)
	})
}
