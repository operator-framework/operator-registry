package unprivileged

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry"
	_ "github.com/docker/distribution/registry/storage/driver/filesystem"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	"github.com/phayes/freeport"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/sumdb/dirhash"
)

func setupRegistry(t *testing.T, ctx context.Context, rootDir string) string {
	dockerPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	config := &configuration.Configuration{}
	config.HTTP.Addr = fmt.Sprintf(":%d", dockerPort)
	if rootDir != "" {
		config.Storage = map[string]configuration.Parameters{"filesystem": map[string]interface{}{
			"rootdirectory": rootDir,
		}}
	} else {
		config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	}
	config.HTTP.DrainTimeout = time.Duration(2) * time.Second

	dockerRegistry, err := registry.NewRegistry(context.Background(), config)
	require.NoError(t, err)

	go func() {
		require.NoError(t, dockerRegistry.ListenAndServe())
	}()

	// Return the registry host string
	return fmt.Sprintf("127.0.0.1:%d", dockerPort)
}

func dirChecksum(t *testing.T, dir string) string {
	sum, err := dirhash.HashDir(dir, "", dirhash.DefaultHash)
	require.NoError(t, err)
	return sum
}

func TestPullAndUnpack(t *testing.T) {
	type args struct {
		dockerRootDir string
		img           string
	}
	type expected struct {
		checksum string
	}
	tests := []struct {
		description string
		args        args
		expected    expected
	}{
		{
			description: "ByTag",
			args: args{
				dockerRootDir: "testdata/golden",
				img:           "/olmtest/kiali:1.4.2",
			},
			expected: expected{
				checksum: dirChecksum(t, "testdata/golden/bundles/kiali"),
			},
		},
		{
			description: "ByDigest",
			args: args{
				dockerRootDir: "testdata/golden",
				img:           "/olmtest/kiali@sha256:a1bec450c104ceddbb25b252275eb59f1f1e6ca68e0ced76462042f72f7057d8",
			},
			expected: expected{
				checksum: dirChecksum(t, "testdata/golden/bundles/kiali"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			ctx, close := context.WithCancel(context.Background())
			defer close()

			host := setupRegistry(t, ctx, tt.args.dockerRootDir)
			r, err := NewRegistry(
				WithLog(logrus.New().WithField("test", t.Name())),
				WithCacheDir(fmt.Sprintf("cache-%x", rand.Int())),
			)
			require.NoError(t, err)

			ref := host + tt.args.img
			require.NoError(t, r.Pull(ctx, ref))

			// Copy golden manifests to a temp dir
			dir := "kiali-unpacked"
			require.NoError(t, r.Unpack(ctx, ref, dir))

			checksum := dirChecksum(t, dir)
			require.Equal(t, tt.expected.checksum, checksum)

			require.NoError(t, r.Close())
			require.NoError(t, os.RemoveAll(dir))
		})
	}
}
