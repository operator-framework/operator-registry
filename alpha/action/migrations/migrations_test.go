package migrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

func TestMigrations(t *testing.T) {
	noneMigration, err := NewMigrations(NoMigrations)
	require.NoError(t, err)
	csvMigration, err := NewMigrations("bundle-object-to-csv-metadata")
	require.NoError(t, err)
	allMigrations, err := NewMigrations(AllMigrations)
	require.NoError(t, err)

	migrationPhaseEvaluators := map[MigrationToken]func(*declcfg.DeclarativeConfig) error{
		MigrationToken(NoMigrations): func(d *declcfg.DeclarativeConfig) error {
			if diff := cmp.Diff(*d, unmigratedCatalogFBC()); diff != "" {
				return fmt.Errorf("'none' migrator is not expected to change the config\n%s", diff)
			}
			return nil
		},
		MigrationToken("bundle-object-to-csv-metadata"): func(d *declcfg.DeclarativeConfig) error {
			if diff := cmp.Diff(*d, csvMetadataCatalogFBC()); diff != "" {
				return fmt.Errorf("unexpected result of migration\n%s", diff)
			}
			return nil
		},
		MigrationToken("split-icon"): func(d *declcfg.DeclarativeConfig) error { return nil },
	}

	tests := []struct {
		name      string
		migrators *Migrations
	}{
		{
			name:      "NoMigrations",
			migrators: noneMigration,
		},
		{
			name:      "BundleObjectToCSVMetadata",
			migrators: csvMigration,
		},
		{
			name:      "MigrationSequence",
			migrators: allMigrations,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var config = unmigratedCatalogFBC()

			for _, m := range test.migrators.Migrations {
				err := m.Migrate(&config)
				require.NoError(t, err)
				err = migrationPhaseEvaluators[m.Token()](&config)
				require.NoError(t, err)
			}
		})
	}
}

func mustBuildCSVMetadata(r io.Reader) property.Property {
	var csv v1alpha1.ClusterServiceVersion
	if err := json.NewDecoder(r).Decode(&csv); err != nil {
		panic(err)
	}
	return property.MustBuildCSVMetadata(csv)
}

var fooRawCsv = []byte(`{"apiVersion": "operators.coreos.com/v1alpha1", "kind": "ClusterServiceVersion", "metadata": {"name": "foo.v0.1.0"}, "spec": {"displayName": "Foo Operator", "customresourcedefinitions": {"owned": [{"group": "test.foo", "version": "v1", "kind": "Foo", "name": "foos.test.foo"}]}, "version": "0.1.0", "relatedImages": [{"name": "operator", "image": "test.registry/foo-operator/foo:v0.1.0"}]}}`)

var fooRawCrd = []byte(`---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: foos.test.foo
spec:
  group: test.foo
  names:
    kind: Foo
    plural: foos
  versions:
    - name: v1`,
)

func unmigratedCatalogFBC() declcfg.DeclarativeConfig {
	return declcfg.DeclarativeConfig{
		Bundles: []declcfg.Bundle{
			{
				Schema:  "olm.bundle",
				Name:    "foo.v0.1.0",
				Package: "foo",
				Image:   "quay.io/openshift-community-operators/foo:v0.1.0",
				Properties: []property.Property{
					property.MustBuildGVK("test.foo", "v1", "Foo"),
					property.MustBuildGVKRequired("test.bar", "v1alpha1", "Bar"),
					property.MustBuildPackage("foo", "0.1.0"),
					property.MustBuildPackageRequired("bar", "<0.1.0"),
					property.MustBuildBundleObject(fooRawCrd),
					property.MustBuildBundleObject(fooRawCsv),
				},
				Objects: []string{string(fooRawCsv), string(fooRawCrd)},
				CsvJSON: string(fooRawCsv),
			},
		},
	}
}

func csvMetadataCatalogFBC() declcfg.DeclarativeConfig {
	return declcfg.DeclarativeConfig{
		Bundles: []declcfg.Bundle{
			{
				Schema:  "olm.bundle",
				Name:    "foo.v0.1.0",
				Package: "foo",
				Image:   "quay.io/openshift-community-operators/foo:v0.1.0",
				Properties: []property.Property{
					property.MustBuildGVK("test.foo", "v1", "Foo"),
					property.MustBuildGVKRequired("test.bar", "v1alpha1", "Bar"),
					property.MustBuildPackage("foo", "0.1.0"),
					property.MustBuildPackageRequired("bar", "<0.1.0"),
					mustBuildCSVMetadata(bytes.NewReader(fooRawCsv)),
				},
				Objects: []string{string(fooRawCsv), string(fooRawCrd)},
				CsvJSON: string(fooRawCsv),
			},
		},
	}
}
