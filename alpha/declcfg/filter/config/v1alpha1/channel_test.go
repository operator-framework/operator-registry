package v1alpha1

import (
	"bytes"
	"testing"

	mmsemver "github.com/Masterminds/semver/v3"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

func TestChannel_NewChannel(t *testing.T) {
	type testCase struct {
		name      string
		in        declcfg.Channel
		assertion func(*testing.T, *channel, error)
	}
	testCases := []testCase{
		{
			name: "no entries",
			in:   declcfg.Channel{},
			assertion: func(t *testing.T, actual *channel, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, `channel has no entries`)
			},
		},
		{
			name: "single entry",
			in:   declcfg.Channel{Entries: []declcfg.ChannelEntry{{Name: "foo.v1.0.0"}}},
			assertion: func(t *testing.T, actual *channel, err error) {
				assert.Equal(t, &channel{head: &channelEntry{Name: "foo.v1.0.0"}}, actual)
				assert.NoError(t, err)
			},
		},
		{
			name: "multiple entries",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.0.0"},
				{Name: "foo.v2.0.0", Replaces: "foo.v1.0.0"},
			}},
			assertion: func(t *testing.T, actual *channel, err error) {
				assert.Equal(t, &channel{head: &channelEntry{Name: "foo.v2.0.0", Replaces: &channelEntry{Name: "foo.v1.0.0"}}}, actual)
				assert.NoError(t, err)
			},
		},
		{
			name: "multiple heads",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.0.0"},
				{Name: "foo.v2.0.0"},
			}},
			assertion: func(t *testing.T, actual *channel, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, `multiple channel heads found: [foo.v1.0.0 foo.v2.0.0]`)
			},
		},
		{
			name: "multiple heads with replaces",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.0.0"},
				{Name: "foo.v1.1.0", Replaces: "foo.v1.0.0"},
				{Name: "foo.v2.0.0"},
				{Name: "foo.v2.1.0", Replaces: "foo.v2.0.0"},
			}},
			assertion: func(t *testing.T, actual *channel, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, `multiple channel heads found: [foo.v1.1.0 foo.v2.1.0]`)
			},
		},
		{
			name: "replaces and skips",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.0.0"},
				{Name: "foo.v1.1.0"},
				{Name: "foo.v1.1.1"},
				{Name: "foo.v1.1.2", Replaces: "foo.v1.0.0", Skips: []string{"foo.v1.1.1", "foo.v1.1.0"}},
				{Name: "foo.v2.0.0"},
				{Name: "foo.v2.0.1"},
				{Name: "foo.v2.0.2", Replaces: "foo.v1.1.2", Skips: []string{"foo.v2.0.1", "foo.v2.0.0"}},
			}},
			assertion: func(t *testing.T, actual *channel, err error) {
				require.NotNil(t, actual.head)

				// foo.v2.0.2
				assert.Equal(t, "foo.v2.0.2", actual.head.Name)
				assert.Equal(t, sets.New[string]("foo.v2.0.1", "foo.v2.0.0"), channelEntrySetToNames(actual.head.Skips))
				require.NotNil(t, actual.head.Replaces)

				// foo.v1.1.2
				assert.Equal(t, "foo.v1.1.2", actual.head.Replaces.Name)
				assert.Equal(t, sets.New[string]("foo.v1.1.1", "foo.v1.1.0"), channelEntrySetToNames(actual.head.Replaces.Skips))
				require.NotNil(t, actual.head.Replaces.Replaces)

				// foo.v1.0.0
				assert.Equal(t, "foo.v1.0.0", actual.head.Replaces.Replaces.Name)
				assert.Nil(t, actual.head.Replaces.Replaces.Skips)
				assert.Nil(t, actual.head.Replaces.Replaces.Replaces)

				assert.NoError(t, err)
			},
		},
		{
			name: "long replaces chain",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.0.0"},
				{Name: "foo.v1.1.0", Replaces: "foo.v1.0.0"},
				{Name: "foo.v1.2.0", Replaces: "foo.v1.1.0"},
				{Name: "foo.v1.3.0", Replaces: "foo.v1.2.0"},
				{Name: "foo.v1.4.0", Replaces: "foo.v1.3.0"},
				{Name: "foo.v1.5.0", Replaces: "foo.v1.4.0"},
				{Name: "foo.v1.6.0", Replaces: "foo.v1.5.0"},
			}},
			assertion: func(t *testing.T, actual *channel, err error) {
				assert.Equal(t, &channel{head: &channelEntry{
					Name: "foo.v1.6.0",
					Replaces: &channelEntry{
						Name: "foo.v1.5.0",
						Replaces: &channelEntry{
							Name: "foo.v1.4.0",
							Replaces: &channelEntry{
								Name: "foo.v1.3.0",
								Replaces: &channelEntry{
									Name: "foo.v1.2.0",
									Replaces: &channelEntry{
										Name: "foo.v1.1.0",
										Replaces: &channelEntry{
											Name: "foo.v1.0.0",
										},
									},
								},
							},
						},
					},
				},
				}, actual)
				assert.NoError(t, err)
			},
		},
		{
			name: "multiple heads replace same bundle",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.0.0"},
				{Name: "foo.v1.1.0", Replaces: "foo.v1.0.0"},
				{Name: "foo.v1.2.0", Replaces: "foo.v1.0.0"},
			}},
			assertion: func(t *testing.T, actual *channel, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, `multiple channel heads found: [foo.v1.1.0 foo.v1.2.0]`)
			},
		},
		{
			name: "duplicate channel entries",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.0.0"},
				{Name: "foo.v1.1.0", Replaces: "foo.v1.0.0"},
				{Name: "foo.v1.0.0"},
				{Name: "foo.v1.1.0"},
			}},
			assertion: func(t *testing.T, actual *channel, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, `duplicate channel entry "foo.v1.0.0"`)
				assert.ErrorContains(t, err, `duplicate channel entry "foo.v1.1.0"`)
			},
		},
		{
			name: "replace yourself",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.0.0", Replaces: "foo.v1.0.0"},
				{Name: "foo.v1.1.0", Replaces: "foo.v1.0.0"},
			}},
			assertion: func(t *testing.T, actual *channel, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, `replaces itself`)
			},
		},
		{
			name: "skip yourself",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.0.0", Skips: []string{"foo.v1.0.0"}},
				{Name: "foo.v1.1.0", Replaces: "foo.v1.0.0"},
			}},
			assertion: func(t *testing.T, actual *channel, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, `skips itself`)
			},
		},
		{
			name: "replaces cycle",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.0.0", Replaces: "foo.v1.2.0"},
				{Name: "foo.v1.1.0", Replaces: "foo.v1.0.0"},
				{Name: "foo.v1.2.0", Replaces: "foo.v1.1.0"},
				{Name: "foo.v1.3.0", Replaces: "foo.v1.2.0"},
			}},
			assertion: func(t *testing.T, actual *channel, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, `detected a cycle in the upgrade graph of the channel`)
			},
		},
		{
			name: "replaces then skips cycle",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.0.0", Skips: []string{"foo.v1.2.0"}},
				{Name: "foo.v1.1.0", Replaces: "foo.v1.0.0"},
				{Name: "foo.v1.2.0", Replaces: "foo.v1.1.0"},
				{Name: "foo.v2.0.0"},
			}},
			assertion: func(t *testing.T, actual *channel, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, `detected a cycle in the upgrade graph of the channel`)
			},
		},
		{
			name: "skips then replaces cycle",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.0.0", Replaces: "foo.v1.2.0"},
				{Name: "foo.v1.1.0", Replaces: "foo.v1.0.0"},
				{Name: "foo.v1.2.0", Skips: []string{"foo.v1.1.0"}},
				{Name: "foo.v2.0.0"},
			}},
			assertion: func(t *testing.T, actual *channel, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, `detected a cycle in the upgrade graph of the channel`)
			},
		},
		{
			name: "skip each other",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.0.0", Skips: []string{"foo.v1.2.0"}},
				{Name: "foo.v1.2.0", Skips: []string{"foo.v1.0.0"}},
				{Name: "foo.v2.0.0"},
			}},
			assertion: func(t *testing.T, actual *channel, err error) {
				assert.Nil(t, actual)
				assert.Error(t, err)
				assert.ErrorContains(t, err, `detected a cycle in the upgrade graph of the channel`)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := newChannel(tc.in, nil)
			tc.assertion(t, out, err)
		})
	}
}

func channelEntrySetToNames(s sets.Set[*channelEntry]) sets.Set[string] {
	names := sets.New[string]()
	for e := range s {
		names.Insert(e.Name)
	}
	return names
}

func TestChannel_FilterByVersionRange(t *testing.T) {
	type testCase struct {
		name             string
		in               declcfg.Channel
		versionRange     string
		versionMap       map[string]*mmsemver.Version
		expected         []string
		expectedWarnings []string
	}
	testCases := []testCase{
		{
			name: "single entry",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.0.0"},
			}},
			versionRange: ">=1.0.0 <2.0.0",
			versionMap: map[string]*mmsemver.Version{
				"foo.v1.0.0": mmsemver.MustParse("1.0.0"),
			},
			expected: []string{"foo.v1.0.0"},
		},
		{
			name: "single entry, out of range",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v2.0.0"},
			}},
			versionRange: ">=1.0.0 <2.0.0",
			versionMap: map[string]*mmsemver.Version{
				"foo.v2.0.0": mmsemver.MustParse("2.0.0"),
			},
			expected: []string{},
		},
		{
			name: "include head, but not tail",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.1.0", Replaces: "foo.v1.0.0"},
				{Name: "foo.v1.0.0", Replaces: "foo.v0.1.0"},
				{Name: "foo.v0.1.0"},
			}},
			versionRange: ">=1.0.0 <2.0.0",
			versionMap: map[string]*mmsemver.Version{
				"foo.v1.1.0": mmsemver.MustParse("1.1.0"),
				"foo.v1.0.0": mmsemver.MustParse("1.0.0"),
				"foo.v0.1.0": mmsemver.MustParse("0.1.0"),
			},
			expected: []string{"foo.v1.0.0", "foo.v1.1.0"},
		},
		{
			name: "include tail, but not head",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v2.0.0", Replaces: "foo.v1.1.0"},
				{Name: "foo.v1.1.0", Replaces: "foo.v1.0.0"},
				{Name: "foo.v1.0.0"},
			}},
			versionRange: ">=1.0.0 <2.0.0",
			versionMap: map[string]*mmsemver.Version{
				"foo.v2.0.0": mmsemver.MustParse("2.0.0"),
				"foo.v1.1.0": mmsemver.MustParse("1.1.0"),
				"foo.v1.0.0": mmsemver.MustParse("1.0.0"),
			},
			expected: []string{"foo.v1.0.0", "foo.v1.1.0"},
		},
		{
			name: "neither head nor tail",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v2.0.0", Replaces: "foo.v1.1.0"},
				{Name: "foo.v1.1.0", Replaces: "foo.v1.0.0"},
				{Name: "foo.v1.0.0", Replaces: "foo.v0.1.0"},
				{Name: "foo.v0.1.0"},
			}},
			versionRange: ">=1.0.0 <2.0.0",
			versionMap: map[string]*mmsemver.Version{
				"foo.v2.0.0": mmsemver.MustParse("2.0.0"),
				"foo.v1.1.0": mmsemver.MustParse("1.1.0"),
				"foo.v1.0.0": mmsemver.MustParse("1.0.0"),
				"foo.v0.1.0": mmsemver.MustParse("0.1.0"),
			},
			expected: []string{"foo.v1.0.0", "foo.v1.1.0"},
		},
		{
			// This case is definitely possible in practice, but produces a surprising result. The algorithm does not
			// consider skipped bundles as possible new heads because they are not on the replaces chain. Therefore,
			// even though foo.v1.0.0 is in the version range and could be a viable channel head on its own, we also
			// include foo.v2.0.0 because it is the only bundle on the replaces chain.
			//
			// This is a limitation of the algorithm and may be addressed in the future. Making the algorithm produce
			// the expected result would require a more complex algorithm that considers skipped bundles as potential
			// heads.
			name: "head out-of-range, one skip in range",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v2.0.0", Skips: []string{"foo.v1.0.0"}},
				{Name: "foo.v1.0.0"},
			}},
			versionRange: ">=1.0.0 <2.0.0",
			versionMap: map[string]*mmsemver.Version{
				"foo.v2.0.0": mmsemver.MustParse("2.0.0"),
				"foo.v1.0.0": mmsemver.MustParse("1.0.0"),
			},
			expected: []string{"foo.v1.0.0", "foo.v2.0.0"},
		},
		{
			name: "include head outside of range",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v2.0.0", Replaces: "foo.v1.1.1", Skips: []string{"foo.v1.1.0"}},
				{Name: "foo.v1.1.1"},
				{Name: "foo.v1.1.0"},
			}},
			versionRange: ">=1.0.0 <2.0.0",
			versionMap: map[string]*mmsemver.Version{
				"foo.v2.0.0": mmsemver.MustParse("2.0.0"),
				"foo.v1.1.1": mmsemver.MustParse("1.1.1"),
				"foo.v1.1.0": mmsemver.MustParse("1.1.0"),
			},
			expected: []string{"foo.v1.1.0", "foo.v1.1.1", "foo.v2.0.0"},
			expectedWarnings: []string{
				`including bundle "foo.v2.0.0" with version "2.0.0": it falls outside the specified range of ">=1.0.0 <2.0.0" but is required to ensure inclusion of all bundles in the range`,
			},
		},
		{
			name: "include intermediate outside of range",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.1.0", Replaces: "foo.v2.0.0"},
				{Name: "foo.v2.0.0", Replaces: "foo.v1.0.0"},
				{Name: "foo.v1.0.0", Replaces: "foo.v0.1.0"},
				{Name: "foo.v0.1.0"},
			}},
			versionRange: ">=1.0.0 <2.0.0",
			versionMap: map[string]*mmsemver.Version{
				"foo.v2.0.0": mmsemver.MustParse("2.0.0"),
				"foo.v1.1.0": mmsemver.MustParse("1.1.0"),
				"foo.v1.0.0": mmsemver.MustParse("1.0.0"),
				"foo.v0.1.0": mmsemver.MustParse("0.1.0"),
			},
			expected: []string{"foo.v1.0.0", "foo.v1.1.0", "foo.v2.0.0"},
			expectedWarnings: []string{
				`including bundle "foo.v2.0.0" with version "2.0.0": it falls outside the specified range of ">=1.0.0 <2.0.0" but is required to ensure inclusion of all bundles in the range`,
			},
		},
		{
			name: "include unversioned head outside of range",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v2.0.0", Replaces: "foo.v1.1.1", Skips: []string{"foo.v1.1.0"}},
				{Name: "foo.v1.1.1"},
				{Name: "foo.v1.1.0"},
			}},
			versionRange: ">=1.0.0 <2.0.0",
			versionMap: map[string]*mmsemver.Version{
				"foo.v1.1.1": mmsemver.MustParse("1.1.1"),
				"foo.v1.1.0": mmsemver.MustParse("1.1.0"),
			},
			expected: []string{"foo.v1.1.0", "foo.v1.1.1", "foo.v2.0.0"},
			expectedWarnings: []string{
				`including bundle "foo.v2.0.0": it is unversioned but is required to ensure inclusion of all bundles in the range`,
			},
		},
		{
			name: "minimize set of included bundles out of range",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.1.3", Replaces: "foo.v1.1.2", Skips: []string{"foo.v1.1.0"}},
				{Name: "foo.v1.1.2", Replaces: "foo.v1.1.1", Skips: []string{"foo.v1.1.0"}},
				{Name: "foo.v1.1.1", Skips: []string{"foo.v1.1.0"}},
				{Name: "foo.v1.1.0"},
			}},
			versionRange: ">=1.1.0 <=1.1.1",
			versionMap: map[string]*mmsemver.Version{
				"foo.v1.1.3": mmsemver.MustParse("1.1.3"),
				"foo.v1.1.2": mmsemver.MustParse("1.1.2"),
				"foo.v1.1.1": mmsemver.MustParse("1.1.1"),
				"foo.v1.1.0": mmsemver.MustParse("1.1.0"),
			},
			expected: []string{"foo.v1.1.0", "foo.v1.1.1"},
		},
		{
			name: "include unversioned intermediate outside of range",
			in: declcfg.Channel{Entries: []declcfg.ChannelEntry{
				{Name: "foo.v1.1.0", Replaces: "foo.v2.0.0"},
				{Name: "foo.v2.0.0", Replaces: "foo.v1.0.0"},
				{Name: "foo.v1.0.0", Replaces: "foo.v0.1.0"},
				{Name: "foo.v0.1.0"},
			}},
			versionRange: ">=1.0.0 <2.0.0",
			versionMap: map[string]*mmsemver.Version{
				"foo.v1.1.0": mmsemver.MustParse("1.1.0"),
				"foo.v1.0.0": mmsemver.MustParse("1.0.0"),
				"foo.v0.1.0": mmsemver.MustParse("0.1.0"),
			},
			expected: []string{"foo.v1.0.0", "foo.v1.1.0", "foo.v2.0.0"},
			expectedWarnings: []string{
				`including bundle "foo.v2.0.0": it is unversioned but is required to ensure inclusion of all bundles in the range`,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logOutput := &bytes.Buffer{}
			logger := logrus.New()
			logger.SetOutput(logOutput)
			logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true, DisableQuote: true})
			entry := logrus.NewEntry(logger)

			out, err := newChannel(tc.in, entry)
			require.NoError(t, err)
			vr, err := mmsemver.NewConstraint(tc.versionRange)
			require.NoError(t, err)
			actual := out.filterByVersionRange(vr, tc.versionMap)
			assert.Equal(t, tc.expected, sets.List(actual))
			for _, expectedWarning := range tc.expectedWarnings {
				assert.Contains(t, logOutput.String(), expectedWarning)
			}
		})
	}
}
