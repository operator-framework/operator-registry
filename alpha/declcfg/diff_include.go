package declcfg

import (
	"github.com/operator-framework/operator-registry/alpha/model"
)

// makeUpgradeGraph creates a DAG of bundles with map key Bundle.Replaces.
func makeUpgradeGraph(ch *model.Channel) map[string][]*model.Bundle {
	graph := map[string][]*model.Bundle{}
	for _, b := range ch.Bundles {
		b := b
		if b.Replaces != "" {
			graph[b.Replaces] = append(graph[b.Replaces], b)
		}
	}
	return graph
}

// findIntersectingBundles finds the intersecting bundle of start and end in the
// replaces upgrade graph graph by traversing down to the lowest graph node,
// then returns every bundle higher than the intersection. It is possible
// to find no intersection; this should only happen when start and end
// are not part of the same upgrade graph.
// Output bundle order is not guaranteed.
// Precondition: start must be a bundle in ch.
// Precondition: end must be ch's head.
func findIntersectingBundles(ch *model.Channel, start, end *model.Bundle, graph map[string][]*model.Bundle) ([]*model.Bundle, bool) {
	// The intersecting set is equal to end if start is end.
	if start.Name == end.Name {
		return []*model.Bundle{end}, true
	}

	// Construct start's replaces chain for comparison against end's.
	startChain := map[string]*model.Bundle{start.Name: nil}
	for curr := start; curr != nil && curr.Replaces != ""; curr = ch.Bundles[curr.Replaces] {
		startChain[curr.Replaces] = curr
	}

	// Trace end's replaces chain until it intersects with start's, or the root is reached.
	var intersection string
	if _, inChain := startChain[end.Name]; inChain {
		intersection = end.Name
	} else {
		for curr := end; curr != nil && curr.Replaces != ""; curr = ch.Bundles[curr.Replaces] {
			if _, inChain := startChain[curr.Replaces]; inChain {
				intersection = curr.Replaces
				break
			}
		}
	}

	// No intersection is found, delegate behavior to caller.
	if intersection == "" {
		return nil, false
	}

	// Find all bundles that replace the intersection via BFS,
	// i.e. the set of bundles that fill the update graph between start and end.
	replacesIntersection := graph[intersection]
	replacesSet := map[string]*model.Bundle{}
	for _, b := range replacesIntersection {
		currName := ""
		for next := []*model.Bundle{b}; len(next) > 0; next = next[1:] {
			currName = next[0].Name
			if _, hasReplaces := replacesSet[currName]; !hasReplaces {
				replacers := graph[currName]
				next = append(next, replacers...)
				replacesSet[currName] = ch.Bundles[currName]
			}
		}
	}

	// Remove every bundle between start and intersection exclusively,
	// since these bundles must already exist in the destination channel.
	for rep := start; rep != nil && rep.Name != intersection; rep = ch.Bundles[rep.Replaces] {
		delete(replacesSet, rep.Name)
	}

	// Ensure both start and end are added to the output.
	replacesSet[start.Name] = start
	replacesSet[end.Name] = end
	var intersectingBundles []*model.Bundle
	for _, b := range replacesSet {
		intersectingBundles = append(intersectingBundles, b)
	}
	return intersectingBundles, true
}
