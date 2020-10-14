package registry

import (
	"fmt"

	"github.com/blang/semver"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type ReplacesInputStream struct {
	graph    GraphLoader
	packages map[string]map[string]*ImageInput
}

func NewReplacesInputStream(graph GraphLoader, toAdd []*ImageInput) (*ReplacesInputStream, error) {
	stream := &ReplacesInputStream{
		graph:    graph,
		packages: map[string]map[string]*ImageInput{},
	}

	// Sort the bundle images into buckets by package
	for _, image := range toAdd {
		pkg := image.Bundle.Package
		if _, ok := stream.packages[pkg]; !ok {
			stream.packages[pkg] = map[string]*ImageInput{}
		}
		stream.packages[pkg][image.Bundle.Name] = image
	}

	// Validate each package, dropping any invalid packages so that the stream can still be used with remaining packages.
	var errs []error
	for pkg, images := range stream.packages {
		// Multiple input images require some ordering between them -- using skips and replaces -- to ensure an index add operation is deterministic
		connected, err := graphConnected(images)
		if err != nil {
			errs = append(errs, fmt.Errorf("Error determining connectedness of update graph for package %s. Package will be skipped: %s", pkg, err))
			delete(stream.packages, pkg)
			continue
		}
		if !connected {
			errs = append(errs, fmt.Errorf("Given images for package %s do not form a connected graph, index add would be nondeterministic. Package will be skipped.", pkg))
			delete(stream.packages, pkg)
		}
	}

	return stream, utilerrors.NewAggregate(errs)
}

// connected returns true if the given set of bundles forms a connected graph.
func graphConnected(graph map[string]*ImageInput) (bool, error) {
	if len(graph) <= 1 {
		// Zero or one bundles will result in a deterministic add, and should be considered connected
		return true, nil
	}

	// For every bundle in the input, there should be at least one that we can traverse to all others from (assuming this is a directed graph)
	for _, node := range graph {
		size, err := graphSize(node.Bundle.Name, graph, map[string]struct{}{})
		if err != nil {
			return false, err
		}
		if size == len(graph) {
			return true, nil
		}
	}

	return false, nil
}

// graphSize performs a depth-first search of a graph and returns the number of nodes reachable from a starting node.
// This is analogous to the "size" of the subgraph containing the starting node.
func graphSize(current string, graph map[string]*ImageInput, visited map[string]struct{}) (size int, err error) {
	c, ok := graph[current]
	if !ok {
		// The node doesn't exist, which means it's a skips w/o a matching entry or a replaces in the graph
		// We handle these cases elsewhere, so don't mark them visited
		return
	}

	if _, ok := visited[current]; ok {
		// Cycles don't contribute to the size of a graph
		return
	}

	// Discovered a new node, increase the size of the graph
	visited[current] = struct{}{}
	size++

	// Gather edges
	var (
		neighbors []string
		replaces  string
	)
	replaces, err = c.Bundle.Replaces()
	if err != nil {
		return
	}
	neighbors = append(neighbors, replaces)

	var skips []string
	skips, err = c.Bundle.Skips()
	if err != nil {
		return
	}
	neighbors = append(neighbors, skips...)

	var skipRanged []string
	skipRanged, err = skippedBySkipRange(c.Bundle, graph)
	if err != nil {
		return
	}
	neighbors = append(neighbors, skipRanged...)

	var subgraphSize int
	for _, neighbor := range neighbors {
		subgraphSize, err = graphSize(neighbor, graph, visited)
		if err != nil {
			return
		}

		// Incorporate the size of the subgraph
		size += subgraphSize
	}

	return
}

// skippedBySkipRange returns the set of bundles with versions skipped by
func skippedBySkipRange(node *Bundle, graph map[string]*ImageInput) ([]string, error) {
	rawSkipRange, err := node.SkipRange()
	if err != nil {
		return nil, err
	}
	if rawSkipRange == "" {
		// This bundle doesn't use skipRange.
		return nil, nil
	}

	skipRange, err := semver.ParseRange(rawSkipRange)
	if err != nil {
		return nil, err
	}

	var skipped []string
	for _, n := range graph {
		rawVersion, err := n.Bundle.Version()
		if err != nil {
			return nil, err
		}
		if rawVersion == "" {
			// Version isn't specified and is unaffected by skipRange
			continue
		}
		version, err := semver.Parse(rawVersion)
		if err != nil {
			return nil, err
		}
		if skipRange(version) {
			skipped = append(skipped, n.Bundle.Name)
		}
	}

	return skipped, nil
}

// canAdd checks that a new bundle can be added in replaces mode (i.e. the replaces defined for the bundle already exists)
func (r *ReplacesInputStream) canAdd(bundle *Bundle, packageGraph *Package) error {
	replaces, err := bundle.Replaces()
	if err != nil {
		return fmt.Errorf("Invalid bundle replaces: %s", err)
	}

	if replaces != "" && !packageGraph.HasCsv(replaces) {
		// We can't add this until a replacement exists
		return fmt.Errorf("Invalid bundle %s, bundle specifies a non-existent replacement %s", bundle.Name, replaces)
	}

	images, ok := r.packages[packageGraph.Name]
	if !ok || images == nil {
		// This shouldn't happen unless canAdd is being called without the correct setup
		panic(fmt.Sprintf("Programmer error: package graph %s incorrectly initialized", packageGraph.Name))
	}

	var neighbors []string
	skips, err := bundle.Skips()
	if err != nil {
		return fmt.Errorf("Invalid bundle skips: %s", err)
	}
	neighbors = append(neighbors, skips...)

	skipRanged, err := skippedBySkipRange(bundle, images)
	if err != nil {
		return fmt.Errorf("Invalid bundle skipRange: %s", err)
	}
	neighbors = append(neighbors, skipRanged...)

	for _, neighbor := range neighbors {
		if _, ok := images[neighbor]; ok {
			// Found an edge to a remaining input bundle, can't add this bundle yet
			return fmt.Errorf("Invalid index add order for bundle %s, cannot be added before %s", bundle.Name, neighbors)
		}
	}

	// No edges to any remaining input bundles, this bundle can be added
	return nil
}

// Next returns the next available bundle image from the stream, returning a nil image if the stream is exhausted.
func (r *ReplacesInputStream) Next() (*ImageInput, error) {
	var errs []error
	for pkg, images := range r.packages {
		if len(images) < 1 {
			// No more images to add for this package, clean up
			delete(r.packages, pkg)
			continue
		}

		packageGraph, err := r.graph.Generate(pkg)
		if err != nil {
			if err != ErrPackageNotInDatabase {
				// Can't parse this package any further
				delete(r.packages, pkg)
				errs = append(errs, err)
				continue
			}

			// Adding a brand new package is a different story
			packageGraph = &Package{Name: pkg}
		}

		// Find the next bundle in topological order
		var packageErrs []error
		for _, image := range images {
			if err := r.canAdd(image.Bundle, packageGraph); err != nil {
				// Can't parse this bundle any further right now
				packageErrs = append(packageErrs, err)
				continue
			}

			// Found something we can add
			delete(r.packages[pkg], image.Bundle.Name)
			if len(r.packages[pkg]) < 1 {
				// Remove package if exhausted
				delete(r.packages, pkg)
			}

			return image, nil
		}

		// No viable bundle found in the package, can't parse it any further
		if len(packageErrs) > 0 {
			delete(r.packages, pkg)
			errs = append(errs, packageErrs...)
		}
	}

	// We've exhausted all valid input bundles, any errors here indicate invalid input of some kind
	return nil, utilerrors.NewAggregate(errs)
}

// Empty returns true if there are no bundles in the stream.
func (r *ReplacesInputStream) Empty() bool {
	return len(r.packages) < 1
}
