package containerdregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry/fakestore"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	ref := image.SimpleReference("test-repo-1")
	b := NewBuilder(digest.SHA256)
	co := BuildConfig{
		OmitTimestamp: true,
	}

	confDesc, data, err := b.newConfig(co)
	require.NoError(t, err, "config creation failed")
	assert.Equal(t, "sha256:3fa7d886eb9059a0d64e421f48d6144cb45d33a102e405fafe58c99df13583f9", confDesc.Digest.String())
	assert.Equal(t, `{"created":"0001-01-01T00:00:00Z","architecture":"amd64","os":"linux","config":{},"rootfs":{"type":"layers","diff_ids":[]},"history":[{"created":"0001-01-01T00:00:00Z","created_by":"operator-registry","empty_layer":true}]}`, string(data))

	mfstDesc, data, err := b.newManifest(ref, confDesc, co)
	require.NoError(t, err, "manifest creation failed")
	assert.Equal(t, "sha256:55055ba9a2956483c98fbf4466f95d93b2de28e13ebdce45ebafe1b8d3b714de", mfstDesc.Digest.String())
	assert.Equal(t, fmt.Sprintf(`{"schemaVersion":2,"config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"%s","size":222},"layers":[]}`, confDesc.Digest.String()), string(data))

	indxDesc, data, err := b.newIndex([]ocispecv1.Descriptor{*mfstDesc})
	require.NoError(t, err, "index creation failed")
	assert.Equal(t, "sha256:7155913849a4a574ddb0f3cf0930c87310c04b4dd8dbb0d9654c79b0a6b79ba1", indxDesc.Digest.String())
	assert.Equal(t, fmt.Sprintf(`{"schemaVersion":2,"manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"%s","size":191,"annotations":{"org.opencontainers.image.ref.name":"test-repo-1"},"platform":{"architecture":"amd64","os":"linux"}}]}`, mfstDesc.Digest.String()), string(data))

	desc, _, diffID, err := b.newLayer(true, nil)
	require.NoError(t, err, "layer creation failed")
	assert.Equal(t, "sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1", desc.Digest.String())
	assert.Equal(t, "sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef", diffID.String())

}

func TestInit(t *testing.T) {
	ref := image.SimpleReference("test-repo-1")
	r, cleanup := setupRegistry(t)
	defer cleanup()

	root := filepath.Clean("../testdata/golden/bundles/kiali")
	err := r.init(root, ref)
	require.NoError(t, err, "failed to initialize file tree")

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if path == root {
			return nil
		}
		if _, ok := r.builder(ref).buildRoot[root][path]; !ok {
			assert.True(t, ok, "missing entry: "+path)
			return nil
		}
		assert.True(t, reflect.DeepEqual(info, r.builder(ref).buildRoot[root][path].info), "stat mismatch: "+path)
		return nil
	})
}

func TestDiff(t *testing.T) {
	ref := image.SimpleReference("test-repo-1")
	r, cleanup := setupRegistry(t)
	defer cleanup()

	dir, err := ioutil.TempDir(".", "difftest-*")
	require.NoError(t, err, "failed to create tmpdir")
	defer os.RemoveAll(dir)
	err = os.MkdirAll(filepath.Join(dir, "d1", "d2"), 0755)
	require.NoError(t, err, "failed to create test files")
	f, err := os.OpenFile(filepath.Join(dir, "d1", "d2", "f1"), os.O_RDONLY|os.O_CREATE|os.O_TRUNC, 0600)
	require.NoError(t, err, "failed to create testfile")
	f.Close()

	err = r.init(dir, ref)
	require.NoError(t, err, "failed to initialize file tree")

	expected := map[string]string{
		filepath.Join("d1", fmt.Sprintf("%sd2", WhPrefix)):       filepath.Join("d1", fmt.Sprintf("%sd2", WhPrefix)),
		filepath.Join("d1", "d2", fmt.Sprintf("%sf1", WhPrefix)): filepath.Join("d1", "d2", fmt.Sprintf("%sf1", WhPrefix)),
		filepath.Join(dir, "d1", "f1"):                           filepath.Join("d1", "f1"),
	}
	err = os.RemoveAll(filepath.Join(dir, "d1", "d2"))
	require.NoError(t, err, "failed to remove test dir "+filepath.Join(dir, "d1", "d2"))
	f, err = os.OpenFile(filepath.Join(dir, "d1", "f1"), os.O_RDONLY|os.O_CREATE|os.O_TRUNC, 0600)
	require.NoError(t, err, "failed to create testfile")
	f.Close()

	diffMap, err := r.diff(dir, "", ref)
	require.NoError(t, err, "failed to calculate file tree diff")

	assert.Equal(t, expected, diffMap)
}

func TestAddNewManifest(t *testing.T) {
	ref := image.SimpleReference("test-repo-1")
	b := NewBuilder(digest.SHA256)
	co := BuildConfig{
		OmitTimestamp: true,
	}
	cs := fakestore.NewFakeContentStore()
	indexDesc, err := b.addNewManifest(context.TODO(), cs, ref, nil, co)
	require.NoError(t, err, "failed to add manifest")
	assert.Equal(t, "sha256:7155913849a4a574ddb0f3cf0930c87310c04b4dd8dbb0d9654c79b0a6b79ba1", indexDesc.Digest.String())

	indexDesc, err = b.addNewManifest(context.TODO(), cs, ref, indexDesc, co)
	require.NoError(t, err, "failed to add manifest")

	// data, err := content.ReadBlob(context.TODO(), cs, *indexDesc)
	// require.NoError(t, err, "failed to read index blob")
	// assert.Equal(t, `{"schemaVersion":2,"manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:573eb60994bfbdc7be1e739a39df18ee05f00e86a2ffdde7d2959d1984661e05","size":191,"annotations":{"org.opencontainers.image.ref.name":"test-repo-1"},"platform":{"architecture":"amd64","os":"linux"}},{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:573eb60994bfbdc7be1e739a39df18ee05f00e86a2ffdde7d2959d1984661e05","size":191,"annotations":{"org.opencontainers.image.ref.name":"test-repo-1"},"platform":{"architecture":"amd64","os":"linux"}}]}`, string(data))

	assert.Equal(t, "sha256:0610d4bbb5e7c28439bc104b8f74bf5b336af694285b2093acfaf3233129140b", indexDesc.Digest.String())
}

func TestUpdateManifests(t *testing.T) {
	ref := image.SimpleReference("test-repo-1")
	b := NewBuilder(digest.SHA256)
	co := BuildConfig{
		OmitTimestamp: true,
	}
	cs := fakestore.NewFakeContentStore()
	confDesc, data, err := b.newConfig(co)
	require.NoError(t, err, "failed to create config")

	err = writeBlob(cs, ref.String(), *confDesc, data)
	require.NoError(t, err, "failed to write config")

	mfstDesc, data, err := b.newManifest(ref, confDesc, co)
	require.NoError(t, err, "failed to create manifest")

	err = writeBlob(cs, ref.String(), *mfstDesc, data)
	require.NoError(t, err, "failed to write manifest")

	indxDesc, data, err := b.newIndex([]ocispecv1.Descriptor{*mfstDesc})
	require.NoError(t, err, "failed to create index")

	err = writeBlob(cs, ref.String(), *indxDesc, data)
	require.NoError(t, err, "failed to write index")

	indxDesc, err = b.updateManifests(context.TODO(), cs, *indxDesc, *indxDesc, ref, nil, func(manifest *ocispecv1.Manifest) error {
		if manifest.Annotations == nil {
			manifest.Annotations = make(map[string]string)
		}
		manifest.Annotations["test-annotation"] = "test-value"
		return nil
	})
	require.NoError(t, err, "failed to update manifest")

	manifest, err := images.Manifest(context.TODO(), cs, *indxDesc, nil)
	require.NoError(t, err, "failed to get manifest")
	assert.EqualValues(t, map[string]string{"test-annotation": "test-value"}, manifest.Annotations)
}

func TestUpdateImageConfig(t *testing.T) {
	ref := image.SimpleReference("test-repo-1")
	b := NewBuilder(digest.SHA256)
	co := BuildConfig{
		OmitTimestamp: true,
	}
	cs := fakestore.NewFakeContentStore()
	confDesc, data, err := b.newConfig(co)
	require.NoError(t, err, "failed to create config")

	err = writeBlob(cs, ref.String(), *confDesc, data)
	require.NoError(t, err, "failed to write config")

	desc, _, diffID, err := b.newLayer(true, nil)
	require.NoError(t, err, "failed to create layer")

	addLayer(desc, diffID)(&co)
	manifest := &ocispecv1.Manifest{
		Versioned: ocispec.Versioned{
			SchemaVersion: ociSchemaVersion,
		},
		Config:      *confDesc,
		Layers:      []ocispecv1.Descriptor{},
		Annotations: map[string]string{},
	}
	err = b.updateImageConfig(context.TODO(), cs, ref, manifest, co)
	require.NoError(t, err, "failed to update config")

	assert.EqualValues(t, []ocispecv1.Descriptor{*desc}, manifest.Layers)
	p, err := content.ReadBlob(context.TODO(), cs, manifest.Config)
	require.NoError(t, err, "Failed to read config blob")

	var config ocispecv1.Image
	err = json.Unmarshal(p, &config)
	require.NoError(t, err, "Failed to unmarshal config blob")

	assert.EqualValues(t, []digest.Digest{diffID}, config.RootFS.DiffIDs)
}

func setupRegistry(t *testing.T) (r *Registry, cleanup func()) {
	cacheDir, err := ioutil.TempDir("", "cache-*")
	require.NoError(t, err, "failed to create temp cache dir")
	cleanup = func() {
		assert.NoError(t, os.RemoveAll(cacheDir), "tmpdir cleanup failed")
		require.NoError(t, r.Destroy(), "registry cleanup failed")
	}

	logrus.SetLevel(logrus.DebugLevel)

	r, err = NewRegistry(
		WithLog(logrus.New().WithField("test", t.Name())),
		WithCacheDir(cacheDir),
		SkipTLS(true),
	)
	require.NoError(t, err, "failed to create registry")

	r.Store = &store{
		cs: fakestore.NewFakeContentStore(),
		is: fakestore.NewFakeImageStore(),
	}

	return r, cleanup
}
