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
- type: olm.csv.metadata
  value:
    annotations:
      olm.skipRange: <0.1.0
    apiServiceDefinitions: {}
    crdDescriptions:
      owned:
      - kind: Foo
        name: foos.test.foo
        version: v1
    displayName: Foo Operator
    provider: {}
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
- type: olm.csv.metadata
  value:
    annotations:
      olm.skipRange: <0.2.0
    apiServiceDefinitions: {}
    crdDescriptions:
      owned:
      - kind: Foo
        name: foos.test.foo
        version: v1
    displayName: Foo Operator
    provider: {}
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
- type: olm.csv.metadata
  value:
    apiServiceDefinitions: {}
    crdDescriptions:
      owned:
      - kind: Bar
        name: bars.test.bar
        version: v1alpha1
    provider: {}
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
- type: olm.csv.metadata
  value:
    annotations:
      olm.skipRange: <0.2.0
    apiServiceDefinitions: {}
    crdDescriptions:
      owned:
      - kind: Bar
        name: bars.test.bar
        version: v1alpha1
    provider: {}
relatedImages:
- image: test.registry/bar-operator/bar-bundle:v0.2.0
  name: ""
- image: test.registry/bar-operator/bar:v0.2.0
  name: operator
schema: olm.bundle
`
}
