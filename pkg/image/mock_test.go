package image

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestMockRegistry(t *testing.T) {
	exists := SimpleReference("exists")
	dne := SimpleReference("dne")
	ctx := context.Background()

	tmpDir, err := ioutil.TempDir("", "reg-test-mock-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	r := MockRegistry{
		RemoteImages: map[Reference]*MockImage{
			exists: &MockImage{
				Labels: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
				FS: fstest.MapFS{
					"file1": &fstest.MapFile{
						Data: []byte("data1"),
					},
					"subdir/file2": &fstest.MapFile{
						Data: []byte("data2"),
					},
				},
			},
		},
	}

	// Test pull of non-existent ref
	require.Error(t, r.Pull(ctx, dne))

	// Test unpack and labels of unpulled ref
	require.Error(t, r.Unpack(ctx, exists, tmpDir))
	_, err = r.Labels(ctx, exists)
	require.Error(t, err)

	// Test pull of existing ref
	require.NoError(t, r.Pull(ctx, exists))

	// Test unpack and labels of existing ref
	require.NoError(t, r.Unpack(ctx, exists, tmpDir))
	checkFile(t, filepath.Join(tmpDir, "file1"))
	checkFile(t, filepath.Join(tmpDir, "subdir", "file2"))

	labels, err := r.Labels(ctx, exists)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"key1": "value1", "key2": "value2"}, labels)

	// Test destroy
	require.NoError(t, r.Destroy())

	// Test unpack and labels of unpulled ref
	require.Error(t, r.Unpack(ctx, exists, tmpDir))
	_, err = r.Labels(ctx, exists)
	require.Error(t, err)
}

func checkFile(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	require.NoError(t, err)
}
