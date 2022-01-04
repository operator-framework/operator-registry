package registry

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func mustMarshal(t *testing.T, value interface{}) json.RawMessage {
	v, err := json.Marshal(value)
	require.NoError(t, err, "bad test data")
	return v
}

func TestBundleParser(t *testing.T) {
	blankLines := regexp.MustCompile(`[\t\r\n]+`)
	format := func(s string) []byte {
		trimmed := strings.TrimSpace(s)
		return []byte(blankLines.ReplaceAllString(trimmed, "\n"))
	}

	bundleFS := func() fstest.MapFS {
		// Creating a new MapFS for each test case allows concurrent testing
		return fstest.MapFS{
			"manifests/csv.yaml": &fstest.MapFile{
				Data: format(`
apiVersion: operators.coreos.com/v1alpha1
kind: ClusterServiceVersion
metadata:
  name: foo.v1.1.0
spec:
  version: 1.1.0
  replaces: v1.1.0
  install:
    strategy: deployment
    spec:
      permissions:
      - serviceAccountName: foo-operator
        rules:
        - apiGroups:
          - test.io
          resources:
          - foos
          - bars
          verbs:
          - "*"
      deployments:
      - name: etcd-operator
        spec:
          replicas: 1
          selector:
            matchLabels:
              name: foo-operator
          template:
            metadata:
              name: foo-operator
              labels:
                name: foo-operator
            spec:
              serviceAccountName: foo-operator
              containers:
              - name: foo-controller
                command:
                - foo
                image: quay.io/test/foo-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2
  customresourcedefinitions:
    required:
    - name: bars.test.io
      version: v2
      kind: Bar
    owned:
    - name: foos.test.io
      version: v1
      kind: Foo
				`),
			},
			"manifests/crd.yaml": &fstest.MapFile{
				Data: format(`
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: foos.test.io
spec:
  group: test.io
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              thing:
                type: string
  scope: Namespaced
  names:
    plural: foos
    singular: foo
    kind: Foo
				`),
			},
			"metadata/annotations.yaml": &fstest.MapFile{
				Data: format(`
annotations:
  operators.operatorframework.io.bundle.package.v1: "foo"
  operators.operatorframework.io.bundle.channels.v1: "alpha,stable"
  operators.operatorframework.io.bundle.channel.default.v1: "stable"
				`),
			},
		}
	}

	for _, tt := range []struct {
		name   string
		root   fs.FS
		err    bool
		bundle *Bundle
	}{
		{
			name: "NilFS",
			err:  true,
		},
		{
			name: "MissingManifests",
			root: func() fstest.MapFS {
				r := bundleFS()
				for p := range r {
					if strings.HasPrefix(p, "manifests") {
						delete(r, p)
					}
				}

				return r
			}(),
			err: true,
		},
		{
			name: "MissingMetadata",
			root: func() fstest.MapFS {
				r := bundleFS()
				for p := range r {
					if strings.HasPrefix(p, "metadata") {
						delete(r, p)
					}
				}

				return r
			}(),
			err: true,
		},
		{
			name: "MissingAnnotationsFile",
			root: func() fstest.MapFS {
				r := bundleFS()
				delete(r, "metadata/annotations.yaml")

				return r
			}(),
			err: true,
		},
		{
			name: "ManifestsOnly",
			root: bundleFS(),
			bundle: &Bundle{
				Name:     "foo.v1.1.0",
				Package:  "foo",
				Channels: []string{"alpha", "stable"},
				Properties: []Property{
					{
						Type: PackageType,
						Value: mustMarshal(t, PackageProperty{
							PackageName: "foo",
							Version:     "1.1.0",
						}),
					},
					{
						Type: GVKType,
						Value: mustMarshal(t, GVKProperty{
							Group:   "test.io",
							Version: "v1",
							Kind:    "Foo",
						}),
					},
				},
				Annotations: &Annotations{
					PackageName:        "foo",
					Channels:           "alpha,stable",
					DefaultChannelName: "stable",
				},
			},
		},
		{
			name: "WithDependenciesFile",
			root: func() fstest.MapFS {
				r := bundleFS()
				r["metadata/dependencies.yaml"] = &fstest.MapFile{
					Data: format(`
dependencies:
- type: olm.package
  value:
    packageName: bar
    version: 2.0.0
					`),
				}

				return r
			}(),
			bundle: &Bundle{
				Name:     "foo.v1.1.0",
				Package:  "foo",
				Channels: []string{"alpha", "stable"},
				Dependencies: []*Dependency{
					{
						Type: PackageType,
						Value: mustMarshal(t, PackageDependency{
							PackageName: "bar",
							Version:     "2.0.0",
						}),
					},
				},
				Properties: []Property{
					{
						Type: PackageType,
						Value: mustMarshal(t, PackageProperty{
							PackageName: "foo",
							Version:     "1.1.0",
						}),
					},
					{
						Type: GVKType,
						Value: mustMarshal(t, GVKProperty{
							Group:   "test.io",
							Version: "v1",
							Kind:    "Foo",
						}),
					},
				},
				Annotations: &Annotations{
					PackageName:        "foo",
					Channels:           "alpha,stable",
					DefaultChannelName: "stable",
				},
			},
		},
		{
			name: "PropertiesAnnotation",
			root: func() fstest.MapFS {
				r := bundleFS()
				r["manifests/csv.yaml"] = &fstest.MapFile{
					Data: format(fmt.Sprintf(`
apiVersion: operators.coreos.com/v1alpha1
kind: ClusterServiceVersion
metadata:
  name: foo.v1.1.0
  annotations:
    '%s': '%s'
spec:
  version: 1.1.0
  replaces: v1.1.0
				`,
						PropertyKey,
						mustMarshal(t, []Property{
							{
								Type: LabelType,
								Value: mustMarshal(t,
									LabelProperty{
										Label: "baz",
									},
								),
							},
						}),
					)),
				}

				return r
			}(),
			bundle: &Bundle{
				Name:     "foo.v1.1.0",
				Package:  "foo",
				Channels: []string{"alpha", "stable"},
				Properties: []Property{
					{
						Type: LabelType,
						Value: mustMarshal(t, LabelProperty{
							Label: "baz",
						}),
					},
					{
						Type: PackageType,
						Value: mustMarshal(t, PackageProperty{
							PackageName: "foo",
							Version:     "1.1.0",
						}),
					},
					{
						Type: GVKType,
						Value: mustMarshal(t, GVKProperty{
							Group:   "test.io",
							Version: "v1",
							Kind:    "Foo",
						}),
					},
				},
				Annotations: &Annotations{
					PackageName:        "foo",
					Channels:           "alpha,stable",
					DefaultChannelName: "stable",
				},
			},
		},
		{
			name: "PropertiesFile",
			root: func() fstest.MapFS {
				r := bundleFS()
				r["metadata/properties.yaml"] = &fstest.MapFile{
					Data: format(`
properties:
- type: olm.package
  value:
    packageName: fizz
    version: 1.1.0
					`),
				}

				return r
			}(),
			bundle: &Bundle{
				Name:     "foo.v1.1.0",
				Package:  "foo",
				Channels: []string{"alpha", "stable"},
				Properties: []Property{
					// There are no special semantics for specific property types in the parser.
					// Invalid property type combinations should be handled by client-side validation.
					{
						Type: PackageType,
						Value: mustMarshal(t, PackageProperty{
							PackageName: "foo",
							Version:     "1.1.0",
						}),
					},
					{
						Type: PackageType,
						Value: mustMarshal(t, PackageProperty{
							PackageName: "fizz",
							Version:     "1.1.0",
						}),
					},
					{
						Type: GVKType,
						Value: mustMarshal(t, GVKProperty{
							Group:   "test.io",
							Version: "v1",
							Kind:    "Foo",
						}),
					},
				},
				Annotations: &Annotations{
					PackageName:        "foo", // This field comes from the annotations file.
					Channels:           "alpha,stable",
					DefaultChannelName: "stable",
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parser := newBundleParser(logrus.NewEntry(logrus.StandardLogger()))

			bundle, err := parser.Parse(tt.root)
			if tt.err {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, bundle)
			assert.Equal(t, tt.bundle.Name, bundle.Name)
			assert.Equal(t, tt.bundle.Package, bundle.Package)
			assert.ElementsMatch(t, tt.bundle.Channels, bundle.Channels)
			assert.ElementsMatch(t, tt.bundle.Dependencies, bundle.Dependencies)
			assert.ElementsMatch(t, tt.bundle.Properties, bundle.Properties)
			assert.Equal(t, tt.bundle.Annotations, bundle.Annotations)
		})
	}

}

func TestDerivedProperties(t *testing.T) {
	type args struct {
		csv         *ClusterServiceVersion
		version     string
		annotations *Annotations
		crds        []*apiextensionsv1.CustomResourceDefinition
	}
	type expected struct {
		err        bool
		properties []Property
	}

	for _, tt := range []struct {
		name     string
		args     args
		expected expected
	}{
		{
			name: "MissingCSV",
			expected: expected{
				err: true,
			},
		},
		{
			name: "NoProperties",
			args: args{
				csv: &ClusterServiceVersion{
					Spec: json.RawMessage(`{}`),
				},
			},
		},
		{
			name: "BadCSVAnnotationsIgnored",
			args: args{
				csv: &ClusterServiceVersion{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							PropertyKey: "bad",
						},
					},
					Spec: json.RawMessage(`{}`),
				},
			},
		},
		{
			name: "OpaqueFromCSVAnnotations",
			args: args{
				csv: &ClusterServiceVersion{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							PropertyKey: string(mustMarshal(t, []Property{
								{
									Type:  "foo",
									Value: json.RawMessage(`{}`),
								},
							})),
						},
					},
					Spec: json.RawMessage(`{}`),
				},
			},
			expected: expected{
				properties: []Property{
					{
						Type:  "foo",
						Value: json.RawMessage(`{}`),
					},
				},
			},
		},
		{
			name: "PackageFromAnnotations",
			args: args{
				csv: &ClusterServiceVersion{
					Spec: json.RawMessage(`{}`),
				},
				annotations: &Annotations{PackageName: "bar"},
				version:     "1.0.0",
			},
			expected: expected{
				properties: []Property{
					{
						Type: PackageType,
						Value: mustMarshal(t, PackageProperty{
							PackageName: "bar",
							Version:     "1.0.0",
						}),
					},
				},
			},
		},
		{
			name: "GVKs",
			args: args{
				csv: &ClusterServiceVersion{
					Spec: json.RawMessage(`{}`),
				},
				crds: []*apiextensionsv1.CustomResourceDefinition{
					{
						Spec: apiextensionsv1.CustomResourceDefinitionSpec{
							Group: "test.io",
							Names: apiextensionsv1.CustomResourceDefinitionNames{
								Kind:   "Foo",
								Plural: "Foos",
							},
							Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
								{Name: "v1"},
								{Name: "v2alpha1"},
							},
						},
					},
					{
						Spec: apiextensionsv1.CustomResourceDefinitionSpec{
							Group: "test.io",
							Names: apiextensionsv1.CustomResourceDefinitionNames{
								Kind:   "Bar",
								Plural: "Bars",
							},
							Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
								{Name: "v1alpha1"},
							},
						},
					},
				},
			},
			expected: expected{
				properties: []Property{
					{
						Type: GVKType,
						Value: mustMarshal(t, GVKProperty{
							Group:   "test.io",
							Version: "v1",
							Kind:    "Foo",
						}),
					},
					{
						Type: GVKType,
						Value: mustMarshal(t, GVKProperty{
							Group:   "test.io",
							Version: "v2alpha1",
							Kind:    "Foo",
						}),
					},
					{
						Type: GVKType,
						Value: mustMarshal(t, GVKProperty{
							Group:   "test.io",
							Version: "v1alpha1",
							Kind:    "Bar",
						}),
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parser := newBundleParser(logrus.NewEntry(logrus.StandardLogger()))

			in := &Bundle{
				csv:         tt.args.csv,
				version:     tt.args.version,
				Annotations: tt.args.annotations,
				v1crds:      tt.args.crds,
			}

			properties, err := parser.derivedProperties(in)
			if tt.expected.err {
				assert.Error(t, err)
				assert.Nil(t, properties)
				return
			}

			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.expected.properties, properties)
		})
	}

}

func TestPropertySet(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   []Property
		out  []Property
	}{
		{
			name: "RemoveDuplicates",
			in: []Property{
				{
					Type:  "foo",
					Value: json.RawMessage("bar"),
				},
				{
					Type:  "foo",
					Value: json.RawMessage("bar"),
				},
				{
					Type:  "foo",
					Value: json.RawMessage("baz"),
				},
			},
			out: []Property{
				{
					Type:  "foo",
					Value: json.RawMessage("bar"),
				},
				{
					Type:  "foo",
					Value: json.RawMessage("baz"),
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.out, propertySet(tt.in))
		})
	}
}
