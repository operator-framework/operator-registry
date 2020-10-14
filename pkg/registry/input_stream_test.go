package registry_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	storagenames "k8s.io/apiserver/pkg/storage/names"

	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/registry/registryfakes"
)

func genName(base string) string {
	return storagenames.SimpleNameGenerator.GenerateName(base)
}

func unstructuredCSV() *unstructured.Unstructured {
	csv := &unstructured.Unstructured{}
	csv.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
		// NOTE: For some reason, this is all bundle.Add cares about
		Kind: "ClusterServiceVersion",
	})

	return csv
}

type connectedGraph []*registry.ImageInput

func newConnectedGraph(r *rand.Rand, max int) connectedGraph {
	size := r.Intn(max + 1) // +1 prevents an invalid zero argument and includes max (generate should never be given a negative max)
	nodes := make([]string, size)
	graph := make(connectedGraph, size)
	for i := 0; i < size; i++ {
		config := nodeConfig{name: genName("")}
		nodes[i] = config.name

		// Create edges with skips and replaces (except for the first node)
		if i > 0 {
			// skips = []string{nodes[i-1]}
			config.skips = nodes[:i]
			config.replaces = nodes[i-1]
		}
		graph[i] = newGraphNode(&config)
	}

	return graph
}

type nodeConfig struct {
	name      string
	version   string
	replaces  string
	skipRange string
	skips     []string
}

func newGraphNode(config *nodeConfig) *registry.ImageInput {
	image := &registry.ImageInput{
		Bundle: &registry.Bundle{
			Name: config.name,
		},
	}

	csv := unstructuredCSV()
	content := csv.UnstructuredContent() // We don't bother setting a name since the bundle name is used everywhere
	if content == nil {
		content = map[string]interface{}{}
	}

	content["spec"] = map[string]interface{}{
		"version":  config.version,
		"skips":    config.skips,
		"replaces": config.replaces,
	}
	csv.SetUnstructuredContent(content)
	csv.SetAnnotations(map[string]string{
		registry.SkipRangeAnnotationKey: config.skipRange,
	})
	image.Bundle.Add(csv)

	return image
}

func (connectedGraph) Generate(r *rand.Rand, max int) reflect.Value {
	return reflect.ValueOf(newConnectedGraph(r, max))
}

type disconnectedGraph []*registry.ImageInput

func (disconnectedGraph) Generate(r *rand.Rand, max int) reflect.Value {
	numSubgraphs := r.Intn(max + 1)
	if numSubgraphs < 2 {
		// We always need at least at least 2 connected subgraphs to form a disconnected graph
		numSubgraphs = 2
	}

	graph := disconnectedGraph{}
	visited := map[string]struct{}{}
	for i := 0; i < numSubgraphs; i++ {
		for _, node := range newConnectedGraph(r, max) {
			name := node.Bundle.Name
			if _, ok := visited[name]; ok {
				// Elide any duplicate nodes -- this shouldn't make the final graph any less disconnected
				continue
			}

			graph = append(graph, node)
			visited[name] = struct{}{}
		}
	}

	return reflect.ValueOf(graph)
}

func TestNewStreamErrorsOnDisconnectedPackageGraphs(t *testing.T) {
	f := func(disconnected []disconnectedGraph, connected []connectedGraph) bool {
		if len(disconnected) < 1 {
			// Discard attempts with no disconnected graph
			return true
		}

		// Add connected and disconnected packages
		var images []*registry.ImageInput
		for _, graph := range connected {
			pkg := genName("")
			for _, image := range graph {
				image.Bundle.Package = pkg
				images = append(images, image)
			}
		}
		for _, graph := range disconnected {
			pkg := genName("")
			for _, image := range graph {
				image.Bundle.Package = pkg
				images = append(images, image)
			}
		}

		// Any disconnected package graphs should cause an error
		_, err := registry.NewReplacesInputStream(nil, images)

		return err != nil
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestNewStreamSucceedsOnConnectedPackageGraphs(t *testing.T) {
	f := func(connected []connectedGraph) bool {
		if len(connected) < 1 {
			// Discard attempts without packages
			return true
		}

		// Add connected packages
		var images []*registry.ImageInput
		for _, graph := range connected {
			pkg := genName("")
			for _, image := range graph {
				image.Bundle.Package = pkg
				images = append(images, image)
			}
		}

		// Without disconnected packages, this should always succeed
		_, err := registry.NewReplacesInputStream(nil, images)

		return err == nil
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestNextReturnsEntireConnectedGraph(t *testing.T) {
	f := func(connected []connectedGraph) bool {
		if len(connected) < 1 {
			// Discard attempts without packages
			return true
		}

		// After each next, update the fake package(s) to reflect the addition
		returnedImages := map[registry.BundleKey]map[registry.BundleKey]struct{}{}

		// Add connected packages
		packages := map[string]*registry.Package{}
		remaining := map[string]struct{}{}
		var images []*registry.ImageInput
		for _, graph := range connected {
			pkg := genName("")
			packages[pkg] = &registry.Package{
				Name: pkg,
				Channels: map[string]registry.Channel{
					// It's okay that the fake packages share this info for this test, since we only need this to prevent the
					// stream from complaining about missing replaces as we successively add results to the graph.
					"": registry.Channel{ // The channel name doesn't matter for this test
						Nodes: returnedImages,
					},
				},
			}
			for _, image := range graph {
				image.Bundle.Package = pkg
				images = append(images, image)
				remaining[fmt.Sprintf("%s/%s", pkg, image.Bundle.Name)] = struct{}{}
			}
		}

		loader := &registryfakes.FakeGraphLoader{
			GenerateStub: func(pkg string) (*registry.Package, error) {
				if _, ok := packages[pkg]; !ok {
					t.Errorf("unknown package %s given as argument for generate call", pkg)
				}
				return packages[pkg], nil
			},
		}

		// Without disconnected packages, this should always succeed
		stream, err := registry.NewReplacesInputStream(loader, images)
		if err != nil {
			t.Error(err)
			return false
		}

		next, err := stream.Next()
		for next != nil {
			name := next.Bundle.Name
			key := fmt.Sprintf("%s/%s", next.Bundle.Package, name)
			if _, ok := remaining[key]; !ok {
				t.Errorf("next returned duplicate or otherwise unknown bundle %s: %#v", key, *next.Bundle)
				return false
			}
			returnedImages[registry.BundleKey{CsvName: name}] = nil // The value doesn't matter for this test

			delete(remaining, key)
			next, err = stream.Next()
		}

		return len(remaining) < 1
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

type inSequence struct {
	input    []*registry.ImageInput
	sequence []string
	pkg      string
}

func (*inSequence) Generate(r *rand.Rand, max int) reflect.Value {
	var (
		graph    = newConnectedGraph(r, max)
		pkg      = genName("")
		input    []*registry.ImageInput
		sequence []string
	)
	for _, image := range graph {
		image.Bundle.Package = pkg
		input = append(input, image)
		sequence = append(sequence, image.Bundle.Name)
	}

	// Ensure we're not depending on any ordering
	r.Shuffle(len(input), func(i, j int) { input[i], input[j] = input[j], input[i] })

	return reflect.ValueOf(&inSequence{
		input:    input,
		sequence: sequence,
		pkg:      pkg,
	})
}

func TestNextReturnsSequence(t *testing.T) {
	f := func(is *inSequence) bool {
		if len(is.input) < 1 {
			// Discard attempts without packages
			return true
		}

		// After each next, update the fake package to reflect the addition
		returnedImages := map[registry.BundleKey]map[registry.BundleKey]struct{}{}
		loader := &registryfakes.FakeGraphLoader{
			GenerateStub: func(p string) (*registry.Package, error) {
				if p != is.pkg {
					t.Errorf("unknown package %s given as argument for generate call", p)
				}
				return &registry.Package{
					Name: is.pkg,
					Channels: map[string]registry.Channel{
						// The name of the channel doesn't matter since it isn't checked
						"": registry.Channel{
							Nodes: returnedImages,
						},
					},
				}, nil
			},
		}

		// Without disconnected packages, this should always succeed
		stream, err := registry.NewReplacesInputStream(loader, is.input)
		if err != nil {
			t.Error(err)
			return false
		}

		for _, expected := range is.sequence {
			next, err := stream.Next()
			if err != nil {
				t.Error(err)
				return false
			}

			if next == nil {
				t.Errorf("stream exhausted before expected sequence")
				return false
			}

			name := next.Bundle.Name
			if name != expected {
				t.Errorf("bundle returned out of sequence: got %s expected %s", next.Bundle.Name, expected)
				return false
			}

			returnedImages[registry.BundleKey{CsvName: next.Bundle.Name}] = nil // The value doesn't matter for this test
		}

		next, err := stream.Next()
		if err != nil {
			t.Error(err)
			return false
		}
		if next != nil {
			t.Errorf("expected stream to end after the sequence was exhausted")
			return false
		}

		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

// Tests to write:
// 1. A single graph can be connected with skipRange
// 2. Two disconnected graphs can be connected with skipRange
// 3. Stranded updates are
func TestNextHandlesSkipRange(t *testing.T) {

}
