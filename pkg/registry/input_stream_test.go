package registry_test

import (
	"math"
	"math/rand"
	"reflect"
	"strings"
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
	pkg            string
	name           string
	version        string
	replaces       string
	defaultChannel string
	channels       []string
}

func newImageInput(config *inputConfig) *registry.ImageInput {
	image := &registry.ImageInput{
		Bundle: &registry.Bundle{
			Package:  config.pkg,
			Name:     config.name,
			Channels: config.channels,
			Annotations: &registry.Annotations{
				PackageName:        config.pkg,
				Channels:           strings.Join(config.channels, ","),
				DefaultChannelName: config.defaultChannel,
			},
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

func TestDefaultChannelAffectsOrder(t *testing.T) {
	type args struct {
		input []*registry.ImageInput
	}
	type expect struct {
		err     bool
		ordered []string
	}
	for _, tt := range []struct {
		description string
		args        args
		expect      expect
	}{
		{
			description: "BundleDefiningDefaultChannelIsFirst",
			args: args{
				input: []*registry.ImageInput{
					newImageInput(&inputConfig{
						name:           "a",
						version:        "1.0.0",
						channels:       []string{"stable"},
						defaultChannel: "alpha",
					}),
					newImageInput(&inputConfig{
						name:           "b",
						version:        "1.1.0",
						channels:       []string{"alpha", "stable"},
						defaultChannel: "alpha",
					}),
				},
			},
			expect: expect{
				ordered: []string{"b", "a"},
			},
		},
		{
			description: "ConflictingReplacesAndDefaultChannelReturnsError",
			args: args{
				input: []*registry.ImageInput{
					newImageInput(&inputConfig{
						name:           "a",
						version:        "1.0.0",
						channels:       []string{"stable"},
						defaultChannel: "alpha",
					}),
					newImageInput(&inputConfig{
						name:           "b",
						version:        "1.1.0",
						replaces:       "a",
						channels:       []string{"alpha", "stable"},
						defaultChannel: "alpha",
					}),
				},
			},
			expect: expect{
				err: true,
			},
		},
	} {
		t.Run(tt.description, func(t *testing.T) {
			addedChannels := map[string]registry.Channel{}
			loader := &registryfakes.FakeGraphLoader{
				GenerateStub: func(pkg string) (*registry.Package, error) {
					return &registry.Package{
						Name:     pkg,
						Channels: addedChannels,
					}, nil
				},
			}

			stream, err := registry.NewReplacesInputStream(loader, tt.args.input)
			if err != nil {
				t.Error(err)
				return
			}

			if tt.expect.err {
				_, err := stream.Next()
				if err == nil {
					t.Error("expected next to return error, returned nil instead")
				}
				return
			}

			for _, expected := range tt.expect.ordered {
				next, err := stream.Next()
				if err != nil {
					t.Errorf("next returned unexpected error %s", err)
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
				for _, channel := range next.Bundle.Channels {
					added := addedChannels[channel]
					if added.Nodes == nil {
						added.Nodes = map[registry.BundleKey]map[registry.BundleKey]struct{}{}
					}
					added.Nodes[registry.BundleKey{CsvName: name}] = nil
					addedChannels[channel] = added
				}
			}

			if !stream.Empty() {
				t.Errorf("stream still contains content, expected end of content")
			}
		})
	}
}

type missingReplaces struct {
	input []*registry.ImageInput
	valid int
}

func (missingReplaces) Generate(r *rand.Rand, max int) reflect.Value {
	if max < 1 {
		max++
	}
	size := r.Intn(max + 1) // [1, max] inputs total
	if size < 1 {
		size++
	}
	valid := r.Intn(size)       // [0, size) valid inputs (at least one invalid)
	pkgSize := r.Intn(size + 1) // Each distinct package contains [1, size] inputs
	if pkgSize == 0 {
		pkgSize++
	}
	var pkg string

	input := make([]*registry.ImageInput, size)
	var version semver.Version
	for i := 0; i < size; i++ {
		if size%pkgSize == 0 {
			pkg = genName("")
		}

		version = bumpVersion(r, version)
		config := inputConfig{
			pkg:     pkg,
			name:    genName(""),
			version: version.String(), // Be sure each bundle has a valid sort order via semver for tests using this generator
		}

		if i >= valid {
			config.replaces = genName("missing-")
		}

		input[i] = newImageInput(&config)
	}

	// Ensure we're not depending on any ordering
	r.Shuffle(len(input), func(i, j int) { input[i], input[j] = input[j], input[i] })

	return reflect.ValueOf(missingReplaces{
		input: input,
		valid: valid,
	})
}

func TestMissingReplacesReturnsError(t *testing.T) {
	f := func(missing missingReplaces) bool {
		if len(missing.input) < 1 {
			return true
		}

		loader := &registryfakes.FakeGraphLoader{
			GenerateStub: func(pkg string) (*registry.Package, error) {
				return &registry.Package{
					Name: pkg,
				}, nil
			},
		}

		stream, err := registry.NewReplacesInputStream(loader, missing.input)
		if err != nil {
			t.Error(err)
			return false
		}

		for i := 0; i < missing.valid; i++ {
			next, err := stream.Next()
			if err != nil {
				t.Errorf("next returned unexpected error: %s", err)
				return false
			}
			if next == nil {
				t.Errorf("next returned unexpected nil")
				return false
			}
		}

		// The next call should result in an error
		next, err := stream.Next()
		if err == nil {
			t.Errorf("expected next to return an error")
			return false
		}
		if next != nil {
			t.Errorf("expected next to return nil on error, got %v instead", next)
			return false
		}

		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}
