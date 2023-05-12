package action_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
)

func TestMigrate(t *testing.T) {
	type spec struct {
		name          string
		migrate       action.Migrate
		expectedFiles map[string]string
		expectErr     error
	}

	sqliteBundles := map[image.Reference]string{
		image.SimpleReference("test.registry/foo-operator/foo-bundle:v0.1.0"): "testdata/foo-bundle-v0.1.0",
		image.SimpleReference("test.registry/foo-operator/foo-bundle:v0.2.0"): "testdata/foo-bundle-v0.2.0",
		image.SimpleReference("test.registry/bar-operator/bar-bundle:v0.1.0"): "testdata/bar-bundle-v0.1.0",
		image.SimpleReference("test.registry/bar-operator/bar-bundle:v0.2.0"): "testdata/bar-bundle-v0.2.0",
	}

	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "index.db")
	err := generateSqliteFile(dbFile, sqliteBundles)
	require.NoError(t, err)

	reg, err := newMigrateRegistry(t, sqliteBundles)
	require.NoError(t, err)

	specs := []spec{
		{
			name: "SqliteImage/Success",
			migrate: action.Migrate{
				CatalogRef: "test.registry/migrate/catalog:sqlite",
				OutputDir:  filepath.Join(tmpDir, "sqlite-image"),
				WriteFunc:  declcfg.WriteYAML,
				FileExt:    ".yaml",
				Registry:   reg,
			},
			expectedFiles: map[string]string{
				"foo/catalog.yaml": migrateFooCatalog(),
				"bar/catalog.yaml": migrateBarCatalog(),
			},
		},
		{
			name: "SqliteFile/Success",
			migrate: action.Migrate{
				CatalogRef: dbFile,
				OutputDir:  filepath.Join(tmpDir, "sqlite-file"),
				WriteFunc:  declcfg.WriteYAML,
				FileExt:    ".yaml",
				Registry:   reg,
			},
			expectedFiles: map[string]string{
				"foo/catalog.yaml": migrateFooCatalog(),
				"bar/catalog.yaml": migrateBarCatalog(),
			},
		},
		{
			name: "DeclcfgImage/Failure",
			migrate: action.Migrate{
				CatalogRef: "test.registry/foo-operator/foo-index-declcfg:v0.2.0",
				OutputDir:  filepath.Join(tmpDir, "declcfg-image"),
				WriteFunc:  declcfg.WriteYAML,
				FileExt:    ".yaml",
				Registry:   reg,
			},
			expectErr: action.ErrNotAllowed,
		},
		{
			name: "DeclcfgDir/Failure",
			migrate: action.Migrate{
				CatalogRef: "testdata/foo-index-v0.2.0-declcfg",
				OutputDir:  filepath.Join(tmpDir, "declcfg-dir"),
				WriteFunc:  declcfg.WriteYAML,
				FileExt:    ".yaml",
				Registry:   reg,
			},
			expectErr: action.ErrNotAllowed,
		},
		{
			name: "BundleImage/Failure",
			migrate: action.Migrate{
				CatalogRef: "test.registry/foo-operator/foo-bundle:v0.1.0",
				OutputDir:  filepath.Join(tmpDir, "bundle-image"),
				WriteFunc:  declcfg.WriteYAML,
				FileExt:    ".yaml",
				Registry:   reg,
			},
			expectErr: action.ErrNotAllowed,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			err := s.migrate.Run(context.Background())
			require.ErrorIs(t, err, s.expectErr)
			for file, expectedData := range s.expectedFiles {
				path := filepath.Join(s.migrate.OutputDir, file)
				actualData, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, expectedData, string(actualData))
			}
		})
	}
}

func newMigrateRegistry(t *testing.T, imageMap map[image.Reference]string) (image.Registry, error) {
	subSqliteImage, err := generateSqliteFS(t, imageMap)
	if err != nil {
		return nil, err
	}

	subDeclcfgImage, err := fs.Sub(declcfgImage, "testdata/foo-index-v0.2.0-declcfg")
	if err != nil {
		return nil, err
	}

	subBundleImageV1, err := fs.Sub(bundleImageV1, "testdata/foo-bundle-v0.1.0")
	if err != nil {
		return nil, err
	}

	reg := &image.MockRegistry{RemoteImages: map[image.Reference]*image.MockImage{
		image.SimpleReference("test.registry/migrate/catalog:sqlite"): {
			Labels: map[string]string{
				containertools.DbLocationLabel: "/database/index.db",
			},
			FS: subSqliteImage,
		},
		image.SimpleReference("test.registry/foo-operator/foo-index-declcfg:v0.2.0"): {
			Labels: map[string]string{
				"operators.operatorframework.io.index.configs.v1": "/foo",
			},
			FS: subDeclcfgImage,
		},
		image.SimpleReference("test.registry/foo-operator/foo-bundle:v0.1.0"): {
			Labels: map[string]string{
				bundle.PackageLabel: "foo",
			},
			FS: subBundleImageV1,
		},
	}}

	return reg, nil
}

func migrateFooCatalog() string {
	return `---
defaultChannel: beta
name: foo
schema: olm.package
---
entries:
- name: foo.v0.1.0
  skipRange: <0.1.0
- name: foo.v0.2.0
  replaces: foo.v0.1.0
  skipRange: <0.2.0
  skips:
  - foo.v0.1.1
  - foo.v0.1.2
name: beta
package: foo
schema: olm.channel
---
entries:
- name: foo.v0.1.0
  skipRange: <0.1.0
- name: foo.v0.2.0
  replaces: foo.v0.1.0
  skipRange: <0.2.0
  skips:
  - foo.v0.1.1
  - foo.v0.1.2
name: stable
package: foo
schema: olm.channel
---
image: test.registry/foo-operator/foo-bundle:v0.1.0
name: foo.v0.1.0
package: foo
properties:
- type: olm.gvk
  value:
    group: test.foo
    kind: Foo
    version: v1
- type: olm.gvk.required
  value:
    group: test.bar
    kind: Bar
    version: v1alpha1
- type: olm.package
  value:
    packageName: foo
    version: 0.1.0
- type: olm.package.required
  value:
    packageName: bar
    versionRange: <0.1.0
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoiYXBpZXh0ZW5zaW9ucy5rOHMuaW8vdjEiLCJraW5kIjoiQ3VzdG9tUmVzb3VyY2VEZWZpbml0aW9uIiwibWV0YWRhdGEiOnsibmFtZSI6ImZvb3MudGVzdC5mb28ifSwic3BlYyI6eyJncm91cCI6InRlc3QuZm9vIiwibmFtZXMiOnsia2luZCI6IkZvbyIsInBsdXJhbCI6ImZvb3MifSwidmVyc2lvbnMiOlt7Im5hbWUiOiJ2MSJ9XX19
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoib3BlcmF0b3JzLmNvcmVvcy5jb20vdjFhbHBoYTEiLCJraW5kIjoiQ2x1c3RlclNlcnZpY2VWZXJzaW9uIiwibWV0YWRhdGEiOnsiYW5ub3RhdGlvbnMiOnsib2xtLnNraXBSYW5nZSI6Ilx1MDAzYzAuMS4wIn0sIm5hbWUiOiJmb28udjAuMS4wIn0sInNwZWMiOnsiY3VzdG9tcmVzb3VyY2VkZWZpbml0aW9ucyI6eyJvd25lZCI6W3siZ3JvdXAiOiJ0ZXN0LmZvbyIsImtpbmQiOiJGb28iLCJuYW1lIjoiZm9vcy50ZXN0LmZvbyIsInZlcnNpb24iOiJ2MSJ9XX0sImRpc3BsYXlOYW1lIjoiRm9vIE9wZXJhdG9yIiwicmVsYXRlZEltYWdlcyI6W3siaW1hZ2UiOiJ0ZXN0LnJlZ2lzdHJ5L2Zvby1vcGVyYXRvci9mb286djAuMS4wIiwibmFtZSI6Im9wZXJhdG9yIn1dLCJ2ZXJzaW9uIjoiMC4xLjAifX0=
relatedImages:
- image: test.registry/foo-operator/foo-bundle:v0.1.0
  name: ""
- image: test.registry/foo-operator/foo:v0.1.0
  name: operator
schema: olm.bundle
---
image: test.registry/foo-operator/foo-bundle:v0.2.0
name: foo.v0.2.0
package: foo
properties:
- type: olm.gvk
  value:
    group: test.foo
    kind: Foo
    version: v1
- type: olm.gvk.required
  value:
    group: test.bar
    kind: Bar
    version: v1alpha1
- type: olm.package
  value:
    packageName: foo
    version: 0.2.0
- type: olm.package.required
  value:
    packageName: bar
    versionRange: <0.1.0
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoiYXBpZXh0ZW5zaW9ucy5rOHMuaW8vdjEiLCJraW5kIjoiQ3VzdG9tUmVzb3VyY2VEZWZpbml0aW9uIiwibWV0YWRhdGEiOnsibmFtZSI6ImZvb3MudGVzdC5mb28ifSwic3BlYyI6eyJncm91cCI6InRlc3QuZm9vIiwibmFtZXMiOnsia2luZCI6IkZvbyIsInBsdXJhbCI6ImZvb3MifSwidmVyc2lvbnMiOlt7Im5hbWUiOiJ2MSJ9XX19
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoib3BlcmF0b3JzLmNvcmVvcy5jb20vdjFhbHBoYTEiLCJraW5kIjoiQ2x1c3RlclNlcnZpY2VWZXJzaW9uIiwibWV0YWRhdGEiOnsiYW5ub3RhdGlvbnMiOnsib2xtLnNraXBSYW5nZSI6Ilx1MDAzYzAuMi4wIn0sIm5hbWUiOiJmb28udjAuMi4wIn0sInNwZWMiOnsiY3VzdG9tcmVzb3VyY2VkZWZpbml0aW9ucyI6eyJvd25lZCI6W3siZ3JvdXAiOiJ0ZXN0LmZvbyIsImtpbmQiOiJGb28iLCJuYW1lIjoiZm9vcy50ZXN0LmZvbyIsInZlcnNpb24iOiJ2MSJ9XX0sImRpc3BsYXlOYW1lIjoiRm9vIE9wZXJhdG9yIiwiaW5zdGFsbCI6eyJzcGVjIjp7ImRlcGxveW1lbnRzIjpbeyJuYW1lIjoiZm9vLW9wZXJhdG9yIiwic3BlYyI6eyJ0ZW1wbGF0ZSI6eyJzcGVjIjp7ImNvbnRhaW5lcnMiOlt7ImltYWdlIjoidGVzdC5yZWdpc3RyeS9mb28tb3BlcmF0b3IvZm9vOnYwLjIuMCJ9XSwiaW5pdENvbnRhaW5lcnMiOlt7ImltYWdlIjoidGVzdC5yZWdpc3RyeS9mb28tb3BlcmF0b3IvZm9vLWluaXQ6djAuMi4wIn1dfX19fSx7Im5hbWUiOiJmb28tb3BlcmF0b3ItMiIsInNwZWMiOnsidGVtcGxhdGUiOnsic3BlYyI6eyJjb250YWluZXJzIjpbeyJpbWFnZSI6InRlc3QucmVnaXN0cnkvZm9vLW9wZXJhdG9yL2Zvby0yOnYwLjIuMCJ9XSwiaW5pdENvbnRhaW5lcnMiOlt7ImltYWdlIjoidGVzdC5yZWdpc3RyeS9mb28tb3BlcmF0b3IvZm9vLWluaXQtMjp2MC4yLjAifV19fX19XX0sInN0cmF0ZWd5IjoiZGVwbG95bWVudCJ9LCJyZWxhdGVkSW1hZ2VzIjpbeyJpbWFnZSI6InRlc3QucmVnaXN0cnkvZm9vLW9wZXJhdG9yL2Zvbzp2MC4yLjAiLCJuYW1lIjoib3BlcmF0b3IifSx7ImltYWdlIjoidGVzdC5yZWdpc3RyeS9mb28tb3BlcmF0b3IvZm9vLW90aGVyOnYwLjIuMCIsIm5hbWUiOiJvdGhlciJ9XSwicmVwbGFjZXMiOiJmb28udjAuMS4wIiwic2tpcHMiOlsiZm9vLnYwLjEuMSIsImZvby52MC4xLjIiXSwidmVyc2lvbiI6IjAuMi4wIn19
relatedImages:
- image: test.registry/foo-operator/foo-2:v0.2.0
  name: ""
- image: test.registry/foo-operator/foo-bundle:v0.2.0
  name: ""
- image: test.registry/foo-operator/foo-init-2:v0.2.0
  name: ""
- image: test.registry/foo-operator/foo-init:v0.2.0
  name: ""
- image: test.registry/foo-operator/foo-other:v0.2.0
  name: other
- image: test.registry/foo-operator/foo:v0.2.0
  name: operator
schema: olm.bundle
`
}

func migrateBarCatalog() string {
	return `---
defaultChannel: alpha
name: bar
schema: olm.package
---
entries:
- name: bar.v0.1.0
- name: bar.v0.2.0
  skipRange: <0.2.0
  skips:
  - bar.v0.1.0
name: alpha
package: bar
schema: olm.channel
---
image: test.registry/bar-operator/bar-bundle:v0.1.0
name: bar.v0.1.0
package: bar
properties:
- type: olm.gvk
  value:
    group: test.bar
    kind: Bar
    version: v1alpha1
- type: olm.package
  value:
    packageName: bar
    version: 0.1.0
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoiYXBpZXh0ZW5zaW9ucy5rOHMuaW8vdjEiLCJraW5kIjoiQ3VzdG9tUmVzb3VyY2VEZWZpbml0aW9uIiwibWV0YWRhdGEiOnsibmFtZSI6ImJhcnMudGVzdC5iYXIifSwic3BlYyI6eyJncm91cCI6InRlc3QuYmFyIiwibmFtZXMiOnsia2luZCI6IkJhciIsInBsdXJhbCI6ImJhcnMifSwidmVyc2lvbnMiOlt7Im5hbWUiOiJ2MWFscGhhMSJ9XX19
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoib3BlcmF0b3JzLmNvcmVvcy5jb20vdjFhbHBoYTEiLCJraW5kIjoiQ2x1c3RlclNlcnZpY2VWZXJzaW9uIiwibWV0YWRhdGEiOnsibmFtZSI6ImJhci52MC4xLjAifSwic3BlYyI6eyJjdXN0b21yZXNvdXJjZWRlZmluaXRpb25zIjp7Im93bmVkIjpbeyJncm91cCI6InRlc3QuYmFyIiwia2luZCI6IkJhciIsIm5hbWUiOiJiYXJzLnRlc3QuYmFyIiwidmVyc2lvbiI6InYxYWxwaGExIn1dfSwicmVsYXRlZEltYWdlcyI6W3siaW1hZ2UiOiJ0ZXN0LnJlZ2lzdHJ5L2Jhci1vcGVyYXRvci9iYXI6djAuMS4wIiwibmFtZSI6Im9wZXJhdG9yIn1dLCJ2ZXJzaW9uIjoiMC4xLjAifX0=
relatedImages:
- image: test.registry/bar-operator/bar-bundle:v0.1.0
  name: ""
- image: test.registry/bar-operator/bar:v0.1.0
  name: operator
schema: olm.bundle
---
image: test.registry/bar-operator/bar-bundle:v0.2.0
name: bar.v0.2.0
package: bar
properties:
- type: olm.gvk
  value:
    group: test.bar
    kind: Bar
    version: v1alpha1
- type: olm.package
  value:
    packageName: bar
    version: 0.2.0
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoiYXBpZXh0ZW5zaW9ucy5rOHMuaW8vdjEiLCJraW5kIjoiQ3VzdG9tUmVzb3VyY2VEZWZpbml0aW9uIiwibWV0YWRhdGEiOnsibmFtZSI6ImJhcnMudGVzdC5iYXIifSwic3BlYyI6eyJncm91cCI6InRlc3QuYmFyIiwibmFtZXMiOnsia2luZCI6IkJhciIsInBsdXJhbCI6ImJhcnMifSwidmVyc2lvbnMiOlt7Im5hbWUiOiJ2MWFscGhhMSJ9XX19
- type: olm.bundle.object
  value:
    data: eyJhcGlWZXJzaW9uIjoib3BlcmF0b3JzLmNvcmVvcy5jb20vdjFhbHBoYTEiLCJraW5kIjoiQ2x1c3RlclNlcnZpY2VWZXJzaW9uIiwibWV0YWRhdGEiOnsiYW5ub3RhdGlvbnMiOnsib2xtLnNraXBSYW5nZSI6Ilx1MDAzYzAuMi4wIn0sIm5hbWUiOiJiYXIudjAuMi4wIn0sInNwZWMiOnsiY3VzdG9tcmVzb3VyY2VkZWZpbml0aW9ucyI6eyJvd25lZCI6W3siZ3JvdXAiOiJ0ZXN0LmJhciIsImtpbmQiOiJCYXIiLCJuYW1lIjoiYmFycy50ZXN0LmJhciIsInZlcnNpb24iOiJ2MWFscGhhMSJ9XX0sInJlbGF0ZWRJbWFnZXMiOlt7ImltYWdlIjoidGVzdC5yZWdpc3RyeS9iYXItb3BlcmF0b3IvYmFyOnYwLjIuMCIsIm5hbWUiOiJvcGVyYXRvciJ9XSwic2tpcHMiOlsiYmFyLnYwLjEuMCJdLCJ2ZXJzaW9uIjoiMC4yLjAifX0=
relatedImages:
- image: test.registry/bar-operator/bar-bundle:v0.2.0
  name: ""
- image: test.registry/bar-operator/bar:v0.2.0
  name: operator
schema: olm.bundle
`
}
