package composite

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"testing"

	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/stretchr/testify/require"
)

// TODO: Should we consolidate all these tests into a singular test function?
// It was intentional to keep them separate for now, but it would significantly reduce code replication to combine into one function
func TestBasicBuilder(t *testing.T) {
	type testCase struct {
		name               string
		validate           bool
		basicBuilder       *BasicBuilder
		templateDefinition TemplateDefinition
		files              map[string]string
		buildAssertions    func(t *testing.T, dir string, buildErr error)
		validateAssertions func(t *testing.T, validateErr error)
	}

	testDir := t.TempDir()
	validConfigTemplate := `{
		"input": "%s",
		"output": "%s"
	}`

	testCases := []testCase{
		{
			name:     "successful basic build yaml output",
			validate: true,
			basicBuilder: NewBasicBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: BasicBuilderSchema,
				Config: []byte(fmt.Sprintf(validConfigTemplate, path.Join(testDir, "components/basic.yaml"), "catalog.yaml")),
			},
			files: map[string]string{
				"components/basic.yaml": basicYaml,
			},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.NoError(t, buildErr)
				// check if the catalog.yaml file exists in the correct place
				filePath := path.Join(dir, "catalog.yaml")
				_, err := os.Stat(filePath)
				require.NoError(t, err)
				file, err := os.Open(filePath)
				require.NoError(t, err)
				defer file.Close()
				fileData, err := io.ReadAll(file)
				require.NoError(t, err)
				require.Equal(t, string(fileData), basicBuiltFbcYaml)
			},
			validateAssertions: func(t *testing.T, validateErr error) {
				require.NoError(t, validateErr)
			},
		},
		{
			name:     "successful basic build json output",
			validate: true,
			basicBuilder: NewBasicBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "json",
			}),
			templateDefinition: TemplateDefinition{
				Schema: BasicBuilderSchema,
				Config: []byte(fmt.Sprintf(validConfigTemplate, path.Join(testDir, "components/basic.yaml"), "catalog.json")),
			},
			files: map[string]string{
				"components/basic.yaml": basicYaml,
			},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.NoError(t, buildErr)
				// check if the catalog.yaml file exists in the correct place
				filePath := path.Join(dir, "catalog.json")
				_, err := os.Stat(filePath)
				require.NoError(t, err)
				file, err := os.Open(filePath)
				require.NoError(t, err)
				defer file.Close()
				fileData, err := io.ReadAll(file)
				require.NoError(t, err)
				require.Equal(t, string(fileData), basicBuiltFbcJson)
			},
			validateAssertions: func(t *testing.T, validateErr error) {
				require.NoError(t, validateErr)
			},
		},
		{
			name:     "invalid template configuration",
			validate: false,
			basicBuilder: NewBasicBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: BasicBuilderSchema,
				Config: []byte(`{
					"invalid": "components/basic.yaml",
				}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Contains(t, buildErr.Error(), "unmarshalling basic template config:")
			},
		},
		{
			name:     "invalid output type",
			validate: false,
			basicBuilder: NewBasicBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "invalid",
			}),
			templateDefinition: TemplateDefinition{
				Schema: BasicBuilderSchema,
				Config: []byte(fmt.Sprintf(validConfigTemplate, path.Join(testDir, "components/basic.yaml"), "catalog.yaml")),
			},
			files: map[string]string{
				"components/basic.yaml": basicYaml,
			},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Contains(t, buildErr.Error(), fmt.Sprintf("invalid --output value %q, expected (json|yaml)", "invalid"))
			},
		},
		{
			name:     "invalid schema",
			validate: false,
			basicBuilder: NewBasicBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: "olm.invalid",
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Contains(t, buildErr.Error(), fmt.Sprintf("schema %q does not match the basic template builder schema %q", "olm.invalid", BasicBuilderSchema))
			},
		},
		{
			name:     "template config has empty input",
			validate: false,
			basicBuilder: NewBasicBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: BasicBuilderSchema,
				Config: []byte(`{
					"output": "catalog.yaml"
				}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Equal(t,
					buildErr.Error(),
					"basic template configuration is invalid: basic template config must have a non-empty input (templateDefinition.config.input)")
			},
		},
		{
			name:     "template config has empty output",
			validate: false,
			basicBuilder: NewBasicBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: BasicBuilderSchema,
				Config: []byte(`{
					"input": "components/basic.yaml"
				}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Equal(t,
					buildErr.Error(),
					"basic template configuration is invalid: basic template config must have a non-empty output (templateDefinition.config.output)")
			},
		},
		{
			name:     "template config has empty input & output",
			validate: false,
			basicBuilder: NewBasicBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: BasicBuilderSchema,
				Config: []byte(`{}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Equal(t,
					buildErr.Error(),
					"basic template configuration is invalid: basic template config must have a non-empty input (templateDefinition.config.input),basic template config must have a non-empty output (templateDefinition.config.output)")
			},
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outDir := fmt.Sprintf("basic-%d", i)
			outPath := path.Join(testDir, outDir)
			err := os.MkdirAll(outPath, 0o777)
			require.NoError(t, err)

			// create files in temp dir
			for fileName, fileContents := range tc.files {
				err := os.MkdirAll(path.Join(testDir, path.Dir(fileName)), 0o777)
				require.NoError(t, err)
				file, err := os.Create(path.Join(testDir, fileName))
				require.NoError(t, err)
				_, err = file.WriteString(fileContents)
				require.NoError(t, err)
			}

			cacheDir, err := os.MkdirTemp("", "opm-registry-")
			require.NoError(t, err)

			reg, err := containerdregistry.NewRegistry(
				containerdregistry.WithCacheDir(cacheDir),
			)
			defer reg.Destroy()
			require.NoError(t, err)

			buildErr := tc.basicBuilder.Build(context.Background(), reg, outDir, tc.templateDefinition)
			tc.buildAssertions(t, outPath, buildErr)

			if tc.validate {
				validateErr := tc.basicBuilder.Validate(outDir)
				tc.validateAssertions(t, validateErr)
			}
		})
	}
}

const basicYaml = `---
defaultChannel: preview
name: webhook-operator
schema: olm.package
---
image: quay.io/olmtest/webhook-operator-bundle:0.0.3
name: webhook-operator.v0.0.1
schema: olm.bundle
---
schema: olm.channel
package: webhook-operator
name: preview
entries:
  - name: webhook-operator.v0.0.1
`

const basicBuiltFbcYaml = `---
defaultChannel: preview
name: webhook-operator
schema: olm.package
---
entries:
- name: webhook-operator.v0.0.1
name: preview
package: webhook-operator
schema: olm.channel
---
image: quay.io/olmtest/webhook-operator-bundle:0.0.3
name: webhook-operator.v0.0.1
package: webhook-operator
properties:
- type: olm.gvk
  value:
    group: webhook.operators.coreos.io
    kind: WebhookTest
    version: v1
- type: olm.gvk
  value:
    group: webhook.operators.coreos.io
    kind: WebhookTest
    version: v2
- type: olm.package
  value:
    packageName: webhook-operator
    version: 0.0.1
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoicmJhYy5hdXRob3JpemF0aW9uLms4cy5pby92MWJldGExIiwia2luZCI6IkNsdXN0ZXJSb2xlIiwibWV0YWRhdGEiOnsiY3JlYXRpb25UaW1lc3RhbXAiOm51bGwsIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLW1ldHJpY3MtcmVhZGVyIn0sInJ1bGVzIjpbeyJub25SZXNvdXJjZVVSTHMiOlsiL21ldHJpY3MiXSwidmVyYnMiOlsiZ2V0Il19XX0=
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoib3BlcmF0b3JzLmNvcmVvcy5jb20vdjFhbHBoYTEiLCJraW5kIjoiQ2x1c3RlclNlcnZpY2VWZXJzaW9uIiwibWV0YWRhdGEiOnsiYW5ub3RhdGlvbnMiOnsiYWxtLWV4YW1wbGVzIjoiW1xuICB7XG4gICAgXCJhcGlWZXJzaW9uXCI6IFwid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvL3YxXCIsXG4gICAgXCJraW5kXCI6IFwiV2ViaG9va1Rlc3RcIixcbiAgICBcIm1ldGFkYXRhXCI6IHtcbiAgICAgIFwibmFtZVwiOiBcIndlYmhvb2t0ZXN0LXNhbXBsZVwiLFxuICAgICAgXCJuYW1lc3BhY2VcIjogXCJ3ZWJob29rLW9wZXJhdG9yLXN5c3RlbVwiXG4gICAgfSxcbiAgICBcInNwZWNcIjoge1xuICAgICAgXCJ2YWxpZFwiOiB0cnVlXG4gICAgfVxuICB9XG5dIiwiY2FwYWJpbGl0aWVzIjoiQmFzaWMgSW5zdGFsbCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9idWlsZGVyIjoib3BlcmF0b3Itc2RrLXYxLjAuMCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9wcm9qZWN0X2xheW91dCI6ImdvIn0sIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLnYwLjAuMSIsIm5hbWVzcGFjZSI6InBsYWNlaG9sZGVyIn0sInNwZWMiOnsiYXBpc2VydmljZWRlZmluaXRpb25zIjp7fSwiY3VzdG9tcmVzb3VyY2VkZWZpbml0aW9ucyI6eyJvd25lZCI6W3sia2luZCI6IldlYmhvb2tUZXN0IiwibmFtZSI6IndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iLCJ2ZXJzaW9uIjoidjEifV19LCJkZXNjcmlwdGlvbiI6IldlYmhvb2sgT3BlcmF0b3IgZGVzY3JpcHRpb24uIFRPRE8uIiwiZGlzcGxheU5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwiaWNvbiI6W3siYmFzZTY0ZGF0YSI6IiIsIm1lZGlhdHlwZSI6IiJ9XSwiaW5zdGFsbCI6eyJzcGVjIjp7ImNsdXN0ZXJQZXJtaXNzaW9ucyI6W3sicnVsZXMiOlt7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cyJdLCJ2ZXJicyI6WyJjcmVhdGUiLCJkZWxldGUiLCJnZXQiLCJsaXN0IiwicGF0Y2giLCJ1cGRhdGUiLCJ3YXRjaCJdfSx7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cy9zdGF0dXMiXSwidmVyYnMiOlsiZ2V0IiwicGF0Y2giLCJ1cGRhdGUiXX0seyJhcGlHcm91cHMiOlsiYXV0aGVudGljYXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJ0b2tlbnJldmlld3MiXSwidmVyYnMiOlsiY3JlYXRlIl19LHsiYXBpR3JvdXBzIjpbImF1dGhvcml6YXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJzdWJqZWN0YWNjZXNzcmV2aWV3cyJdLCJ2ZXJicyI6WyJjcmVhdGUiXX1dLCJzZXJ2aWNlQWNjb3VudE5hbWUiOiJkZWZhdWx0In1dLCJkZXBsb3ltZW50cyI6W3sibmFtZSI6IndlYmhvb2stb3BlcmF0b3Itd2ViaG9vayIsInNwZWMiOnsicmVwbGljYXMiOjEsInNlbGVjdG9yIjp7Im1hdGNoTGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInN0cmF0ZWd5Ijp7fSwidGVtcGxhdGUiOnsibWV0YWRhdGEiOnsibGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInNwZWMiOnsiY29udGFpbmVycyI6W3siYXJncyI6WyItLXNlY3VyZS1saXN0ZW4tYWRkcmVzcz0wLjAuMC4wOjg0NDMiLCItLXVwc3RyZWFtPWh0dHA6Ly8xMjcuMC4wLjE6ODA4MC8iLCItLWxvZ3Rvc3RkZXJyPXRydWUiLCItLXY9MTAiXSwiaW1hZ2UiOiJnY3IuaW8va3ViZWJ1aWxkZXIva3ViZS1yYmFjLXByb3h5OnYwLjUuMCIsIm5hbWUiOiJrdWJlLXJiYWMtcHJveHkiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6ODQ0MywibmFtZSI6Imh0dHBzIn1dLCJyZXNvdXJjZXMiOnt9fSx7ImFyZ3MiOlsiLS1tZXRyaWNzLWFkZHI9MTI3LjAuMC4xOjgwODAiLCItLWVuYWJsZS1sZWFkZXItZWxlY3Rpb24iXSwiY29tbWFuZCI6WyIvbWFuYWdlciJdLCJpbWFnZSI6InF1YXkuaW8vb2xtdGVzdC93ZWJob29rLW9wZXJhdG9yOjAuMC4zIiwibmFtZSI6Im1hbmFnZXIiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6OTQ0MywibmFtZSI6IndlYmhvb2stc2VydmVyIiwicHJvdG9jb2wiOiJUQ1AifV0sInJlc291cmNlcyI6eyJsaW1pdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjMwTWkifSwicmVxdWVzdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjIwTWkifX19XSwidGVybWluYXRpb25HcmFjZVBlcmlvZFNlY29uZHMiOjEwfX19fV0sInBlcm1pc3Npb25zIjpbeyJydWxlcyI6W3siYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiY29uZmlnbWFwcyJdLCJ2ZXJicyI6WyJnZXQiLCJsaXN0Iiwid2F0Y2giLCJjcmVhdGUiLCJ1cGRhdGUiLCJwYXRjaCIsImRlbGV0ZSJdfSx7ImFwaUdyb3VwcyI6WyIiXSwicmVzb3VyY2VzIjpbImNvbmZpZ21hcHMvc3RhdHVzIl0sInZlcmJzIjpbImdldCIsInVwZGF0ZSIsInBhdGNoIl19LHsiYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiZXZlbnRzIl0sInZlcmJzIjpbImNyZWF0ZSJdfV0sInNlcnZpY2VBY2NvdW50TmFtZSI6ImRlZmF1bHQifV19LCJzdHJhdGVneSI6ImRlcGxveW1lbnQifSwiaW5zdGFsbE1vZGVzIjpbeyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiT3duTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiU2luZ2xlTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiTXVsdGlOYW1lc3BhY2UifSx7InN1cHBvcnRlZCI6dHJ1ZSwidHlwZSI6IkFsbE5hbWVzcGFjZXMifV0sImtleXdvcmRzIjpbIndlYmhvb2stb3BlcmF0b3IiXSwibGlua3MiOlt7Im5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwidXJsIjoiaHR0cHM6Ly93ZWJob29rLW9wZXJhdG9yLmRvbWFpbiJ9XSwibWFpbnRhaW5lcnMiOlt7ImVtYWlsIjoieW91ckBlbWFpbC5jb20iLCJuYW1lIjoiTWFpbnRhaW5lciBOYW1lIn1dLCJtYXR1cml0eSI6ImFscGhhIiwicHJvdmlkZXIiOnsibmFtZSI6IlByb3ZpZGVyIE5hbWUiLCJ1cmwiOiJodHRwczovL3lvdXIuZG9tYWluIn0sInZlcnNpb24iOiIwLjAuMSIsIndlYmhvb2tkZWZpbml0aW9ucyI6W3siYWRtaXNzaW9uUmV2aWV3VmVyc2lvbnMiOlsidjFiZXRhMSIsInYxIl0sImNvbnRhaW5lclBvcnQiOjQ0MywiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6InZ3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiVmFsaWRhdGluZ0FkbWlzc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii92YWxpZGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImRlcGxveW1lbnROYW1lIjoid2ViaG9vay1vcGVyYXRvci13ZWJob29rIiwiZmFpbHVyZVBvbGljeSI6IkZhaWwiLCJnZW5lcmF0ZU5hbWUiOiJtd2ViaG9va3Rlc3Qua2IuaW8iLCJydWxlcyI6W3siYXBpR3JvdXBzIjpbIndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJdLCJhcGlWZXJzaW9ucyI6WyJ2MSJdLCJvcGVyYXRpb25zIjpbIkNSRUFURSIsIlVQREFURSJdLCJyZXNvdXJjZXMiOlsid2ViaG9va3Rlc3RzIl19XSwic2lkZUVmZmVjdHMiOiJOb25lIiwidGFyZ2V0UG9ydCI6NDM0MywidHlwZSI6Ik11dGF0aW5nQWRtaXNzaW9uV2ViaG9vayIsIndlYmhvb2tQYXRoIjoiL211dGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImNvbnZlcnNpb25DUkRzIjpbIndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6ImN3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiQ29udmVyc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii9jb252ZXJ0In1dfX0=
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoiYXBpZXh0ZW5zaW9ucy5rOHMuaW8vdjFiZXRhMSIsImtpbmQiOiJDdXN0b21SZXNvdXJjZURlZmluaXRpb24iLCJtZXRhZGF0YSI6eyJhbm5vdGF0aW9ucyI6eyJjb250cm9sbGVyLWdlbi5rdWJlYnVpbGRlci5pby92ZXJzaW9uIjoidjAuMy4wIn0sImNyZWF0aW9uVGltZXN0YW1wIjpudWxsLCJuYW1lIjoid2ViaG9va3Rlc3RzLndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJ9LCJzcGVjIjp7Imdyb3VwIjoid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIiwibmFtZXMiOnsia2luZCI6IldlYmhvb2tUZXN0IiwibGlzdEtpbmQiOiJXZWJob29rVGVzdExpc3QiLCJwbHVyYWwiOiJ3ZWJob29rdGVzdHMiLCJzaW5ndWxhciI6IndlYmhvb2t0ZXN0In0sInByZXNlcnZlVW5rbm93bkZpZWxkcyI6ZmFsc2UsInNjb3BlIjoiTmFtZXNwYWNlZCIsInZlcnNpb24iOiJ2MSIsInZlcnNpb25zIjpbeyJuYW1lIjoidjEiLCJzY2hlbWEiOnsib3BlbkFQSVYzU2NoZW1hIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3QgaXMgdGhlIFNjaGVtYSBmb3IgdGhlIHdlYmhvb2t0ZXN0cyBBUEkiLCJwcm9wZXJ0aWVzIjp7ImFwaVZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJBUElWZXJzaW9uIGRlZmluZXMgdGhlIHZlcnNpb25lZCBzY2hlbWEgb2YgdGhpcyByZXByZXNlbnRhdGlvbiBvZiBhbiBvYmplY3QuIFNlcnZlcnMgc2hvdWxkIGNvbnZlcnQgcmVjb2duaXplZCBzY2hlbWFzIHRvIHRoZSBsYXRlc3QgaW50ZXJuYWwgdmFsdWUsIGFuZCBtYXkgcmVqZWN0IHVucmVjb2duaXplZCB2YWx1ZXMuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjcmVzb3VyY2VzIiwidHlwZSI6InN0cmluZyJ9LCJraW5kIjp7ImRlc2NyaXB0aW9uIjoiS2luZCBpcyBhIHN0cmluZyB2YWx1ZSByZXByZXNlbnRpbmcgdGhlIFJFU1QgcmVzb3VyY2UgdGhpcyBvYmplY3QgcmVwcmVzZW50cy4gU2VydmVycyBtYXkgaW5mZXIgdGhpcyBmcm9tIHRoZSBlbmRwb2ludCB0aGUgY2xpZW50IHN1Ym1pdHMgcmVxdWVzdHMgdG8uIENhbm5vdCBiZSB1cGRhdGVkLiBJbiBDYW1lbENhc2UuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjdHlwZXMta2luZHMiLCJ0eXBlIjoic3RyaW5nIn0sIm1ldGFkYXRhIjp7InR5cGUiOiJvYmplY3QifSwic3BlYyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3BlYyBkZWZpbmVzIHRoZSBkZXNpcmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwicHJvcGVydGllcyI6eyJtdXRhdGUiOnsiZGVzY3JpcHRpb24iOiJNdXRhdGUgaXMgYSBmaWVsZCB0aGF0IHdpbGwgYmUgc2V0IHRvIHRydWUgYnkgdGhlIG11dGF0aW5nIHdlYmhvb2suIiwidHlwZSI6ImJvb2xlYW4ifSwidmFsaWQiOnsiZGVzY3JpcHRpb24iOiJWYWxpZCBtdXN0IGJlIHNldCB0byB0cnVlIG9yIHRoZSB2YWxpZGF0aW9uIHdlYmhvb2sgd2lsbCByZWplY3QgdGhlIHJlc291cmNlLiIsInR5cGUiOiJib29sZWFuIn19LCJyZXF1aXJlZCI6WyJ2YWxpZCJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjp0cnVlfSx7Im5hbWUiOiJ2MiIsInNjaGVtYSI6eyJvcGVuQVBJVjNTY2hlbWEiOnsiZGVzY3JpcHRpb24iOiJXZWJob29rVGVzdCBpcyB0aGUgU2NoZW1hIGZvciB0aGUgd2ViaG9va3Rlc3RzIEFQSSIsInByb3BlcnRpZXMiOnsiYXBpVmVyc2lvbiI6eyJkZXNjcmlwdGlvbiI6IkFQSVZlcnNpb24gZGVmaW5lcyB0aGUgdmVyc2lvbmVkIHNjaGVtYSBvZiB0aGlzIHJlcHJlc2VudGF0aW9uIG9mIGFuIG9iamVjdC4gU2VydmVycyBzaG91bGQgY29udmVydCByZWNvZ25pemVkIHNjaGVtYXMgdG8gdGhlIGxhdGVzdCBpbnRlcm5hbCB2YWx1ZSwgYW5kIG1heSByZWplY3QgdW5yZWNvZ25pemVkIHZhbHVlcy4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCNyZXNvdXJjZXMiLCJ0eXBlIjoic3RyaW5nIn0sImtpbmQiOnsiZGVzY3JpcHRpb24iOiJLaW5kIGlzIGEgc3RyaW5nIHZhbHVlIHJlcHJlc2VudGluZyB0aGUgUkVTVCByZXNvdXJjZSB0aGlzIG9iamVjdCByZXByZXNlbnRzLiBTZXJ2ZXJzIG1heSBpbmZlciB0aGlzIGZyb20gdGhlIGVuZHBvaW50IHRoZSBjbGllbnQgc3VibWl0cyByZXF1ZXN0cyB0by4gQ2Fubm90IGJlIHVwZGF0ZWQuIEluIENhbWVsQ2FzZS4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCN0eXBlcy1raW5kcyIsInR5cGUiOiJzdHJpbmcifSwibWV0YWRhdGEiOnsidHlwZSI6Im9iamVjdCJ9LCJzcGVjIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3RTcGVjIGRlZmluZXMgdGhlIGRlc2lyZWQgc3RhdGUgb2YgV2ViaG9va1Rlc3QiLCJwcm9wZXJ0aWVzIjp7ImNvbnZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJDb252ZXJzaW9uIGlzIGFuIGV4YW1wbGUgZmllbGQgb2YgV2ViaG9va1Rlc3QuIEVkaXQgV2ViaG9va1Rlc3RfdHlwZXMuZ28gdG8gcmVtb3ZlL3VwZGF0ZSIsInByb3BlcnRpZXMiOnsibXV0YXRlIjp7ImRlc2NyaXB0aW9uIjoiTXV0YXRlIGlzIGEgZmllbGQgdGhhdCB3aWxsIGJlIHNldCB0byB0cnVlIGJ5IHRoZSBtdXRhdGluZyB3ZWJob29rLiIsInR5cGUiOiJib29sZWFuIn0sInZhbGlkIjp7ImRlc2NyaXB0aW9uIjoiVmFsaWQgbXVzdCBiZSBzZXQgdG8gdHJ1ZSBvciB0aGUgdmFsaWRhdGlvbiB3ZWJob29rIHdpbGwgcmVqZWN0IHRoZSByZXNvdXJjZS4iLCJ0eXBlIjoiYm9vbGVhbiJ9fSwicmVxdWlyZWQiOlsidmFsaWQiXSwidHlwZSI6Im9iamVjdCJ9fSwicmVxdWlyZWQiOlsiY29udmVyc2lvbiJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjpmYWxzZX1dfSwic3RhdHVzIjp7ImFjY2VwdGVkTmFtZXMiOnsia2luZCI6IiIsInBsdXJhbCI6IiJ9LCJjb25kaXRpb25zIjpbXSwic3RvcmVkVmVyc2lvbnMiOltdfX0=
relatedImages:
- image: gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0
  name: ""
- image: quay.io/olmtest/webhook-operator-bundle:0.0.3
  name: ""
- image: quay.io/olmtest/webhook-operator:0.0.3
  name: ""
schema: olm.bundle
`

const basicBuiltFbcJson = `{
    "schema": "olm.package",
    "name": "webhook-operator",
    "defaultChannel": "preview"
}
{
    "schema": "olm.channel",
    "name": "preview",
    "package": "webhook-operator",
    "entries": [
        {
            "name": "webhook-operator.v0.0.1"
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "webhook-operator.v0.0.1",
    "package": "webhook-operator",
    "image": "quay.io/olmtest/webhook-operator-bundle:0.0.3",
    "properties": [
        {
            "type": "olm.gvk",
            "value": {
                "group": "webhook.operators.coreos.io",
                "kind": "WebhookTest",
                "version": "v1"
            }
        },
        {
            "type": "olm.gvk",
            "value": {
                "group": "webhook.operators.coreos.io",
                "kind": "WebhookTest",
                "version": "v2"
            }
        },
        {
            "type": "olm.package",
            "value": {
                "packageName": "webhook-operator",
                "version": "0.0.1"
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJhcGlWZXJzaW9uIjoicmJhYy5hdXRob3JpemF0aW9uLms4cy5pby92MWJldGExIiwia2luZCI6IkNsdXN0ZXJSb2xlIiwibWV0YWRhdGEiOnsiY3JlYXRpb25UaW1lc3RhbXAiOm51bGwsIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLW1ldHJpY3MtcmVhZGVyIn0sInJ1bGVzIjpbeyJub25SZXNvdXJjZVVSTHMiOlsiL21ldHJpY3MiXSwidmVyYnMiOlsiZ2V0Il19XX0="
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJhcGlWZXJzaW9uIjoib3BlcmF0b3JzLmNvcmVvcy5jb20vdjFhbHBoYTEiLCJraW5kIjoiQ2x1c3RlclNlcnZpY2VWZXJzaW9uIiwibWV0YWRhdGEiOnsiYW5ub3RhdGlvbnMiOnsiYWxtLWV4YW1wbGVzIjoiW1xuICB7XG4gICAgXCJhcGlWZXJzaW9uXCI6IFwid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvL3YxXCIsXG4gICAgXCJraW5kXCI6IFwiV2ViaG9va1Rlc3RcIixcbiAgICBcIm1ldGFkYXRhXCI6IHtcbiAgICAgIFwibmFtZVwiOiBcIndlYmhvb2t0ZXN0LXNhbXBsZVwiLFxuICAgICAgXCJuYW1lc3BhY2VcIjogXCJ3ZWJob29rLW9wZXJhdG9yLXN5c3RlbVwiXG4gICAgfSxcbiAgICBcInNwZWNcIjoge1xuICAgICAgXCJ2YWxpZFwiOiB0cnVlXG4gICAgfVxuICB9XG5dIiwiY2FwYWJpbGl0aWVzIjoiQmFzaWMgSW5zdGFsbCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9idWlsZGVyIjoib3BlcmF0b3Itc2RrLXYxLjAuMCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9wcm9qZWN0X2xheW91dCI6ImdvIn0sIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLnYwLjAuMSIsIm5hbWVzcGFjZSI6InBsYWNlaG9sZGVyIn0sInNwZWMiOnsiYXBpc2VydmljZWRlZmluaXRpb25zIjp7fSwiY3VzdG9tcmVzb3VyY2VkZWZpbml0aW9ucyI6eyJvd25lZCI6W3sia2luZCI6IldlYmhvb2tUZXN0IiwibmFtZSI6IndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iLCJ2ZXJzaW9uIjoidjEifV19LCJkZXNjcmlwdGlvbiI6IldlYmhvb2sgT3BlcmF0b3IgZGVzY3JpcHRpb24uIFRPRE8uIiwiZGlzcGxheU5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwiaWNvbiI6W3siYmFzZTY0ZGF0YSI6IiIsIm1lZGlhdHlwZSI6IiJ9XSwiaW5zdGFsbCI6eyJzcGVjIjp7ImNsdXN0ZXJQZXJtaXNzaW9ucyI6W3sicnVsZXMiOlt7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cyJdLCJ2ZXJicyI6WyJjcmVhdGUiLCJkZWxldGUiLCJnZXQiLCJsaXN0IiwicGF0Y2giLCJ1cGRhdGUiLCJ3YXRjaCJdfSx7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cy9zdGF0dXMiXSwidmVyYnMiOlsiZ2V0IiwicGF0Y2giLCJ1cGRhdGUiXX0seyJhcGlHcm91cHMiOlsiYXV0aGVudGljYXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJ0b2tlbnJldmlld3MiXSwidmVyYnMiOlsiY3JlYXRlIl19LHsiYXBpR3JvdXBzIjpbImF1dGhvcml6YXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJzdWJqZWN0YWNjZXNzcmV2aWV3cyJdLCJ2ZXJicyI6WyJjcmVhdGUiXX1dLCJzZXJ2aWNlQWNjb3VudE5hbWUiOiJkZWZhdWx0In1dLCJkZXBsb3ltZW50cyI6W3sibmFtZSI6IndlYmhvb2stb3BlcmF0b3Itd2ViaG9vayIsInNwZWMiOnsicmVwbGljYXMiOjEsInNlbGVjdG9yIjp7Im1hdGNoTGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInN0cmF0ZWd5Ijp7fSwidGVtcGxhdGUiOnsibWV0YWRhdGEiOnsibGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInNwZWMiOnsiY29udGFpbmVycyI6W3siYXJncyI6WyItLXNlY3VyZS1saXN0ZW4tYWRkcmVzcz0wLjAuMC4wOjg0NDMiLCItLXVwc3RyZWFtPWh0dHA6Ly8xMjcuMC4wLjE6ODA4MC8iLCItLWxvZ3Rvc3RkZXJyPXRydWUiLCItLXY9MTAiXSwiaW1hZ2UiOiJnY3IuaW8va3ViZWJ1aWxkZXIva3ViZS1yYmFjLXByb3h5OnYwLjUuMCIsIm5hbWUiOiJrdWJlLXJiYWMtcHJveHkiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6ODQ0MywibmFtZSI6Imh0dHBzIn1dLCJyZXNvdXJjZXMiOnt9fSx7ImFyZ3MiOlsiLS1tZXRyaWNzLWFkZHI9MTI3LjAuMC4xOjgwODAiLCItLWVuYWJsZS1sZWFkZXItZWxlY3Rpb24iXSwiY29tbWFuZCI6WyIvbWFuYWdlciJdLCJpbWFnZSI6InF1YXkuaW8vb2xtdGVzdC93ZWJob29rLW9wZXJhdG9yOjAuMC4zIiwibmFtZSI6Im1hbmFnZXIiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6OTQ0MywibmFtZSI6IndlYmhvb2stc2VydmVyIiwicHJvdG9jb2wiOiJUQ1AifV0sInJlc291cmNlcyI6eyJsaW1pdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjMwTWkifSwicmVxdWVzdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjIwTWkifX19XSwidGVybWluYXRpb25HcmFjZVBlcmlvZFNlY29uZHMiOjEwfX19fV0sInBlcm1pc3Npb25zIjpbeyJydWxlcyI6W3siYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiY29uZmlnbWFwcyJdLCJ2ZXJicyI6WyJnZXQiLCJsaXN0Iiwid2F0Y2giLCJjcmVhdGUiLCJ1cGRhdGUiLCJwYXRjaCIsImRlbGV0ZSJdfSx7ImFwaUdyb3VwcyI6WyIiXSwicmVzb3VyY2VzIjpbImNvbmZpZ21hcHMvc3RhdHVzIl0sInZlcmJzIjpbImdldCIsInVwZGF0ZSIsInBhdGNoIl19LHsiYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiZXZlbnRzIl0sInZlcmJzIjpbImNyZWF0ZSJdfV0sInNlcnZpY2VBY2NvdW50TmFtZSI6ImRlZmF1bHQifV19LCJzdHJhdGVneSI6ImRlcGxveW1lbnQifSwiaW5zdGFsbE1vZGVzIjpbeyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiT3duTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiU2luZ2xlTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiTXVsdGlOYW1lc3BhY2UifSx7InN1cHBvcnRlZCI6dHJ1ZSwidHlwZSI6IkFsbE5hbWVzcGFjZXMifV0sImtleXdvcmRzIjpbIndlYmhvb2stb3BlcmF0b3IiXSwibGlua3MiOlt7Im5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwidXJsIjoiaHR0cHM6Ly93ZWJob29rLW9wZXJhdG9yLmRvbWFpbiJ9XSwibWFpbnRhaW5lcnMiOlt7ImVtYWlsIjoieW91ckBlbWFpbC5jb20iLCJuYW1lIjoiTWFpbnRhaW5lciBOYW1lIn1dLCJtYXR1cml0eSI6ImFscGhhIiwicHJvdmlkZXIiOnsibmFtZSI6IlByb3ZpZGVyIE5hbWUiLCJ1cmwiOiJodHRwczovL3lvdXIuZG9tYWluIn0sInZlcnNpb24iOiIwLjAuMSIsIndlYmhvb2tkZWZpbml0aW9ucyI6W3siYWRtaXNzaW9uUmV2aWV3VmVyc2lvbnMiOlsidjFiZXRhMSIsInYxIl0sImNvbnRhaW5lclBvcnQiOjQ0MywiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6InZ3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiVmFsaWRhdGluZ0FkbWlzc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii92YWxpZGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImRlcGxveW1lbnROYW1lIjoid2ViaG9vay1vcGVyYXRvci13ZWJob29rIiwiZmFpbHVyZVBvbGljeSI6IkZhaWwiLCJnZW5lcmF0ZU5hbWUiOiJtd2ViaG9va3Rlc3Qua2IuaW8iLCJydWxlcyI6W3siYXBpR3JvdXBzIjpbIndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJdLCJhcGlWZXJzaW9ucyI6WyJ2MSJdLCJvcGVyYXRpb25zIjpbIkNSRUFURSIsIlVQREFURSJdLCJyZXNvdXJjZXMiOlsid2ViaG9va3Rlc3RzIl19XSwic2lkZUVmZmVjdHMiOiJOb25lIiwidGFyZ2V0UG9ydCI6NDM0MywidHlwZSI6Ik11dGF0aW5nQWRtaXNzaW9uV2ViaG9vayIsIndlYmhvb2tQYXRoIjoiL211dGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImNvbnZlcnNpb25DUkRzIjpbIndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6ImN3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiQ29udmVyc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii9jb252ZXJ0In1dfX0="
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJhcGlWZXJzaW9uIjoiYXBpZXh0ZW5zaW9ucy5rOHMuaW8vdjFiZXRhMSIsImtpbmQiOiJDdXN0b21SZXNvdXJjZURlZmluaXRpb24iLCJtZXRhZGF0YSI6eyJhbm5vdGF0aW9ucyI6eyJjb250cm9sbGVyLWdlbi5rdWJlYnVpbGRlci5pby92ZXJzaW9uIjoidjAuMy4wIn0sImNyZWF0aW9uVGltZXN0YW1wIjpudWxsLCJuYW1lIjoid2ViaG9va3Rlc3RzLndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJ9LCJzcGVjIjp7Imdyb3VwIjoid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIiwibmFtZXMiOnsia2luZCI6IldlYmhvb2tUZXN0IiwibGlzdEtpbmQiOiJXZWJob29rVGVzdExpc3QiLCJwbHVyYWwiOiJ3ZWJob29rdGVzdHMiLCJzaW5ndWxhciI6IndlYmhvb2t0ZXN0In0sInByZXNlcnZlVW5rbm93bkZpZWxkcyI6ZmFsc2UsInNjb3BlIjoiTmFtZXNwYWNlZCIsInZlcnNpb24iOiJ2MSIsInZlcnNpb25zIjpbeyJuYW1lIjoidjEiLCJzY2hlbWEiOnsib3BlbkFQSVYzU2NoZW1hIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3QgaXMgdGhlIFNjaGVtYSBmb3IgdGhlIHdlYmhvb2t0ZXN0cyBBUEkiLCJwcm9wZXJ0aWVzIjp7ImFwaVZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJBUElWZXJzaW9uIGRlZmluZXMgdGhlIHZlcnNpb25lZCBzY2hlbWEgb2YgdGhpcyByZXByZXNlbnRhdGlvbiBvZiBhbiBvYmplY3QuIFNlcnZlcnMgc2hvdWxkIGNvbnZlcnQgcmVjb2duaXplZCBzY2hlbWFzIHRvIHRoZSBsYXRlc3QgaW50ZXJuYWwgdmFsdWUsIGFuZCBtYXkgcmVqZWN0IHVucmVjb2duaXplZCB2YWx1ZXMuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjcmVzb3VyY2VzIiwidHlwZSI6InN0cmluZyJ9LCJraW5kIjp7ImRlc2NyaXB0aW9uIjoiS2luZCBpcyBhIHN0cmluZyB2YWx1ZSByZXByZXNlbnRpbmcgdGhlIFJFU1QgcmVzb3VyY2UgdGhpcyBvYmplY3QgcmVwcmVzZW50cy4gU2VydmVycyBtYXkgaW5mZXIgdGhpcyBmcm9tIHRoZSBlbmRwb2ludCB0aGUgY2xpZW50IHN1Ym1pdHMgcmVxdWVzdHMgdG8uIENhbm5vdCBiZSB1cGRhdGVkLiBJbiBDYW1lbENhc2UuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjdHlwZXMta2luZHMiLCJ0eXBlIjoic3RyaW5nIn0sIm1ldGFkYXRhIjp7InR5cGUiOiJvYmplY3QifSwic3BlYyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3BlYyBkZWZpbmVzIHRoZSBkZXNpcmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwicHJvcGVydGllcyI6eyJtdXRhdGUiOnsiZGVzY3JpcHRpb24iOiJNdXRhdGUgaXMgYSBmaWVsZCB0aGF0IHdpbGwgYmUgc2V0IHRvIHRydWUgYnkgdGhlIG11dGF0aW5nIHdlYmhvb2suIiwidHlwZSI6ImJvb2xlYW4ifSwidmFsaWQiOnsiZGVzY3JpcHRpb24iOiJWYWxpZCBtdXN0IGJlIHNldCB0byB0cnVlIG9yIHRoZSB2YWxpZGF0aW9uIHdlYmhvb2sgd2lsbCByZWplY3QgdGhlIHJlc291cmNlLiIsInR5cGUiOiJib29sZWFuIn19LCJyZXF1aXJlZCI6WyJ2YWxpZCJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjp0cnVlfSx7Im5hbWUiOiJ2MiIsInNjaGVtYSI6eyJvcGVuQVBJVjNTY2hlbWEiOnsiZGVzY3JpcHRpb24iOiJXZWJob29rVGVzdCBpcyB0aGUgU2NoZW1hIGZvciB0aGUgd2ViaG9va3Rlc3RzIEFQSSIsInByb3BlcnRpZXMiOnsiYXBpVmVyc2lvbiI6eyJkZXNjcmlwdGlvbiI6IkFQSVZlcnNpb24gZGVmaW5lcyB0aGUgdmVyc2lvbmVkIHNjaGVtYSBvZiB0aGlzIHJlcHJlc2VudGF0aW9uIG9mIGFuIG9iamVjdC4gU2VydmVycyBzaG91bGQgY29udmVydCByZWNvZ25pemVkIHNjaGVtYXMgdG8gdGhlIGxhdGVzdCBpbnRlcm5hbCB2YWx1ZSwgYW5kIG1heSByZWplY3QgdW5yZWNvZ25pemVkIHZhbHVlcy4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCNyZXNvdXJjZXMiLCJ0eXBlIjoic3RyaW5nIn0sImtpbmQiOnsiZGVzY3JpcHRpb24iOiJLaW5kIGlzIGEgc3RyaW5nIHZhbHVlIHJlcHJlc2VudGluZyB0aGUgUkVTVCByZXNvdXJjZSB0aGlzIG9iamVjdCByZXByZXNlbnRzLiBTZXJ2ZXJzIG1heSBpbmZlciB0aGlzIGZyb20gdGhlIGVuZHBvaW50IHRoZSBjbGllbnQgc3VibWl0cyByZXF1ZXN0cyB0by4gQ2Fubm90IGJlIHVwZGF0ZWQuIEluIENhbWVsQ2FzZS4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCN0eXBlcy1raW5kcyIsInR5cGUiOiJzdHJpbmcifSwibWV0YWRhdGEiOnsidHlwZSI6Im9iamVjdCJ9LCJzcGVjIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3RTcGVjIGRlZmluZXMgdGhlIGRlc2lyZWQgc3RhdGUgb2YgV2ViaG9va1Rlc3QiLCJwcm9wZXJ0aWVzIjp7ImNvbnZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJDb252ZXJzaW9uIGlzIGFuIGV4YW1wbGUgZmllbGQgb2YgV2ViaG9va1Rlc3QuIEVkaXQgV2ViaG9va1Rlc3RfdHlwZXMuZ28gdG8gcmVtb3ZlL3VwZGF0ZSIsInByb3BlcnRpZXMiOnsibXV0YXRlIjp7ImRlc2NyaXB0aW9uIjoiTXV0YXRlIGlzIGEgZmllbGQgdGhhdCB3aWxsIGJlIHNldCB0byB0cnVlIGJ5IHRoZSBtdXRhdGluZyB3ZWJob29rLiIsInR5cGUiOiJib29sZWFuIn0sInZhbGlkIjp7ImRlc2NyaXB0aW9uIjoiVmFsaWQgbXVzdCBiZSBzZXQgdG8gdHJ1ZSBvciB0aGUgdmFsaWRhdGlvbiB3ZWJob29rIHdpbGwgcmVqZWN0IHRoZSByZXNvdXJjZS4iLCJ0eXBlIjoiYm9vbGVhbiJ9fSwicmVxdWlyZWQiOlsidmFsaWQiXSwidHlwZSI6Im9iamVjdCJ9fSwicmVxdWlyZWQiOlsiY29udmVyc2lvbiJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjpmYWxzZX1dfSwic3RhdHVzIjp7ImFjY2VwdGVkTmFtZXMiOnsia2luZCI6IiIsInBsdXJhbCI6IiJ9LCJjb25kaXRpb25zIjpbXSwic3RvcmVkVmVyc2lvbnMiOltdfX0="
            }
        }
    ],
    "relatedImages": [
        {
            "name": "",
            "image": "gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0"
        },
        {
            "name": "",
            "image": "quay.io/olmtest/webhook-operator-bundle:0.0.3"
        },
        {
            "name": "",
            "image": "quay.io/olmtest/webhook-operator:0.0.3"
        }
    ]
}
`

func TestSemverBuilder(t *testing.T) {
	type testCase struct {
		name               string
		validate           bool
		semverBuilder      *SemverBuilder
		templateDefinition TemplateDefinition
		files              map[string]string
		buildAssertions    func(t *testing.T, dir string, buildErr error)
		validateAssertions func(t *testing.T, validateErr error)
	}

	testDir := t.TempDir()
	validConfigTemplate := `{
		"input": "%s",
		"output": "%s"
	}`

	testCases := []testCase{
		{
			name:     "successful semver build yaml output",
			validate: true,
			semverBuilder: NewSemverBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: SemverBuilderSchema,
				Config: []byte(fmt.Sprintf(validConfigTemplate, path.Join(testDir, "components/semver.yaml"), "catalog.yaml")),
			},
			files: map[string]string{
				"components/semver.yaml": semverYaml,
			},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.NoError(t, buildErr)
				// check if the catalog.yaml file exists in the correct place
				filePath := path.Join(dir, "catalog.yaml")
				_, err := os.Stat(filePath)
				require.NoError(t, err)
				file, err := os.Open(filePath)
				require.NoError(t, err)
				defer file.Close()
				fileData, err := io.ReadAll(file)
				require.NoError(t, err)
				require.Equal(t, string(fileData), semverBuiltFbcYaml)
			},
			validateAssertions: func(t *testing.T, validateErr error) {
				require.NoError(t, validateErr)
			},
		},
		{
			name:     "successful semver build json output",
			validate: true,
			semverBuilder: NewSemverBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "json",
			}),
			templateDefinition: TemplateDefinition{
				Schema: SemverBuilderSchema,
				Config: []byte(fmt.Sprintf(validConfigTemplate, path.Join(testDir, "components/semver.yaml"), "catalog.json")),
			},
			files: map[string]string{
				"components/semver.yaml": semverYaml,
			},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.NoError(t, buildErr)
				// check if the catalog.yaml file exists in the correct place
				filePath := path.Join(dir, "catalog.json")
				_, err := os.Stat(filePath)
				require.NoError(t, err)
				file, err := os.Open(filePath)
				require.NoError(t, err)
				defer file.Close()
				fileData, err := io.ReadAll(file)
				require.NoError(t, err)
				require.Equal(t, string(fileData), semverBuiltFbcJson)
			},
			validateAssertions: func(t *testing.T, validateErr error) {
				require.NoError(t, validateErr)
			},
		},
		{
			name:     "invalid template configuration",
			validate: false,
			semverBuilder: NewSemverBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: SemverBuilderSchema,
				Config: []byte(`{
					"invalid": "components/semver.yaml",
				}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Contains(t, buildErr.Error(), "unmarshalling semver template config:")
			},
		},
		{
			name:     "invalid output type",
			validate: false,
			semverBuilder: NewSemverBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "invalid",
			}),
			templateDefinition: TemplateDefinition{
				Schema: SemverBuilderSchema,
				Config: []byte(fmt.Sprintf(validConfigTemplate, path.Join(testDir, "components/semver.yaml"), "catalog.yaml")),
			},
			files: map[string]string{
				"components/semver.yaml": semverYaml,
			},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Contains(t, buildErr.Error(), fmt.Sprintf("invalid --output value %q, expected (json|yaml)", "invalid"))
			},
		},
		{
			name:     "invalid schema",
			validate: false,
			semverBuilder: NewSemverBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: "olm.invalid",
				Config: []byte(fmt.Sprintf(validConfigTemplate, path.Join(testDir, "components/semver.yaml"), "catalog.yaml")),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Contains(t, buildErr.Error(), fmt.Sprintf("schema %q does not match the semver template builder schema %q", "olm.invalid", SemverBuilderSchema))
			},
		},
		{
			name:     "template config has empty input",
			validate: false,
			semverBuilder: NewSemverBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: SemverBuilderSchema,
				Config: []byte(`{
					"output": "catalog.yaml"
				}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Equal(t,
					buildErr.Error(),
					"semver template configuration is invalid: semver template config must have a non-empty input (templateDefinition.config.input)")
			},
		},
		{
			name:     "template config has empty output",
			validate: false,
			semverBuilder: NewSemverBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: SemverBuilderSchema,
				Config: []byte(`{
					"input": "components/semver.yaml"
				}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Equal(t,
					buildErr.Error(),
					"semver template configuration is invalid: semver template config must have a non-empty output (templateDefinition.config.output)")
			},
		},
		{
			name:     "template config has empty input & output",
			validate: false,
			semverBuilder: NewSemverBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: SemverBuilderSchema,
				Config: []byte(`{}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Equal(t,
					buildErr.Error(),
					"semver template configuration is invalid: semver template config must have a non-empty input (templateDefinition.config.input),semver template config must have a non-empty output (templateDefinition.config.output)")
			},
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outDir := fmt.Sprintf("semver-%d", i)
			outPath := path.Join(testDir, outDir)
			err := os.MkdirAll(outPath, 0o777)
			require.NoError(t, err)

			// create files in temp dir
			for fileName, fileContents := range tc.files {
				err := os.MkdirAll(path.Join(testDir, path.Dir(fileName)), 0o777)
				require.NoError(t, err)
				file, err := os.Create(path.Join(testDir, fileName))
				require.NoError(t, err)
				_, err = file.WriteString(fileContents)
				require.NoError(t, err)
				// fmt.Printf("wrote file: %q\n", path.Join(testDir, fileName))
			}

			cacheDir, err := os.MkdirTemp("", "opm-registry-")
			require.NoError(t, err)

			reg, err := containerdregistry.NewRegistry(
				containerdregistry.WithCacheDir(cacheDir),
			)
			defer reg.Destroy()
			require.NoError(t, err)

			buildErr := tc.semverBuilder.Build(context.Background(), reg, outDir, tc.templateDefinition)
			tc.buildAssertions(t, outPath, buildErr)

			if tc.validate {
				validateErr := tc.semverBuilder.Validate(outDir)
				tc.validateAssertions(t, validateErr)
			}
		})
	}
}

const semverYaml = `---
Schema: olm.semver
GenerateMajorChannels: true
GenerateMinorChannels: true
Stable:
  Bundles:
  - Image: quay.io/olmtest/webhook-operator-bundle:0.0.3
`

const semverBuiltFbcYaml = `---
defaultChannel: stable-v0
name: webhook-operator
schema: olm.package
---
entries:
- name: webhook-operator.v0.0.1
name: stable-v0
package: webhook-operator
schema: olm.channel
---
entries:
- name: webhook-operator.v0.0.1
name: stable-v0.0
package: webhook-operator
schema: olm.channel
---
image: quay.io/olmtest/webhook-operator-bundle:0.0.3
name: webhook-operator.v0.0.1
package: webhook-operator
properties:
- type: olm.gvk
  value:
    group: webhook.operators.coreos.io
    kind: WebhookTest
    version: v1
- type: olm.gvk
  value:
    group: webhook.operators.coreos.io
    kind: WebhookTest
    version: v2
- type: olm.package
  value:
    packageName: webhook-operator
    version: 0.0.1
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoicmJhYy5hdXRob3JpemF0aW9uLms4cy5pby92MWJldGExIiwia2luZCI6IkNsdXN0ZXJSb2xlIiwibWV0YWRhdGEiOnsiY3JlYXRpb25UaW1lc3RhbXAiOm51bGwsIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLW1ldHJpY3MtcmVhZGVyIn0sInJ1bGVzIjpbeyJub25SZXNvdXJjZVVSTHMiOlsiL21ldHJpY3MiXSwidmVyYnMiOlsiZ2V0Il19XX0=
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoib3BlcmF0b3JzLmNvcmVvcy5jb20vdjFhbHBoYTEiLCJraW5kIjoiQ2x1c3RlclNlcnZpY2VWZXJzaW9uIiwibWV0YWRhdGEiOnsiYW5ub3RhdGlvbnMiOnsiYWxtLWV4YW1wbGVzIjoiW1xuICB7XG4gICAgXCJhcGlWZXJzaW9uXCI6IFwid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvL3YxXCIsXG4gICAgXCJraW5kXCI6IFwiV2ViaG9va1Rlc3RcIixcbiAgICBcIm1ldGFkYXRhXCI6IHtcbiAgICAgIFwibmFtZVwiOiBcIndlYmhvb2t0ZXN0LXNhbXBsZVwiLFxuICAgICAgXCJuYW1lc3BhY2VcIjogXCJ3ZWJob29rLW9wZXJhdG9yLXN5c3RlbVwiXG4gICAgfSxcbiAgICBcInNwZWNcIjoge1xuICAgICAgXCJ2YWxpZFwiOiB0cnVlXG4gICAgfVxuICB9XG5dIiwiY2FwYWJpbGl0aWVzIjoiQmFzaWMgSW5zdGFsbCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9idWlsZGVyIjoib3BlcmF0b3Itc2RrLXYxLjAuMCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9wcm9qZWN0X2xheW91dCI6ImdvIn0sIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLnYwLjAuMSIsIm5hbWVzcGFjZSI6InBsYWNlaG9sZGVyIn0sInNwZWMiOnsiYXBpc2VydmljZWRlZmluaXRpb25zIjp7fSwiY3VzdG9tcmVzb3VyY2VkZWZpbml0aW9ucyI6eyJvd25lZCI6W3sia2luZCI6IldlYmhvb2tUZXN0IiwibmFtZSI6IndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iLCJ2ZXJzaW9uIjoidjEifV19LCJkZXNjcmlwdGlvbiI6IldlYmhvb2sgT3BlcmF0b3IgZGVzY3JpcHRpb24uIFRPRE8uIiwiZGlzcGxheU5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwiaWNvbiI6W3siYmFzZTY0ZGF0YSI6IiIsIm1lZGlhdHlwZSI6IiJ9XSwiaW5zdGFsbCI6eyJzcGVjIjp7ImNsdXN0ZXJQZXJtaXNzaW9ucyI6W3sicnVsZXMiOlt7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cyJdLCJ2ZXJicyI6WyJjcmVhdGUiLCJkZWxldGUiLCJnZXQiLCJsaXN0IiwicGF0Y2giLCJ1cGRhdGUiLCJ3YXRjaCJdfSx7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cy9zdGF0dXMiXSwidmVyYnMiOlsiZ2V0IiwicGF0Y2giLCJ1cGRhdGUiXX0seyJhcGlHcm91cHMiOlsiYXV0aGVudGljYXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJ0b2tlbnJldmlld3MiXSwidmVyYnMiOlsiY3JlYXRlIl19LHsiYXBpR3JvdXBzIjpbImF1dGhvcml6YXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJzdWJqZWN0YWNjZXNzcmV2aWV3cyJdLCJ2ZXJicyI6WyJjcmVhdGUiXX1dLCJzZXJ2aWNlQWNjb3VudE5hbWUiOiJkZWZhdWx0In1dLCJkZXBsb3ltZW50cyI6W3sibmFtZSI6IndlYmhvb2stb3BlcmF0b3Itd2ViaG9vayIsInNwZWMiOnsicmVwbGljYXMiOjEsInNlbGVjdG9yIjp7Im1hdGNoTGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInN0cmF0ZWd5Ijp7fSwidGVtcGxhdGUiOnsibWV0YWRhdGEiOnsibGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInNwZWMiOnsiY29udGFpbmVycyI6W3siYXJncyI6WyItLXNlY3VyZS1saXN0ZW4tYWRkcmVzcz0wLjAuMC4wOjg0NDMiLCItLXVwc3RyZWFtPWh0dHA6Ly8xMjcuMC4wLjE6ODA4MC8iLCItLWxvZ3Rvc3RkZXJyPXRydWUiLCItLXY9MTAiXSwiaW1hZ2UiOiJnY3IuaW8va3ViZWJ1aWxkZXIva3ViZS1yYmFjLXByb3h5OnYwLjUuMCIsIm5hbWUiOiJrdWJlLXJiYWMtcHJveHkiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6ODQ0MywibmFtZSI6Imh0dHBzIn1dLCJyZXNvdXJjZXMiOnt9fSx7ImFyZ3MiOlsiLS1tZXRyaWNzLWFkZHI9MTI3LjAuMC4xOjgwODAiLCItLWVuYWJsZS1sZWFkZXItZWxlY3Rpb24iXSwiY29tbWFuZCI6WyIvbWFuYWdlciJdLCJpbWFnZSI6InF1YXkuaW8vb2xtdGVzdC93ZWJob29rLW9wZXJhdG9yOjAuMC4zIiwibmFtZSI6Im1hbmFnZXIiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6OTQ0MywibmFtZSI6IndlYmhvb2stc2VydmVyIiwicHJvdG9jb2wiOiJUQ1AifV0sInJlc291cmNlcyI6eyJsaW1pdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjMwTWkifSwicmVxdWVzdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjIwTWkifX19XSwidGVybWluYXRpb25HcmFjZVBlcmlvZFNlY29uZHMiOjEwfX19fV0sInBlcm1pc3Npb25zIjpbeyJydWxlcyI6W3siYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiY29uZmlnbWFwcyJdLCJ2ZXJicyI6WyJnZXQiLCJsaXN0Iiwid2F0Y2giLCJjcmVhdGUiLCJ1cGRhdGUiLCJwYXRjaCIsImRlbGV0ZSJdfSx7ImFwaUdyb3VwcyI6WyIiXSwicmVzb3VyY2VzIjpbImNvbmZpZ21hcHMvc3RhdHVzIl0sInZlcmJzIjpbImdldCIsInVwZGF0ZSIsInBhdGNoIl19LHsiYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiZXZlbnRzIl0sInZlcmJzIjpbImNyZWF0ZSJdfV0sInNlcnZpY2VBY2NvdW50TmFtZSI6ImRlZmF1bHQifV19LCJzdHJhdGVneSI6ImRlcGxveW1lbnQifSwiaW5zdGFsbE1vZGVzIjpbeyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiT3duTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiU2luZ2xlTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiTXVsdGlOYW1lc3BhY2UifSx7InN1cHBvcnRlZCI6dHJ1ZSwidHlwZSI6IkFsbE5hbWVzcGFjZXMifV0sImtleXdvcmRzIjpbIndlYmhvb2stb3BlcmF0b3IiXSwibGlua3MiOlt7Im5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwidXJsIjoiaHR0cHM6Ly93ZWJob29rLW9wZXJhdG9yLmRvbWFpbiJ9XSwibWFpbnRhaW5lcnMiOlt7ImVtYWlsIjoieW91ckBlbWFpbC5jb20iLCJuYW1lIjoiTWFpbnRhaW5lciBOYW1lIn1dLCJtYXR1cml0eSI6ImFscGhhIiwicHJvdmlkZXIiOnsibmFtZSI6IlByb3ZpZGVyIE5hbWUiLCJ1cmwiOiJodHRwczovL3lvdXIuZG9tYWluIn0sInZlcnNpb24iOiIwLjAuMSIsIndlYmhvb2tkZWZpbml0aW9ucyI6W3siYWRtaXNzaW9uUmV2aWV3VmVyc2lvbnMiOlsidjFiZXRhMSIsInYxIl0sImNvbnRhaW5lclBvcnQiOjQ0MywiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6InZ3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiVmFsaWRhdGluZ0FkbWlzc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii92YWxpZGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImRlcGxveW1lbnROYW1lIjoid2ViaG9vay1vcGVyYXRvci13ZWJob29rIiwiZmFpbHVyZVBvbGljeSI6IkZhaWwiLCJnZW5lcmF0ZU5hbWUiOiJtd2ViaG9va3Rlc3Qua2IuaW8iLCJydWxlcyI6W3siYXBpR3JvdXBzIjpbIndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJdLCJhcGlWZXJzaW9ucyI6WyJ2MSJdLCJvcGVyYXRpb25zIjpbIkNSRUFURSIsIlVQREFURSJdLCJyZXNvdXJjZXMiOlsid2ViaG9va3Rlc3RzIl19XSwic2lkZUVmZmVjdHMiOiJOb25lIiwidGFyZ2V0UG9ydCI6NDM0MywidHlwZSI6Ik11dGF0aW5nQWRtaXNzaW9uV2ViaG9vayIsIndlYmhvb2tQYXRoIjoiL211dGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImNvbnZlcnNpb25DUkRzIjpbIndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6ImN3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiQ29udmVyc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii9jb252ZXJ0In1dfX0=
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoiYXBpZXh0ZW5zaW9ucy5rOHMuaW8vdjFiZXRhMSIsImtpbmQiOiJDdXN0b21SZXNvdXJjZURlZmluaXRpb24iLCJtZXRhZGF0YSI6eyJhbm5vdGF0aW9ucyI6eyJjb250cm9sbGVyLWdlbi5rdWJlYnVpbGRlci5pby92ZXJzaW9uIjoidjAuMy4wIn0sImNyZWF0aW9uVGltZXN0YW1wIjpudWxsLCJuYW1lIjoid2ViaG9va3Rlc3RzLndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJ9LCJzcGVjIjp7Imdyb3VwIjoid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIiwibmFtZXMiOnsia2luZCI6IldlYmhvb2tUZXN0IiwibGlzdEtpbmQiOiJXZWJob29rVGVzdExpc3QiLCJwbHVyYWwiOiJ3ZWJob29rdGVzdHMiLCJzaW5ndWxhciI6IndlYmhvb2t0ZXN0In0sInByZXNlcnZlVW5rbm93bkZpZWxkcyI6ZmFsc2UsInNjb3BlIjoiTmFtZXNwYWNlZCIsInZlcnNpb24iOiJ2MSIsInZlcnNpb25zIjpbeyJuYW1lIjoidjEiLCJzY2hlbWEiOnsib3BlbkFQSVYzU2NoZW1hIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3QgaXMgdGhlIFNjaGVtYSBmb3IgdGhlIHdlYmhvb2t0ZXN0cyBBUEkiLCJwcm9wZXJ0aWVzIjp7ImFwaVZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJBUElWZXJzaW9uIGRlZmluZXMgdGhlIHZlcnNpb25lZCBzY2hlbWEgb2YgdGhpcyByZXByZXNlbnRhdGlvbiBvZiBhbiBvYmplY3QuIFNlcnZlcnMgc2hvdWxkIGNvbnZlcnQgcmVjb2duaXplZCBzY2hlbWFzIHRvIHRoZSBsYXRlc3QgaW50ZXJuYWwgdmFsdWUsIGFuZCBtYXkgcmVqZWN0IHVucmVjb2duaXplZCB2YWx1ZXMuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjcmVzb3VyY2VzIiwidHlwZSI6InN0cmluZyJ9LCJraW5kIjp7ImRlc2NyaXB0aW9uIjoiS2luZCBpcyBhIHN0cmluZyB2YWx1ZSByZXByZXNlbnRpbmcgdGhlIFJFU1QgcmVzb3VyY2UgdGhpcyBvYmplY3QgcmVwcmVzZW50cy4gU2VydmVycyBtYXkgaW5mZXIgdGhpcyBmcm9tIHRoZSBlbmRwb2ludCB0aGUgY2xpZW50IHN1Ym1pdHMgcmVxdWVzdHMgdG8uIENhbm5vdCBiZSB1cGRhdGVkLiBJbiBDYW1lbENhc2UuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjdHlwZXMta2luZHMiLCJ0eXBlIjoic3RyaW5nIn0sIm1ldGFkYXRhIjp7InR5cGUiOiJvYmplY3QifSwic3BlYyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3BlYyBkZWZpbmVzIHRoZSBkZXNpcmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwicHJvcGVydGllcyI6eyJtdXRhdGUiOnsiZGVzY3JpcHRpb24iOiJNdXRhdGUgaXMgYSBmaWVsZCB0aGF0IHdpbGwgYmUgc2V0IHRvIHRydWUgYnkgdGhlIG11dGF0aW5nIHdlYmhvb2suIiwidHlwZSI6ImJvb2xlYW4ifSwidmFsaWQiOnsiZGVzY3JpcHRpb24iOiJWYWxpZCBtdXN0IGJlIHNldCB0byB0cnVlIG9yIHRoZSB2YWxpZGF0aW9uIHdlYmhvb2sgd2lsbCByZWplY3QgdGhlIHJlc291cmNlLiIsInR5cGUiOiJib29sZWFuIn19LCJyZXF1aXJlZCI6WyJ2YWxpZCJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjp0cnVlfSx7Im5hbWUiOiJ2MiIsInNjaGVtYSI6eyJvcGVuQVBJVjNTY2hlbWEiOnsiZGVzY3JpcHRpb24iOiJXZWJob29rVGVzdCBpcyB0aGUgU2NoZW1hIGZvciB0aGUgd2ViaG9va3Rlc3RzIEFQSSIsInByb3BlcnRpZXMiOnsiYXBpVmVyc2lvbiI6eyJkZXNjcmlwdGlvbiI6IkFQSVZlcnNpb24gZGVmaW5lcyB0aGUgdmVyc2lvbmVkIHNjaGVtYSBvZiB0aGlzIHJlcHJlc2VudGF0aW9uIG9mIGFuIG9iamVjdC4gU2VydmVycyBzaG91bGQgY29udmVydCByZWNvZ25pemVkIHNjaGVtYXMgdG8gdGhlIGxhdGVzdCBpbnRlcm5hbCB2YWx1ZSwgYW5kIG1heSByZWplY3QgdW5yZWNvZ25pemVkIHZhbHVlcy4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCNyZXNvdXJjZXMiLCJ0eXBlIjoic3RyaW5nIn0sImtpbmQiOnsiZGVzY3JpcHRpb24iOiJLaW5kIGlzIGEgc3RyaW5nIHZhbHVlIHJlcHJlc2VudGluZyB0aGUgUkVTVCByZXNvdXJjZSB0aGlzIG9iamVjdCByZXByZXNlbnRzLiBTZXJ2ZXJzIG1heSBpbmZlciB0aGlzIGZyb20gdGhlIGVuZHBvaW50IHRoZSBjbGllbnQgc3VibWl0cyByZXF1ZXN0cyB0by4gQ2Fubm90IGJlIHVwZGF0ZWQuIEluIENhbWVsQ2FzZS4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCN0eXBlcy1raW5kcyIsInR5cGUiOiJzdHJpbmcifSwibWV0YWRhdGEiOnsidHlwZSI6Im9iamVjdCJ9LCJzcGVjIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3RTcGVjIGRlZmluZXMgdGhlIGRlc2lyZWQgc3RhdGUgb2YgV2ViaG9va1Rlc3QiLCJwcm9wZXJ0aWVzIjp7ImNvbnZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJDb252ZXJzaW9uIGlzIGFuIGV4YW1wbGUgZmllbGQgb2YgV2ViaG9va1Rlc3QuIEVkaXQgV2ViaG9va1Rlc3RfdHlwZXMuZ28gdG8gcmVtb3ZlL3VwZGF0ZSIsInByb3BlcnRpZXMiOnsibXV0YXRlIjp7ImRlc2NyaXB0aW9uIjoiTXV0YXRlIGlzIGEgZmllbGQgdGhhdCB3aWxsIGJlIHNldCB0byB0cnVlIGJ5IHRoZSBtdXRhdGluZyB3ZWJob29rLiIsInR5cGUiOiJib29sZWFuIn0sInZhbGlkIjp7ImRlc2NyaXB0aW9uIjoiVmFsaWQgbXVzdCBiZSBzZXQgdG8gdHJ1ZSBvciB0aGUgdmFsaWRhdGlvbiB3ZWJob29rIHdpbGwgcmVqZWN0IHRoZSByZXNvdXJjZS4iLCJ0eXBlIjoiYm9vbGVhbiJ9fSwicmVxdWlyZWQiOlsidmFsaWQiXSwidHlwZSI6Im9iamVjdCJ9fSwicmVxdWlyZWQiOlsiY29udmVyc2lvbiJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjpmYWxzZX1dfSwic3RhdHVzIjp7ImFjY2VwdGVkTmFtZXMiOnsia2luZCI6IiIsInBsdXJhbCI6IiJ9LCJjb25kaXRpb25zIjpbXSwic3RvcmVkVmVyc2lvbnMiOltdfX0=
relatedImages:
- image: gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0
  name: ""
- image: quay.io/olmtest/webhook-operator-bundle:0.0.3
  name: ""
- image: quay.io/olmtest/webhook-operator:0.0.3
  name: ""
schema: olm.bundle
`

const semverBuiltFbcJson = `{
    "schema": "olm.package",
    "name": "webhook-operator",
    "defaultChannel": "stable-v0"
}
{
    "schema": "olm.channel",
    "name": "stable-v0",
    "package": "webhook-operator",
    "entries": [
        {
            "name": "webhook-operator.v0.0.1"
        }
    ]
}
{
    "schema": "olm.channel",
    "name": "stable-v0.0",
    "package": "webhook-operator",
    "entries": [
        {
            "name": "webhook-operator.v0.0.1"
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "webhook-operator.v0.0.1",
    "package": "webhook-operator",
    "image": "quay.io/olmtest/webhook-operator-bundle:0.0.3",
    "properties": [
        {
            "type": "olm.gvk",
            "value": {
                "group": "webhook.operators.coreos.io",
                "kind": "WebhookTest",
                "version": "v1"
            }
        },
        {
            "type": "olm.gvk",
            "value": {
                "group": "webhook.operators.coreos.io",
                "kind": "WebhookTest",
                "version": "v2"
            }
        },
        {
            "type": "olm.package",
            "value": {
                "packageName": "webhook-operator",
                "version": "0.0.1"
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJhcGlWZXJzaW9uIjoicmJhYy5hdXRob3JpemF0aW9uLms4cy5pby92MWJldGExIiwia2luZCI6IkNsdXN0ZXJSb2xlIiwibWV0YWRhdGEiOnsiY3JlYXRpb25UaW1lc3RhbXAiOm51bGwsIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLW1ldHJpY3MtcmVhZGVyIn0sInJ1bGVzIjpbeyJub25SZXNvdXJjZVVSTHMiOlsiL21ldHJpY3MiXSwidmVyYnMiOlsiZ2V0Il19XX0="
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJhcGlWZXJzaW9uIjoib3BlcmF0b3JzLmNvcmVvcy5jb20vdjFhbHBoYTEiLCJraW5kIjoiQ2x1c3RlclNlcnZpY2VWZXJzaW9uIiwibWV0YWRhdGEiOnsiYW5ub3RhdGlvbnMiOnsiYWxtLWV4YW1wbGVzIjoiW1xuICB7XG4gICAgXCJhcGlWZXJzaW9uXCI6IFwid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvL3YxXCIsXG4gICAgXCJraW5kXCI6IFwiV2ViaG9va1Rlc3RcIixcbiAgICBcIm1ldGFkYXRhXCI6IHtcbiAgICAgIFwibmFtZVwiOiBcIndlYmhvb2t0ZXN0LXNhbXBsZVwiLFxuICAgICAgXCJuYW1lc3BhY2VcIjogXCJ3ZWJob29rLW9wZXJhdG9yLXN5c3RlbVwiXG4gICAgfSxcbiAgICBcInNwZWNcIjoge1xuICAgICAgXCJ2YWxpZFwiOiB0cnVlXG4gICAgfVxuICB9XG5dIiwiY2FwYWJpbGl0aWVzIjoiQmFzaWMgSW5zdGFsbCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9idWlsZGVyIjoib3BlcmF0b3Itc2RrLXYxLjAuMCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9wcm9qZWN0X2xheW91dCI6ImdvIn0sIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLnYwLjAuMSIsIm5hbWVzcGFjZSI6InBsYWNlaG9sZGVyIn0sInNwZWMiOnsiYXBpc2VydmljZWRlZmluaXRpb25zIjp7fSwiY3VzdG9tcmVzb3VyY2VkZWZpbml0aW9ucyI6eyJvd25lZCI6W3sia2luZCI6IldlYmhvb2tUZXN0IiwibmFtZSI6IndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iLCJ2ZXJzaW9uIjoidjEifV19LCJkZXNjcmlwdGlvbiI6IldlYmhvb2sgT3BlcmF0b3IgZGVzY3JpcHRpb24uIFRPRE8uIiwiZGlzcGxheU5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwiaWNvbiI6W3siYmFzZTY0ZGF0YSI6IiIsIm1lZGlhdHlwZSI6IiJ9XSwiaW5zdGFsbCI6eyJzcGVjIjp7ImNsdXN0ZXJQZXJtaXNzaW9ucyI6W3sicnVsZXMiOlt7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cyJdLCJ2ZXJicyI6WyJjcmVhdGUiLCJkZWxldGUiLCJnZXQiLCJsaXN0IiwicGF0Y2giLCJ1cGRhdGUiLCJ3YXRjaCJdfSx7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cy9zdGF0dXMiXSwidmVyYnMiOlsiZ2V0IiwicGF0Y2giLCJ1cGRhdGUiXX0seyJhcGlHcm91cHMiOlsiYXV0aGVudGljYXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJ0b2tlbnJldmlld3MiXSwidmVyYnMiOlsiY3JlYXRlIl19LHsiYXBpR3JvdXBzIjpbImF1dGhvcml6YXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJzdWJqZWN0YWNjZXNzcmV2aWV3cyJdLCJ2ZXJicyI6WyJjcmVhdGUiXX1dLCJzZXJ2aWNlQWNjb3VudE5hbWUiOiJkZWZhdWx0In1dLCJkZXBsb3ltZW50cyI6W3sibmFtZSI6IndlYmhvb2stb3BlcmF0b3Itd2ViaG9vayIsInNwZWMiOnsicmVwbGljYXMiOjEsInNlbGVjdG9yIjp7Im1hdGNoTGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInN0cmF0ZWd5Ijp7fSwidGVtcGxhdGUiOnsibWV0YWRhdGEiOnsibGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInNwZWMiOnsiY29udGFpbmVycyI6W3siYXJncyI6WyItLXNlY3VyZS1saXN0ZW4tYWRkcmVzcz0wLjAuMC4wOjg0NDMiLCItLXVwc3RyZWFtPWh0dHA6Ly8xMjcuMC4wLjE6ODA4MC8iLCItLWxvZ3Rvc3RkZXJyPXRydWUiLCItLXY9MTAiXSwiaW1hZ2UiOiJnY3IuaW8va3ViZWJ1aWxkZXIva3ViZS1yYmFjLXByb3h5OnYwLjUuMCIsIm5hbWUiOiJrdWJlLXJiYWMtcHJveHkiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6ODQ0MywibmFtZSI6Imh0dHBzIn1dLCJyZXNvdXJjZXMiOnt9fSx7ImFyZ3MiOlsiLS1tZXRyaWNzLWFkZHI9MTI3LjAuMC4xOjgwODAiLCItLWVuYWJsZS1sZWFkZXItZWxlY3Rpb24iXSwiY29tbWFuZCI6WyIvbWFuYWdlciJdLCJpbWFnZSI6InF1YXkuaW8vb2xtdGVzdC93ZWJob29rLW9wZXJhdG9yOjAuMC4zIiwibmFtZSI6Im1hbmFnZXIiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6OTQ0MywibmFtZSI6IndlYmhvb2stc2VydmVyIiwicHJvdG9jb2wiOiJUQ1AifV0sInJlc291cmNlcyI6eyJsaW1pdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjMwTWkifSwicmVxdWVzdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjIwTWkifX19XSwidGVybWluYXRpb25HcmFjZVBlcmlvZFNlY29uZHMiOjEwfX19fV0sInBlcm1pc3Npb25zIjpbeyJydWxlcyI6W3siYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiY29uZmlnbWFwcyJdLCJ2ZXJicyI6WyJnZXQiLCJsaXN0Iiwid2F0Y2giLCJjcmVhdGUiLCJ1cGRhdGUiLCJwYXRjaCIsImRlbGV0ZSJdfSx7ImFwaUdyb3VwcyI6WyIiXSwicmVzb3VyY2VzIjpbImNvbmZpZ21hcHMvc3RhdHVzIl0sInZlcmJzIjpbImdldCIsInVwZGF0ZSIsInBhdGNoIl19LHsiYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiZXZlbnRzIl0sInZlcmJzIjpbImNyZWF0ZSJdfV0sInNlcnZpY2VBY2NvdW50TmFtZSI6ImRlZmF1bHQifV19LCJzdHJhdGVneSI6ImRlcGxveW1lbnQifSwiaW5zdGFsbE1vZGVzIjpbeyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiT3duTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiU2luZ2xlTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiTXVsdGlOYW1lc3BhY2UifSx7InN1cHBvcnRlZCI6dHJ1ZSwidHlwZSI6IkFsbE5hbWVzcGFjZXMifV0sImtleXdvcmRzIjpbIndlYmhvb2stb3BlcmF0b3IiXSwibGlua3MiOlt7Im5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwidXJsIjoiaHR0cHM6Ly93ZWJob29rLW9wZXJhdG9yLmRvbWFpbiJ9XSwibWFpbnRhaW5lcnMiOlt7ImVtYWlsIjoieW91ckBlbWFpbC5jb20iLCJuYW1lIjoiTWFpbnRhaW5lciBOYW1lIn1dLCJtYXR1cml0eSI6ImFscGhhIiwicHJvdmlkZXIiOnsibmFtZSI6IlByb3ZpZGVyIE5hbWUiLCJ1cmwiOiJodHRwczovL3lvdXIuZG9tYWluIn0sInZlcnNpb24iOiIwLjAuMSIsIndlYmhvb2tkZWZpbml0aW9ucyI6W3siYWRtaXNzaW9uUmV2aWV3VmVyc2lvbnMiOlsidjFiZXRhMSIsInYxIl0sImNvbnRhaW5lclBvcnQiOjQ0MywiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6InZ3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiVmFsaWRhdGluZ0FkbWlzc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii92YWxpZGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImRlcGxveW1lbnROYW1lIjoid2ViaG9vay1vcGVyYXRvci13ZWJob29rIiwiZmFpbHVyZVBvbGljeSI6IkZhaWwiLCJnZW5lcmF0ZU5hbWUiOiJtd2ViaG9va3Rlc3Qua2IuaW8iLCJydWxlcyI6W3siYXBpR3JvdXBzIjpbIndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJdLCJhcGlWZXJzaW9ucyI6WyJ2MSJdLCJvcGVyYXRpb25zIjpbIkNSRUFURSIsIlVQREFURSJdLCJyZXNvdXJjZXMiOlsid2ViaG9va3Rlc3RzIl19XSwic2lkZUVmZmVjdHMiOiJOb25lIiwidGFyZ2V0UG9ydCI6NDM0MywidHlwZSI6Ik11dGF0aW5nQWRtaXNzaW9uV2ViaG9vayIsIndlYmhvb2tQYXRoIjoiL211dGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImNvbnZlcnNpb25DUkRzIjpbIndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6ImN3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiQ29udmVyc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii9jb252ZXJ0In1dfX0="
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJhcGlWZXJzaW9uIjoiYXBpZXh0ZW5zaW9ucy5rOHMuaW8vdjFiZXRhMSIsImtpbmQiOiJDdXN0b21SZXNvdXJjZURlZmluaXRpb24iLCJtZXRhZGF0YSI6eyJhbm5vdGF0aW9ucyI6eyJjb250cm9sbGVyLWdlbi5rdWJlYnVpbGRlci5pby92ZXJzaW9uIjoidjAuMy4wIn0sImNyZWF0aW9uVGltZXN0YW1wIjpudWxsLCJuYW1lIjoid2ViaG9va3Rlc3RzLndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJ9LCJzcGVjIjp7Imdyb3VwIjoid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIiwibmFtZXMiOnsia2luZCI6IldlYmhvb2tUZXN0IiwibGlzdEtpbmQiOiJXZWJob29rVGVzdExpc3QiLCJwbHVyYWwiOiJ3ZWJob29rdGVzdHMiLCJzaW5ndWxhciI6IndlYmhvb2t0ZXN0In0sInByZXNlcnZlVW5rbm93bkZpZWxkcyI6ZmFsc2UsInNjb3BlIjoiTmFtZXNwYWNlZCIsInZlcnNpb24iOiJ2MSIsInZlcnNpb25zIjpbeyJuYW1lIjoidjEiLCJzY2hlbWEiOnsib3BlbkFQSVYzU2NoZW1hIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3QgaXMgdGhlIFNjaGVtYSBmb3IgdGhlIHdlYmhvb2t0ZXN0cyBBUEkiLCJwcm9wZXJ0aWVzIjp7ImFwaVZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJBUElWZXJzaW9uIGRlZmluZXMgdGhlIHZlcnNpb25lZCBzY2hlbWEgb2YgdGhpcyByZXByZXNlbnRhdGlvbiBvZiBhbiBvYmplY3QuIFNlcnZlcnMgc2hvdWxkIGNvbnZlcnQgcmVjb2duaXplZCBzY2hlbWFzIHRvIHRoZSBsYXRlc3QgaW50ZXJuYWwgdmFsdWUsIGFuZCBtYXkgcmVqZWN0IHVucmVjb2duaXplZCB2YWx1ZXMuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjcmVzb3VyY2VzIiwidHlwZSI6InN0cmluZyJ9LCJraW5kIjp7ImRlc2NyaXB0aW9uIjoiS2luZCBpcyBhIHN0cmluZyB2YWx1ZSByZXByZXNlbnRpbmcgdGhlIFJFU1QgcmVzb3VyY2UgdGhpcyBvYmplY3QgcmVwcmVzZW50cy4gU2VydmVycyBtYXkgaW5mZXIgdGhpcyBmcm9tIHRoZSBlbmRwb2ludCB0aGUgY2xpZW50IHN1Ym1pdHMgcmVxdWVzdHMgdG8uIENhbm5vdCBiZSB1cGRhdGVkLiBJbiBDYW1lbENhc2UuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjdHlwZXMta2luZHMiLCJ0eXBlIjoic3RyaW5nIn0sIm1ldGFkYXRhIjp7InR5cGUiOiJvYmplY3QifSwic3BlYyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3BlYyBkZWZpbmVzIHRoZSBkZXNpcmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwicHJvcGVydGllcyI6eyJtdXRhdGUiOnsiZGVzY3JpcHRpb24iOiJNdXRhdGUgaXMgYSBmaWVsZCB0aGF0IHdpbGwgYmUgc2V0IHRvIHRydWUgYnkgdGhlIG11dGF0aW5nIHdlYmhvb2suIiwidHlwZSI6ImJvb2xlYW4ifSwidmFsaWQiOnsiZGVzY3JpcHRpb24iOiJWYWxpZCBtdXN0IGJlIHNldCB0byB0cnVlIG9yIHRoZSB2YWxpZGF0aW9uIHdlYmhvb2sgd2lsbCByZWplY3QgdGhlIHJlc291cmNlLiIsInR5cGUiOiJib29sZWFuIn19LCJyZXF1aXJlZCI6WyJ2YWxpZCJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjp0cnVlfSx7Im5hbWUiOiJ2MiIsInNjaGVtYSI6eyJvcGVuQVBJVjNTY2hlbWEiOnsiZGVzY3JpcHRpb24iOiJXZWJob29rVGVzdCBpcyB0aGUgU2NoZW1hIGZvciB0aGUgd2ViaG9va3Rlc3RzIEFQSSIsInByb3BlcnRpZXMiOnsiYXBpVmVyc2lvbiI6eyJkZXNjcmlwdGlvbiI6IkFQSVZlcnNpb24gZGVmaW5lcyB0aGUgdmVyc2lvbmVkIHNjaGVtYSBvZiB0aGlzIHJlcHJlc2VudGF0aW9uIG9mIGFuIG9iamVjdC4gU2VydmVycyBzaG91bGQgY29udmVydCByZWNvZ25pemVkIHNjaGVtYXMgdG8gdGhlIGxhdGVzdCBpbnRlcm5hbCB2YWx1ZSwgYW5kIG1heSByZWplY3QgdW5yZWNvZ25pemVkIHZhbHVlcy4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCNyZXNvdXJjZXMiLCJ0eXBlIjoic3RyaW5nIn0sImtpbmQiOnsiZGVzY3JpcHRpb24iOiJLaW5kIGlzIGEgc3RyaW5nIHZhbHVlIHJlcHJlc2VudGluZyB0aGUgUkVTVCByZXNvdXJjZSB0aGlzIG9iamVjdCByZXByZXNlbnRzLiBTZXJ2ZXJzIG1heSBpbmZlciB0aGlzIGZyb20gdGhlIGVuZHBvaW50IHRoZSBjbGllbnQgc3VibWl0cyByZXF1ZXN0cyB0by4gQ2Fubm90IGJlIHVwZGF0ZWQuIEluIENhbWVsQ2FzZS4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCN0eXBlcy1raW5kcyIsInR5cGUiOiJzdHJpbmcifSwibWV0YWRhdGEiOnsidHlwZSI6Im9iamVjdCJ9LCJzcGVjIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3RTcGVjIGRlZmluZXMgdGhlIGRlc2lyZWQgc3RhdGUgb2YgV2ViaG9va1Rlc3QiLCJwcm9wZXJ0aWVzIjp7ImNvbnZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJDb252ZXJzaW9uIGlzIGFuIGV4YW1wbGUgZmllbGQgb2YgV2ViaG9va1Rlc3QuIEVkaXQgV2ViaG9va1Rlc3RfdHlwZXMuZ28gdG8gcmVtb3ZlL3VwZGF0ZSIsInByb3BlcnRpZXMiOnsibXV0YXRlIjp7ImRlc2NyaXB0aW9uIjoiTXV0YXRlIGlzIGEgZmllbGQgdGhhdCB3aWxsIGJlIHNldCB0byB0cnVlIGJ5IHRoZSBtdXRhdGluZyB3ZWJob29rLiIsInR5cGUiOiJib29sZWFuIn0sInZhbGlkIjp7ImRlc2NyaXB0aW9uIjoiVmFsaWQgbXVzdCBiZSBzZXQgdG8gdHJ1ZSBvciB0aGUgdmFsaWRhdGlvbiB3ZWJob29rIHdpbGwgcmVqZWN0IHRoZSByZXNvdXJjZS4iLCJ0eXBlIjoiYm9vbGVhbiJ9fSwicmVxdWlyZWQiOlsidmFsaWQiXSwidHlwZSI6Im9iamVjdCJ9fSwicmVxdWlyZWQiOlsiY29udmVyc2lvbiJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjpmYWxzZX1dfSwic3RhdHVzIjp7ImFjY2VwdGVkTmFtZXMiOnsia2luZCI6IiIsInBsdXJhbCI6IiJ9LCJjb25kaXRpb25zIjpbXSwic3RvcmVkVmVyc2lvbnMiOltdfX0="
            }
        }
    ],
    "relatedImages": [
        {
            "name": "",
            "image": "gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0"
        },
        {
            "name": "",
            "image": "quay.io/olmtest/webhook-operator-bundle:0.0.3"
        },
        {
            "name": "",
            "image": "quay.io/olmtest/webhook-operator:0.0.3"
        }
    ]
}
`

func TestRawBuilder(t *testing.T) {
	type testCase struct {
		name               string
		validate           bool
		rawBuilder         *RawBuilder
		templateDefinition TemplateDefinition
		files              map[string]string
		buildAssertions    func(t *testing.T, dir string, buildErr error)
		validateAssertions func(t *testing.T, validateErr error)
	}

	testDir := t.TempDir()
	validConfigTemplate := `{
		"input": "%s",
		"output": "%s"
	}`

	testCases := []testCase{
		{
			name:     "successful raw build yaml output",
			validate: true,
			rawBuilder: NewRawBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: RawBuilderSchema,
				Config: []byte(fmt.Sprintf(validConfigTemplate, path.Join(testDir, "components/raw.yaml"), "catalog.yaml")),
			},
			files: map[string]string{
				"components/raw.yaml": rawYaml,
			},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.NoError(t, buildErr)
				// check if the catalog.yaml file exists in the correct place
				filePath := path.Join(dir, "catalog.yaml")
				_, err := os.Stat(filePath)
				require.NoError(t, err)
				file, err := os.Open(filePath)
				require.NoError(t, err)
				defer file.Close()
				fileData, err := io.ReadAll(file)
				require.NoError(t, err)
				require.Equal(t, string(fileData), rawBuiltFbcYaml)
			},
			validateAssertions: func(t *testing.T, validateErr error) {
				require.NoError(t, validateErr)
			},
		},
		{
			name:     "successful raw build json output",
			validate: true,
			rawBuilder: NewRawBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "json",
			}),
			templateDefinition: TemplateDefinition{
				Schema: RawBuilderSchema,
				Config: []byte(fmt.Sprintf(validConfigTemplate, path.Join(testDir, "components/raw.yaml"), "catalog.json")),
			},
			files: map[string]string{
				"components/raw.yaml": rawYaml,
			},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.NoError(t, buildErr)
				// check if the catalog.yaml file exists in the correct place
				filePath := path.Join(dir, "catalog.json")
				_, err := os.Stat(filePath)
				require.NoError(t, err)
				file, err := os.Open(filePath)
				require.NoError(t, err)
				defer file.Close()
				fileData, err := io.ReadAll(file)
				require.NoError(t, err)
				require.Equal(t, string(fileData), rawBuiltFbcJson)
			},
			validateAssertions: func(t *testing.T, validateErr error) {
				require.NoError(t, validateErr)
			},
		},
		{
			name:     "invalid template configuration",
			validate: false,
			rawBuilder: NewRawBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: RawBuilderSchema,
				Config: []byte(`{
					"invalid": "components/raw.yaml",
				}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Contains(t, buildErr.Error(), "unmarshalling raw template config:")
			},
		},
		{
			name:     "invalid output type",
			validate: false,
			rawBuilder: NewRawBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "invalid",
			}),
			templateDefinition: TemplateDefinition{
				Schema: RawBuilderSchema,
				Config: []byte(fmt.Sprintf(validConfigTemplate, path.Join(testDir, "components/raw.yaml"), "catalog.yaml")),
			},
			files: map[string]string{
				"components/raw.yaml": semverYaml,
			},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Contains(t, buildErr.Error(), fmt.Sprintf("invalid --output value %q, expected (json|yaml)", "invalid"))
			},
		},
		{
			name:     "invalid schema",
			validate: false,
			rawBuilder: NewRawBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: "olm.invalid",
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Contains(t, buildErr.Error(), fmt.Sprintf("schema %q does not match the raw template builder schema %q", "olm.invalid", RawBuilderSchema))
			},
		},
		{
			name:     "template config has empty input",
			validate: false,
			rawBuilder: NewRawBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: RawBuilderSchema,
				Config: []byte(`{
					"output": "catalog.yaml"
				}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Equal(t,
					buildErr.Error(),
					"raw template configuration is invalid: raw template config must have a non-empty input (templateDefinition.config.input)")
			},
		},
		{
			name:     "template config has empty output",
			validate: false,
			rawBuilder: NewRawBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: RawBuilderSchema,
				Config: []byte(`{
					"input": "components/raw.yaml"
				}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Equal(t,
					buildErr.Error(),
					"raw template configuration is invalid: raw template config must have a non-empty output (templateDefinition.config.output)")
			},
		},
		{
			name:     "template config has empty input & output",
			validate: false,
			rawBuilder: NewRawBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: RawBuilderSchema,
				Config: []byte(`{}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Equal(t,
					buildErr.Error(),
					"raw template configuration is invalid: raw template config must have a non-empty input (templateDefinition.config.input),raw template config must have a non-empty output (templateDefinition.config.output)")
			},
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outDir := fmt.Sprintf("raw-%d", i)
			outPath := path.Join(testDir, outDir)
			err := os.MkdirAll(outPath, 0o777)
			require.NoError(t, err)

			// create files in temp dir
			for fileName, fileContents := range tc.files {
				err := os.MkdirAll(path.Join(testDir, path.Dir(fileName)), 0o777)
				require.NoError(t, err)
				file, err := os.Create(path.Join(testDir, fileName))
				require.NoError(t, err)
				_, err = file.WriteString(fileContents)
				require.NoError(t, err)
			}

			cacheDir, err := os.MkdirTemp("", "opm-registry-")
			require.NoError(t, err)

			reg, err := containerdregistry.NewRegistry(
				containerdregistry.WithCacheDir(cacheDir),
			)
			defer reg.Destroy()
			require.NoError(t, err)

			buildErr := tc.rawBuilder.Build(context.Background(), reg, outDir, tc.templateDefinition)
			tc.buildAssertions(t, outPath, buildErr)

			if tc.validate {
				validateErr := tc.rawBuilder.Validate(outDir)
				tc.validateAssertions(t, validateErr)
			}
		})
	}
}

const rawYaml = `---
defaultChannel: preview
name: webhook-operator-412
schema: olm.package
---
image: quay.io/olmtest/webhook-operator-bundle:0.0.3
name: webhook-operator.v0.0.1
package: webhook-operator-412
properties:
- type: olm.gvk
  value:
    group: webhook.operators.coreos.io
    kind: WebhookTest
    version: v1
- type: olm.gvk
  value:
    group: webhook.operators.coreos.io
    kind: WebhookTest
    version: v2
- type: olm.package
  value:
    packageName: webhook-operator-412
    version: 0.0.1
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoicmJhYy5hdXRob3JpemF0aW9uLms4cy5pby92MWJldGExIiwia2luZCI6IkNsdXN0ZXJSb2xlIiwibWV0YWRhdGEiOnsiY3JlYXRpb25UaW1lc3RhbXAiOm51bGwsIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLW1ldHJpY3MtcmVhZGVyIn0sInJ1bGVzIjpbeyJub25SZXNvdXJjZVVSTHMiOlsiL21ldHJpY3MiXSwidmVyYnMiOlsiZ2V0Il19XX0=
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoib3BlcmF0b3JzLmNvcmVvcy5jb20vdjFhbHBoYTEiLCJraW5kIjoiQ2x1c3RlclNlcnZpY2VWZXJzaW9uIiwibWV0YWRhdGEiOnsiYW5ub3RhdGlvbnMiOnsiYWxtLWV4YW1wbGVzIjoiW1xuICB7XG4gICAgXCJhcGlWZXJzaW9uXCI6IFwid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvL3YxXCIsXG4gICAgXCJraW5kXCI6IFwiV2ViaG9va1Rlc3RcIixcbiAgICBcIm1ldGFkYXRhXCI6IHtcbiAgICAgIFwibmFtZVwiOiBcIndlYmhvb2t0ZXN0LXNhbXBsZVwiLFxuICAgICAgXCJuYW1lc3BhY2VcIjogXCJ3ZWJob29rLW9wZXJhdG9yLXN5c3RlbVwiXG4gICAgfSxcbiAgICBcInNwZWNcIjoge1xuICAgICAgXCJ2YWxpZFwiOiB0cnVlXG4gICAgfVxuICB9XG5dIiwiY2FwYWJpbGl0aWVzIjoiQmFzaWMgSW5zdGFsbCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9idWlsZGVyIjoib3BlcmF0b3Itc2RrLXYxLjAuMCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9wcm9qZWN0X2xheW91dCI6ImdvIn0sIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLnYwLjAuMSIsIm5hbWVzcGFjZSI6InBsYWNlaG9sZGVyIn0sInNwZWMiOnsiYXBpc2VydmljZWRlZmluaXRpb25zIjp7fSwiY3VzdG9tcmVzb3VyY2VkZWZpbml0aW9ucyI6eyJvd25lZCI6W3sia2luZCI6IldlYmhvb2tUZXN0IiwibmFtZSI6IndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iLCJ2ZXJzaW9uIjoidjEifV19LCJkZXNjcmlwdGlvbiI6IldlYmhvb2sgT3BlcmF0b3IgZGVzY3JpcHRpb24uIFRPRE8uIiwiZGlzcGxheU5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwiaWNvbiI6W3siYmFzZTY0ZGF0YSI6IiIsIm1lZGlhdHlwZSI6IiJ9XSwiaW5zdGFsbCI6eyJzcGVjIjp7ImNsdXN0ZXJQZXJtaXNzaW9ucyI6W3sicnVsZXMiOlt7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cyJdLCJ2ZXJicyI6WyJjcmVhdGUiLCJkZWxldGUiLCJnZXQiLCJsaXN0IiwicGF0Y2giLCJ1cGRhdGUiLCJ3YXRjaCJdfSx7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cy9zdGF0dXMiXSwidmVyYnMiOlsiZ2V0IiwicGF0Y2giLCJ1cGRhdGUiXX0seyJhcGlHcm91cHMiOlsiYXV0aGVudGljYXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJ0b2tlbnJldmlld3MiXSwidmVyYnMiOlsiY3JlYXRlIl19LHsiYXBpR3JvdXBzIjpbImF1dGhvcml6YXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJzdWJqZWN0YWNjZXNzcmV2aWV3cyJdLCJ2ZXJicyI6WyJjcmVhdGUiXX1dLCJzZXJ2aWNlQWNjb3VudE5hbWUiOiJkZWZhdWx0In1dLCJkZXBsb3ltZW50cyI6W3sibmFtZSI6IndlYmhvb2stb3BlcmF0b3Itd2ViaG9vayIsInNwZWMiOnsicmVwbGljYXMiOjEsInNlbGVjdG9yIjp7Im1hdGNoTGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInN0cmF0ZWd5Ijp7fSwidGVtcGxhdGUiOnsibWV0YWRhdGEiOnsibGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInNwZWMiOnsiY29udGFpbmVycyI6W3siYXJncyI6WyItLXNlY3VyZS1saXN0ZW4tYWRkcmVzcz0wLjAuMC4wOjg0NDMiLCItLXVwc3RyZWFtPWh0dHA6Ly8xMjcuMC4wLjE6ODA4MC8iLCItLWxvZ3Rvc3RkZXJyPXRydWUiLCItLXY9MTAiXSwiaW1hZ2UiOiJnY3IuaW8va3ViZWJ1aWxkZXIva3ViZS1yYmFjLXByb3h5OnYwLjUuMCIsIm5hbWUiOiJrdWJlLXJiYWMtcHJveHkiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6ODQ0MywibmFtZSI6Imh0dHBzIn1dLCJyZXNvdXJjZXMiOnt9fSx7ImFyZ3MiOlsiLS1tZXRyaWNzLWFkZHI9MTI3LjAuMC4xOjgwODAiLCItLWVuYWJsZS1sZWFkZXItZWxlY3Rpb24iXSwiY29tbWFuZCI6WyIvbWFuYWdlciJdLCJpbWFnZSI6InF1YXkuaW8vb2xtdGVzdC93ZWJob29rLW9wZXJhdG9yOjAuMC4zIiwibmFtZSI6Im1hbmFnZXIiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6OTQ0MywibmFtZSI6IndlYmhvb2stc2VydmVyIiwicHJvdG9jb2wiOiJUQ1AifV0sInJlc291cmNlcyI6eyJsaW1pdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjMwTWkifSwicmVxdWVzdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjIwTWkifX19XSwidGVybWluYXRpb25HcmFjZVBlcmlvZFNlY29uZHMiOjEwfX19fV0sInBlcm1pc3Npb25zIjpbeyJydWxlcyI6W3siYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiY29uZmlnbWFwcyJdLCJ2ZXJicyI6WyJnZXQiLCJsaXN0Iiwid2F0Y2giLCJjcmVhdGUiLCJ1cGRhdGUiLCJwYXRjaCIsImRlbGV0ZSJdfSx7ImFwaUdyb3VwcyI6WyIiXSwicmVzb3VyY2VzIjpbImNvbmZpZ21hcHMvc3RhdHVzIl0sInZlcmJzIjpbImdldCIsInVwZGF0ZSIsInBhdGNoIl19LHsiYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiZXZlbnRzIl0sInZlcmJzIjpbImNyZWF0ZSJdfV0sInNlcnZpY2VBY2NvdW50TmFtZSI6ImRlZmF1bHQifV19LCJzdHJhdGVneSI6ImRlcGxveW1lbnQifSwiaW5zdGFsbE1vZGVzIjpbeyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiT3duTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiU2luZ2xlTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiTXVsdGlOYW1lc3BhY2UifSx7InN1cHBvcnRlZCI6dHJ1ZSwidHlwZSI6IkFsbE5hbWVzcGFjZXMifV0sImtleXdvcmRzIjpbIndlYmhvb2stb3BlcmF0b3IiXSwibGlua3MiOlt7Im5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwidXJsIjoiaHR0cHM6Ly93ZWJob29rLW9wZXJhdG9yLmRvbWFpbiJ9XSwibWFpbnRhaW5lcnMiOlt7ImVtYWlsIjoieW91ckBlbWFpbC5jb20iLCJuYW1lIjoiTWFpbnRhaW5lciBOYW1lIn1dLCJtYXR1cml0eSI6ImFscGhhIiwicHJvdmlkZXIiOnsibmFtZSI6IlByb3ZpZGVyIE5hbWUiLCJ1cmwiOiJodHRwczovL3lvdXIuZG9tYWluIn0sInZlcnNpb24iOiIwLjAuMSIsIndlYmhvb2tkZWZpbml0aW9ucyI6W3siYWRtaXNzaW9uUmV2aWV3VmVyc2lvbnMiOlsidjFiZXRhMSIsInYxIl0sImNvbnRhaW5lclBvcnQiOjQ0MywiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6InZ3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiVmFsaWRhdGluZ0FkbWlzc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii92YWxpZGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImRlcGxveW1lbnROYW1lIjoid2ViaG9vay1vcGVyYXRvci13ZWJob29rIiwiZmFpbHVyZVBvbGljeSI6IkZhaWwiLCJnZW5lcmF0ZU5hbWUiOiJtd2ViaG9va3Rlc3Qua2IuaW8iLCJydWxlcyI6W3siYXBpR3JvdXBzIjpbIndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJdLCJhcGlWZXJzaW9ucyI6WyJ2MSJdLCJvcGVyYXRpb25zIjpbIkNSRUFURSIsIlVQREFURSJdLCJyZXNvdXJjZXMiOlsid2ViaG9va3Rlc3RzIl19XSwic2lkZUVmZmVjdHMiOiJOb25lIiwidGFyZ2V0UG9ydCI6NDM0MywidHlwZSI6Ik11dGF0aW5nQWRtaXNzaW9uV2ViaG9vayIsIndlYmhvb2tQYXRoIjoiL211dGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImNvbnZlcnNpb25DUkRzIjpbIndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6ImN3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiQ29udmVyc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii9jb252ZXJ0In1dfX0=
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoiYXBpZXh0ZW5zaW9ucy5rOHMuaW8vdjFiZXRhMSIsImtpbmQiOiJDdXN0b21SZXNvdXJjZURlZmluaXRpb24iLCJtZXRhZGF0YSI6eyJhbm5vdGF0aW9ucyI6eyJjb250cm9sbGVyLWdlbi5rdWJlYnVpbGRlci5pby92ZXJzaW9uIjoidjAuMy4wIn0sImNyZWF0aW9uVGltZXN0YW1wIjpudWxsLCJuYW1lIjoid2ViaG9va3Rlc3RzLndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJ9LCJzcGVjIjp7Imdyb3VwIjoid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIiwibmFtZXMiOnsia2luZCI6IldlYmhvb2tUZXN0IiwibGlzdEtpbmQiOiJXZWJob29rVGVzdExpc3QiLCJwbHVyYWwiOiJ3ZWJob29rdGVzdHMiLCJzaW5ndWxhciI6IndlYmhvb2t0ZXN0In0sInByZXNlcnZlVW5rbm93bkZpZWxkcyI6ZmFsc2UsInNjb3BlIjoiTmFtZXNwYWNlZCIsInZlcnNpb24iOiJ2MSIsInZlcnNpb25zIjpbeyJuYW1lIjoidjEiLCJzY2hlbWEiOnsib3BlbkFQSVYzU2NoZW1hIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3QgaXMgdGhlIFNjaGVtYSBmb3IgdGhlIHdlYmhvb2t0ZXN0cyBBUEkiLCJwcm9wZXJ0aWVzIjp7ImFwaVZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJBUElWZXJzaW9uIGRlZmluZXMgdGhlIHZlcnNpb25lZCBzY2hlbWEgb2YgdGhpcyByZXByZXNlbnRhdGlvbiBvZiBhbiBvYmplY3QuIFNlcnZlcnMgc2hvdWxkIGNvbnZlcnQgcmVjb2duaXplZCBzY2hlbWFzIHRvIHRoZSBsYXRlc3QgaW50ZXJuYWwgdmFsdWUsIGFuZCBtYXkgcmVqZWN0IHVucmVjb2duaXplZCB2YWx1ZXMuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjcmVzb3VyY2VzIiwidHlwZSI6InN0cmluZyJ9LCJraW5kIjp7ImRlc2NyaXB0aW9uIjoiS2luZCBpcyBhIHN0cmluZyB2YWx1ZSByZXByZXNlbnRpbmcgdGhlIFJFU1QgcmVzb3VyY2UgdGhpcyBvYmplY3QgcmVwcmVzZW50cy4gU2VydmVycyBtYXkgaW5mZXIgdGhpcyBmcm9tIHRoZSBlbmRwb2ludCB0aGUgY2xpZW50IHN1Ym1pdHMgcmVxdWVzdHMgdG8uIENhbm5vdCBiZSB1cGRhdGVkLiBJbiBDYW1lbENhc2UuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjdHlwZXMta2luZHMiLCJ0eXBlIjoic3RyaW5nIn0sIm1ldGFkYXRhIjp7InR5cGUiOiJvYmplY3QifSwic3BlYyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3BlYyBkZWZpbmVzIHRoZSBkZXNpcmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwicHJvcGVydGllcyI6eyJtdXRhdGUiOnsiZGVzY3JpcHRpb24iOiJNdXRhdGUgaXMgYSBmaWVsZCB0aGF0IHdpbGwgYmUgc2V0IHRvIHRydWUgYnkgdGhlIG11dGF0aW5nIHdlYmhvb2suIiwidHlwZSI6ImJvb2xlYW4ifSwidmFsaWQiOnsiZGVzY3JpcHRpb24iOiJWYWxpZCBtdXN0IGJlIHNldCB0byB0cnVlIG9yIHRoZSB2YWxpZGF0aW9uIHdlYmhvb2sgd2lsbCByZWplY3QgdGhlIHJlc291cmNlLiIsInR5cGUiOiJib29sZWFuIn19LCJyZXF1aXJlZCI6WyJ2YWxpZCJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjp0cnVlfSx7Im5hbWUiOiJ2MiIsInNjaGVtYSI6eyJvcGVuQVBJVjNTY2hlbWEiOnsiZGVzY3JpcHRpb24iOiJXZWJob29rVGVzdCBpcyB0aGUgU2NoZW1hIGZvciB0aGUgd2ViaG9va3Rlc3RzIEFQSSIsInByb3BlcnRpZXMiOnsiYXBpVmVyc2lvbiI6eyJkZXNjcmlwdGlvbiI6IkFQSVZlcnNpb24gZGVmaW5lcyB0aGUgdmVyc2lvbmVkIHNjaGVtYSBvZiB0aGlzIHJlcHJlc2VudGF0aW9uIG9mIGFuIG9iamVjdC4gU2VydmVycyBzaG91bGQgY29udmVydCByZWNvZ25pemVkIHNjaGVtYXMgdG8gdGhlIGxhdGVzdCBpbnRlcm5hbCB2YWx1ZSwgYW5kIG1heSByZWplY3QgdW5yZWNvZ25pemVkIHZhbHVlcy4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCNyZXNvdXJjZXMiLCJ0eXBlIjoic3RyaW5nIn0sImtpbmQiOnsiZGVzY3JpcHRpb24iOiJLaW5kIGlzIGEgc3RyaW5nIHZhbHVlIHJlcHJlc2VudGluZyB0aGUgUkVTVCByZXNvdXJjZSB0aGlzIG9iamVjdCByZXByZXNlbnRzLiBTZXJ2ZXJzIG1heSBpbmZlciB0aGlzIGZyb20gdGhlIGVuZHBvaW50IHRoZSBjbGllbnQgc3VibWl0cyByZXF1ZXN0cyB0by4gQ2Fubm90IGJlIHVwZGF0ZWQuIEluIENhbWVsQ2FzZS4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCN0eXBlcy1raW5kcyIsInR5cGUiOiJzdHJpbmcifSwibWV0YWRhdGEiOnsidHlwZSI6Im9iamVjdCJ9LCJzcGVjIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3RTcGVjIGRlZmluZXMgdGhlIGRlc2lyZWQgc3RhdGUgb2YgV2ViaG9va1Rlc3QiLCJwcm9wZXJ0aWVzIjp7ImNvbnZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJDb252ZXJzaW9uIGlzIGFuIGV4YW1wbGUgZmllbGQgb2YgV2ViaG9va1Rlc3QuIEVkaXQgV2ViaG9va1Rlc3RfdHlwZXMuZ28gdG8gcmVtb3ZlL3VwZGF0ZSIsInByb3BlcnRpZXMiOnsibXV0YXRlIjp7ImRlc2NyaXB0aW9uIjoiTXV0YXRlIGlzIGEgZmllbGQgdGhhdCB3aWxsIGJlIHNldCB0byB0cnVlIGJ5IHRoZSBtdXRhdGluZyB3ZWJob29rLiIsInR5cGUiOiJib29sZWFuIn0sInZhbGlkIjp7ImRlc2NyaXB0aW9uIjoiVmFsaWQgbXVzdCBiZSBzZXQgdG8gdHJ1ZSBvciB0aGUgdmFsaWRhdGlvbiB3ZWJob29rIHdpbGwgcmVqZWN0IHRoZSByZXNvdXJjZS4iLCJ0eXBlIjoiYm9vbGVhbiJ9fSwicmVxdWlyZWQiOlsidmFsaWQiXSwidHlwZSI6Im9iamVjdCJ9fSwicmVxdWlyZWQiOlsiY29udmVyc2lvbiJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjpmYWxzZX1dfSwic3RhdHVzIjp7ImFjY2VwdGVkTmFtZXMiOnsia2luZCI6IiIsInBsdXJhbCI6IiJ9LCJjb25kaXRpb25zIjpbXSwic3RvcmVkVmVyc2lvbnMiOltdfX0=
relatedImages:
- image: gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0
  name: ""
- image: quay.io/olmtest/webhook-operator-bundle:0.0.3
  name: ""
- image: quay.io/olmtest/webhook-operator:0.0.3
  name: ""
schema: olm.bundle
---
schema: olm.channel
package: webhook-operator-412
name: preview
entries:
  - name: webhook-operator.v0.0.1
`

const rawBuiltFbcYaml = `---
defaultChannel: preview
name: webhook-operator-412
schema: olm.package
---
entries:
- name: webhook-operator.v0.0.1
name: preview
package: webhook-operator-412
schema: olm.channel
---
image: quay.io/olmtest/webhook-operator-bundle:0.0.3
name: webhook-operator.v0.0.1
package: webhook-operator-412
properties:
- type: olm.gvk
  value:
    group: webhook.operators.coreos.io
    kind: WebhookTest
    version: v1
- type: olm.gvk
  value:
    group: webhook.operators.coreos.io
    kind: WebhookTest
    version: v2
- type: olm.package
  value:
    packageName: webhook-operator-412
    version: 0.0.1
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoicmJhYy5hdXRob3JpemF0aW9uLms4cy5pby92MWJldGExIiwia2luZCI6IkNsdXN0ZXJSb2xlIiwibWV0YWRhdGEiOnsiY3JlYXRpb25UaW1lc3RhbXAiOm51bGwsIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLW1ldHJpY3MtcmVhZGVyIn0sInJ1bGVzIjpbeyJub25SZXNvdXJjZVVSTHMiOlsiL21ldHJpY3MiXSwidmVyYnMiOlsiZ2V0Il19XX0=
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoib3BlcmF0b3JzLmNvcmVvcy5jb20vdjFhbHBoYTEiLCJraW5kIjoiQ2x1c3RlclNlcnZpY2VWZXJzaW9uIiwibWV0YWRhdGEiOnsiYW5ub3RhdGlvbnMiOnsiYWxtLWV4YW1wbGVzIjoiW1xuICB7XG4gICAgXCJhcGlWZXJzaW9uXCI6IFwid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvL3YxXCIsXG4gICAgXCJraW5kXCI6IFwiV2ViaG9va1Rlc3RcIixcbiAgICBcIm1ldGFkYXRhXCI6IHtcbiAgICAgIFwibmFtZVwiOiBcIndlYmhvb2t0ZXN0LXNhbXBsZVwiLFxuICAgICAgXCJuYW1lc3BhY2VcIjogXCJ3ZWJob29rLW9wZXJhdG9yLXN5c3RlbVwiXG4gICAgfSxcbiAgICBcInNwZWNcIjoge1xuICAgICAgXCJ2YWxpZFwiOiB0cnVlXG4gICAgfVxuICB9XG5dIiwiY2FwYWJpbGl0aWVzIjoiQmFzaWMgSW5zdGFsbCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9idWlsZGVyIjoib3BlcmF0b3Itc2RrLXYxLjAuMCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9wcm9qZWN0X2xheW91dCI6ImdvIn0sIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLnYwLjAuMSIsIm5hbWVzcGFjZSI6InBsYWNlaG9sZGVyIn0sInNwZWMiOnsiYXBpc2VydmljZWRlZmluaXRpb25zIjp7fSwiY3VzdG9tcmVzb3VyY2VkZWZpbml0aW9ucyI6eyJvd25lZCI6W3sia2luZCI6IldlYmhvb2tUZXN0IiwibmFtZSI6IndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iLCJ2ZXJzaW9uIjoidjEifV19LCJkZXNjcmlwdGlvbiI6IldlYmhvb2sgT3BlcmF0b3IgZGVzY3JpcHRpb24uIFRPRE8uIiwiZGlzcGxheU5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwiaWNvbiI6W3siYmFzZTY0ZGF0YSI6IiIsIm1lZGlhdHlwZSI6IiJ9XSwiaW5zdGFsbCI6eyJzcGVjIjp7ImNsdXN0ZXJQZXJtaXNzaW9ucyI6W3sicnVsZXMiOlt7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cyJdLCJ2ZXJicyI6WyJjcmVhdGUiLCJkZWxldGUiLCJnZXQiLCJsaXN0IiwicGF0Y2giLCJ1cGRhdGUiLCJ3YXRjaCJdfSx7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cy9zdGF0dXMiXSwidmVyYnMiOlsiZ2V0IiwicGF0Y2giLCJ1cGRhdGUiXX0seyJhcGlHcm91cHMiOlsiYXV0aGVudGljYXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJ0b2tlbnJldmlld3MiXSwidmVyYnMiOlsiY3JlYXRlIl19LHsiYXBpR3JvdXBzIjpbImF1dGhvcml6YXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJzdWJqZWN0YWNjZXNzcmV2aWV3cyJdLCJ2ZXJicyI6WyJjcmVhdGUiXX1dLCJzZXJ2aWNlQWNjb3VudE5hbWUiOiJkZWZhdWx0In1dLCJkZXBsb3ltZW50cyI6W3sibmFtZSI6IndlYmhvb2stb3BlcmF0b3Itd2ViaG9vayIsInNwZWMiOnsicmVwbGljYXMiOjEsInNlbGVjdG9yIjp7Im1hdGNoTGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInN0cmF0ZWd5Ijp7fSwidGVtcGxhdGUiOnsibWV0YWRhdGEiOnsibGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInNwZWMiOnsiY29udGFpbmVycyI6W3siYXJncyI6WyItLXNlY3VyZS1saXN0ZW4tYWRkcmVzcz0wLjAuMC4wOjg0NDMiLCItLXVwc3RyZWFtPWh0dHA6Ly8xMjcuMC4wLjE6ODA4MC8iLCItLWxvZ3Rvc3RkZXJyPXRydWUiLCItLXY9MTAiXSwiaW1hZ2UiOiJnY3IuaW8va3ViZWJ1aWxkZXIva3ViZS1yYmFjLXByb3h5OnYwLjUuMCIsIm5hbWUiOiJrdWJlLXJiYWMtcHJveHkiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6ODQ0MywibmFtZSI6Imh0dHBzIn1dLCJyZXNvdXJjZXMiOnt9fSx7ImFyZ3MiOlsiLS1tZXRyaWNzLWFkZHI9MTI3LjAuMC4xOjgwODAiLCItLWVuYWJsZS1sZWFkZXItZWxlY3Rpb24iXSwiY29tbWFuZCI6WyIvbWFuYWdlciJdLCJpbWFnZSI6InF1YXkuaW8vb2xtdGVzdC93ZWJob29rLW9wZXJhdG9yOjAuMC4zIiwibmFtZSI6Im1hbmFnZXIiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6OTQ0MywibmFtZSI6IndlYmhvb2stc2VydmVyIiwicHJvdG9jb2wiOiJUQ1AifV0sInJlc291cmNlcyI6eyJsaW1pdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjMwTWkifSwicmVxdWVzdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjIwTWkifX19XSwidGVybWluYXRpb25HcmFjZVBlcmlvZFNlY29uZHMiOjEwfX19fV0sInBlcm1pc3Npb25zIjpbeyJydWxlcyI6W3siYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiY29uZmlnbWFwcyJdLCJ2ZXJicyI6WyJnZXQiLCJsaXN0Iiwid2F0Y2giLCJjcmVhdGUiLCJ1cGRhdGUiLCJwYXRjaCIsImRlbGV0ZSJdfSx7ImFwaUdyb3VwcyI6WyIiXSwicmVzb3VyY2VzIjpbImNvbmZpZ21hcHMvc3RhdHVzIl0sInZlcmJzIjpbImdldCIsInVwZGF0ZSIsInBhdGNoIl19LHsiYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiZXZlbnRzIl0sInZlcmJzIjpbImNyZWF0ZSJdfV0sInNlcnZpY2VBY2NvdW50TmFtZSI6ImRlZmF1bHQifV19LCJzdHJhdGVneSI6ImRlcGxveW1lbnQifSwiaW5zdGFsbE1vZGVzIjpbeyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiT3duTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiU2luZ2xlTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiTXVsdGlOYW1lc3BhY2UifSx7InN1cHBvcnRlZCI6dHJ1ZSwidHlwZSI6IkFsbE5hbWVzcGFjZXMifV0sImtleXdvcmRzIjpbIndlYmhvb2stb3BlcmF0b3IiXSwibGlua3MiOlt7Im5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwidXJsIjoiaHR0cHM6Ly93ZWJob29rLW9wZXJhdG9yLmRvbWFpbiJ9XSwibWFpbnRhaW5lcnMiOlt7ImVtYWlsIjoieW91ckBlbWFpbC5jb20iLCJuYW1lIjoiTWFpbnRhaW5lciBOYW1lIn1dLCJtYXR1cml0eSI6ImFscGhhIiwicHJvdmlkZXIiOnsibmFtZSI6IlByb3ZpZGVyIE5hbWUiLCJ1cmwiOiJodHRwczovL3lvdXIuZG9tYWluIn0sInZlcnNpb24iOiIwLjAuMSIsIndlYmhvb2tkZWZpbml0aW9ucyI6W3siYWRtaXNzaW9uUmV2aWV3VmVyc2lvbnMiOlsidjFiZXRhMSIsInYxIl0sImNvbnRhaW5lclBvcnQiOjQ0MywiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6InZ3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiVmFsaWRhdGluZ0FkbWlzc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii92YWxpZGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImRlcGxveW1lbnROYW1lIjoid2ViaG9vay1vcGVyYXRvci13ZWJob29rIiwiZmFpbHVyZVBvbGljeSI6IkZhaWwiLCJnZW5lcmF0ZU5hbWUiOiJtd2ViaG9va3Rlc3Qua2IuaW8iLCJydWxlcyI6W3siYXBpR3JvdXBzIjpbIndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJdLCJhcGlWZXJzaW9ucyI6WyJ2MSJdLCJvcGVyYXRpb25zIjpbIkNSRUFURSIsIlVQREFURSJdLCJyZXNvdXJjZXMiOlsid2ViaG9va3Rlc3RzIl19XSwic2lkZUVmZmVjdHMiOiJOb25lIiwidGFyZ2V0UG9ydCI6NDM0MywidHlwZSI6Ik11dGF0aW5nQWRtaXNzaW9uV2ViaG9vayIsIndlYmhvb2tQYXRoIjoiL211dGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImNvbnZlcnNpb25DUkRzIjpbIndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6ImN3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiQ29udmVyc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii9jb252ZXJ0In1dfX0=
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoiYXBpZXh0ZW5zaW9ucy5rOHMuaW8vdjFiZXRhMSIsImtpbmQiOiJDdXN0b21SZXNvdXJjZURlZmluaXRpb24iLCJtZXRhZGF0YSI6eyJhbm5vdGF0aW9ucyI6eyJjb250cm9sbGVyLWdlbi5rdWJlYnVpbGRlci5pby92ZXJzaW9uIjoidjAuMy4wIn0sImNyZWF0aW9uVGltZXN0YW1wIjpudWxsLCJuYW1lIjoid2ViaG9va3Rlc3RzLndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJ9LCJzcGVjIjp7Imdyb3VwIjoid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIiwibmFtZXMiOnsia2luZCI6IldlYmhvb2tUZXN0IiwibGlzdEtpbmQiOiJXZWJob29rVGVzdExpc3QiLCJwbHVyYWwiOiJ3ZWJob29rdGVzdHMiLCJzaW5ndWxhciI6IndlYmhvb2t0ZXN0In0sInByZXNlcnZlVW5rbm93bkZpZWxkcyI6ZmFsc2UsInNjb3BlIjoiTmFtZXNwYWNlZCIsInZlcnNpb24iOiJ2MSIsInZlcnNpb25zIjpbeyJuYW1lIjoidjEiLCJzY2hlbWEiOnsib3BlbkFQSVYzU2NoZW1hIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3QgaXMgdGhlIFNjaGVtYSBmb3IgdGhlIHdlYmhvb2t0ZXN0cyBBUEkiLCJwcm9wZXJ0aWVzIjp7ImFwaVZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJBUElWZXJzaW9uIGRlZmluZXMgdGhlIHZlcnNpb25lZCBzY2hlbWEgb2YgdGhpcyByZXByZXNlbnRhdGlvbiBvZiBhbiBvYmplY3QuIFNlcnZlcnMgc2hvdWxkIGNvbnZlcnQgcmVjb2duaXplZCBzY2hlbWFzIHRvIHRoZSBsYXRlc3QgaW50ZXJuYWwgdmFsdWUsIGFuZCBtYXkgcmVqZWN0IHVucmVjb2duaXplZCB2YWx1ZXMuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjcmVzb3VyY2VzIiwidHlwZSI6InN0cmluZyJ9LCJraW5kIjp7ImRlc2NyaXB0aW9uIjoiS2luZCBpcyBhIHN0cmluZyB2YWx1ZSByZXByZXNlbnRpbmcgdGhlIFJFU1QgcmVzb3VyY2UgdGhpcyBvYmplY3QgcmVwcmVzZW50cy4gU2VydmVycyBtYXkgaW5mZXIgdGhpcyBmcm9tIHRoZSBlbmRwb2ludCB0aGUgY2xpZW50IHN1Ym1pdHMgcmVxdWVzdHMgdG8uIENhbm5vdCBiZSB1cGRhdGVkLiBJbiBDYW1lbENhc2UuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjdHlwZXMta2luZHMiLCJ0eXBlIjoic3RyaW5nIn0sIm1ldGFkYXRhIjp7InR5cGUiOiJvYmplY3QifSwic3BlYyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3BlYyBkZWZpbmVzIHRoZSBkZXNpcmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwicHJvcGVydGllcyI6eyJtdXRhdGUiOnsiZGVzY3JpcHRpb24iOiJNdXRhdGUgaXMgYSBmaWVsZCB0aGF0IHdpbGwgYmUgc2V0IHRvIHRydWUgYnkgdGhlIG11dGF0aW5nIHdlYmhvb2suIiwidHlwZSI6ImJvb2xlYW4ifSwidmFsaWQiOnsiZGVzY3JpcHRpb24iOiJWYWxpZCBtdXN0IGJlIHNldCB0byB0cnVlIG9yIHRoZSB2YWxpZGF0aW9uIHdlYmhvb2sgd2lsbCByZWplY3QgdGhlIHJlc291cmNlLiIsInR5cGUiOiJib29sZWFuIn19LCJyZXF1aXJlZCI6WyJ2YWxpZCJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjp0cnVlfSx7Im5hbWUiOiJ2MiIsInNjaGVtYSI6eyJvcGVuQVBJVjNTY2hlbWEiOnsiZGVzY3JpcHRpb24iOiJXZWJob29rVGVzdCBpcyB0aGUgU2NoZW1hIGZvciB0aGUgd2ViaG9va3Rlc3RzIEFQSSIsInByb3BlcnRpZXMiOnsiYXBpVmVyc2lvbiI6eyJkZXNjcmlwdGlvbiI6IkFQSVZlcnNpb24gZGVmaW5lcyB0aGUgdmVyc2lvbmVkIHNjaGVtYSBvZiB0aGlzIHJlcHJlc2VudGF0aW9uIG9mIGFuIG9iamVjdC4gU2VydmVycyBzaG91bGQgY29udmVydCByZWNvZ25pemVkIHNjaGVtYXMgdG8gdGhlIGxhdGVzdCBpbnRlcm5hbCB2YWx1ZSwgYW5kIG1heSByZWplY3QgdW5yZWNvZ25pemVkIHZhbHVlcy4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCNyZXNvdXJjZXMiLCJ0eXBlIjoic3RyaW5nIn0sImtpbmQiOnsiZGVzY3JpcHRpb24iOiJLaW5kIGlzIGEgc3RyaW5nIHZhbHVlIHJlcHJlc2VudGluZyB0aGUgUkVTVCByZXNvdXJjZSB0aGlzIG9iamVjdCByZXByZXNlbnRzLiBTZXJ2ZXJzIG1heSBpbmZlciB0aGlzIGZyb20gdGhlIGVuZHBvaW50IHRoZSBjbGllbnQgc3VibWl0cyByZXF1ZXN0cyB0by4gQ2Fubm90IGJlIHVwZGF0ZWQuIEluIENhbWVsQ2FzZS4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCN0eXBlcy1raW5kcyIsInR5cGUiOiJzdHJpbmcifSwibWV0YWRhdGEiOnsidHlwZSI6Im9iamVjdCJ9LCJzcGVjIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3RTcGVjIGRlZmluZXMgdGhlIGRlc2lyZWQgc3RhdGUgb2YgV2ViaG9va1Rlc3QiLCJwcm9wZXJ0aWVzIjp7ImNvbnZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJDb252ZXJzaW9uIGlzIGFuIGV4YW1wbGUgZmllbGQgb2YgV2ViaG9va1Rlc3QuIEVkaXQgV2ViaG9va1Rlc3RfdHlwZXMuZ28gdG8gcmVtb3ZlL3VwZGF0ZSIsInByb3BlcnRpZXMiOnsibXV0YXRlIjp7ImRlc2NyaXB0aW9uIjoiTXV0YXRlIGlzIGEgZmllbGQgdGhhdCB3aWxsIGJlIHNldCB0byB0cnVlIGJ5IHRoZSBtdXRhdGluZyB3ZWJob29rLiIsInR5cGUiOiJib29sZWFuIn0sInZhbGlkIjp7ImRlc2NyaXB0aW9uIjoiVmFsaWQgbXVzdCBiZSBzZXQgdG8gdHJ1ZSBvciB0aGUgdmFsaWRhdGlvbiB3ZWJob29rIHdpbGwgcmVqZWN0IHRoZSByZXNvdXJjZS4iLCJ0eXBlIjoiYm9vbGVhbiJ9fSwicmVxdWlyZWQiOlsidmFsaWQiXSwidHlwZSI6Im9iamVjdCJ9fSwicmVxdWlyZWQiOlsiY29udmVyc2lvbiJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjpmYWxzZX1dfSwic3RhdHVzIjp7ImFjY2VwdGVkTmFtZXMiOnsia2luZCI6IiIsInBsdXJhbCI6IiJ9LCJjb25kaXRpb25zIjpbXSwic3RvcmVkVmVyc2lvbnMiOltdfX0=
relatedImages:
- image: gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0
  name: ""
- image: quay.io/olmtest/webhook-operator-bundle:0.0.3
  name: ""
- image: quay.io/olmtest/webhook-operator:0.0.3
  name: ""
schema: olm.bundle
`

const rawBuiltFbcJson = `{
    "schema": "olm.package",
    "name": "webhook-operator-412",
    "defaultChannel": "preview"
}
{
    "schema": "olm.channel",
    "name": "preview",
    "package": "webhook-operator-412",
    "entries": [
        {
            "name": "webhook-operator.v0.0.1"
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "webhook-operator.v0.0.1",
    "package": "webhook-operator-412",
    "image": "quay.io/olmtest/webhook-operator-bundle:0.0.3",
    "properties": [
        {
            "type": "olm.gvk",
            "value": {
                "group": "webhook.operators.coreos.io",
                "kind": "WebhookTest",
                "version": "v1"
            }
        },
        {
            "type": "olm.gvk",
            "value": {
                "group": "webhook.operators.coreos.io",
                "kind": "WebhookTest",
                "version": "v2"
            }
        },
        {
            "type": "olm.package",
            "value": {
                "packageName": "webhook-operator-412",
                "version": "0.0.1"
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJhcGlWZXJzaW9uIjoicmJhYy5hdXRob3JpemF0aW9uLms4cy5pby92MWJldGExIiwia2luZCI6IkNsdXN0ZXJSb2xlIiwibWV0YWRhdGEiOnsiY3JlYXRpb25UaW1lc3RhbXAiOm51bGwsIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLW1ldHJpY3MtcmVhZGVyIn0sInJ1bGVzIjpbeyJub25SZXNvdXJjZVVSTHMiOlsiL21ldHJpY3MiXSwidmVyYnMiOlsiZ2V0Il19XX0="
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJhcGlWZXJzaW9uIjoib3BlcmF0b3JzLmNvcmVvcy5jb20vdjFhbHBoYTEiLCJraW5kIjoiQ2x1c3RlclNlcnZpY2VWZXJzaW9uIiwibWV0YWRhdGEiOnsiYW5ub3RhdGlvbnMiOnsiYWxtLWV4YW1wbGVzIjoiW1xuICB7XG4gICAgXCJhcGlWZXJzaW9uXCI6IFwid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvL3YxXCIsXG4gICAgXCJraW5kXCI6IFwiV2ViaG9va1Rlc3RcIixcbiAgICBcIm1ldGFkYXRhXCI6IHtcbiAgICAgIFwibmFtZVwiOiBcIndlYmhvb2t0ZXN0LXNhbXBsZVwiLFxuICAgICAgXCJuYW1lc3BhY2VcIjogXCJ3ZWJob29rLW9wZXJhdG9yLXN5c3RlbVwiXG4gICAgfSxcbiAgICBcInNwZWNcIjoge1xuICAgICAgXCJ2YWxpZFwiOiB0cnVlXG4gICAgfVxuICB9XG5dIiwiY2FwYWJpbGl0aWVzIjoiQmFzaWMgSW5zdGFsbCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9idWlsZGVyIjoib3BlcmF0b3Itc2RrLXYxLjAuMCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9wcm9qZWN0X2xheW91dCI6ImdvIn0sIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLnYwLjAuMSIsIm5hbWVzcGFjZSI6InBsYWNlaG9sZGVyIn0sInNwZWMiOnsiYXBpc2VydmljZWRlZmluaXRpb25zIjp7fSwiY3VzdG9tcmVzb3VyY2VkZWZpbml0aW9ucyI6eyJvd25lZCI6W3sia2luZCI6IldlYmhvb2tUZXN0IiwibmFtZSI6IndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iLCJ2ZXJzaW9uIjoidjEifV19LCJkZXNjcmlwdGlvbiI6IldlYmhvb2sgT3BlcmF0b3IgZGVzY3JpcHRpb24uIFRPRE8uIiwiZGlzcGxheU5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwiaWNvbiI6W3siYmFzZTY0ZGF0YSI6IiIsIm1lZGlhdHlwZSI6IiJ9XSwiaW5zdGFsbCI6eyJzcGVjIjp7ImNsdXN0ZXJQZXJtaXNzaW9ucyI6W3sicnVsZXMiOlt7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cyJdLCJ2ZXJicyI6WyJjcmVhdGUiLCJkZWxldGUiLCJnZXQiLCJsaXN0IiwicGF0Y2giLCJ1cGRhdGUiLCJ3YXRjaCJdfSx7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cy9zdGF0dXMiXSwidmVyYnMiOlsiZ2V0IiwicGF0Y2giLCJ1cGRhdGUiXX0seyJhcGlHcm91cHMiOlsiYXV0aGVudGljYXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJ0b2tlbnJldmlld3MiXSwidmVyYnMiOlsiY3JlYXRlIl19LHsiYXBpR3JvdXBzIjpbImF1dGhvcml6YXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJzdWJqZWN0YWNjZXNzcmV2aWV3cyJdLCJ2ZXJicyI6WyJjcmVhdGUiXX1dLCJzZXJ2aWNlQWNjb3VudE5hbWUiOiJkZWZhdWx0In1dLCJkZXBsb3ltZW50cyI6W3sibmFtZSI6IndlYmhvb2stb3BlcmF0b3Itd2ViaG9vayIsInNwZWMiOnsicmVwbGljYXMiOjEsInNlbGVjdG9yIjp7Im1hdGNoTGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInN0cmF0ZWd5Ijp7fSwidGVtcGxhdGUiOnsibWV0YWRhdGEiOnsibGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInNwZWMiOnsiY29udGFpbmVycyI6W3siYXJncyI6WyItLXNlY3VyZS1saXN0ZW4tYWRkcmVzcz0wLjAuMC4wOjg0NDMiLCItLXVwc3RyZWFtPWh0dHA6Ly8xMjcuMC4wLjE6ODA4MC8iLCItLWxvZ3Rvc3RkZXJyPXRydWUiLCItLXY9MTAiXSwiaW1hZ2UiOiJnY3IuaW8va3ViZWJ1aWxkZXIva3ViZS1yYmFjLXByb3h5OnYwLjUuMCIsIm5hbWUiOiJrdWJlLXJiYWMtcHJveHkiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6ODQ0MywibmFtZSI6Imh0dHBzIn1dLCJyZXNvdXJjZXMiOnt9fSx7ImFyZ3MiOlsiLS1tZXRyaWNzLWFkZHI9MTI3LjAuMC4xOjgwODAiLCItLWVuYWJsZS1sZWFkZXItZWxlY3Rpb24iXSwiY29tbWFuZCI6WyIvbWFuYWdlciJdLCJpbWFnZSI6InF1YXkuaW8vb2xtdGVzdC93ZWJob29rLW9wZXJhdG9yOjAuMC4zIiwibmFtZSI6Im1hbmFnZXIiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6OTQ0MywibmFtZSI6IndlYmhvb2stc2VydmVyIiwicHJvdG9jb2wiOiJUQ1AifV0sInJlc291cmNlcyI6eyJsaW1pdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjMwTWkifSwicmVxdWVzdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjIwTWkifX19XSwidGVybWluYXRpb25HcmFjZVBlcmlvZFNlY29uZHMiOjEwfX19fV0sInBlcm1pc3Npb25zIjpbeyJydWxlcyI6W3siYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiY29uZmlnbWFwcyJdLCJ2ZXJicyI6WyJnZXQiLCJsaXN0Iiwid2F0Y2giLCJjcmVhdGUiLCJ1cGRhdGUiLCJwYXRjaCIsImRlbGV0ZSJdfSx7ImFwaUdyb3VwcyI6WyIiXSwicmVzb3VyY2VzIjpbImNvbmZpZ21hcHMvc3RhdHVzIl0sInZlcmJzIjpbImdldCIsInVwZGF0ZSIsInBhdGNoIl19LHsiYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiZXZlbnRzIl0sInZlcmJzIjpbImNyZWF0ZSJdfV0sInNlcnZpY2VBY2NvdW50TmFtZSI6ImRlZmF1bHQifV19LCJzdHJhdGVneSI6ImRlcGxveW1lbnQifSwiaW5zdGFsbE1vZGVzIjpbeyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiT3duTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiU2luZ2xlTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiTXVsdGlOYW1lc3BhY2UifSx7InN1cHBvcnRlZCI6dHJ1ZSwidHlwZSI6IkFsbE5hbWVzcGFjZXMifV0sImtleXdvcmRzIjpbIndlYmhvb2stb3BlcmF0b3IiXSwibGlua3MiOlt7Im5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwidXJsIjoiaHR0cHM6Ly93ZWJob29rLW9wZXJhdG9yLmRvbWFpbiJ9XSwibWFpbnRhaW5lcnMiOlt7ImVtYWlsIjoieW91ckBlbWFpbC5jb20iLCJuYW1lIjoiTWFpbnRhaW5lciBOYW1lIn1dLCJtYXR1cml0eSI6ImFscGhhIiwicHJvdmlkZXIiOnsibmFtZSI6IlByb3ZpZGVyIE5hbWUiLCJ1cmwiOiJodHRwczovL3lvdXIuZG9tYWluIn0sInZlcnNpb24iOiIwLjAuMSIsIndlYmhvb2tkZWZpbml0aW9ucyI6W3siYWRtaXNzaW9uUmV2aWV3VmVyc2lvbnMiOlsidjFiZXRhMSIsInYxIl0sImNvbnRhaW5lclBvcnQiOjQ0MywiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6InZ3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiVmFsaWRhdGluZ0FkbWlzc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii92YWxpZGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImRlcGxveW1lbnROYW1lIjoid2ViaG9vay1vcGVyYXRvci13ZWJob29rIiwiZmFpbHVyZVBvbGljeSI6IkZhaWwiLCJnZW5lcmF0ZU5hbWUiOiJtd2ViaG9va3Rlc3Qua2IuaW8iLCJydWxlcyI6W3siYXBpR3JvdXBzIjpbIndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJdLCJhcGlWZXJzaW9ucyI6WyJ2MSJdLCJvcGVyYXRpb25zIjpbIkNSRUFURSIsIlVQREFURSJdLCJyZXNvdXJjZXMiOlsid2ViaG9va3Rlc3RzIl19XSwic2lkZUVmZmVjdHMiOiJOb25lIiwidGFyZ2V0UG9ydCI6NDM0MywidHlwZSI6Ik11dGF0aW5nQWRtaXNzaW9uV2ViaG9vayIsIndlYmhvb2tQYXRoIjoiL211dGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImNvbnZlcnNpb25DUkRzIjpbIndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6ImN3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiQ29udmVyc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii9jb252ZXJ0In1dfX0="
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJhcGlWZXJzaW9uIjoiYXBpZXh0ZW5zaW9ucy5rOHMuaW8vdjFiZXRhMSIsImtpbmQiOiJDdXN0b21SZXNvdXJjZURlZmluaXRpb24iLCJtZXRhZGF0YSI6eyJhbm5vdGF0aW9ucyI6eyJjb250cm9sbGVyLWdlbi5rdWJlYnVpbGRlci5pby92ZXJzaW9uIjoidjAuMy4wIn0sImNyZWF0aW9uVGltZXN0YW1wIjpudWxsLCJuYW1lIjoid2ViaG9va3Rlc3RzLndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJ9LCJzcGVjIjp7Imdyb3VwIjoid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIiwibmFtZXMiOnsia2luZCI6IldlYmhvb2tUZXN0IiwibGlzdEtpbmQiOiJXZWJob29rVGVzdExpc3QiLCJwbHVyYWwiOiJ3ZWJob29rdGVzdHMiLCJzaW5ndWxhciI6IndlYmhvb2t0ZXN0In0sInByZXNlcnZlVW5rbm93bkZpZWxkcyI6ZmFsc2UsInNjb3BlIjoiTmFtZXNwYWNlZCIsInZlcnNpb24iOiJ2MSIsInZlcnNpb25zIjpbeyJuYW1lIjoidjEiLCJzY2hlbWEiOnsib3BlbkFQSVYzU2NoZW1hIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3QgaXMgdGhlIFNjaGVtYSBmb3IgdGhlIHdlYmhvb2t0ZXN0cyBBUEkiLCJwcm9wZXJ0aWVzIjp7ImFwaVZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJBUElWZXJzaW9uIGRlZmluZXMgdGhlIHZlcnNpb25lZCBzY2hlbWEgb2YgdGhpcyByZXByZXNlbnRhdGlvbiBvZiBhbiBvYmplY3QuIFNlcnZlcnMgc2hvdWxkIGNvbnZlcnQgcmVjb2duaXplZCBzY2hlbWFzIHRvIHRoZSBsYXRlc3QgaW50ZXJuYWwgdmFsdWUsIGFuZCBtYXkgcmVqZWN0IHVucmVjb2duaXplZCB2YWx1ZXMuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjcmVzb3VyY2VzIiwidHlwZSI6InN0cmluZyJ9LCJraW5kIjp7ImRlc2NyaXB0aW9uIjoiS2luZCBpcyBhIHN0cmluZyB2YWx1ZSByZXByZXNlbnRpbmcgdGhlIFJFU1QgcmVzb3VyY2UgdGhpcyBvYmplY3QgcmVwcmVzZW50cy4gU2VydmVycyBtYXkgaW5mZXIgdGhpcyBmcm9tIHRoZSBlbmRwb2ludCB0aGUgY2xpZW50IHN1Ym1pdHMgcmVxdWVzdHMgdG8uIENhbm5vdCBiZSB1cGRhdGVkLiBJbiBDYW1lbENhc2UuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjdHlwZXMta2luZHMiLCJ0eXBlIjoic3RyaW5nIn0sIm1ldGFkYXRhIjp7InR5cGUiOiJvYmplY3QifSwic3BlYyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3BlYyBkZWZpbmVzIHRoZSBkZXNpcmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwicHJvcGVydGllcyI6eyJtdXRhdGUiOnsiZGVzY3JpcHRpb24iOiJNdXRhdGUgaXMgYSBmaWVsZCB0aGF0IHdpbGwgYmUgc2V0IHRvIHRydWUgYnkgdGhlIG11dGF0aW5nIHdlYmhvb2suIiwidHlwZSI6ImJvb2xlYW4ifSwidmFsaWQiOnsiZGVzY3JpcHRpb24iOiJWYWxpZCBtdXN0IGJlIHNldCB0byB0cnVlIG9yIHRoZSB2YWxpZGF0aW9uIHdlYmhvb2sgd2lsbCByZWplY3QgdGhlIHJlc291cmNlLiIsInR5cGUiOiJib29sZWFuIn19LCJyZXF1aXJlZCI6WyJ2YWxpZCJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjp0cnVlfSx7Im5hbWUiOiJ2MiIsInNjaGVtYSI6eyJvcGVuQVBJVjNTY2hlbWEiOnsiZGVzY3JpcHRpb24iOiJXZWJob29rVGVzdCBpcyB0aGUgU2NoZW1hIGZvciB0aGUgd2ViaG9va3Rlc3RzIEFQSSIsInByb3BlcnRpZXMiOnsiYXBpVmVyc2lvbiI6eyJkZXNjcmlwdGlvbiI6IkFQSVZlcnNpb24gZGVmaW5lcyB0aGUgdmVyc2lvbmVkIHNjaGVtYSBvZiB0aGlzIHJlcHJlc2VudGF0aW9uIG9mIGFuIG9iamVjdC4gU2VydmVycyBzaG91bGQgY29udmVydCByZWNvZ25pemVkIHNjaGVtYXMgdG8gdGhlIGxhdGVzdCBpbnRlcm5hbCB2YWx1ZSwgYW5kIG1heSByZWplY3QgdW5yZWNvZ25pemVkIHZhbHVlcy4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCNyZXNvdXJjZXMiLCJ0eXBlIjoic3RyaW5nIn0sImtpbmQiOnsiZGVzY3JpcHRpb24iOiJLaW5kIGlzIGEgc3RyaW5nIHZhbHVlIHJlcHJlc2VudGluZyB0aGUgUkVTVCByZXNvdXJjZSB0aGlzIG9iamVjdCByZXByZXNlbnRzLiBTZXJ2ZXJzIG1heSBpbmZlciB0aGlzIGZyb20gdGhlIGVuZHBvaW50IHRoZSBjbGllbnQgc3VibWl0cyByZXF1ZXN0cyB0by4gQ2Fubm90IGJlIHVwZGF0ZWQuIEluIENhbWVsQ2FzZS4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCN0eXBlcy1raW5kcyIsInR5cGUiOiJzdHJpbmcifSwibWV0YWRhdGEiOnsidHlwZSI6Im9iamVjdCJ9LCJzcGVjIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3RTcGVjIGRlZmluZXMgdGhlIGRlc2lyZWQgc3RhdGUgb2YgV2ViaG9va1Rlc3QiLCJwcm9wZXJ0aWVzIjp7ImNvbnZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJDb252ZXJzaW9uIGlzIGFuIGV4YW1wbGUgZmllbGQgb2YgV2ViaG9va1Rlc3QuIEVkaXQgV2ViaG9va1Rlc3RfdHlwZXMuZ28gdG8gcmVtb3ZlL3VwZGF0ZSIsInByb3BlcnRpZXMiOnsibXV0YXRlIjp7ImRlc2NyaXB0aW9uIjoiTXV0YXRlIGlzIGEgZmllbGQgdGhhdCB3aWxsIGJlIHNldCB0byB0cnVlIGJ5IHRoZSBtdXRhdGluZyB3ZWJob29rLiIsInR5cGUiOiJib29sZWFuIn0sInZhbGlkIjp7ImRlc2NyaXB0aW9uIjoiVmFsaWQgbXVzdCBiZSBzZXQgdG8gdHJ1ZSBvciB0aGUgdmFsaWRhdGlvbiB3ZWJob29rIHdpbGwgcmVqZWN0IHRoZSByZXNvdXJjZS4iLCJ0eXBlIjoiYm9vbGVhbiJ9fSwicmVxdWlyZWQiOlsidmFsaWQiXSwidHlwZSI6Im9iamVjdCJ9fSwicmVxdWlyZWQiOlsiY29udmVyc2lvbiJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjpmYWxzZX1dfSwic3RhdHVzIjp7ImFjY2VwdGVkTmFtZXMiOnsia2luZCI6IiIsInBsdXJhbCI6IiJ9LCJjb25kaXRpb25zIjpbXSwic3RvcmVkVmVyc2lvbnMiOltdfX0="
            }
        }
    ],
    "relatedImages": [
        {
            "name": "",
            "image": "gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0"
        },
        {
            "name": "",
            "image": "quay.io/olmtest/webhook-operator-bundle:0.0.3"
        },
        {
            "name": "",
            "image": "quay.io/olmtest/webhook-operator:0.0.3"
        }
    ]
}
`

func TestCustomBuilder(t *testing.T) {
	type testCase struct {
		name               string
		validate           bool
		customBuilder      *CustomBuilder
		templateDefinition TemplateDefinition
		files              map[string]string
		buildAssertions    func(t *testing.T, dir string, buildErr error)
		validateAssertions func(t *testing.T, validateErr error)
	}

	testDir := t.TempDir()
	validTemplateConfig := `{
		"command": "%s",
		"args": ["%s"],
		"output": "%s"
	}`

	testCases := []testCase{
		{
			name:     "successful custom build yaml output",
			validate: true,
			customBuilder: NewCustomBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: CustomBuilderSchema,
				Config: []byte(fmt.Sprintf(validTemplateConfig, "cat", path.Join(testDir, "components/custom.yaml"), "catalog.yaml")),
			},
			files: map[string]string{
				"components/custom.yaml": customYaml,
			},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.NoError(t, buildErr)
				// check if the catalog.yaml file exists in the correct place
				filePath := path.Join(dir, "catalog.yaml")
				_, err := os.Stat(filePath)
				require.NoError(t, err)
				file, err := os.Open(filePath)
				require.NoError(t, err)
				defer file.Close()
				fileData, err := io.ReadAll(file)
				require.NoError(t, err)
				require.Equal(t, string(fileData), customBuiltFbcYaml)
			},
			validateAssertions: func(t *testing.T, validateErr error) {
				require.NoError(t, validateErr)
			},
		},
		{
			name:     "successful custom build json output",
			validate: true,
			customBuilder: NewCustomBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "json",
			}),
			templateDefinition: TemplateDefinition{
				Schema: CustomBuilderSchema,
				Config: []byte(fmt.Sprintf(validTemplateConfig, "cat", path.Join(testDir, "components/custom.yaml"), "catalog.json")),
			},
			files: map[string]string{
				"components/custom.yaml": customYaml,
			},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.NoError(t, buildErr)
				// check if the catalog.yaml file exists in the correct place
				filePath := path.Join(dir, "catalog.json")
				_, err := os.Stat(filePath)
				require.NoError(t, err)
				file, err := os.Open(filePath)
				require.NoError(t, err)
				defer file.Close()
				fileData, err := io.ReadAll(file)
				require.NoError(t, err)
				require.Equal(t, string(fileData), customBuiltFbcJson)
			},
			validateAssertions: func(t *testing.T, validateErr error) {
				require.NoError(t, validateErr)
			},
		},
		{
			name:     "invalid template configuration",
			validate: false,
			customBuilder: NewCustomBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: CustomBuilderSchema,
				Config: []byte(`{
					"invalid": "components/custom.yaml",
				}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Contains(t, buildErr.Error(), "unmarshalling custom template config:")
			},
		},
		{
			name:     "invalid schema",
			validate: false,
			customBuilder: NewCustomBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: "olm.invalid",
				Config: []byte(`{
					"input": "components/custom.yaml",
					"output": "catalog.yaml"
				}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Contains(t, buildErr.Error(), fmt.Sprintf("schema %q does not match the custom template builder schema %q", "olm.invalid", CustomBuilderSchema))
			},
		},
		{
			name:     "template config has empty command",
			validate: false,
			customBuilder: NewCustomBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: CustomBuilderSchema,
				Config: []byte(`{
					"output": "catalog.yaml"
				}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Equal(t,
					"custom template configuration is invalid: custom template config must have a non-empty command (templateDefinition.config.command)",
					buildErr.Error(),
				)
			},
		},
		{
			name:     "template config has empty output",
			validate: false,
			customBuilder: NewCustomBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: CustomBuilderSchema,
				Config: []byte(`{
					"command": "ls"
				}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Equal(t,
					"custom template configuration is invalid: custom template config must have a non-empty output (templateDefinition.config.output)",
					buildErr.Error(),
				)
			},
		},
		{
			name:     "template config has empty command & output",
			validate: false,
			customBuilder: NewCustomBuilder(BuilderConfig{
				WorkingDir:    testDir,
				OutputType: "yaml",
			}),
			templateDefinition: TemplateDefinition{
				Schema: CustomBuilderSchema,
				Config: []byte(`{}`),
			},
			files: map[string]string{},
			buildAssertions: func(t *testing.T, dir string, buildErr error) {
				require.Error(t, buildErr)
				require.Equal(t,
					"custom template configuration is invalid: custom template config must have a non-empty command (templateDefinition.config.command),custom template config must have a non-empty output (templateDefinition.config.output)",
					buildErr.Error(),
				)
			},
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outDir := fmt.Sprintf("custom-%d", i)
			outPath := path.Join(testDir, outDir)
			err := os.MkdirAll(outPath, 0o777)
			require.NoError(t, err)

			// create files in temp dir
			for fileName, fileContents := range tc.files {
				err := os.MkdirAll(path.Join(testDir, path.Dir(fileName)), 0o777)
				require.NoError(t, err)
				file, err := os.Create(path.Join(testDir, fileName))
				require.NoError(t, err)
				_, err = file.WriteString(fileContents)
				require.NoError(t, err)
			}

			cacheDir, err := os.MkdirTemp("", "opm-registry-")
			require.NoError(t, err)

			reg, err := containerdregistry.NewRegistry(
				containerdregistry.WithCacheDir(cacheDir),
			)
			defer reg.Destroy()
			require.NoError(t, err)

			buildErr := tc.customBuilder.Build(context.Background(), reg, outDir, tc.templateDefinition)
			tc.buildAssertions(t, outPath, buildErr)

			if tc.validate {
				validateErr := tc.customBuilder.Validate(outDir)
				tc.validateAssertions(t, validateErr)
			}
		})
	}
}

const customYaml = `---
defaultChannel: preview
name: webhook-operator-413
schema: olm.package
---
image: quay.io/olmtest/webhook-operator-bundle:0.0.3
name: webhook-operator.v0.0.1
package: webhook-operator-413
properties:
- type: olm.gvk
  value:
    group: webhook.operators.coreos.io
    kind: WebhookTest
    version: v1
- type: olm.gvk
  value:
    group: webhook.operators.coreos.io
    kind: WebhookTest
    version: v2
- type: olm.package
  value:
    packageName: webhook-operator-413
    version: 0.0.1
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoicmJhYy5hdXRob3JpemF0aW9uLms4cy5pby92MWJldGExIiwia2luZCI6IkNsdXN0ZXJSb2xlIiwibWV0YWRhdGEiOnsiY3JlYXRpb25UaW1lc3RhbXAiOm51bGwsIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLW1ldHJpY3MtcmVhZGVyIn0sInJ1bGVzIjpbeyJub25SZXNvdXJjZVVSTHMiOlsiL21ldHJpY3MiXSwidmVyYnMiOlsiZ2V0Il19XX0=
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoib3BlcmF0b3JzLmNvcmVvcy5jb20vdjFhbHBoYTEiLCJraW5kIjoiQ2x1c3RlclNlcnZpY2VWZXJzaW9uIiwibWV0YWRhdGEiOnsiYW5ub3RhdGlvbnMiOnsiYWxtLWV4YW1wbGVzIjoiW1xuICB7XG4gICAgXCJhcGlWZXJzaW9uXCI6IFwid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvL3YxXCIsXG4gICAgXCJraW5kXCI6IFwiV2ViaG9va1Rlc3RcIixcbiAgICBcIm1ldGFkYXRhXCI6IHtcbiAgICAgIFwibmFtZVwiOiBcIndlYmhvb2t0ZXN0LXNhbXBsZVwiLFxuICAgICAgXCJuYW1lc3BhY2VcIjogXCJ3ZWJob29rLW9wZXJhdG9yLXN5c3RlbVwiXG4gICAgfSxcbiAgICBcInNwZWNcIjoge1xuICAgICAgXCJ2YWxpZFwiOiB0cnVlXG4gICAgfVxuICB9XG5dIiwiY2FwYWJpbGl0aWVzIjoiQmFzaWMgSW5zdGFsbCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9idWlsZGVyIjoib3BlcmF0b3Itc2RrLXYxLjAuMCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9wcm9qZWN0X2xheW91dCI6ImdvIn0sIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLnYwLjAuMSIsIm5hbWVzcGFjZSI6InBsYWNlaG9sZGVyIn0sInNwZWMiOnsiYXBpc2VydmljZWRlZmluaXRpb25zIjp7fSwiY3VzdG9tcmVzb3VyY2VkZWZpbml0aW9ucyI6eyJvd25lZCI6W3sia2luZCI6IldlYmhvb2tUZXN0IiwibmFtZSI6IndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iLCJ2ZXJzaW9uIjoidjEifV19LCJkZXNjcmlwdGlvbiI6IldlYmhvb2sgT3BlcmF0b3IgZGVzY3JpcHRpb24uIFRPRE8uIiwiZGlzcGxheU5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwiaWNvbiI6W3siYmFzZTY0ZGF0YSI6IiIsIm1lZGlhdHlwZSI6IiJ9XSwiaW5zdGFsbCI6eyJzcGVjIjp7ImNsdXN0ZXJQZXJtaXNzaW9ucyI6W3sicnVsZXMiOlt7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cyJdLCJ2ZXJicyI6WyJjcmVhdGUiLCJkZWxldGUiLCJnZXQiLCJsaXN0IiwicGF0Y2giLCJ1cGRhdGUiLCJ3YXRjaCJdfSx7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cy9zdGF0dXMiXSwidmVyYnMiOlsiZ2V0IiwicGF0Y2giLCJ1cGRhdGUiXX0seyJhcGlHcm91cHMiOlsiYXV0aGVudGljYXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJ0b2tlbnJldmlld3MiXSwidmVyYnMiOlsiY3JlYXRlIl19LHsiYXBpR3JvdXBzIjpbImF1dGhvcml6YXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJzdWJqZWN0YWNjZXNzcmV2aWV3cyJdLCJ2ZXJicyI6WyJjcmVhdGUiXX1dLCJzZXJ2aWNlQWNjb3VudE5hbWUiOiJkZWZhdWx0In1dLCJkZXBsb3ltZW50cyI6W3sibmFtZSI6IndlYmhvb2stb3BlcmF0b3Itd2ViaG9vayIsInNwZWMiOnsicmVwbGljYXMiOjEsInNlbGVjdG9yIjp7Im1hdGNoTGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInN0cmF0ZWd5Ijp7fSwidGVtcGxhdGUiOnsibWV0YWRhdGEiOnsibGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInNwZWMiOnsiY29udGFpbmVycyI6W3siYXJncyI6WyItLXNlY3VyZS1saXN0ZW4tYWRkcmVzcz0wLjAuMC4wOjg0NDMiLCItLXVwc3RyZWFtPWh0dHA6Ly8xMjcuMC4wLjE6ODA4MC8iLCItLWxvZ3Rvc3RkZXJyPXRydWUiLCItLXY9MTAiXSwiaW1hZ2UiOiJnY3IuaW8va3ViZWJ1aWxkZXIva3ViZS1yYmFjLXByb3h5OnYwLjUuMCIsIm5hbWUiOiJrdWJlLXJiYWMtcHJveHkiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6ODQ0MywibmFtZSI6Imh0dHBzIn1dLCJyZXNvdXJjZXMiOnt9fSx7ImFyZ3MiOlsiLS1tZXRyaWNzLWFkZHI9MTI3LjAuMC4xOjgwODAiLCItLWVuYWJsZS1sZWFkZXItZWxlY3Rpb24iXSwiY29tbWFuZCI6WyIvbWFuYWdlciJdLCJpbWFnZSI6InF1YXkuaW8vb2xtdGVzdC93ZWJob29rLW9wZXJhdG9yOjAuMC4zIiwibmFtZSI6Im1hbmFnZXIiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6OTQ0MywibmFtZSI6IndlYmhvb2stc2VydmVyIiwicHJvdG9jb2wiOiJUQ1AifV0sInJlc291cmNlcyI6eyJsaW1pdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjMwTWkifSwicmVxdWVzdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjIwTWkifX19XSwidGVybWluYXRpb25HcmFjZVBlcmlvZFNlY29uZHMiOjEwfX19fV0sInBlcm1pc3Npb25zIjpbeyJydWxlcyI6W3siYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiY29uZmlnbWFwcyJdLCJ2ZXJicyI6WyJnZXQiLCJsaXN0Iiwid2F0Y2giLCJjcmVhdGUiLCJ1cGRhdGUiLCJwYXRjaCIsImRlbGV0ZSJdfSx7ImFwaUdyb3VwcyI6WyIiXSwicmVzb3VyY2VzIjpbImNvbmZpZ21hcHMvc3RhdHVzIl0sInZlcmJzIjpbImdldCIsInVwZGF0ZSIsInBhdGNoIl19LHsiYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiZXZlbnRzIl0sInZlcmJzIjpbImNyZWF0ZSJdfV0sInNlcnZpY2VBY2NvdW50TmFtZSI6ImRlZmF1bHQifV19LCJzdHJhdGVneSI6ImRlcGxveW1lbnQifSwiaW5zdGFsbE1vZGVzIjpbeyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiT3duTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiU2luZ2xlTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiTXVsdGlOYW1lc3BhY2UifSx7InN1cHBvcnRlZCI6dHJ1ZSwidHlwZSI6IkFsbE5hbWVzcGFjZXMifV0sImtleXdvcmRzIjpbIndlYmhvb2stb3BlcmF0b3IiXSwibGlua3MiOlt7Im5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwidXJsIjoiaHR0cHM6Ly93ZWJob29rLW9wZXJhdG9yLmRvbWFpbiJ9XSwibWFpbnRhaW5lcnMiOlt7ImVtYWlsIjoieW91ckBlbWFpbC5jb20iLCJuYW1lIjoiTWFpbnRhaW5lciBOYW1lIn1dLCJtYXR1cml0eSI6ImFscGhhIiwicHJvdmlkZXIiOnsibmFtZSI6IlByb3ZpZGVyIE5hbWUiLCJ1cmwiOiJodHRwczovL3lvdXIuZG9tYWluIn0sInZlcnNpb24iOiIwLjAuMSIsIndlYmhvb2tkZWZpbml0aW9ucyI6W3siYWRtaXNzaW9uUmV2aWV3VmVyc2lvbnMiOlsidjFiZXRhMSIsInYxIl0sImNvbnRhaW5lclBvcnQiOjQ0MywiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6InZ3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiVmFsaWRhdGluZ0FkbWlzc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii92YWxpZGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImRlcGxveW1lbnROYW1lIjoid2ViaG9vay1vcGVyYXRvci13ZWJob29rIiwiZmFpbHVyZVBvbGljeSI6IkZhaWwiLCJnZW5lcmF0ZU5hbWUiOiJtd2ViaG9va3Rlc3Qua2IuaW8iLCJydWxlcyI6W3siYXBpR3JvdXBzIjpbIndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJdLCJhcGlWZXJzaW9ucyI6WyJ2MSJdLCJvcGVyYXRpb25zIjpbIkNSRUFURSIsIlVQREFURSJdLCJyZXNvdXJjZXMiOlsid2ViaG9va3Rlc3RzIl19XSwic2lkZUVmZmVjdHMiOiJOb25lIiwidGFyZ2V0UG9ydCI6NDM0MywidHlwZSI6Ik11dGF0aW5nQWRtaXNzaW9uV2ViaG9vayIsIndlYmhvb2tQYXRoIjoiL211dGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImNvbnZlcnNpb25DUkRzIjpbIndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6ImN3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiQ29udmVyc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii9jb252ZXJ0In1dfX0=
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoiYXBpZXh0ZW5zaW9ucy5rOHMuaW8vdjFiZXRhMSIsImtpbmQiOiJDdXN0b21SZXNvdXJjZURlZmluaXRpb24iLCJtZXRhZGF0YSI6eyJhbm5vdGF0aW9ucyI6eyJjb250cm9sbGVyLWdlbi5rdWJlYnVpbGRlci5pby92ZXJzaW9uIjoidjAuMy4wIn0sImNyZWF0aW9uVGltZXN0YW1wIjpudWxsLCJuYW1lIjoid2ViaG9va3Rlc3RzLndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJ9LCJzcGVjIjp7Imdyb3VwIjoid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIiwibmFtZXMiOnsia2luZCI6IldlYmhvb2tUZXN0IiwibGlzdEtpbmQiOiJXZWJob29rVGVzdExpc3QiLCJwbHVyYWwiOiJ3ZWJob29rdGVzdHMiLCJzaW5ndWxhciI6IndlYmhvb2t0ZXN0In0sInByZXNlcnZlVW5rbm93bkZpZWxkcyI6ZmFsc2UsInNjb3BlIjoiTmFtZXNwYWNlZCIsInZlcnNpb24iOiJ2MSIsInZlcnNpb25zIjpbeyJuYW1lIjoidjEiLCJzY2hlbWEiOnsib3BlbkFQSVYzU2NoZW1hIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3QgaXMgdGhlIFNjaGVtYSBmb3IgdGhlIHdlYmhvb2t0ZXN0cyBBUEkiLCJwcm9wZXJ0aWVzIjp7ImFwaVZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJBUElWZXJzaW9uIGRlZmluZXMgdGhlIHZlcnNpb25lZCBzY2hlbWEgb2YgdGhpcyByZXByZXNlbnRhdGlvbiBvZiBhbiBvYmplY3QuIFNlcnZlcnMgc2hvdWxkIGNvbnZlcnQgcmVjb2duaXplZCBzY2hlbWFzIHRvIHRoZSBsYXRlc3QgaW50ZXJuYWwgdmFsdWUsIGFuZCBtYXkgcmVqZWN0IHVucmVjb2duaXplZCB2YWx1ZXMuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjcmVzb3VyY2VzIiwidHlwZSI6InN0cmluZyJ9LCJraW5kIjp7ImRlc2NyaXB0aW9uIjoiS2luZCBpcyBhIHN0cmluZyB2YWx1ZSByZXByZXNlbnRpbmcgdGhlIFJFU1QgcmVzb3VyY2UgdGhpcyBvYmplY3QgcmVwcmVzZW50cy4gU2VydmVycyBtYXkgaW5mZXIgdGhpcyBmcm9tIHRoZSBlbmRwb2ludCB0aGUgY2xpZW50IHN1Ym1pdHMgcmVxdWVzdHMgdG8uIENhbm5vdCBiZSB1cGRhdGVkLiBJbiBDYW1lbENhc2UuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjdHlwZXMta2luZHMiLCJ0eXBlIjoic3RyaW5nIn0sIm1ldGFkYXRhIjp7InR5cGUiOiJvYmplY3QifSwic3BlYyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3BlYyBkZWZpbmVzIHRoZSBkZXNpcmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwicHJvcGVydGllcyI6eyJtdXRhdGUiOnsiZGVzY3JpcHRpb24iOiJNdXRhdGUgaXMgYSBmaWVsZCB0aGF0IHdpbGwgYmUgc2V0IHRvIHRydWUgYnkgdGhlIG11dGF0aW5nIHdlYmhvb2suIiwidHlwZSI6ImJvb2xlYW4ifSwidmFsaWQiOnsiZGVzY3JpcHRpb24iOiJWYWxpZCBtdXN0IGJlIHNldCB0byB0cnVlIG9yIHRoZSB2YWxpZGF0aW9uIHdlYmhvb2sgd2lsbCByZWplY3QgdGhlIHJlc291cmNlLiIsInR5cGUiOiJib29sZWFuIn19LCJyZXF1aXJlZCI6WyJ2YWxpZCJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjp0cnVlfSx7Im5hbWUiOiJ2MiIsInNjaGVtYSI6eyJvcGVuQVBJVjNTY2hlbWEiOnsiZGVzY3JpcHRpb24iOiJXZWJob29rVGVzdCBpcyB0aGUgU2NoZW1hIGZvciB0aGUgd2ViaG9va3Rlc3RzIEFQSSIsInByb3BlcnRpZXMiOnsiYXBpVmVyc2lvbiI6eyJkZXNjcmlwdGlvbiI6IkFQSVZlcnNpb24gZGVmaW5lcyB0aGUgdmVyc2lvbmVkIHNjaGVtYSBvZiB0aGlzIHJlcHJlc2VudGF0aW9uIG9mIGFuIG9iamVjdC4gU2VydmVycyBzaG91bGQgY29udmVydCByZWNvZ25pemVkIHNjaGVtYXMgdG8gdGhlIGxhdGVzdCBpbnRlcm5hbCB2YWx1ZSwgYW5kIG1heSByZWplY3QgdW5yZWNvZ25pemVkIHZhbHVlcy4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCNyZXNvdXJjZXMiLCJ0eXBlIjoic3RyaW5nIn0sImtpbmQiOnsiZGVzY3JpcHRpb24iOiJLaW5kIGlzIGEgc3RyaW5nIHZhbHVlIHJlcHJlc2VudGluZyB0aGUgUkVTVCByZXNvdXJjZSB0aGlzIG9iamVjdCByZXByZXNlbnRzLiBTZXJ2ZXJzIG1heSBpbmZlciB0aGlzIGZyb20gdGhlIGVuZHBvaW50IHRoZSBjbGllbnQgc3VibWl0cyByZXF1ZXN0cyB0by4gQ2Fubm90IGJlIHVwZGF0ZWQuIEluIENhbWVsQ2FzZS4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCN0eXBlcy1raW5kcyIsInR5cGUiOiJzdHJpbmcifSwibWV0YWRhdGEiOnsidHlwZSI6Im9iamVjdCJ9LCJzcGVjIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3RTcGVjIGRlZmluZXMgdGhlIGRlc2lyZWQgc3RhdGUgb2YgV2ViaG9va1Rlc3QiLCJwcm9wZXJ0aWVzIjp7ImNvbnZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJDb252ZXJzaW9uIGlzIGFuIGV4YW1wbGUgZmllbGQgb2YgV2ViaG9va1Rlc3QuIEVkaXQgV2ViaG9va1Rlc3RfdHlwZXMuZ28gdG8gcmVtb3ZlL3VwZGF0ZSIsInByb3BlcnRpZXMiOnsibXV0YXRlIjp7ImRlc2NyaXB0aW9uIjoiTXV0YXRlIGlzIGEgZmllbGQgdGhhdCB3aWxsIGJlIHNldCB0byB0cnVlIGJ5IHRoZSBtdXRhdGluZyB3ZWJob29rLiIsInR5cGUiOiJib29sZWFuIn0sInZhbGlkIjp7ImRlc2NyaXB0aW9uIjoiVmFsaWQgbXVzdCBiZSBzZXQgdG8gdHJ1ZSBvciB0aGUgdmFsaWRhdGlvbiB3ZWJob29rIHdpbGwgcmVqZWN0IHRoZSByZXNvdXJjZS4iLCJ0eXBlIjoiYm9vbGVhbiJ9fSwicmVxdWlyZWQiOlsidmFsaWQiXSwidHlwZSI6Im9iamVjdCJ9fSwicmVxdWlyZWQiOlsiY29udmVyc2lvbiJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjpmYWxzZX1dfSwic3RhdHVzIjp7ImFjY2VwdGVkTmFtZXMiOnsia2luZCI6IiIsInBsdXJhbCI6IiJ9LCJjb25kaXRpb25zIjpbXSwic3RvcmVkVmVyc2lvbnMiOltdfX0=
relatedImages:
- image: gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0
  name: ""
- image: quay.io/olmtest/webhook-operator-bundle:0.0.3
  name: ""
- image: quay.io/olmtest/webhook-operator:0.0.3
  name: ""
schema: olm.bundle
---
schema: olm.channel
package: webhook-operator-413
name: preview
entries:
  - name: webhook-operator.v0.0.1
`

const customBuiltFbcYaml = `---
defaultChannel: preview
name: webhook-operator-413
schema: olm.package
---
entries:
- name: webhook-operator.v0.0.1
name: preview
package: webhook-operator-413
schema: olm.channel
---
image: quay.io/olmtest/webhook-operator-bundle:0.0.3
name: webhook-operator.v0.0.1
package: webhook-operator-413
properties:
- type: olm.gvk
  value:
    group: webhook.operators.coreos.io
    kind: WebhookTest
    version: v1
- type: olm.gvk
  value:
    group: webhook.operators.coreos.io
    kind: WebhookTest
    version: v2
- type: olm.package
  value:
    packageName: webhook-operator-413
    version: 0.0.1
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoicmJhYy5hdXRob3JpemF0aW9uLms4cy5pby92MWJldGExIiwia2luZCI6IkNsdXN0ZXJSb2xlIiwibWV0YWRhdGEiOnsiY3JlYXRpb25UaW1lc3RhbXAiOm51bGwsIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLW1ldHJpY3MtcmVhZGVyIn0sInJ1bGVzIjpbeyJub25SZXNvdXJjZVVSTHMiOlsiL21ldHJpY3MiXSwidmVyYnMiOlsiZ2V0Il19XX0=
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoib3BlcmF0b3JzLmNvcmVvcy5jb20vdjFhbHBoYTEiLCJraW5kIjoiQ2x1c3RlclNlcnZpY2VWZXJzaW9uIiwibWV0YWRhdGEiOnsiYW5ub3RhdGlvbnMiOnsiYWxtLWV4YW1wbGVzIjoiW1xuICB7XG4gICAgXCJhcGlWZXJzaW9uXCI6IFwid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvL3YxXCIsXG4gICAgXCJraW5kXCI6IFwiV2ViaG9va1Rlc3RcIixcbiAgICBcIm1ldGFkYXRhXCI6IHtcbiAgICAgIFwibmFtZVwiOiBcIndlYmhvb2t0ZXN0LXNhbXBsZVwiLFxuICAgICAgXCJuYW1lc3BhY2VcIjogXCJ3ZWJob29rLW9wZXJhdG9yLXN5c3RlbVwiXG4gICAgfSxcbiAgICBcInNwZWNcIjoge1xuICAgICAgXCJ2YWxpZFwiOiB0cnVlXG4gICAgfVxuICB9XG5dIiwiY2FwYWJpbGl0aWVzIjoiQmFzaWMgSW5zdGFsbCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9idWlsZGVyIjoib3BlcmF0b3Itc2RrLXYxLjAuMCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9wcm9qZWN0X2xheW91dCI6ImdvIn0sIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLnYwLjAuMSIsIm5hbWVzcGFjZSI6InBsYWNlaG9sZGVyIn0sInNwZWMiOnsiYXBpc2VydmljZWRlZmluaXRpb25zIjp7fSwiY3VzdG9tcmVzb3VyY2VkZWZpbml0aW9ucyI6eyJvd25lZCI6W3sia2luZCI6IldlYmhvb2tUZXN0IiwibmFtZSI6IndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iLCJ2ZXJzaW9uIjoidjEifV19LCJkZXNjcmlwdGlvbiI6IldlYmhvb2sgT3BlcmF0b3IgZGVzY3JpcHRpb24uIFRPRE8uIiwiZGlzcGxheU5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwiaWNvbiI6W3siYmFzZTY0ZGF0YSI6IiIsIm1lZGlhdHlwZSI6IiJ9XSwiaW5zdGFsbCI6eyJzcGVjIjp7ImNsdXN0ZXJQZXJtaXNzaW9ucyI6W3sicnVsZXMiOlt7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cyJdLCJ2ZXJicyI6WyJjcmVhdGUiLCJkZWxldGUiLCJnZXQiLCJsaXN0IiwicGF0Y2giLCJ1cGRhdGUiLCJ3YXRjaCJdfSx7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cy9zdGF0dXMiXSwidmVyYnMiOlsiZ2V0IiwicGF0Y2giLCJ1cGRhdGUiXX0seyJhcGlHcm91cHMiOlsiYXV0aGVudGljYXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJ0b2tlbnJldmlld3MiXSwidmVyYnMiOlsiY3JlYXRlIl19LHsiYXBpR3JvdXBzIjpbImF1dGhvcml6YXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJzdWJqZWN0YWNjZXNzcmV2aWV3cyJdLCJ2ZXJicyI6WyJjcmVhdGUiXX1dLCJzZXJ2aWNlQWNjb3VudE5hbWUiOiJkZWZhdWx0In1dLCJkZXBsb3ltZW50cyI6W3sibmFtZSI6IndlYmhvb2stb3BlcmF0b3Itd2ViaG9vayIsInNwZWMiOnsicmVwbGljYXMiOjEsInNlbGVjdG9yIjp7Im1hdGNoTGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInN0cmF0ZWd5Ijp7fSwidGVtcGxhdGUiOnsibWV0YWRhdGEiOnsibGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInNwZWMiOnsiY29udGFpbmVycyI6W3siYXJncyI6WyItLXNlY3VyZS1saXN0ZW4tYWRkcmVzcz0wLjAuMC4wOjg0NDMiLCItLXVwc3RyZWFtPWh0dHA6Ly8xMjcuMC4wLjE6ODA4MC8iLCItLWxvZ3Rvc3RkZXJyPXRydWUiLCItLXY9MTAiXSwiaW1hZ2UiOiJnY3IuaW8va3ViZWJ1aWxkZXIva3ViZS1yYmFjLXByb3h5OnYwLjUuMCIsIm5hbWUiOiJrdWJlLXJiYWMtcHJveHkiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6ODQ0MywibmFtZSI6Imh0dHBzIn1dLCJyZXNvdXJjZXMiOnt9fSx7ImFyZ3MiOlsiLS1tZXRyaWNzLWFkZHI9MTI3LjAuMC4xOjgwODAiLCItLWVuYWJsZS1sZWFkZXItZWxlY3Rpb24iXSwiY29tbWFuZCI6WyIvbWFuYWdlciJdLCJpbWFnZSI6InF1YXkuaW8vb2xtdGVzdC93ZWJob29rLW9wZXJhdG9yOjAuMC4zIiwibmFtZSI6Im1hbmFnZXIiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6OTQ0MywibmFtZSI6IndlYmhvb2stc2VydmVyIiwicHJvdG9jb2wiOiJUQ1AifV0sInJlc291cmNlcyI6eyJsaW1pdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjMwTWkifSwicmVxdWVzdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjIwTWkifX19XSwidGVybWluYXRpb25HcmFjZVBlcmlvZFNlY29uZHMiOjEwfX19fV0sInBlcm1pc3Npb25zIjpbeyJydWxlcyI6W3siYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiY29uZmlnbWFwcyJdLCJ2ZXJicyI6WyJnZXQiLCJsaXN0Iiwid2F0Y2giLCJjcmVhdGUiLCJ1cGRhdGUiLCJwYXRjaCIsImRlbGV0ZSJdfSx7ImFwaUdyb3VwcyI6WyIiXSwicmVzb3VyY2VzIjpbImNvbmZpZ21hcHMvc3RhdHVzIl0sInZlcmJzIjpbImdldCIsInVwZGF0ZSIsInBhdGNoIl19LHsiYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiZXZlbnRzIl0sInZlcmJzIjpbImNyZWF0ZSJdfV0sInNlcnZpY2VBY2NvdW50TmFtZSI6ImRlZmF1bHQifV19LCJzdHJhdGVneSI6ImRlcGxveW1lbnQifSwiaW5zdGFsbE1vZGVzIjpbeyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiT3duTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiU2luZ2xlTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiTXVsdGlOYW1lc3BhY2UifSx7InN1cHBvcnRlZCI6dHJ1ZSwidHlwZSI6IkFsbE5hbWVzcGFjZXMifV0sImtleXdvcmRzIjpbIndlYmhvb2stb3BlcmF0b3IiXSwibGlua3MiOlt7Im5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwidXJsIjoiaHR0cHM6Ly93ZWJob29rLW9wZXJhdG9yLmRvbWFpbiJ9XSwibWFpbnRhaW5lcnMiOlt7ImVtYWlsIjoieW91ckBlbWFpbC5jb20iLCJuYW1lIjoiTWFpbnRhaW5lciBOYW1lIn1dLCJtYXR1cml0eSI6ImFscGhhIiwicHJvdmlkZXIiOnsibmFtZSI6IlByb3ZpZGVyIE5hbWUiLCJ1cmwiOiJodHRwczovL3lvdXIuZG9tYWluIn0sInZlcnNpb24iOiIwLjAuMSIsIndlYmhvb2tkZWZpbml0aW9ucyI6W3siYWRtaXNzaW9uUmV2aWV3VmVyc2lvbnMiOlsidjFiZXRhMSIsInYxIl0sImNvbnRhaW5lclBvcnQiOjQ0MywiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6InZ3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiVmFsaWRhdGluZ0FkbWlzc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii92YWxpZGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImRlcGxveW1lbnROYW1lIjoid2ViaG9vay1vcGVyYXRvci13ZWJob29rIiwiZmFpbHVyZVBvbGljeSI6IkZhaWwiLCJnZW5lcmF0ZU5hbWUiOiJtd2ViaG9va3Rlc3Qua2IuaW8iLCJydWxlcyI6W3siYXBpR3JvdXBzIjpbIndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJdLCJhcGlWZXJzaW9ucyI6WyJ2MSJdLCJvcGVyYXRpb25zIjpbIkNSRUFURSIsIlVQREFURSJdLCJyZXNvdXJjZXMiOlsid2ViaG9va3Rlc3RzIl19XSwic2lkZUVmZmVjdHMiOiJOb25lIiwidGFyZ2V0UG9ydCI6NDM0MywidHlwZSI6Ik11dGF0aW5nQWRtaXNzaW9uV2ViaG9vayIsIndlYmhvb2tQYXRoIjoiL211dGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImNvbnZlcnNpb25DUkRzIjpbIndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6ImN3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiQ29udmVyc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii9jb252ZXJ0In1dfX0=
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoiYXBpZXh0ZW5zaW9ucy5rOHMuaW8vdjFiZXRhMSIsImtpbmQiOiJDdXN0b21SZXNvdXJjZURlZmluaXRpb24iLCJtZXRhZGF0YSI6eyJhbm5vdGF0aW9ucyI6eyJjb250cm9sbGVyLWdlbi5rdWJlYnVpbGRlci5pby92ZXJzaW9uIjoidjAuMy4wIn0sImNyZWF0aW9uVGltZXN0YW1wIjpudWxsLCJuYW1lIjoid2ViaG9va3Rlc3RzLndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJ9LCJzcGVjIjp7Imdyb3VwIjoid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIiwibmFtZXMiOnsia2luZCI6IldlYmhvb2tUZXN0IiwibGlzdEtpbmQiOiJXZWJob29rVGVzdExpc3QiLCJwbHVyYWwiOiJ3ZWJob29rdGVzdHMiLCJzaW5ndWxhciI6IndlYmhvb2t0ZXN0In0sInByZXNlcnZlVW5rbm93bkZpZWxkcyI6ZmFsc2UsInNjb3BlIjoiTmFtZXNwYWNlZCIsInZlcnNpb24iOiJ2MSIsInZlcnNpb25zIjpbeyJuYW1lIjoidjEiLCJzY2hlbWEiOnsib3BlbkFQSVYzU2NoZW1hIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3QgaXMgdGhlIFNjaGVtYSBmb3IgdGhlIHdlYmhvb2t0ZXN0cyBBUEkiLCJwcm9wZXJ0aWVzIjp7ImFwaVZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJBUElWZXJzaW9uIGRlZmluZXMgdGhlIHZlcnNpb25lZCBzY2hlbWEgb2YgdGhpcyByZXByZXNlbnRhdGlvbiBvZiBhbiBvYmplY3QuIFNlcnZlcnMgc2hvdWxkIGNvbnZlcnQgcmVjb2duaXplZCBzY2hlbWFzIHRvIHRoZSBsYXRlc3QgaW50ZXJuYWwgdmFsdWUsIGFuZCBtYXkgcmVqZWN0IHVucmVjb2duaXplZCB2YWx1ZXMuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjcmVzb3VyY2VzIiwidHlwZSI6InN0cmluZyJ9LCJraW5kIjp7ImRlc2NyaXB0aW9uIjoiS2luZCBpcyBhIHN0cmluZyB2YWx1ZSByZXByZXNlbnRpbmcgdGhlIFJFU1QgcmVzb3VyY2UgdGhpcyBvYmplY3QgcmVwcmVzZW50cy4gU2VydmVycyBtYXkgaW5mZXIgdGhpcyBmcm9tIHRoZSBlbmRwb2ludCB0aGUgY2xpZW50IHN1Ym1pdHMgcmVxdWVzdHMgdG8uIENhbm5vdCBiZSB1cGRhdGVkLiBJbiBDYW1lbENhc2UuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjdHlwZXMta2luZHMiLCJ0eXBlIjoic3RyaW5nIn0sIm1ldGFkYXRhIjp7InR5cGUiOiJvYmplY3QifSwic3BlYyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3BlYyBkZWZpbmVzIHRoZSBkZXNpcmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwicHJvcGVydGllcyI6eyJtdXRhdGUiOnsiZGVzY3JpcHRpb24iOiJNdXRhdGUgaXMgYSBmaWVsZCB0aGF0IHdpbGwgYmUgc2V0IHRvIHRydWUgYnkgdGhlIG11dGF0aW5nIHdlYmhvb2suIiwidHlwZSI6ImJvb2xlYW4ifSwidmFsaWQiOnsiZGVzY3JpcHRpb24iOiJWYWxpZCBtdXN0IGJlIHNldCB0byB0cnVlIG9yIHRoZSB2YWxpZGF0aW9uIHdlYmhvb2sgd2lsbCByZWplY3QgdGhlIHJlc291cmNlLiIsInR5cGUiOiJib29sZWFuIn19LCJyZXF1aXJlZCI6WyJ2YWxpZCJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjp0cnVlfSx7Im5hbWUiOiJ2MiIsInNjaGVtYSI6eyJvcGVuQVBJVjNTY2hlbWEiOnsiZGVzY3JpcHRpb24iOiJXZWJob29rVGVzdCBpcyB0aGUgU2NoZW1hIGZvciB0aGUgd2ViaG9va3Rlc3RzIEFQSSIsInByb3BlcnRpZXMiOnsiYXBpVmVyc2lvbiI6eyJkZXNjcmlwdGlvbiI6IkFQSVZlcnNpb24gZGVmaW5lcyB0aGUgdmVyc2lvbmVkIHNjaGVtYSBvZiB0aGlzIHJlcHJlc2VudGF0aW9uIG9mIGFuIG9iamVjdC4gU2VydmVycyBzaG91bGQgY29udmVydCByZWNvZ25pemVkIHNjaGVtYXMgdG8gdGhlIGxhdGVzdCBpbnRlcm5hbCB2YWx1ZSwgYW5kIG1heSByZWplY3QgdW5yZWNvZ25pemVkIHZhbHVlcy4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCNyZXNvdXJjZXMiLCJ0eXBlIjoic3RyaW5nIn0sImtpbmQiOnsiZGVzY3JpcHRpb24iOiJLaW5kIGlzIGEgc3RyaW5nIHZhbHVlIHJlcHJlc2VudGluZyB0aGUgUkVTVCByZXNvdXJjZSB0aGlzIG9iamVjdCByZXByZXNlbnRzLiBTZXJ2ZXJzIG1heSBpbmZlciB0aGlzIGZyb20gdGhlIGVuZHBvaW50IHRoZSBjbGllbnQgc3VibWl0cyByZXF1ZXN0cyB0by4gQ2Fubm90IGJlIHVwZGF0ZWQuIEluIENhbWVsQ2FzZS4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCN0eXBlcy1raW5kcyIsInR5cGUiOiJzdHJpbmcifSwibWV0YWRhdGEiOnsidHlwZSI6Im9iamVjdCJ9LCJzcGVjIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3RTcGVjIGRlZmluZXMgdGhlIGRlc2lyZWQgc3RhdGUgb2YgV2ViaG9va1Rlc3QiLCJwcm9wZXJ0aWVzIjp7ImNvbnZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJDb252ZXJzaW9uIGlzIGFuIGV4YW1wbGUgZmllbGQgb2YgV2ViaG9va1Rlc3QuIEVkaXQgV2ViaG9va1Rlc3RfdHlwZXMuZ28gdG8gcmVtb3ZlL3VwZGF0ZSIsInByb3BlcnRpZXMiOnsibXV0YXRlIjp7ImRlc2NyaXB0aW9uIjoiTXV0YXRlIGlzIGEgZmllbGQgdGhhdCB3aWxsIGJlIHNldCB0byB0cnVlIGJ5IHRoZSBtdXRhdGluZyB3ZWJob29rLiIsInR5cGUiOiJib29sZWFuIn0sInZhbGlkIjp7ImRlc2NyaXB0aW9uIjoiVmFsaWQgbXVzdCBiZSBzZXQgdG8gdHJ1ZSBvciB0aGUgdmFsaWRhdGlvbiB3ZWJob29rIHdpbGwgcmVqZWN0IHRoZSByZXNvdXJjZS4iLCJ0eXBlIjoiYm9vbGVhbiJ9fSwicmVxdWlyZWQiOlsidmFsaWQiXSwidHlwZSI6Im9iamVjdCJ9fSwicmVxdWlyZWQiOlsiY29udmVyc2lvbiJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjpmYWxzZX1dfSwic3RhdHVzIjp7ImFjY2VwdGVkTmFtZXMiOnsia2luZCI6IiIsInBsdXJhbCI6IiJ9LCJjb25kaXRpb25zIjpbXSwic3RvcmVkVmVyc2lvbnMiOltdfX0=
relatedImages:
- image: gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0
  name: ""
- image: quay.io/olmtest/webhook-operator-bundle:0.0.3
  name: ""
- image: quay.io/olmtest/webhook-operator:0.0.3
  name: ""
schema: olm.bundle
`

const customBuiltFbcJson = `{
    "schema": "olm.package",
    "name": "webhook-operator-413",
    "defaultChannel": "preview"
}
{
    "schema": "olm.channel",
    "name": "preview",
    "package": "webhook-operator-413",
    "entries": [
        {
            "name": "webhook-operator.v0.0.1"
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "webhook-operator.v0.0.1",
    "package": "webhook-operator-413",
    "image": "quay.io/olmtest/webhook-operator-bundle:0.0.3",
    "properties": [
        {
            "type": "olm.gvk",
            "value": {
                "group": "webhook.operators.coreos.io",
                "kind": "WebhookTest",
                "version": "v1"
            }
        },
        {
            "type": "olm.gvk",
            "value": {
                "group": "webhook.operators.coreos.io",
                "kind": "WebhookTest",
                "version": "v2"
            }
        },
        {
            "type": "olm.package",
            "value": {
                "packageName": "webhook-operator-413",
                "version": "0.0.1"
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJhcGlWZXJzaW9uIjoicmJhYy5hdXRob3JpemF0aW9uLms4cy5pby92MWJldGExIiwia2luZCI6IkNsdXN0ZXJSb2xlIiwibWV0YWRhdGEiOnsiY3JlYXRpb25UaW1lc3RhbXAiOm51bGwsIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLW1ldHJpY3MtcmVhZGVyIn0sInJ1bGVzIjpbeyJub25SZXNvdXJjZVVSTHMiOlsiL21ldHJpY3MiXSwidmVyYnMiOlsiZ2V0Il19XX0="
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJhcGlWZXJzaW9uIjoib3BlcmF0b3JzLmNvcmVvcy5jb20vdjFhbHBoYTEiLCJraW5kIjoiQ2x1c3RlclNlcnZpY2VWZXJzaW9uIiwibWV0YWRhdGEiOnsiYW5ub3RhdGlvbnMiOnsiYWxtLWV4YW1wbGVzIjoiW1xuICB7XG4gICAgXCJhcGlWZXJzaW9uXCI6IFwid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvL3YxXCIsXG4gICAgXCJraW5kXCI6IFwiV2ViaG9va1Rlc3RcIixcbiAgICBcIm1ldGFkYXRhXCI6IHtcbiAgICAgIFwibmFtZVwiOiBcIndlYmhvb2t0ZXN0LXNhbXBsZVwiLFxuICAgICAgXCJuYW1lc3BhY2VcIjogXCJ3ZWJob29rLW9wZXJhdG9yLXN5c3RlbVwiXG4gICAgfSxcbiAgICBcInNwZWNcIjoge1xuICAgICAgXCJ2YWxpZFwiOiB0cnVlXG4gICAgfVxuICB9XG5dIiwiY2FwYWJpbGl0aWVzIjoiQmFzaWMgSW5zdGFsbCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9idWlsZGVyIjoib3BlcmF0b3Itc2RrLXYxLjAuMCIsIm9wZXJhdG9ycy5vcGVyYXRvcmZyYW1ld29yay5pby9wcm9qZWN0X2xheW91dCI6ImdvIn0sIm5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLnYwLjAuMSIsIm5hbWVzcGFjZSI6InBsYWNlaG9sZGVyIn0sInNwZWMiOnsiYXBpc2VydmljZWRlZmluaXRpb25zIjp7fSwiY3VzdG9tcmVzb3VyY2VkZWZpbml0aW9ucyI6eyJvd25lZCI6W3sia2luZCI6IldlYmhvb2tUZXN0IiwibmFtZSI6IndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iLCJ2ZXJzaW9uIjoidjEifV19LCJkZXNjcmlwdGlvbiI6IldlYmhvb2sgT3BlcmF0b3IgZGVzY3JpcHRpb24uIFRPRE8uIiwiZGlzcGxheU5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwiaWNvbiI6W3siYmFzZTY0ZGF0YSI6IiIsIm1lZGlhdHlwZSI6IiJ9XSwiaW5zdGFsbCI6eyJzcGVjIjp7ImNsdXN0ZXJQZXJtaXNzaW9ucyI6W3sicnVsZXMiOlt7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cyJdLCJ2ZXJicyI6WyJjcmVhdGUiLCJkZWxldGUiLCJnZXQiLCJsaXN0IiwicGF0Y2giLCJ1cGRhdGUiLCJ3YXRjaCJdfSx7ImFwaUdyb3VwcyI6WyJ3ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwicmVzb3VyY2VzIjpbIndlYmhvb2t0ZXN0cy9zdGF0dXMiXSwidmVyYnMiOlsiZ2V0IiwicGF0Y2giLCJ1cGRhdGUiXX0seyJhcGlHcm91cHMiOlsiYXV0aGVudGljYXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJ0b2tlbnJldmlld3MiXSwidmVyYnMiOlsiY3JlYXRlIl19LHsiYXBpR3JvdXBzIjpbImF1dGhvcml6YXRpb24uazhzLmlvIl0sInJlc291cmNlcyI6WyJzdWJqZWN0YWNjZXNzcmV2aWV3cyJdLCJ2ZXJicyI6WyJjcmVhdGUiXX1dLCJzZXJ2aWNlQWNjb3VudE5hbWUiOiJkZWZhdWx0In1dLCJkZXBsb3ltZW50cyI6W3sibmFtZSI6IndlYmhvb2stb3BlcmF0b3Itd2ViaG9vayIsInNwZWMiOnsicmVwbGljYXMiOjEsInNlbGVjdG9yIjp7Im1hdGNoTGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInN0cmF0ZWd5Ijp7fSwidGVtcGxhdGUiOnsibWV0YWRhdGEiOnsibGFiZWxzIjp7ImNvbnRyb2wtcGxhbmUiOiJjb250cm9sbGVyLW1hbmFnZXIifX0sInNwZWMiOnsiY29udGFpbmVycyI6W3siYXJncyI6WyItLXNlY3VyZS1saXN0ZW4tYWRkcmVzcz0wLjAuMC4wOjg0NDMiLCItLXVwc3RyZWFtPWh0dHA6Ly8xMjcuMC4wLjE6ODA4MC8iLCItLWxvZ3Rvc3RkZXJyPXRydWUiLCItLXY9MTAiXSwiaW1hZ2UiOiJnY3IuaW8va3ViZWJ1aWxkZXIva3ViZS1yYmFjLXByb3h5OnYwLjUuMCIsIm5hbWUiOiJrdWJlLXJiYWMtcHJveHkiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6ODQ0MywibmFtZSI6Imh0dHBzIn1dLCJyZXNvdXJjZXMiOnt9fSx7ImFyZ3MiOlsiLS1tZXRyaWNzLWFkZHI9MTI3LjAuMC4xOjgwODAiLCItLWVuYWJsZS1sZWFkZXItZWxlY3Rpb24iXSwiY29tbWFuZCI6WyIvbWFuYWdlciJdLCJpbWFnZSI6InF1YXkuaW8vb2xtdGVzdC93ZWJob29rLW9wZXJhdG9yOjAuMC4zIiwibmFtZSI6Im1hbmFnZXIiLCJwb3J0cyI6W3siY29udGFpbmVyUG9ydCI6OTQ0MywibmFtZSI6IndlYmhvb2stc2VydmVyIiwicHJvdG9jb2wiOiJUQ1AifV0sInJlc291cmNlcyI6eyJsaW1pdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjMwTWkifSwicmVxdWVzdHMiOnsiY3B1IjoiMTAwbSIsIm1lbW9yeSI6IjIwTWkifX19XSwidGVybWluYXRpb25HcmFjZVBlcmlvZFNlY29uZHMiOjEwfX19fV0sInBlcm1pc3Npb25zIjpbeyJydWxlcyI6W3siYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiY29uZmlnbWFwcyJdLCJ2ZXJicyI6WyJnZXQiLCJsaXN0Iiwid2F0Y2giLCJjcmVhdGUiLCJ1cGRhdGUiLCJwYXRjaCIsImRlbGV0ZSJdfSx7ImFwaUdyb3VwcyI6WyIiXSwicmVzb3VyY2VzIjpbImNvbmZpZ21hcHMvc3RhdHVzIl0sInZlcmJzIjpbImdldCIsInVwZGF0ZSIsInBhdGNoIl19LHsiYXBpR3JvdXBzIjpbIiJdLCJyZXNvdXJjZXMiOlsiZXZlbnRzIl0sInZlcmJzIjpbImNyZWF0ZSJdfV0sInNlcnZpY2VBY2NvdW50TmFtZSI6ImRlZmF1bHQifV19LCJzdHJhdGVneSI6ImRlcGxveW1lbnQifSwiaW5zdGFsbE1vZGVzIjpbeyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiT3duTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiU2luZ2xlTmFtZXNwYWNlIn0seyJzdXBwb3J0ZWQiOmZhbHNlLCJ0eXBlIjoiTXVsdGlOYW1lc3BhY2UifSx7InN1cHBvcnRlZCI6dHJ1ZSwidHlwZSI6IkFsbE5hbWVzcGFjZXMifV0sImtleXdvcmRzIjpbIndlYmhvb2stb3BlcmF0b3IiXSwibGlua3MiOlt7Im5hbWUiOiJXZWJob29rIE9wZXJhdG9yIiwidXJsIjoiaHR0cHM6Ly93ZWJob29rLW9wZXJhdG9yLmRvbWFpbiJ9XSwibWFpbnRhaW5lcnMiOlt7ImVtYWlsIjoieW91ckBlbWFpbC5jb20iLCJuYW1lIjoiTWFpbnRhaW5lciBOYW1lIn1dLCJtYXR1cml0eSI6ImFscGhhIiwicHJvdmlkZXIiOnsibmFtZSI6IlByb3ZpZGVyIE5hbWUiLCJ1cmwiOiJodHRwczovL3lvdXIuZG9tYWluIn0sInZlcnNpb24iOiIwLjAuMSIsIndlYmhvb2tkZWZpbml0aW9ucyI6W3siYWRtaXNzaW9uUmV2aWV3VmVyc2lvbnMiOlsidjFiZXRhMSIsInYxIl0sImNvbnRhaW5lclBvcnQiOjQ0MywiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6InZ3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiVmFsaWRhdGluZ0FkbWlzc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii92YWxpZGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImRlcGxveW1lbnROYW1lIjoid2ViaG9vay1vcGVyYXRvci13ZWJob29rIiwiZmFpbHVyZVBvbGljeSI6IkZhaWwiLCJnZW5lcmF0ZU5hbWUiOiJtd2ViaG9va3Rlc3Qua2IuaW8iLCJydWxlcyI6W3siYXBpR3JvdXBzIjpbIndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJdLCJhcGlWZXJzaW9ucyI6WyJ2MSJdLCJvcGVyYXRpb25zIjpbIkNSRUFURSIsIlVQREFURSJdLCJyZXNvdXJjZXMiOlsid2ViaG9va3Rlc3RzIl19XSwic2lkZUVmZmVjdHMiOiJOb25lIiwidGFyZ2V0UG9ydCI6NDM0MywidHlwZSI6Ik11dGF0aW5nQWRtaXNzaW9uV2ViaG9vayIsIndlYmhvb2tQYXRoIjoiL211dGF0ZS13ZWJob29rLW9wZXJhdG9ycy1jb3Jlb3MtaW8tdjEtd2ViaG9va3Rlc3QifSx7ImFkbWlzc2lvblJldmlld1ZlcnNpb25zIjpbInYxYmV0YTEiLCJ2MSJdLCJjb250YWluZXJQb3J0Ijo0NDMsImNvbnZlcnNpb25DUkRzIjpbIndlYmhvb2t0ZXN0cy53ZWJob29rLm9wZXJhdG9ycy5jb3Jlb3MuaW8iXSwiZGVwbG95bWVudE5hbWUiOiJ3ZWJob29rLW9wZXJhdG9yLXdlYmhvb2siLCJmYWlsdXJlUG9saWN5IjoiRmFpbCIsImdlbmVyYXRlTmFtZSI6ImN3ZWJob29rdGVzdC5rYi5pbyIsInJ1bGVzIjpbeyJhcGlHcm91cHMiOlsid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIl0sImFwaVZlcnNpb25zIjpbInYxIl0sIm9wZXJhdGlvbnMiOlsiQ1JFQVRFIiwiVVBEQVRFIl0sInJlc291cmNlcyI6WyJ3ZWJob29rdGVzdHMiXX1dLCJzaWRlRWZmZWN0cyI6Ik5vbmUiLCJ0YXJnZXRQb3J0Ijo0MzQzLCJ0eXBlIjoiQ29udmVyc2lvbldlYmhvb2siLCJ3ZWJob29rUGF0aCI6Ii9jb252ZXJ0In1dfX0="
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJhcGlWZXJzaW9uIjoiYXBpZXh0ZW5zaW9ucy5rOHMuaW8vdjFiZXRhMSIsImtpbmQiOiJDdXN0b21SZXNvdXJjZURlZmluaXRpb24iLCJtZXRhZGF0YSI6eyJhbm5vdGF0aW9ucyI6eyJjb250cm9sbGVyLWdlbi5rdWJlYnVpbGRlci5pby92ZXJzaW9uIjoidjAuMy4wIn0sImNyZWF0aW9uVGltZXN0YW1wIjpudWxsLCJuYW1lIjoid2ViaG9va3Rlc3RzLndlYmhvb2sub3BlcmF0b3JzLmNvcmVvcy5pbyJ9LCJzcGVjIjp7Imdyb3VwIjoid2ViaG9vay5vcGVyYXRvcnMuY29yZW9zLmlvIiwibmFtZXMiOnsia2luZCI6IldlYmhvb2tUZXN0IiwibGlzdEtpbmQiOiJXZWJob29rVGVzdExpc3QiLCJwbHVyYWwiOiJ3ZWJob29rdGVzdHMiLCJzaW5ndWxhciI6IndlYmhvb2t0ZXN0In0sInByZXNlcnZlVW5rbm93bkZpZWxkcyI6ZmFsc2UsInNjb3BlIjoiTmFtZXNwYWNlZCIsInZlcnNpb24iOiJ2MSIsInZlcnNpb25zIjpbeyJuYW1lIjoidjEiLCJzY2hlbWEiOnsib3BlbkFQSVYzU2NoZW1hIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3QgaXMgdGhlIFNjaGVtYSBmb3IgdGhlIHdlYmhvb2t0ZXN0cyBBUEkiLCJwcm9wZXJ0aWVzIjp7ImFwaVZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJBUElWZXJzaW9uIGRlZmluZXMgdGhlIHZlcnNpb25lZCBzY2hlbWEgb2YgdGhpcyByZXByZXNlbnRhdGlvbiBvZiBhbiBvYmplY3QuIFNlcnZlcnMgc2hvdWxkIGNvbnZlcnQgcmVjb2duaXplZCBzY2hlbWFzIHRvIHRoZSBsYXRlc3QgaW50ZXJuYWwgdmFsdWUsIGFuZCBtYXkgcmVqZWN0IHVucmVjb2duaXplZCB2YWx1ZXMuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjcmVzb3VyY2VzIiwidHlwZSI6InN0cmluZyJ9LCJraW5kIjp7ImRlc2NyaXB0aW9uIjoiS2luZCBpcyBhIHN0cmluZyB2YWx1ZSByZXByZXNlbnRpbmcgdGhlIFJFU1QgcmVzb3VyY2UgdGhpcyBvYmplY3QgcmVwcmVzZW50cy4gU2VydmVycyBtYXkgaW5mZXIgdGhpcyBmcm9tIHRoZSBlbmRwb2ludCB0aGUgY2xpZW50IHN1Ym1pdHMgcmVxdWVzdHMgdG8uIENhbm5vdCBiZSB1cGRhdGVkLiBJbiBDYW1lbENhc2UuIE1vcmUgaW5mbzogaHR0cHM6Ly9naXQuazhzLmlvL2NvbW11bml0eS9jb250cmlidXRvcnMvZGV2ZWwvc2lnLWFyY2hpdGVjdHVyZS9hcGktY29udmVudGlvbnMubWQjdHlwZXMta2luZHMiLCJ0eXBlIjoic3RyaW5nIn0sIm1ldGFkYXRhIjp7InR5cGUiOiJvYmplY3QifSwic3BlYyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3BlYyBkZWZpbmVzIHRoZSBkZXNpcmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwicHJvcGVydGllcyI6eyJtdXRhdGUiOnsiZGVzY3JpcHRpb24iOiJNdXRhdGUgaXMgYSBmaWVsZCB0aGF0IHdpbGwgYmUgc2V0IHRvIHRydWUgYnkgdGhlIG11dGF0aW5nIHdlYmhvb2suIiwidHlwZSI6ImJvb2xlYW4ifSwidmFsaWQiOnsiZGVzY3JpcHRpb24iOiJWYWxpZCBtdXN0IGJlIHNldCB0byB0cnVlIG9yIHRoZSB2YWxpZGF0aW9uIHdlYmhvb2sgd2lsbCByZWplY3QgdGhlIHJlc291cmNlLiIsInR5cGUiOiJib29sZWFuIn19LCJyZXF1aXJlZCI6WyJ2YWxpZCJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjp0cnVlfSx7Im5hbWUiOiJ2MiIsInNjaGVtYSI6eyJvcGVuQVBJVjNTY2hlbWEiOnsiZGVzY3JpcHRpb24iOiJXZWJob29rVGVzdCBpcyB0aGUgU2NoZW1hIGZvciB0aGUgd2ViaG9va3Rlc3RzIEFQSSIsInByb3BlcnRpZXMiOnsiYXBpVmVyc2lvbiI6eyJkZXNjcmlwdGlvbiI6IkFQSVZlcnNpb24gZGVmaW5lcyB0aGUgdmVyc2lvbmVkIHNjaGVtYSBvZiB0aGlzIHJlcHJlc2VudGF0aW9uIG9mIGFuIG9iamVjdC4gU2VydmVycyBzaG91bGQgY29udmVydCByZWNvZ25pemVkIHNjaGVtYXMgdG8gdGhlIGxhdGVzdCBpbnRlcm5hbCB2YWx1ZSwgYW5kIG1heSByZWplY3QgdW5yZWNvZ25pemVkIHZhbHVlcy4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCNyZXNvdXJjZXMiLCJ0eXBlIjoic3RyaW5nIn0sImtpbmQiOnsiZGVzY3JpcHRpb24iOiJLaW5kIGlzIGEgc3RyaW5nIHZhbHVlIHJlcHJlc2VudGluZyB0aGUgUkVTVCByZXNvdXJjZSB0aGlzIG9iamVjdCByZXByZXNlbnRzLiBTZXJ2ZXJzIG1heSBpbmZlciB0aGlzIGZyb20gdGhlIGVuZHBvaW50IHRoZSBjbGllbnQgc3VibWl0cyByZXF1ZXN0cyB0by4gQ2Fubm90IGJlIHVwZGF0ZWQuIEluIENhbWVsQ2FzZS4gTW9yZSBpbmZvOiBodHRwczovL2dpdC5rOHMuaW8vY29tbXVuaXR5L2NvbnRyaWJ1dG9ycy9kZXZlbC9zaWctYXJjaGl0ZWN0dXJlL2FwaS1jb252ZW50aW9ucy5tZCN0eXBlcy1raW5kcyIsInR5cGUiOiJzdHJpbmcifSwibWV0YWRhdGEiOnsidHlwZSI6Im9iamVjdCJ9LCJzcGVjIjp7ImRlc2NyaXB0aW9uIjoiV2ViaG9va1Rlc3RTcGVjIGRlZmluZXMgdGhlIGRlc2lyZWQgc3RhdGUgb2YgV2ViaG9va1Rlc3QiLCJwcm9wZXJ0aWVzIjp7ImNvbnZlcnNpb24iOnsiZGVzY3JpcHRpb24iOiJDb252ZXJzaW9uIGlzIGFuIGV4YW1wbGUgZmllbGQgb2YgV2ViaG9va1Rlc3QuIEVkaXQgV2ViaG9va1Rlc3RfdHlwZXMuZ28gdG8gcmVtb3ZlL3VwZGF0ZSIsInByb3BlcnRpZXMiOnsibXV0YXRlIjp7ImRlc2NyaXB0aW9uIjoiTXV0YXRlIGlzIGEgZmllbGQgdGhhdCB3aWxsIGJlIHNldCB0byB0cnVlIGJ5IHRoZSBtdXRhdGluZyB3ZWJob29rLiIsInR5cGUiOiJib29sZWFuIn0sInZhbGlkIjp7ImRlc2NyaXB0aW9uIjoiVmFsaWQgbXVzdCBiZSBzZXQgdG8gdHJ1ZSBvciB0aGUgdmFsaWRhdGlvbiB3ZWJob29rIHdpbGwgcmVqZWN0IHRoZSByZXNvdXJjZS4iLCJ0eXBlIjoiYm9vbGVhbiJ9fSwicmVxdWlyZWQiOlsidmFsaWQiXSwidHlwZSI6Im9iamVjdCJ9fSwicmVxdWlyZWQiOlsiY29udmVyc2lvbiJdLCJ0eXBlIjoib2JqZWN0In0sInN0YXR1cyI6eyJkZXNjcmlwdGlvbiI6IldlYmhvb2tUZXN0U3RhdHVzIGRlZmluZXMgdGhlIG9ic2VydmVkIHN0YXRlIG9mIFdlYmhvb2tUZXN0IiwidHlwZSI6Im9iamVjdCJ9fSwidHlwZSI6Im9iamVjdCJ9fSwic2VydmVkIjp0cnVlLCJzdG9yYWdlIjpmYWxzZX1dfSwic3RhdHVzIjp7ImFjY2VwdGVkTmFtZXMiOnsia2luZCI6IiIsInBsdXJhbCI6IiJ9LCJjb25kaXRpb25zIjpbXSwic3RvcmVkVmVyc2lvbnMiOltdfX0="
            }
        }
    ],
    "relatedImages": [
        {
            "name": "",
            "image": "gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0"
        },
        {
            "name": "",
            "image": "quay.io/olmtest/webhook-operator-bundle:0.0.3"
        },
        {
            "name": "",
            "image": "quay.io/olmtest/webhook-operator:0.0.3"
        }
    ]
}
`

func TestValidateFailure(t *testing.T) {
	err := validate(BuilderConfig{}, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no such file or directory")
}
