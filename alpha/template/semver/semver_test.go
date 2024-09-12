package semver

import (
	"fmt"
	"strings"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func TestLinkChannels(t *testing.T) {
	// type bundleVersions map[string]map[string]semver.Version // e.g. d["stable"]["example-operator.v1.0.0"] = 1.0.0
	channelOperatorVersions := bundleVersions{
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

	majorGeneratedUnlinkedChannelsLastXChange := map[string]*declcfg.Channel{
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
			},
		},
	}

	majorGeneratedUnlinkedChannelsLastArchChange := map[string]*declcfg.Channel{
		"candidate-v2": {
			Schema:  "olm.channel",
			Name:    "candidate-v2",
			Package: "a",
			Entries: []declcfg.ChannelEntry{
				{Name: "a-v2.3.2"},
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
			},
		},
	}

	tests := []struct {
		name                  string
		unlinkedChannels      map[string]*declcfg.Channel
		generateMinorChannels bool
		generateMajorChannels bool
		out                   []declcfg.Channel
	}{
		{
			name:                  "No edges between successive major channels",
			unlinkedChannels:      majorGeneratedUnlinkedChannels,
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
						{Name: "a-v1.1.0", Replaces: ""},
						{Name: "a-v1.2.1", Replaces: "a-v1.1.0", Skips: []string{"a-v1.1.0"}},
						{Name: "a-v1.3.1", Replaces: "a-v1.2.1", Skips: []string{"a-v1.2.1"}},
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
						{Name: "a-v2.3.2", Replaces: "a-v2.1.1", Skips: []string{"a-v2.1.1", "a-v2.3.1"}},
					},
				},
			},
		},
		{
			name:                  "No edges between successive major channels where last edge is X change",
			unlinkedChannels:      majorGeneratedUnlinkedChannelsLastXChange,
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
						{Name: "a-v1.1.0", Replaces: ""},
						{Name: "a-v1.2.1", Replaces: "a-v1.1.0", Skips: []string{"a-v1.1.0"}},
						{Name: "a-v1.3.1", Replaces: "a-v1.2.1", Skips: []string{"a-v1.2.1"}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v2",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.1.0", Replaces: ""},
					},
				},
			},
		},
		{
			name:                  "No edges between successive major channels where last edge is archetype change",
			unlinkedChannels:      majorGeneratedUnlinkedChannelsLastArchChange,
			generateMinorChannels: false,
			generateMajorChannels: true,
			out: []declcfg.Channel{
				{
					Schema:  "olm.channel",
					Name:    "candidate-v2",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.3.2", Replaces: ""},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v1",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v1.1.0", Replaces: ""},
						{Name: "a-v1.2.1", Replaces: "a-v1.1.0", Skips: []string{"a-v1.1.0"}},
						{Name: "a-v1.3.1", Replaces: "a-v1.2.1", Skips: []string{"a-v1.2.1"}},
					},
				},
				{
					Schema:  "olm.channel",
					Name:    "stable-v2",
					Package: "a",
					Entries: []declcfg.ChannelEntry{
						{Name: "a-v2.1.0", Replaces: ""},
						{Name: "a-v2.1.1", Replaces: "", Skips: []string{"a-v2.1.0"}},
						{Name: "a-v2.3.1", Replaces: "a-v2.1.1", Skips: []string{"a-v2.1.1"}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := &semverTemplate{pkg: "a", GenerateMajorChannels: tt.generateMajorChannels, GenerateMinorChannels: tt.generateMinorChannels}
			require.ElementsMatch(t, tt.out, sv.linkChannels(tt.unlinkedChannels, &channelOperatorVersions))
		})
	}
}

func TestGenerateChannels(t *testing.T) {
	// type bundleVersions map[string]map[string]semver.Version // e.g. d["stable"]["example-operator.v1.0.0"] = 1.0.0
	channelOperatorVersions := bundleVersions{
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

	majorLinkedChannels := []declcfg.Channel{
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
				{Name: "a-v1.1.0", Replaces: ""},
				{Name: "a-v1.2.1", Replaces: "a-v1.1.0", Skips: []string{"a-v1.1.0"}},
				{Name: "a-v1.3.1-alpha", Replaces: ""},
				{Name: "a-v1.3.1-beta", Replaces: ""},
				{Name: "a-v1.3.1", Replaces: "a-v1.2.1", Skips: []string{"a-v1.2.1", "a-v1.3.1-alpha", "a-v1.3.1-beta"}},
				{Name: "a-v1.4.1-beta1", Replaces: ""},
				{Name: "a-v1.4.1-beta2", Replaces: ""},
				{Name: "a-v1.4.1", Replaces: "a-v1.3.1", Skips: []string{"a-v1.3.1", "a-v1.4.1-beta1", "a-v1.4.1-beta2"}},
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
				{Name: "a-v2.3.2", Replaces: "a-v2.1.1", Skips: []string{"a-v2.1.1", "a-v2.3.1"}},
			},
		},
		{
			Schema:  "olm.channel",
			Name:    "stable-v3",
			Package: "a",
			Entries: []declcfg.ChannelEntry{
				{Name: "a-v3.1.0", Replaces: ""},
				{Name: "a-v3.1.1", Replaces: "", Skips: []string{"a-v3.1.0"}},
			},
		},
	}

	minorLinkedChannels := []declcfg.Channel{
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
	}

	var combinedLinkedChannels []declcfg.Channel
	combinedLinkedChannels = append(combinedLinkedChannels, minorLinkedChannels...)
	combinedLinkedChannels = append(combinedLinkedChannels, majorLinkedChannels...)

	tests := []struct {
		name                  string
		generateMinorChannels bool
		generateMajorChannels bool
		defaultChannel        string
		channelTypePreference streamType
		out                   []declcfg.Channel
	}{
		{
			name:                  "Edges between minor channels",
			generateMinorChannels: true,
			generateMajorChannels: false,
			defaultChannel:        "stable-v3.1",
			channelTypePreference: minorStreamType,
			out:                   minorLinkedChannels,
		},
		{
			name:                  "No edges between major channels",
			generateMinorChannels: false,
			generateMajorChannels: true,
			defaultChannel:        "stable-v3",
			channelTypePreference: majorStreamType,
			out:                   majorLinkedChannels,
		},
		{
			name:                  "Preference for minor default channel",
			generateMinorChannels: true,
			generateMajorChannels: true,
			defaultChannel:        "stable-v3.1",
			channelTypePreference: minorStreamType,
			out:                   combinedLinkedChannels,
		},
		{
			name:                  "Preference for major default channel",
			generateMinorChannels: true,
			generateMajorChannels: true,
			defaultChannel:        "stable-v3",
			channelTypePreference: majorStreamType,
			out:                   combinedLinkedChannels,
		},
		{
			name:                  "Mismatch generate/preference minor/major default channel",
			generateMinorChannels: true,
			generateMajorChannels: false,
			defaultChannel:        "stable-v3.1",
			channelTypePreference: majorStreamType,
			out:                   minorLinkedChannels,
		},
		{
			name:                  "Mismatch generate/preference major/minor default channel",
			generateMinorChannels: false,
			generateMajorChannels: true,
			defaultChannel:        "stable-v3",
			channelTypePreference: minorStreamType,
			out:                   majorLinkedChannels,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := &semverTemplate{GenerateMajorChannels: tt.generateMajorChannels, GenerateMinorChannels: tt.generateMinorChannels, pkg: "a", DefaultChannelTypePreference: tt.channelTypePreference}
			out := sv.generateChannels(&channelOperatorVersions)
			require.ElementsMatch(t, tt.out, out)
			require.Equal(t, tt.defaultChannel, sv.defaultChannel)
		})
	}
}

func TestGetVersionsFromStandardChannel(t *testing.T) {
	tests := []struct {
		name        string
		sv          semverTemplate
		outVersions bundleVersions
		dc          declcfg.DeclarativeConfig
	}{
		{
			name: "sunny day case",
			sv: semverTemplate{
				Stable: semverTemplateChannelBundles{
					[]semverTemplateBundleEntry{
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
			outVersions: bundleVersions{
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
			versions, err := iosv.getVersionsFromStandardChannels(&tt.dc, buildBundleList(tt.sv))
			require.NoError(t, err)
			require.EqualValues(t, tt.outVersions, *versions)
			require.EqualValues(t, "a", iosv.pkg) // verify that we learned the package name and stashed it in the receiver
		})
	}

}

func TestBailOnVersionBuildMetadata(t *testing.T) {
	sv := semverTemplate{
		Stable: semverTemplateChannelBundles{
			[]semverTemplateBundleEntry{
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
		_, err := sv.getVersionsFromStandardChannels(&dc, buildBundleList(sv))
		require.Error(t, err)
	})
}

func TestReadFile(t *testing.T) {

	templateFstr := `---
schema: olm.semver
generateMajorChannels: %s
generateMinorChannels: %s
defaultChannelTypePreference: %s
candidate:
    bundles:
        - image: quay.io/foo/olm:testoperator.v0.1.0
        - image: quay.io/foo/olm:testoperator.v0.1.1
        - image: quay.io/foo/olm:testoperator.v0.1.2
        - image: quay.io/foo/olm:testoperator.v0.1.3
        - image: quay.io/foo/olm:testoperator.v0.2.0
        - image: quay.io/foo/olm:testoperator.v0.2.1
        - image: quay.io/foo/olm:testoperator.v0.2.2
        - image: quay.io/foo/olm:testoperator.v0.3.0
        - image: quay.io/foo/olm:testoperator.v1.0.0
        - image: quay.io/foo/olm:testoperator.v1.0.1
        - image: quay.io/foo/olm:testoperator.v1.1.0
fast:
    bundles:
        - image: quay.io/foo/olm:testoperator.v0.2.1
        - image: quay.io/foo/olm:testoperator.v0.2.2
        - image: quay.io/foo/olm:testoperator.v0.3.0
        - image: quay.io/foo/olm:testoperator.v1.0.1
        - image: quay.io/foo/olm:testoperator.v1.1.0
stable:
    bundles:
        - image: quay.io/foo/olm:testoperator.v1.0.1
`

	type testCase struct {
		name       string
		input      string
		assertions func(*testing.T, *semverTemplate, error)
	}
	testCases := []testCase{
		{
			name:  "valid",
			input: fmt.Sprintf(templateFstr, "true", "true", "minor"),
			assertions: func(t *testing.T, template *semverTemplate, err error) {
				require.NotNil(t, template)
				require.NoError(t, err)
			},
		},
		{
			name: "unknown channel prefix",
			input: `---
schema: olm.semver
generateMajorChannels: true
generateMinorChannels: true
candidate:
    bundles:
        - image: quay.io/foo/olm:testoperator.v0.1.0
        - image: quay.io/foo/olm:testoperator.v0.1.1
        - image: quay.io/foo/olm:testoperator.v0.1.2
        - image: quay.io/foo/olm:testoperator.v0.1.3
        - image: quay.io/foo/olm:testoperator.v0.2.0
        - image: quay.io/foo/olm:testoperator.v0.2.1
        - image: quay.io/foo/olm:testoperator.v0.2.2
        - image: quay.io/foo/olm:testoperator.v0.3.0
        - image: quay.io/foo/olm:testoperator.v1.0.0
        - image: quay.io/foo/olm:testoperator.v1.0.1
        - image: quay.io/foo/olm:testoperator.v1.1.0
fast:
    bundles:
        - image: quay.io/foo/olm:testoperator.v0.2.1
        - image: quay.io/foo/olm:testoperator.v0.2.2
        - image: quay.io/foo/olm:testoperator.v0.3.0
        - image: quay.io/foo/olm:testoperator.v1.0.1
        - image: quay.io/foo/olm:testoperator.v1.1.0
stable:
    bundles:
        - image: quay.io/foo/olm:testoperator.v1.0.1
invalid:
    bundles:
        - image: quay.io/foo/olm:testoperator.v1.0.1
`,
			assertions: func(t *testing.T, template *semverTemplate, err error) {
				require.Nil(t, template)
				require.EqualError(t, err, `error unmarshaling JSON: while decoding JSON: json: unknown field "invalid"`)
			},
		},
		{
			name:  "generate/default mismatch, minor/major",
			input: fmt.Sprintf(templateFstr, "true", "false", "minor"),
			assertions: func(t *testing.T, template *semverTemplate, err error) {
				require.Nil(t, template)
				require.ErrorContains(t, err, "schema attribute mismatch")
			},
		},
		{
			name:  "generate/default mismatch, major/minor",
			input: fmt.Sprintf(templateFstr, "false", "true", "major"),
			assertions: func(t *testing.T, template *semverTemplate, err error) {
				require.Nil(t, template)
				require.ErrorContains(t, err, "schema attribute mismatch")
			},
		},
		{
			name:  "unknown defaultchanneltypepreference",
			input: fmt.Sprintf(templateFstr, "false", "true", "foo"),
			assertions: func(t *testing.T, template *semverTemplate, err error) {
				require.Nil(t, template)
				require.ErrorContains(t, err, "unknown DefaultChannelTypePreference")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sv, err := readFile(strings.NewReader(tc.input))
			tc.assertions(t, sv, err)
		})
	}
}
