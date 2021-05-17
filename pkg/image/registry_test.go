package image_test

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/docker/distribution"
	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/reference"
	repositorymiddleware "github.com/docker/distribution/registry/middleware/repository"
	"github.com/opencontainers/go-digest"
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
		testPullAndUnpack(t, name, registry)
	}
}

func testPullAndUnpack(t *testing.T, name string, newRegistry newRegistryFunc) {
	type args struct {
		dockerRootDir string
		img           string
		pullErrCount  int
		pullErr       error
	}
	type expected struct {
		checksum      string
		pullAssertion require.ErrorAssertionFunc
	}
	tests := []struct {
		description string
		args        args
		expected    expected
	}{
		{
			description: fmt.Sprintf("%s/ByTag", name),
			args: args{
				dockerRootDir: "testdata/golden",
				img:           "/olmtest/kiali:1.4.2",
			},
			expected: expected{
				checksum:      dirChecksum(t, "testdata/golden/bundles/kiali"),
				pullAssertion: require.NoError,
			},
		},
		{
			description: fmt.Sprintf("%s/ByDigest", name),
			args: args{
				dockerRootDir: "testdata/golden",
				img:           "/olmtest/kiali@sha256:a1bec450c104ceddbb25b252275eb59f1f1e6ca68e0ced76462042f72f7057d8",
			},
			expected: expected{
				checksum:      dirChecksum(t, "testdata/golden/bundles/kiali"),
				pullAssertion: require.NoError,
			},
		},
		{
			description: fmt.Sprintf("%s/WithOneRetriableError", name),
			args: args{
				dockerRootDir: "testdata/golden",
				img:           "/olmtest/kiali:1.4.2",
				pullErrCount:  1,
				pullErr:       errors.New("dummy"),
			},
			expected: expected{
				checksum:      dirChecksum(t, "testdata/golden/bundles/kiali"),
				pullAssertion: require.NoError,
			},
		},
		// TODO: figure out how to have the server send a detectable non-retriable error.
		//{
		//  description: fmt.Sprintf("%s/WithNonRetriableError", name),
		//	args: args{
		//		dockerRootDir: "testdata/golden",
		//		img:           "/olmtest/kiali:1.4.2",
		//	},
		//	expected: expected{
		//		pullAssertion: require.Error,
		//	},
		//},
		{
			description: fmt.Sprintf("%s/WithAlwaysRetriableError", name),
			args: args{
				dockerRootDir: "testdata/golden",
				img:           "/olmtest/kiali:1.4.2",
				pullErrCount:  math.MaxInt64,
				pullErr:       errors.New("dummy"),
			},
			expected: expected{
				pullAssertion: require.Error,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			logrus.SetLevel(logrus.DebugLevel)
			ctx, close := context.WithCancel(context.Background())
			defer close()

			configOpts := []libimage.ConfigOpt{}

			if tt.args.pullErrCount > 0 {
				configOpts = append(configOpts, func(config *configuration.Configuration) {
					if config.Middleware == nil {
						config.Middleware = make(map[string][]configuration.Middleware)
					}

					mockRepo := &mockRepo{blobStore: &mockBlobStore{
						maxCount: tt.args.pullErrCount,
						err:      tt.args.pullErr,
					}}
					middlewareName := fmt.Sprintf("test-%x", rand.Int())
					require.NoError(t, repositorymiddleware.Register(middlewareName, mockRepo.init))
					config.Middleware["repository"] = append(config.Middleware["repository"], configuration.Middleware{
						Name: middlewareName,
					})
				})
			}

			host, cafile, err := libimage.RunDockerRegistry(ctx, tt.args.dockerRootDir, configOpts...)
			require.NoError(t, err)

			r, cleanup := newRegistry(t, cafile)
			defer cleanup()

			ref := image.SimpleReference(host + tt.args.img)
			tt.expected.pullAssertion(t, r.Pull(ctx, ref))

			if tt.expected.checksum != "" {
				// Copy golden manifests to a temp dir
				dir := "kiali-unpacked"
				require.NoError(t, r.Unpack(ctx, ref, dir))

				checksum := dirChecksum(t, dir)
				require.Equal(t, tt.expected.checksum, checksum)

				require.NoError(t, os.RemoveAll(dir))
			}
		})
	}
}

func dirChecksum(t *testing.T, dir string) string {
	sum, err := dirhash.HashDir(dir, "", dirhash.DefaultHash)
	require.NoError(t, err)
	return sum
}

var _ distribution.Repository = &mockRepo{}

type mockRepo struct {
	base      distribution.Repository
	blobStore *mockBlobStore
	once      sync.Once
}

func (f *mockRepo) init(ctx context.Context, base distribution.Repository, options map[string]interface{}) (distribution.Repository, error) {
	f.once.Do(func() {
		f.base = base
		f.blobStore.base = base.Blobs(ctx)
	})
	return f, nil
}

func (f *mockRepo) Named() reference.Named {
	return f.base.Named()
}

func (f *mockRepo) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
	return f.base.Manifests(ctx, options...)
}

func (f *mockRepo) Blobs(ctx context.Context) distribution.BlobStore {
	return f.blobStore
}

func (f *mockRepo) Tags(ctx context.Context) distribution.TagService {
	return f.base.Tags(ctx)
}

var _ distribution.BlobStore = &mockBlobStore{}

type mockBlobStore struct {
	base     distribution.BlobStore
	err      error
	maxCount int

	count int
	m     sync.Mutex
}

func (f *mockBlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	f.m.Lock()
	defer f.m.Unlock()
	f.count++
	if f.count <= f.maxCount {
		return distribution.Descriptor{}, f.err
	}
	return f.base.Stat(ctx, dgst)
}

func (f *mockBlobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	return f.base.Get(ctx, dgst)
}

func (f *mockBlobStore) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	return f.base.Open(ctx, dgst)
}

func (f *mockBlobStore) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	return f.base.Put(ctx, mediaType, p)
}

func (f *mockBlobStore) Create(ctx context.Context, options ...distribution.BlobCreateOption) (distribution.BlobWriter, error) {
	return f.base.Create(ctx, options...)
}

func (f *mockBlobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	return f.base.Resume(ctx, id)
}

func (f *mockBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, r *http.Request, dgst digest.Digest) error {
	return f.base.ServeBlob(ctx, w, r, dgst)
}

func (f *mockBlobStore) Delete(ctx context.Context, dgst digest.Digest) error {
	return f.base.Delete(ctx, dgst)
}
