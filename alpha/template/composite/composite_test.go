package composite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

var catalogs = []byte(`
schema: olm.composite.catalogs
catalogs:
  - name: first-catalog
    destination:
      baseImage: quay.io/operator-framework/opm:latest
      workingDir: contributions/first-catalog
    builders:
      - olm.builder.semver
      - olm.builder.basic
  - name: second-catalog
    destination:
      baseImage: quay.io/operator-framework/opm:latest
      workingDir: contributions/second-catalog
    builders:
      - olm.builder.semver
  - name: test-catalog
    destination:
	  baseImage: quay.io/operator-framework/opm:latest
	  workingDir: contributions/test-catalog
	builders:
	  - olm.builder.custom`)

var composite = []byte(`
schema: olm.composite
components:
  - name: first-catalog
    destination:
      path: my-operator
    strategy:
      name: semver
      template:
        schema: olm.builder.semver
        config:
          input: components/contribution1.yaml
          output: catalog.yaml
`)

var testCompositeFormat = []byte(`
- name: test-catalog
  destination:
    path: my-package
  strategy:
    name: custom
    template:
      schema: olm.builder.custom
      config:
        command: cat
        args:
          - "components/v4.13.yaml"
        output: catalog.yaml
`)

var semverContribution = []byte(`---
schema: olm.semver
stable:
  bundles:
  - image: quay.io/openshift-community-operators/community-kubevirt-hyperconverged:v1.4.0
  - image: quay.io/openshift-community-operators/community-kubevirt-hyperconverged:v1.4.1
  - image: quay.io/openshift-community-operators/community-kubevirt-hyperconverged:v1.4.2
  - image: quay.io/openshift-community-operators/community-kubevirt-hyperconverged:v1.4.3
  - image: quay.io/openshift-community-operators/community-kubevirt-hyperconverged:v1.4.4
  - image: quay.io/openshift-community-operators/community-kubevirt-hyperconverged:v1.5.0
  - image: quay.io/openshift-community-operators/community-kubevirt-hyperconverged:v1.5.1
  - image: quay.io/openshift-community-operators/community-kubevirt-hyperconverged:v1.5.2
  - image: quay.io/openshift-community-operators/community-kubevirt-hyperconverged:v1.6.0
  - image: quay.io/openshift-community-operators/community-kubevirt-hyperconverged:v1.7.0
  - image: quay.io/openshift-community-operators/community-kubevirt-hyperconverged:v1.8.0
  - image: quay.io/openshift-community-operators/community-kubevirt-hyperconverged:v1.8.1
`)

var basicContribution = []byte(`---
---
schema: olm.package
name: kubevirt-hyperconverged
defaultChannel: stable
---
schema: olm.channel
package: kubevirt-hyperconverged
name: stable
entries:
- name: kubevirt-hyperconverged-operator.v4.10.0
- name: kubevirt-hyperconverged-operator.v4.10.1
  replaces: kubevirt-hyperconverged-operator.v4.10.0
- name: kubevirt-hyperconverged-operator.v4.10.2
  replaces: kubevirt-hyperconverged-operator.v4.10.1
- name: kubevirt-hyperconverged-operator.v4.10.3
  replaces: kubevirt-hyperconverged-operator.v4.10.2
---
schema: olm.bundle
image: registry.redhat.io/container-native-virtualization/hco-bundle-registry@sha256:35b29e8eb48d9818a1217d5b89e4dcb7a900c5c5e6ae3745683813c5708c86e9
---
schema: olm.bundle
image: registry.redhat.io/container-native-virtualization/hco-bundle-registry@sha256:ac8b60a91411c0fcc4ab2c52db8b6e7682ee747c8969dde9ad8d1a5aa7d44772
---
schema: olm.bundle
image: registry.redhat.io/container-native-virtualization/hco-bundle-registry@sha256:9c10a5c4e5ffad632bc8b4289fe2bc0c181bc7c4c8270a356ac21ebff568a45e
---
schema: olm.bundle
image: registry.redhat.io/container-native-virtualization/hco-bundle-registry@sha256:34cf9e0dd3cc07c487b364c30b3f95bf352be8ca4fe89d473fc624ad7283651d
`)

func TestCompositeRender(t *testing.T) {
	type testCase struct {
		name              string
		compositeTemplate Template
		compositeCfg      CompositeConfig
		validate          bool
		assertions        func(t *testing.T, err error)
	}

	testCases := []testCase{
		// {
		// 	name:     "successful render",
		// 	validate: true,
		// 	compositeTemplate: Template{
		// 		CatalogFile: bytes.NewReader(catalogs),
		// 		ContributionFile: bytes.NewReader(composite),

		// 		CatalogBuilders: CatalogBuilderMap{
		// 			"testcatalog": BuilderMap{
		// 				"olm.builder.test": &TestBuilder{},
		// 			},
		// 		},
		// 	},
		// 	compositeCfg: CompositeConfig{
		// 		Schema: CompositeSchema,
		// 		Components: []Component{
		// 			{
		// 				Name: "testcatalog",
		// 				Destination: ComponentDestination{
		// 					Path: "testcatalog/mypackage",
		// 				},
		// 				Strategy: BuildStrategy{
		// 					Name: "testbuild",
		// 					Template: TemplateDefinition{
		// 						Schema: "olm.builder.test",
		// 						Config: json.RawMessage{},
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// 	assertions: func(t *testing.T, err error) {
		// 		require.NoError(t, err)
		// 	},
		// },
		{
			name:     "component not in catalog config",
			validate: true,
			compositeTemplate: Template{
				CatalogBuilders: CatalogBuilderMap{
					"testcatalog": BuilderMap{
						"olm.builder.test": &TestBuilder{},
					},
				},
			},
			compositeCfg: CompositeConfig{
				Schema: CompositeSchema,
				Components: []Component{
					{
						Name: "invalid",
						Destination: ComponentDestination{
							Path: "testcatalog/mypackage",
						},
						Strategy: BuildStrategy{
							Name: "testbuild",
							Template: TemplateDefinition{
								Schema: "olm.builder.test",
								Config: json.RawMessage{},
							},
						},
					},
				},
			},
			assertions: func(t *testing.T, err error) {
				require.Error(t, err)
				expectedErr := fmt.Sprintf("building component %q: component does not exist in the catalog configuration. Available components are: %s", "invalid", []string{"testcatalog"})
				require.Equal(t, expectedErr, err.Error())
			},
		},
		{
			name:     "builder not in catalog config",
			validate: true,
			compositeTemplate: Template{
				CatalogBuilders: CatalogBuilderMap{
					"testcatalog": BuilderMap{
						"olm.builder.test": &TestBuilder{},
					},
				},
			},
			compositeCfg: CompositeConfig{
				Schema: CompositeSchema,
				Components: []Component{
					{
						Name: "testcatalog",
						Destination: ComponentDestination{
							Path: "testcatalog/mypackage",
						},
						Strategy: BuildStrategy{
							Name: "testbuild",
							Template: TemplateDefinition{
								Schema: "olm.builder.invalid",
								Config: json.RawMessage{},
							},
						},
					},
				},
			},
			assertions: func(t *testing.T, err error) {
				require.Error(t, err)
				expectedErr := fmt.Sprintf("building component %q: no builder found for template schema %q", "testcatalog", "olm.builder.invalid")
				require.Equal(t, expectedErr, err.Error())
			},
		},
		{
			name:     "build step error",
			validate: true,
			compositeTemplate: Template{
				CatalogBuilders: CatalogBuilderMap{
					"testcatalog": BuilderMap{
						"olm.builder.test": &TestBuilder{buildError: true},
					},
				},
			},
			compositeCfg: CompositeConfig{
				Schema: CompositeSchema,
				Components: []Component{
					{
						Name: "testcatalog",
						Destination: ComponentDestination{
							Path: "testcatalog/mypackage",
						},
						Strategy: BuildStrategy{
							Name: "testbuild",
							Template: TemplateDefinition{
								Schema: "olm.builder.test",
								Config: json.RawMessage{},
							},
						},
					},
				},
			},
			assertions: func(t *testing.T, err error) {
				require.Error(t, err)
				expectedErr := fmt.Sprintf("building component %q: %s", "testcatalog", buildErr)
				require.Equal(t, expectedErr, err.Error())
			},
		},
		{
			name:     "validate step error",
			validate: true,
			compositeTemplate: Template{
				CatalogBuilders: CatalogBuilderMap{
					"testcatalog": BuilderMap{
						"olm.builder.test": &TestBuilder{validateError: true},
					},
				},
			},
			compositeCfg: CompositeConfig{
				Schema: CompositeSchema,
				Components: []Component{
					{
						Name: "testcatalog",
						Destination: ComponentDestination{
							Path: "testcatalog/mypackage",
						},
						Strategy: BuildStrategy{
							Name: "testbuild",
							Template: TemplateDefinition{
								Schema: "olm.builder.test",
								Config: json.RawMessage{},
							},
						},
					},
				},
			},
			assertions: func(t *testing.T, err error) {
				require.Error(t, err)
				expectedErr := fmt.Sprintf("validating component %q: %s", "testcatalog", validateErr)
				require.Equal(t, expectedErr, err.Error())
			},
		},
		{
			name:     "validation step skipped",
			validate: false,
			compositeTemplate: Template{
				CatalogBuilders: CatalogBuilderMap{
					"testcatalog": BuilderMap{
						"olm.builder.test": &TestBuilder{validateError: true},
					},
				},
			},
			compositeCfg: CompositeConfig{
				Schema: CompositeSchema,
				Components: []Component{
					{
						Name: "testcatalog",
						Destination: ComponentDestination{
							Path: "testcatalog/mypackage",
						},
						Strategy: BuildStrategy{
							Name: "testbuild",
							Template: TemplateDefinition{
								Schema: "olm.builder.test",
								Config: json.RawMessage{},
							},
						},
					},
				},
			},
			assertions: func(t *testing.T, err error) {
				// the validate step would error but since
				// we are skipping it we expect no error to occur
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.compositeTemplate.Render(context.Background(), tc.validate)
			tc.assertions(t, err)
		})
	}
}
