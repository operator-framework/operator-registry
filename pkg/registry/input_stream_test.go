package registry_test

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/blang/semver"
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

type inputConfig struct {
	pkg      string
	name     string
	version  string
	replaces string
}

func newImageInput(config *inputConfig) *registry.ImageInput {
	image := &registry.ImageInput{
		Bundle: &registry.Bundle{
			Package: config.pkg,
			Name:    config.name,
		},
	}

	csv := unstructuredCSV()
	content := csv.UnstructuredContent() // We don't bother setting a name since the bundle name is used everywhere
	if content == nil {
		content = map[string]interface{}{}
	}
	content["spec"] = map[string]interface{}{
		"version":  config.version,
		"replaces": config.replaces,
	}
	csv.SetUnstructuredContent(content)
	image.Bundle.Add(csv)

	return image
}

func bumpVersion(r *rand.Rand, base semver.Version) semver.Version {
	inc := func(v uint64) uint64 {
		return v + uint64(math.Max(1, float64(r.Intn(3)))) // Bump by 1 to 3 randomly
	}

	return semver.Version{
		Major: inc(base.Major),
		Minor: inc(base.Minor),
		Patch: inc(base.Patch),
	}
}

type semverSequence struct {
	input   []*registry.ImageInput
	ordered []string
}

func (semverSequence) Generate(r *rand.Rand, max int) reflect.Value {
	size := r.Intn(max + 1) // +1 prevents an invalid zero argument and includes max (generate should never be given a negative max)
	input := make([]*registry.ImageInput, size)
	ordered := make([]string, size)
	var version semver.Version
	for i := 0; i < size; i++ {
		// Each bundle is added in order of ascending semver
		version = bumpVersion(r, version)
		config := inputConfig{
			name:    genName(""),
			version: version.String(),
		}

		input[i] = newImageInput(&config)
		ordered[i] = config.name
	}

	// Ensure we're not depending on any ordering
	r.Shuffle(len(input), func(i, j int) { input[i], input[j] = input[j], input[i] })

	return reflect.ValueOf(semverSequence{
		input:   input,
		ordered: ordered,
	})
}

func TestNextReturnsSemverSequence(t *testing.T) {
	f := func(sequences []semverSequence) bool {
		if len(sequences) < 1 {
			return true
		}

		ordered := map[string][]string{}
		var input []*registry.ImageInput
		for _, sequence := range sequences {
			pkg := genName("")
			for _, in := range sequence.input {
				in.Bundle.Package = pkg
				input = append(input, in)
			}
			ordered[pkg] = sequence.ordered
		}

		loader := &registryfakes.FakeGraphLoader{
			GenerateStub: func(pkg string) (*registry.Package, error) {
				return &registry.Package{
					Name: pkg,
				}, nil
			},
		}

		stream, err := registry.NewReplacesInputStream(loader, input)
		if err != nil {
			t.Error(err)
			return false
		}

		for !stream.Empty() {
			next, err := stream.Next()
			if err != nil {
				t.Errorf("next returned unexpected error: %s", err)
				return false
			}
			if next == nil {
				t.Errorf("next returned unexpected nil")
				return false
			}

			pkg := next.Bundle.Package
			ord, ok := ordered[pkg]
			if !ok {
				t.Errorf("next returned bundle for unexpected package %s", pkg)
				return false
			}

			name := next.Bundle.Name
			if len(ord) < 1 {
				t.Errorf("next returned extra bundle for package %s: %s", pkg, name)
				return false
			}

			if name != ord[0] {
				t.Errorf("next returned unexpecting bundle %s/%s, expecting %s/%s: %v", pkg, name, pkg, ord[0], ordered)
				return false
			}

			if len(ord) > 1 {
				ordered[pkg] = ord[1:] // Dequeue expected order
				continue
			}

			// We've been given all the bundles we've expected for this package
			delete(ordered, pkg)
		}

		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestVersionIgnoredForIndividualAdd(t *testing.T) {
	input := []*registry.ImageInput{
		newImageInput(&inputConfig{
			name: genName(""),
		}),
	}

	if _, err := registry.NewReplacesInputStream(nil, input); err != nil {
		t.Error(err)
	}
}

func TestVersionRequiredForBulkAdd(t *testing.T) {
	pkg := genName("")
	input := []*registry.ImageInput{
		newImageInput(&inputConfig{
			pkg:  pkg,
			name: genName(""),
		}),
		newImageInput(&inputConfig{
			pkg:  pkg,
			name: genName(""),
		}),
	}

	if _, err := registry.NewReplacesInputStream(nil, input); err == nil {
		t.Errorf("input stream accepted invalid bundles with missing version fields")
	}
}

func TestReplacesTakesPrecedence(t *testing.T) {
	// Set up semver order to be the opposite of replaces order
	input := []*registry.ImageInput{
		newImageInput(&inputConfig{
			name:     "d",
			replaces: "c",
			version:  "1.0.0",
		}),
		newImageInput(&inputConfig{
			name:     "c",
			replaces: "b",
			version:  "1.0.1",
		}),
		newImageInput(&inputConfig{
			name:     "b",
			replaces: "a",
			version:  "1.2.0",
		}),
		newImageInput(&inputConfig{
			name:    "a",
			version: "1.2.5",
		}),
	}

	added := map[registry.BundleKey]map[registry.BundleKey]struct{}{}
	loader := &registryfakes.FakeGraphLoader{
		GenerateStub: func(pkg string) (*registry.Package, error) {
			return &registry.Package{
				Name: pkg,
				Channels: map[string]registry.Channel{
					"": registry.Channel{
						Nodes: added,
					},
				},
			}, nil
		},
	}

	stream, err := registry.NewReplacesInputStream(loader, input)
	if err != nil {
		t.Error(err)
		return
	}

	// Expect input to be returned in replaces order
	for _, expected := range []string{"a", "b", "c", "d"} {
		next, err := stream.Next()
		if err != nil {
			t.Error(err)
			return
		}
		if next == nil {
			t.Errorf("next returned unexpected nil, expecting %s", expected)
			return
		}

		name := next.Bundle.Name
		if name != expected {
			t.Errorf("next returned unexpected bundle %s, expecting %s", name, expected)
			return
		}

		// Simulate an add
		added[registry.BundleKey{CsvName: name}] = nil // Only the key is compared in HasCsv()
	}

	if !stream.Empty() {
		t.Errorf("stream still contains content, expected end of content")
	}
}

func TestMissingReplacesReturnsError(t *testing.T) {
	input := []*registry.ImageInput{
		newImageInput(&inputConfig{
			name:     genName(""),
			replaces: genName(""), // This won't exist and should cause a failure
		}),
	}

	loader := &registryfakes.FakeGraphLoader{
		GenerateStub: func(pkg string) (*registry.Package, error) {
			return &registry.Package{
				Name: pkg,
			}, nil
		},
	}

	stream, err := registry.NewReplacesInputStream(loader, input)
	if err != nil {
		t.Error(err)
		return
	}

	next, err := stream.Next()
	if err == nil {
		t.Errorf("expected next to return an error")
		return
	}
	if next != nil {
		t.Errorf("expected next to return nil on error, got %v instead", next)
		return
	}
}
