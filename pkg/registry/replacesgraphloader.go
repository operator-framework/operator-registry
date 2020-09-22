package registry

import (
	"fmt"
)

type ReplacesGraphLoader struct {
}

// CanAdd checks that a new bundle can be added in replaces mode (i.e. the replaces
// defined for the bundle already exists)
func (r *ReplacesGraphLoader) CanAdd(bundle *Bundle, graph *Package) (bool, error) {
	replaces, err := bundle.Replaces()
	if err != nil {
		return false, fmt.Errorf("Invalid content, unable to parse bundle")
	}

	// adding the first bundle in the graph
	if replaces == "" {
		return true, nil
	}

	// check that the bundle can be added
	return graph.HasCsv(replaces), nil
}
