package composite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type TestBuilder struct {
	buildError    bool
	validateError bool
}

const buildErr = "TestBuilder Build() error"
const validateErr = "TestBuilder Validate() error"

var _ Builder = &TestBuilder{}

func (tb *TestBuilder) Build(dir string, vd TemplateDefinition) error {
	if tb.buildError {
		return errors.New(buildErr)
	}
	return nil
}

func (tb *TestBuilder) Validate(dir string) error {
	if tb.validateError {
		return errors.New(validateErr)
	}
	return nil
}

func TestCompositeRender(t *testing.T) {
	type testCase struct {
		name              string
		compositeTemplate Template
		compositeCfg      CompositeConfig
		validate          bool
		assertions        func(t *testing.T, err error)
	}

	testCases := []testCase{
		{
			name:     "successful render",
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
								Schema: "olm.builder.test",
								Config: json.RawMessage{},
							},
						},
					},
				},
			},
			assertions: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
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
			err := tc.compositeTemplate.Render(context.Background(), &tc.compositeCfg, tc.validate)
			tc.assertions(t, err)
		})
	}
}
