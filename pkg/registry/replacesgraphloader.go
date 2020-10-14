package registry

import (
	"fmt"

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
			errs = append(errs, fmt.Errorf("Error determining connectedness of package graph %s: %s", pkg, err))
			delete(stream.packages, pkg)
			continue
		}
		if !connected {
			errs = append(errs, fmt.Errorf("Given images for package %s do not form a connected graph, index add will be nondeterministic", pkg))
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

	// For every bundle in the input, there should be at least one that we can traverse to all others from
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
	var replaces string
	replaces, err = c.Bundle.Replaces()
	if err != nil {
		return
	}

	var skips []string
	skips, err = c.Bundle.Skips()
	if err != nil {
		return
	}

	var (
		subgraphSize int
		neighbors    = append(skips, replaces)
	)
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

// canAdd checks that a new bundle can be added in replaces mode (i.e. the replaces defined for the bundle already exists)
func (r *ReplacesInputStream) canAdd(bundle *Bundle, packageGraph *Package) error {
	replaces, err := bundle.Replaces()
	if err != nil {
		return fmt.Errorf("Invalid bundle replaces: %s", err)
	}

	if replaces != "" && !packageGraph.HasCsv(replaces) {
		// We can't add this until a replacement exists
		// TODO(njhale): should this really return an error, or just a boolean?
		return fmt.Errorf("Invalid bundle %s, bundle specifies a non-existent replacement %s", bundle.Name, replaces)
	}

	skips, err := bundle.Skips()
	if err != nil {
		return fmt.Errorf("Invalid bundle skips: %s", err)
	}

	images, ok := r.packages[packageGraph.Name]
	if !ok || images == nil {
		// This shouldn't happen unless canAdd is being called without the correct setup
		panic(fmt.Sprintf("Programmer error: package graph %s incorrectly initialized", packageGraph.Name))
	}

	for _, skip := range skips {
		if _, ok := images[skip]; ok {
			// Found an edge to a remaining input bundle, can't add this bundle yet
			return fmt.Errorf("Invalid index add order for bundle %s, cannot be added before %s", bundle.Name, skip)
		}
	}

	// No edges to any remaining input bundles, this bundle can be added
	return nil
}

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
			// Can't parse this package any further
			delete(r.packages, pkg)
			errs = append(errs, err)
			continue
		}

		// Find the next viable bundle to add
		var packageErrs []error
		for _, image := range images {
			if err := r.canAdd(image.Bundle, packageGraph); err != nil {
				// Can't parse this bundle any further right now
				packageErrs = append(packageErrs, err)
				continue
			}

			// Found something we can add
			delete(r.packages[pkg], image.Bundle.Name)
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
