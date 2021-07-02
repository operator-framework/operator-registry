package encoding

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGzipBase64EncodeDecode(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "Encode-Decode-CSV",
			source: "testdata/etcdoperator.v0.9.4.clusterserviceversion.yaml",
		},
		{
			name:   "Encode-Decode-CRD",
			source: "testdata/etcdclusters.etcd.database.coreos.com.crd.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.source)
			require.NoError(t, err, "unable to load from file %s", tt.source)

			encoded, err := GzipBase64Encode(data)
			require.NoError(t, err, "unexpected error while encoding data")

			require.Lessf(t, len(encoded), len(data),
				"encoded data (%d bytes) isn't lesser than original data (%d bytes)",
				len(encoded), len(data))

			decoded, err := GzipBase64Decode(encoded)
			require.NoError(t, err, "unexpected error while decoding data")

			require.Equal(t, data, decoded, "decoded data doesn't match original data")
		})
	}
}
