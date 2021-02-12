package mirror

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

func CreateTestDb(t *testing.T) (*sql.DB, string, func()) {
	dbName := fmt.Sprintf("test-%d.db", rand.Int())

	db, err := sqlite.Open(dbName)
	require.NoError(t, err)

	load, err := sqlite.NewSQLLiteLoader(db)
	require.NoError(t, err)
	require.NoError(t, load.Migrate(context.TODO()))

	loader := sqlite.NewSQLLoaderForDirectory(load, "../../manifests")
	require.NoError(t, loader.Populate())

	return db, dbName, func() {
		defer func() {
			if err := os.Remove(dbName); err != nil {
				t.Fatal(err)
			}
		}()
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIndexImageMirrorer_Mirror(t *testing.T) {
	_, path, cleanup := CreateTestDb(t)
	defer cleanup()

	var testExtractor DatabaseExtractorFunc = func(from string) (s string, e error) {
		return path, nil
	}
	type fields struct {
		ImageMirrorer     ImageMirrorerFunc
		DatabaseExtractor DatabaseExtractorFunc
		Source            string
		Dest              string
	}
	tests := []struct {
		name    string
		fields  fields
		want    map[string]string
		wantErr error
	}{
		{
			name: "mirror images",
			fields: fields{
				DatabaseExtractor: testExtractor,
				Source:            "example",
				Dest:              "localhost",
			},
			want: map[string]string{
				"quay.io/coreos/etcd-operator@sha256:bd944a211eaf8f31da5e6d69e8541e7cada8f16a9f7a5a570b22478997819943":       "localhost/coreos/etcd-operator@sha256:bd944a211eaf8f31da5e6d69e8541e7cada8f16a9f7a5a570b22478997819943",
				"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2":       "localhost/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2",
				"quay.io/coreos/etcd-operator@sha256:db563baa8194fcfe39d1df744ed70024b0f1f9e9b55b5923c2f3a413c44dc6b8":       "localhost/coreos/etcd-operator@sha256:db563baa8194fcfe39d1df744ed70024b0f1f9e9b55b5923c2f3a413c44dc6b8",
				"quay.io/coreos/etcd@sha256:3816b6daf9b66d6ced6f0f966314e2d4f894982c6b1493061502f8c2bf86ac84":                "localhost/coreos/etcd@sha256:3816b6daf9b66d6ced6f0f966314e2d4f894982c6b1493061502f8c2bf86ac84",
				"quay.io/coreos/etcd@sha256:49d3d4a81e0d030d3f689e7167f23e120abf955f7d08dbedf3ea246485acee9f":                "localhost/coreos/etcd@sha256:49d3d4a81e0d030d3f689e7167f23e120abf955f7d08dbedf3ea246485acee9f",
				"quay.io/coreos/prometheus-operator@sha256:0e92dd9b5789c4b13d53e1319d0a6375bcca4caaf0d698af61198061222a576d": "localhost/coreos/prometheus-operator@sha256:0e92dd9b5789c4b13d53e1319d0a6375bcca4caaf0d698af61198061222a576d",
				"quay.io/coreos/prometheus-operator@sha256:3daa69a8c6c2f1d35dcf1fe48a7cd8b230e55f5229a1ded438f687debade5bcf": "localhost/coreos/prometheus-operator@sha256:3daa69a8c6c2f1d35dcf1fe48a7cd8b230e55f5229a1ded438f687debade5bcf",
				"quay.io/coreos/prometheus-operator@sha256:5037b4e90dbb03ebdefaa547ddf6a1f748c8eeebeedf6b9d9f0913ad662b5731": "localhost/coreos/prometheus-operator@sha256:5037b4e90dbb03ebdefaa547ddf6a1f748c8eeebeedf6b9d9f0913ad662b5731",
				"docker.io/strimzi/cluster-operator:0.11.0":                                                                  "localhost/strimzi/cluster-operator:0.11.0",
				"docker.io/strimzi/cluster-operator:0.11.1":                                                                  "localhost/strimzi/cluster-operator:0.11.1",
				"docker.io/strimzi/operator:0.12.1":                                                                          "localhost/strimzi/operator:0.12.1",
				"docker.io/strimzi/operator:0.12.2":                                                                          "localhost/strimzi/operator:0.12.2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fields.ImageMirrorer == nil {
				tt.fields.ImageMirrorer = func(mapping map[string]string) error {
					require.Equal(t, tt.want, mapping)
					return nil
				}
			}

			b := &IndexImageMirrorer{
				ImageMirrorer:     tt.fields.ImageMirrorer,
				DatabaseExtractor: tt.fields.DatabaseExtractor,
				Source:            tt.fields.Source,
				Dest:              tt.fields.Dest,
			}
			got, err := b.Mirror()
			if err != nil {
				require.Equal(t, tt.wantErr.Error(), err.Error())
			}
			require.Equal(t, tt.want, got)
		})
	}
}
