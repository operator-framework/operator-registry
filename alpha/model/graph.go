package model

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/util/sets"
)

type graph struct {
	nodes map[string]*node
}

func newNode(b *Bundle) *node {
	return &node{
		bundle:     b,
		replacedBy: make(map[string]*node),
		skippedBy:  make(map[string]*node),
		skips:      make(map[string]*node),
	}
}

func newGraph(c *Channel) *graph {
	nodes := map[string]*node{}

	// Add all nodes (without edges)
	for _, b := range c.Bundles {
		nodes[b.Name] = newNode(b)
	}

	// Populate edges between nodes
	for _, b := range c.Bundles {
		n := nodes[b.Name]

		if b.Replaces != "" {
			replaces, ok := nodes[b.Replaces]
			if !ok {
				// the "replaces" edge points to a node outside the channel
				replaces = newNode(&Bundle{Name: b.Replaces})
				replaces.externalToChannel = true
				nodes[b.Replaces] = replaces
			}
			n.replaces = replaces
			n.replaces.replacedBy[n.bundle.Name] = n
		}

		for _, skipName := range b.Skips {
			skip, ok := nodes[skipName]
			if !ok {
				// the "skips" edge points to a node outside the channel
				skip = newNode(&Bundle{Name: skipName})
				skip.externalToChannel = true
			}
			skip.skippedBy[b.Name] = n
			n.skips[skipName] = skip
		}
	}

	return &graph{
		nodes: nodes,
	}
}

type node struct {
	bundle            *Bundle
	replacedBy        map[string]*node
	replaces          *node
	skippedBy         map[string]*node
	skips             map[string]*node
	externalToChannel bool
}

func (n *node) pathsTo(other *node) [][]*node {
	var pathsToInternal func(existingPath []*node, froms map[string]*node, to *node) [][]*node
	pathsToInternal = func(existingPath []*node, froms map[string]*node, to *node) [][]*node {
		if len(froms) == 0 {
			// we never found a path to "to"
			return nil
		}
		var allPaths [][]*node
		for _, f := range froms {
			path := append(slices.Clone(existingPath), f)
			if f == to {
				// we found "to"!
				allPaths = append(allPaths, path)
			} else {
				// From an intermediate node, look only in replacedBy, so that we don't stray off the replaces chain.
				allPaths = append(allPaths, pathsToInternal(path, f.replacedBy, to)...)
			}
		}
		return allPaths
	}

	// From the starting node, look in all ancestors (replacedBy and skippedBy).
	ancestors := map[string]*node{}
	maps.Copy(ancestors, n.replacedBy)
	maps.Copy(ancestors, n.skippedBy)
	return pathsToInternal(nil, ancestors, other)
}

func (g *graph) validate() error {
	result := newValidationError("invalid upgrade graph")
	if err := g.validateNoCycles(); err != nil {
		result.subErrors = append(result.subErrors, err)
	}
	if err := g.validateNoStranded(); err != nil {
		result.subErrors = append(result.subErrors, err)
	}
	return result.orNil()
}

func (g *graph) cycles() [][]*node {
	allCycles := [][]*node{}
	for _, n := range g.nodes {
		allCycles = append(allCycles, n.pathsTo(n)...)
	}
	dedupSameRotations(&allCycles)
	for i, cycle := range allCycles {
		allCycles[i] = append(cycle, cycle[0])
	}
	return allCycles
}

func (g *graph) validateNoCycles() error {
	cycles := g.cycles()
	if len(cycles) == 0 {
		return nil
	}
	result := newValidationError("cycles found in graph")
	for _, cycle := range cycles {
		result.subErrors = append(result.subErrors, errors.New(nodeCycleString(cycle)))
	}
	return result.orNil()
}

// dedupSameRotations removes rotations of the same cycle.
// dedupSameRotations sorts the cycles so that shorter paths
// and paths with lower versions appear earlier in the list.
func dedupSameRotations(paths *[][]*node) {
	slices.SortFunc(*paths, func(a, b []*node) int {
		if len(a) == 0 && len(b) == 0 {
			return 0
		}
		if v := cmp.Compare(len(a), len(b)); v != 0 {
			return v
		}
		return a[0].bundle.Version.Compare(b[0].bundle.Version)
	})
	seen := map[string]struct{}{}
	tmp := (*paths)[:0]
	for _, path := range *paths {
		rotate(&path)
		k := nodeCycleString(path)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		tmp = append(tmp, path)
	}
	*paths = tmp
}

func rotate(in *[]*node) {
	if len(*in) == 0 {
		return
	}
	maxIndex := 0
	for i, n := range (*in)[1:] {
		if n.bundle.Version.GT((*in)[maxIndex].bundle.Version) {
			maxIndex = i + 1
		}
	}
	slices.Reverse((*in)[:maxIndex])
	slices.Reverse((*in)[maxIndex:])
	slices.Reverse((*in))
}

func nodeCycleString(nodes []*node) string {
	return strings.Join(mapSlice(nodes, nodeName), " -> ")
}

func (g *graph) strandedNodes() ([]*node, error) {
	head, err := g.head()
	if err != nil {
		return nil, err
	}
	all := sets.New[*node](maps.Values(g.nodes)...)
	chain := sets.New[*node]()
	skipped := sets.New[*node]()

	cur := head
	for cur != nil && !skipped.Has(cur) && !chain.Has(cur) {
		chain.Insert(cur)
		skipped.Insert(maps.Values(cur.skips)...)
		cur = cur.replaces
	}

	stranded := all.Difference(chain).Difference(skipped).UnsortedList()
	slices.SortFunc(stranded, func(a, b *node) int {
		return a.bundle.Compare(b.bundle)
	})
	return stranded, nil
}

func (g *graph) validateNoStranded() error {
	stranded, err := g.strandedNodes()
	if err != nil {
		return err
	}
	if len(stranded) == 0 {
		return nil
	}

	return fmt.Errorf("channel contains one or more stranded bundles: %s", strings.Join(mapSlice(stranded, nodeName), ", "))
}

func (g *graph) head() (*node, error) {
	heads := []*node{}
	for _, n := range g.nodes {
		if len(n.replacedBy) == 0 && len(n.skippedBy) == 0 {
			heads = append(heads, n)
		}
	}
	if len(heads) == 0 {
		return nil, fmt.Errorf("no channel head found in graph")
	}
	if len(heads) > 1 {
		headNames := mapSlice(heads, nodeName)
		sort.Strings(headNames)
		return nil, fmt.Errorf("multiple channel heads found in graph: %s", strings.Join(headNames, ", "))
	}
	return heads[0], nil
}

func nodeName(n *node) string {
	return n.bundle.Name
}

func mapSlice[I, O any](s []I, fn func(I) O) []O {
	result := make([]O, 0, len(s))
	for _, i := range s {
		result = append(result, fn(i))
	}
	return result
}
