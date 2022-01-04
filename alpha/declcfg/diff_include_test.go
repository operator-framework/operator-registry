package declcfg

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/model"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func TestFindIntersectingBundles(t *testing.T) {
	type bundleSpec struct {
		name, replaces string
		skips          []string
	}

	inputBundles1 := []bundleSpec{
		{"foo.v0.1.0", "", nil},
		{"foo.v0.1.1", "foo.v0.1.0", nil},
		{"foo.v0.2.0", "foo.v0.1.1", nil},
		{"foo.v0.3.0", "foo.v0.2.0", nil},
		{"foo.v1.0.0", "foo.v0.1.0", []string{"foo.v0.1.1", "foo.v0.2.0", "foo.v0.3.0"}},
		{"foo.v1.1.0", "foo.v1.0.0", nil},
		{"foo.v2.0.0", "foo.v0.1.0", []string{"foo.v0.1.1", "foo.v0.2.0", "foo.v0.3.0", "foo.v1.0.0", "foo.v1.1.0"}},
		{"foo.v2.1.0", "foo.v2.0.0", nil},
		{"foo.v3.0.0", "foo.v1.1.0", []string{"foo.v2.0.0", "foo.v2.1.0"}},
	}

	type spec struct {
		name            string
		pkgName         string
		channelName     string
		inputBundles    []bundleSpec
		start, end      bundleSpec
		headName        string
		assertion       require.BoolAssertionFunc
		expIntersecting []bundleSpec
	}

	specs := []spec{
		{
			name:            "Success/StartEndEqual",
			inputBundles:    inputBundles1,
			start:           bundleSpec{"foo.v0.2.0", "foo.v0.1.1", nil},
			end:             bundleSpec{"foo.v0.2.0", "foo.v0.1.1", nil},
			headName:        "foo.v3.0.0",
			assertion:       require.True,
			expIntersecting: []bundleSpec{{"foo.v0.2.0", "foo.v0.1.1", nil}},
		},
		{
			name:            "Success/FullGraph",
			inputBundles:    inputBundles1,
			start:           bundleSpec{"foo.v0.1.0", "", nil},
			end:             bundleSpec{"foo.v3.0.0", "foo.v1.1.0", nil},
			headName:        "foo.v3.0.0",
			assertion:       require.True,
			expIntersecting: inputBundles1,
		},
		{
			name:         "Success/SubGraph1",
			inputBundles: inputBundles1,
			start:        bundleSpec{"foo.v0.2.0", "foo.v0.1.1", nil},
			end:          bundleSpec{"foo.v3.0.0", "foo.v1.1.0", nil},
			headName:     "foo.v3.0.0",
			assertion:    require.True,
			expIntersecting: []bundleSpec{
				{"foo.v0.2.0", "foo.v0.1.1", nil},
				{"foo.v0.3.0", "foo.v0.2.0", nil},
				{"foo.v1.0.0", "foo.v0.1.0", []string{"foo.v0.1.1", "foo.v0.2.0", "foo.v0.3.0"}},
				{"foo.v1.1.0", "foo.v1.0.0", nil},
				{"foo.v2.0.0", "foo.v0.1.0", []string{"foo.v0.1.1", "foo.v0.2.0", "foo.v0.3.0", "foo.v1.0.0", "foo.v1.1.0"}},
				{"foo.v2.1.0", "foo.v2.0.0", nil},
				{"foo.v3.0.0", "foo.v1.1.0", []string{"foo.v2.0.0", "foo.v2.1.0"}},
			},
		},
		{
			name:         "Success/SubGraph2",
			inputBundles: inputBundles1,
			start:        bundleSpec{"foo.v0.1.1", "foo.v0.1.0", nil},
			end:          bundleSpec{"foo.v0.3.0", "foo.v0.2.0", nil},
			headName:     "foo.v3.0.0",
			assertion:    require.True,
			expIntersecting: []bundleSpec{
				{"foo.v0.1.1", "foo.v0.1.0", nil},
				{"foo.v0.2.0", "foo.v0.1.1", nil},
				{"foo.v0.3.0", "foo.v0.2.0", nil},
			},
		},
		{
			// This case returns inputBundles1 minus foo.v0.1.0, which is the intersection,
			// because foo.v2.0.0 is a leaf node (disregarding skips).
			name:            "Success/SubGraph3",
			inputBundles:    inputBundles1,
			start:           bundleSpec{"foo.v0.1.1", "foo.v0.1.0", nil},
			end:             bundleSpec{"foo.v2.0.0", "foo.v0.1.0", nil},
			headName:        "foo.v3.0.0",
			assertion:       require.True,
			expIntersecting: inputBundles1[1:],
		},
		{
			// Even though foo.v0.4.0 is not in the channel, it's replaces (foo.v0.1.1) is.
			name:         "Success/ReplacesInChannel",
			inputBundles: inputBundles1,
			start:        bundleSpec{"foo.v0.2.0", "foo.v0.1.1", nil},
			end:          bundleSpec{"foo.v0.4.0", "foo.v0.1.1", nil},
			headName:     "foo.v3.0.0",
			assertion:    require.True,
			expIntersecting: []bundleSpec{
				{"foo.v0.2.0", "foo.v0.1.1", nil},
				{"foo.v0.3.0", "foo.v0.2.0", nil},
				{"foo.v0.4.0", "foo.v0.1.1", nil},
			},
		},
		{
			name:         "Fail/ReplacesNotInChannel",
			inputBundles: inputBundles1,
			start:        bundleSpec{"foo.v0.2.0", "foo.v0.1.1", nil},
			end:          bundleSpec{"foo.v0.4.0", "foo.v0.1.2", nil},
			headName:     "foo.v3.0.0",
			assertion:    require.False,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			// Construct test
			pkg := &model.Package{Name: "foo"}
			ch := &model.Channel{Name: "stable", Bundles: make(map[string]*model.Bundle, len(s.inputBundles))}
			ch.Package = pkg
			for _, b := range s.inputBundles {
				ch.Bundles[b.name] = newReplacingBundle(b.name, b.replaces, b.skips, ch, pkg)
			}
			expIntersecting := make([]*model.Bundle, len(s.expIntersecting))
			for i, b := range s.expIntersecting {
				expIntersecting[i] = newReplacingBundle(b.name, b.replaces, b.skips, ch, pkg)
			}

			// Ensure the channel is valid and has the correct head.
			require.NoError(t, ch.Validate())
			head, err := ch.Head()
			require.NoError(t, err)
			require.Equal(t, ch.Bundles[s.headName], head)

			start := newReplacingBundle(s.start.name, s.start.replaces, s.start.skips, ch, pkg)
			end := newReplacingBundle(s.end.name, s.end.replaces, s.end.skips, ch, pkg)
			graph := makeUpgradeGraph(ch)
			intersecting, found := findIntersectingBundles(ch, start, end, graph)
			s.assertion(t, found)
			// Compare bundle names only, since mismatch output is too verbose.
			require.ElementsMatch(t, getBundleNames(expIntersecting), getBundleNames(intersecting))
		})
	}

}

func newReplacingBundle(name, replaces string, skips []string, ch *model.Channel, pkg *model.Package) *model.Bundle {
	split := strings.SplitN(name, ".", 2)
	nameStr, verStr := split[0], split[1]
	b := &model.Bundle{
		Name:     name,
		Replaces: replaces,
		Skips:    skips,
		Channel:  ch,
		Package:  pkg,
		Image:    fmt.Sprintf("namespace/%s:%s", nameStr, verStr),
		Properties: []property.Property{
			property.MustBuildPackage(ch.Package.Name, verStr),
		},
	}
	return b
}

func getBundleNames(bundles []*model.Bundle) (names []string) {
	for _, b := range bundles {
		names = append(names, b.Name)
	}
	sort.Strings(names)
	return names
}
