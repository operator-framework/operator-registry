package image_test

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/containers/image/v5/types"
	"github.com/distribution/distribution/v3"
	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/sumdb/dirhash"

	libimage "github.com/operator-framework/operator-registry/internal/testutil/image"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/operator-framework/operator-registry/pkg/image/containersimageregistry"
)

// cleanupFunc is a function that cleans up after some test infra.
type cleanupFunc func()

// newRegistryFunc is a function that creates and returns a new image.Registry to test its cleanupFunc.
type newRegistryFunc func(t *testing.T, serverCert *x509.Certificate) (image.Registry, cleanupFunc)

func caDirForCert(t *testing.T, serverCert *x509.Certificate) string {
	caDir := t.TempDir()
	caFile, err := os.Create(filepath.Join(caDir, "ca.crt"))
	require.NoError(t, err)

	require.NoError(t, pem.Encode(caFile, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCert.Raw,
	}))
	require.NoError(t, caFile.Close())
	return caDir
}

const insecureSignaturePolicy = `{
    "default": [
        {
            "type": "insecureAcceptAnything"
        }
    ],
    "transports":
        {
            "docker-daemon":
                {
                    "": [{"type":"insecureAcceptAnything"}]
                }
        }
}`

func createSignaturePolicyFile(t *testing.T) string {
	policyDir := t.TempDir()
	policyFilePath := filepath.Join(policyDir, "policy.json")
	err := os.WriteFile(policyFilePath, []byte(insecureSignaturePolicy), 0600)
	require.NoError(t, err)
	return policyFilePath
}

func poolForCert(serverCert *x509.Certificate) *x509.CertPool {
	rootCAs := x509.NewCertPool()
	rootCAs.AddCert(serverCert)
	return rootCAs
}

func TestRegistries(t *testing.T) {
	registries := map[string]newRegistryFunc{
		"containersimage": func(t *testing.T, serverCert *x509.Certificate) (image.Registry, cleanupFunc) {
			caDir := caDirForCert(t, serverCert)
			policyFile := createSignaturePolicyFile(t)
			sourceCtx := &types.SystemContext{
				OCICertPath:              caDir,
				DockerCertPath:           caDir,
				DockerPerHostCertDirPath: caDir,
				SignaturePolicyPath:      policyFile,
			}
			r, err := containersimageregistry.New(sourceCtx, containersimageregistry.WithTemporaryImageCache())
			require.NoError(t, err)
			cleanup := func() {
				require.NoError(t, os.RemoveAll(caDir))
				require.NoError(t, r.Destroy())
			}
			return r, cleanup
		},
		"containerd": func(t *testing.T, serverCert *x509.Certificate) (image.Registry, cleanupFunc) {
			val, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
			require.NoError(t, err)
			r, err := containerdregistry.NewRegistry(
				containerdregistry.WithLog(logrus.New().WithField("test", t.Name())),
				containerdregistry.WithCacheDir(fmt.Sprintf("cache-%x", val)),
				containerdregistry.WithRootCAs(poolForCert(serverCert)),
			)
			require.NoError(t, err)
			cleanup := func() {
				require.NoError(t, r.Destroy())
			}

			return r, cleanup
		},
	}

	for name, registry := range registries {
		testPullAndUnpack(t, name, registry)
	}
}

type httpError struct {
	statusCode int
	error      error
}

func (e *httpError) Error() string {
	if e.error != nil {
		return e.error.Error()
	}
	return http.StatusText(e.statusCode)
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
		labels        map[string]string
	}

	expectedLabels := map[string]string{
		"operators.operatorframework.io.bundle.mediatype.v1":       "registry+v1",
		"operators.operatorframework.io.bundle.manifests.v1":       "manifests/",
		"operators.operatorframework.io.bundle.metadata.v1":        "metadata/",
		"operators.operatorframework.io.bundle.package.v1":         "kiali",
		"operators.operatorframework.io.bundle.channels.v1":        "stable,alpha",
		"operators.operatorframework.io.bundle.channel.default.v1": "stable",
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
				labels:        expectedLabels,
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
				labels:        expectedLabels,
				pullAssertion: require.NoError,
			},
		},
		{
			description: fmt.Sprintf("%s/WithOneRetriableError", name),
			args: args{
				dockerRootDir: "testdata/golden",
				img:           "/olmtest/kiali:1.4.2",
				pullErrCount:  1,
				pullErr:       &httpError{statusCode: http.StatusTooManyRequests},
			},
			expected: expected{
				checksum:      dirChecksum(t, "testdata/golden/bundles/kiali"),
				labels:        expectedLabels,
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
				pullErr:       &httpError{statusCode: http.StatusTooManyRequests},
			},
			expected: expected{
				pullAssertion: require.Error,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			logrus.SetLevel(logrus.DebugLevel)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var middlewares []func(next http.Handler) http.Handler
			if tt.args.pullErrCount > 0 {
				middlewares = append(middlewares, failureMiddleware(tt.args.pullErrCount, tt.args.pullErr))
			}

			dockerServer := libimage.RunDockerRegistry(ctx, tt.args.dockerRootDir, middlewares...)
			defer dockerServer.Close()

			r, cleanup := newRegistry(t, dockerServer.Certificate())
			defer cleanup()

			url, err := url.Parse(dockerServer.URL)
			require.NoError(t, err)

			ref := image.SimpleReference(fmt.Sprintf("%s%s", url.Host, tt.args.img))
			t.Log("pulling image", ref)
			pullErr := r.Pull(ctx, ref)
			tt.expected.pullAssertion(t, pullErr)
			if pullErr != nil {
				return
			}

			labels, err := r.Labels(ctx, ref)
			require.NoError(t, err)
			require.Equal(t, tt.expected.labels, labels)

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

func (f *mockBlobStore) Open(ctx context.Context, dgst digest.Digest) (io.ReadSeekCloser, error) {
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

func failureMiddleware(totalCount int, err error) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		count := 0
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if count >= totalCount {
				next.ServeHTTP(w, r)
				return
			}
			count++
			statusCode := http.StatusInternalServerError

			var httpErr *httpError
			if errors.As(err, &httpErr) {
				statusCode = httpErr.statusCode
			}

			http.Error(w, err.Error(), statusCode)
		})
	}
}
