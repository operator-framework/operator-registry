package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBundleDirectoryInterpreter(t *testing.T) {
	tests := []struct {
		dir            string
		fail           bool
		bundleChannels []string
		defaultChannel string
		PackageName    string
	}{
		{
			dir:            "../../registry/testdata/validPackages/etcd/0.6.1",
			fail:           false,
			bundleChannels: []string{"alpha", "beta", "stable"},
			defaultChannel: "alpha",
			PackageName:    "etcd",
		},
		{
			dir:            "../../registry/testdata/validPackages/etcd/0.9.0",
			fail:           false,
			bundleChannels: []string{"alpha", "beta", "stable"},
			defaultChannel: "alpha",
			PackageName:    "etcd",
		},
		{
			dir:            "../../registry/testdata/validPackages/etcd/0.9.2",
			fail:           false,
			bundleChannels: []string{"alpha", "stable"},
			defaultChannel: "alpha",
			PackageName:    "etcd",
		},
		{
			dir:            "../../registry/testdata/validPackages/prometheus/0.14.0",
			fail:           false,
			bundleChannels: []string{"preview"},
			defaultChannel: "preview",
			PackageName:    "prometheus",
		},
		{
			dir:  "../../registry/testdata/invalidPackges/3scale-community-operator/0.3.0",
			fail: true,
		},
	}

	for _, tt := range tests {
		t.Run("Loading Package Graph from "+tt.dir, func(t *testing.T) {
			bundle, err := NewBundleDirInterperter(tt.dir)
			if tt.fail {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.EqualValues(t, tt.bundleChannels, bundle.GetBundleChannels())

			assert.EqualValues(t, tt.defaultChannel, bundle.GetDefaultChannel())

			assert.EqualValues(t, tt.PackageName, bundle.GetPackageName())
		})
	}
}
