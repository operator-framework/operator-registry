package containerdregistry_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/containerd/containerd/namespaces"
	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry/handlers"
	"github.com/docker/distribution/registry/listener"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	"github.com/operator-framework/operator-registry/pkg/image"
	containerd "github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry/fakestore"
	"github.com/phayes/freeport"
	"github.com/rogpeppe/go-internal/dirhash"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerdRegistry(t *testing.T) {
	tests := map[string]func(t *testing.T){
		// New image
		"new":               containerd.TestNew,
		"addNewManifest":    containerd.TestAddNewManifest,
		"updateManifests":   containerd.TestUpdateManifests,
		"updateImageConfig": containerd.TestUpdateImageConfig,
		"newImage":          testNewImage,
		// squashing layers
		"tarTreeAdd":         containerd.TestTreeAdd,
		"tarTreeDelete":      containerd.TestTreeDelete,
		"applyLayerInMemory": containerd.TestApplyLayerInMemory,
		"squashLayers":       containerd.TestSquashLayers,
		// Building image from directory
		"init": containerd.TestInit,
		"diff": containerd.TestDiff,
		"pack": testPack,
		// export oci bundle structure
		"export": testExport,
		// push image to local registry
		"push": testPush,
	}
	for name, tt := range tests {
		t.Run(name, tt)
	}
}

func testNewImage(t *testing.T) {
	ref := image.SimpleReference("test-repo-1")
	r, cleanup := setupRegistry(t)
	defer cleanup()

	ctx := namespaces.WithNamespace(context.TODO(), namespaces.Default)

	err := r.NewImage(ctx, ref, containerd.OmitTimestamp())
	require.NoError(t, err, "failed to create new image for ref "+ref.String())

	img, err := r.Images().Get(ctx, ref.String())
	require.NoError(t, err, "failed to get image from ref "+ref.String())

	assert.Equal(t, "sha256:7155913849a4a574ddb0f3cf0930c87310c04b4dd8dbb0d9654c79b0a6b79ba1", img.Target.Digest.String())
}

func testPack(t *testing.T) {
	ref := image.SimpleReference("test-repo-1")
	r, cleanup := setupRegistry(t)
	defer cleanup()

	dir, err := ioutil.TempDir(".", "packtest-*")
	require.NoError(t, err, "failed to create tmpdir")
	defer os.RemoveAll(dir)
	err = os.MkdirAll(filepath.Join(dir, "d1", "d2"), 0755)
	require.NoError(t, err, "failed to create test files")

	ctx := namespaces.WithNamespace(context.TODO(), namespaces.Default)
	err = r.Pack(ctx, ref, filepath.Join(dir, "d1"), containerd.OmitTimestamp())
	require.NoError(t, err, "pack failed")

	img, err := r.Images().Get(ctx, ref.String())
	require.NoError(t, err, "failed to get image")
	packedDigest := img.Target.Digest.String()

	unpackedDir, err := ioutil.TempDir(".", "unpacktest-*")
	require.NoError(t, err, "failed to create tmpdir")
	defer os.RemoveAll(unpackedDir)

	err = r.Unpack(ctx, ref, unpackedDir)
	require.NoError(t, err, "unpack failed")

	require.Equal(t, dirChecksum(t, filepath.Join(dir, "d1")), dirChecksum(t, filepath.Join(unpackedDir, "d1")))

	err = r.Pack(ctx, ref, unpackedDir, containerd.OmitTimestamp())
	require.NoError(t, err, "repack failed")

	img, err = r.Images().Get(ctx, ref.String())
	require.NoError(t, err, "failed to get image")
	assert.Equal(t, packedDigest, img.Target.Digest.String())
}

func testExport(t *testing.T) {
	ref := image.SimpleReference("test-repo-1")
	r, cleanup := setupRegistry(t)
	defer cleanup()

	ctx := namespaces.WithNamespace(context.TODO(), namespaces.Default)

	err := r.NewImage(ctx, ref, containerd.OmitTimestamp())
	require.NoError(t, err, "failed to create new image")

	exportDir, err := ioutil.TempDir(".", "export-*")
	require.NoError(t, err, "failed to create tmpdir")
	defer os.RemoveAll(exportDir)

	err = r.Export(ctx, ref, exportDir)
	require.NoError(t, err, "export failed")

	require.Equal(t, dirChecksum(t, "../testdata/golden/ocibundle/empty"), dirChecksum(t, exportDir), "export does not match expected bundle")
}

func testPush(t *testing.T) {
	ctx, close := context.WithCancel(context.Background())
	defer close()
	addr, _, err := startDockerRegistry(ctx)
	require.NoError(t, err, "failed to start docker registry")

	ref := image.SimpleReference(fmt.Sprintf("%s/test-repo-1:pushed", addr))
	r, cleanup := setupRegistry(t)
	defer cleanup()

	if _, namespaced := namespaces.Namespace(ctx); !namespaced {
		ctx = namespaces.WithNamespace(ctx, namespaces.Default)
	}

	err = r.NewImage(ctx, ref, containerd.OmitTimestamp())
	require.NoError(t, err, "image creation failed")

	err = r.Push(ctx, ref)
	require.NoError(t, err, "push failed")

	ref2 := image.SimpleReference(fmt.Sprintf("%s/test-repo-1:pulled", addr))
	err = r.NewImage(ctx, ref2, containerd.OmitTimestamp(), containerd.WithBaseImage(ref))
	require.NoError(t, err, "image creation failed")

	img, err := r.Images().Get(ctx, ref.String())
	require.NoError(t, err, "failed to get image")
	img2, err := r.Images().Get(ctx, ref2.String())
	require.NoError(t, err, "failed to get image")

	assert.Equal(t, img.Target.Digest.String(), img2.Target.Digest.String(), "digest mismatch")
}

// startDockerRegistry starts a local docker registry
func startDockerRegistry(parent context.Context) (string, chan struct{}, error) {
	var err error
	var doneChan = make(chan struct{}, 1)
	config := &configuration.Configuration{
		Storage: map[string]configuration.Parameters{
			"inmemory": map[string]interface{}{},
		},
	}
	dockerPort, err := freeport.GetFreePort()
	if err != nil {
		return "", doneChan, err
	}
	addr := fmt.Sprintf("localhost:%d", dockerPort)
	config.HTTP.Addr = addr
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	logger := logrus.StandardLogger()

	// inject a logger into the uuid library. warns us if there is a problem
	// with uuid generation under low entropy.
	// uuid.Loggerf = logger.Warnf

	ctx := context.Background()
	app := handlers.NewApp(ctx, config)
	handler := alive(app)
	handler = panicHandler(handler)

	server := &http.Server{
		Handler: handler,
	}

	ln, err := listener.NewListener(config.HTTP.Net, config.HTTP.Addr)
	if err != nil {
		return "", nil, err
	}

	logger.Infof("listening on %v", ln.Addr())
	serveErr := make(chan error)

	go func() {
		serveErr <- server.Serve(ln)
	}()

	go func() {
		var done bool
		for !done {
			select {
			case <-parent.Done():
				done = true
				if parent.Err() != nil {
					logger.Errorf("Error running server: %v", err)
				}
				break
			case <-doneChan:
				done = true
				break
			}
		}
		var err error
		logger.Infof("Attemptimg to stop server, draining connections for %s", config.HTTP.DrainTimeout.String())
		c, cancel := context.WithTimeout(context.Background(), config.HTTP.DrainTimeout)
		defer cancel()
		serveErr <- server.Shutdown(c)
		for {
			select {
			case <-c.Done():
				if c.Err() != nil {
					logger.Error(err)
				}
			case err = <-serveErr:
				if err != nil {
					logger.Error(err)
				}
			case <-time.After(config.HTTP.DrainTimeout):
				logger.Errorf("Timed out waiting for server to stop")
			}
		}
	}()
	return addr, doneChan, nil
}

func alive(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(http.StatusOK)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func panicHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Panic(fmt.Sprintf("%v", err))
			}
		}()
		handler.ServeHTTP(w, r)
	})
}

func setupRegistry(t *testing.T) (r *containerd.Registry, cleanup func()) {
	cacheDir, err := ioutil.TempDir("", "cache-*")
	require.NoError(t, err, "failed to create temp cache dir")
	cleanup = func() {
		assert.NoError(t, os.RemoveAll(cacheDir), "tmpdir cleanup failed")
		require.NoError(t, r.Destroy(), "registry cleanup failed")
	}

	logrus.SetLevel(logrus.DebugLevel)

	r, err = containerd.NewRegistry(
		containerd.WithLog(logrus.New().WithField("test", t.Name())),
		containerd.WithCacheDir(cacheDir),
		containerd.SkipTLS(true),
	)
	require.NoError(t, err, "failed to create registry")

	r.Store = containerd.NewStore(fakestore.NewFakeContentStore(), fakestore.NewFakeImageStore())

	return r, cleanup
}

func dirChecksum(t *testing.T, dir string) string {
	sum, err := dirhash.HashDir(dir, "", dirhash.DefaultHash)
	require.NoError(t, err, "failed to calculate dir hash", dir)
	return sum
}
