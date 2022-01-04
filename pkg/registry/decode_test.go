package registry

import (
	"io"
	"os"
	"testing"
	"testing/fstest"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDecodeUnstructured(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		assertFunc func(t *testing.T, objGot *unstructured.Unstructured, errGot error)
	}{
		{
			name: "ValidObjectWithKind",
			file: "testdata/valid-unstructured.yaml",
			assertFunc: func(t *testing.T, objGot *unstructured.Unstructured, errGot error) {
				assert.NoError(t, errGot)
				assert.NotNil(t, objGot)

				assert.Equal(t, "FooKind", objGot.GetKind())
			},
		},

		{
			name: "InvalidObjectWithoutKind",
			file: "testdata/invalid-unstructured.yaml",
			assertFunc: func(t *testing.T, objGot *unstructured.Unstructured, errGot error) {
				assert.Error(t, errGot)
				assert.Nil(t, objGot)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := loadFile(t, tt.file)

			objGot, errGot := DecodeUnstructured(reader)

			if tt.assertFunc != nil {
				tt.assertFunc(t, objGot, errGot)
			}
		})
	}
}

func TestDecodePackageManifest(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		assertFunc func(t *testing.T, packageManifestGot *PackageManifest, errGot error)
	}{
		{
			name: "WithValidObject",
			file: "testdata/valid-package-manifest.yaml",
			assertFunc: func(t *testing.T, packageManifestGot *PackageManifest, errGot error) {
				assert.NoError(t, errGot)
				assert.NotNil(t, packageManifestGot)

				assert.Equal(t, "foo", packageManifestGot.PackageName)
			},
		},

		{
			name: "WithoutPackageName",
			file: "testdata/invalid-package-manifest.yaml",
			assertFunc: func(t *testing.T, packageManifestGot *PackageManifest, errGot error) {
				assert.Error(t, errGot)
				assert.Nil(t, packageManifestGot)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := loadFile(t, tt.file)

			packageManifestGot, errGot := DecodePackageManifest(reader)

			if tt.assertFunc != nil {
				tt.assertFunc(t, packageManifestGot, errGot)
			}
		})
	}
}

func TestDecodeFileFS(t *testing.T) {
	type foo struct {
		Bar string
	}

	root := fstest.MapFS{
		"foo.yaml":   &fstest.MapFile{Data: []byte("bar: baz")},
		"multi.yaml": &fstest.MapFile{Data: []byte("bar: baz\n---\nfoo: bar")},
	}

	logger, logHook := test.NewNullLogger()
	entry := logger.WithFields(nil)

	var nilPtr *foo
	require.NoError(t, decodeFileFS(root, "foo.yaml", nilPtr, entry))
	require.Nil(t, nilPtr)
	require.Equal(t, 0, len(logHook.Entries))
	logHook.Reset()

	ptr := &foo{}
	require.NoError(t, decodeFileFS(root, "foo.yaml", ptr, entry))
	require.NotNil(t, ptr)
	require.Equal(t, "baz", ptr.Bar)
	require.Equal(t, 0, len(logHook.Entries))
	logHook.Reset()

	ptr = &foo{}
	require.NoError(t, decodeFileFS(root, "multi.yaml", ptr, entry))
	require.NotNil(t, ptr)
	require.Equal(t, "baz", ptr.Bar)
	require.Equal(t, 1, len(logHook.Entries))
	require.Equal(t, logrus.WarnLevel, logHook.LastEntry().Level)
	require.Equal(t, "found more than one document inside multi.yaml, using only the first one", logHook.LastEntry().Message)
	logHook.Reset()
}

func loadFile(t *testing.T, path string) io.Reader {
	reader, err := os.Open(path)
	require.NoError(t, err, "unable to load from file %s", path)

	return reader
}
