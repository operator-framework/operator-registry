package image_test

import (
	"context"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/sumdb/dirhash"

	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	libimage "github.com/operator-framework/operator-registry/pkg/lib/image"
)

// cleanupFunc is a function that cleans up after some test infra.
type cleanupFunc func()

// newRegistryFunc is a function that creates and returns a new image.Registry to test its cleanupFunc.
type newRegistryFunc func(t *testing.T, cafile string) (image.Registry, cleanupFunc)

func poolForCertFile(t *testing.T, file string) *x509.CertPool {
	rootCAs := x509.NewCertPool()
	certs, err := ioutil.ReadFile(file)
	require.NoError(t, err)
	require.True(t, rootCAs.AppendCertsFromPEM(certs))
	return rootCAs
}

func TestRegistries(t *testing.T) {
	registries := map[string]newRegistryFunc{
		"containerd": func(t *testing.T, cafile string) (image.Registry, cleanupFunc) {
			r, err := containerdregistry.NewRegistry(
				containerdregistry.WithLog(logrus.New().WithField("test", t.Name())),
				containerdregistry.WithCacheDir(fmt.Sprintf("cache-%x", rand.Int())),
				containerdregistry.WithRootCAs(poolForCertFile(t, cafile)),
			)
			require.NoError(t, err)
			cleanup := func() {
				require.NoError(t, r.Destroy())
			}

			return r, cleanup
		},
		// TODO: enable docker tests - currently blocked on a cross-platform way to configure either insecure registries
		// or CA certs
		//"docker": func(t *testing.T, cafile string) (image.Registry, cleanupFunc) {
		//	r, err := execregistry.NewRegistry(containertools.DockerTool,
		//		logrus.New().WithField("test", t.Name()),
		//		cafile,
		//	)
		//	require.NoError(t, err)
		//	cleanup := func() {
		//		require.NoError(t, r.Destroy())
		//	}
		//
		//	return r, cleanup
		//},
		// TODO: Enable buildah tests
		// func(t *testing.T) image.Registry {
		// 	r, err := buildahregistry.NewRegistry(
		// 		buildahregistry.WithLog(logrus.New().WithField("test", t.Name())),
		// 		buildahregistry.WithCacheDir(fmt.Sprintf("cache-%x", rand.Int())),
		// 	)
		// 	require.NoError(t, err)

		// 	return r
		// },
	}

	for name, registry := range registries {
		t.Run(name, func(t *testing.T) {
			testPullAndUnpack(t, registry)
		})

	}
}

func testPullAndUnpack(t *testing.T, newRegistry newRegistryFunc) {
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
			logrus.SetLevel(logrus.DebugLevel)
			ctx, close := context.WithCancel(context.Background())
			defer close()

			host, cafile, err := libimage.RunDockerRegistry(ctx, tt.args.dockerRootDir)
			require.NoError(t, err)

			r, cleanup := newRegistry(t, cafile)
			defer cleanup()

			ref := image.SimpleReference(host + tt.args.img)
			require.NoError(t, r.Pull(ctx, ref))

			// Copy golden manifests to a temp dir
			dir := "kiali-unpacked"
			require.NoError(t, r.Unpack(ctx, ref, dir))

			checksum := dirChecksum(t, dir)
			require.Equal(t, tt.expected.checksum, checksum)

			require.NoError(t, os.RemoveAll(dir))
		})
	}
}

func dirChecksum(t *testing.T, dir string) string {
	sum, err := dirhash.HashDir(dir, "", dirhash.DefaultHash)
	require.NoError(t, err)
	return sum
}
