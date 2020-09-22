package containerdregistry

import (
	"archive/tar"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry/fakestore"
)

func TestSquashLayers(t *testing.T) {
	dir, err := ioutil.TempDir("", "tmpdir-*")
	require.NoError(t, err, "failed to create tmpdir")
	defer os.RemoveAll(dir)

	files := []string{"f1", "f2"}
	tgtTree := newTarTree()
	layers := []ocispecv1.Descriptor{}
	b := NewBuilder("")
	cs := fakestore.NewFakeContentStore()
	for _, v := range files {
		f1 := filepath.Join(dir, v)
		f, err := os.OpenFile(f1, os.O_RDONLY|os.O_CREATE|os.O_TRUNC, 0600)
		require.NoError(t, err, "failed to create testfile")
		f.Close()
		f1Stat, err := os.Stat(f1)
		modTime := time.Time{}
		if err == nil {
			modTime = f1Stat.ModTime().Round(time.Second)
		}
		tgtTree.add(&tarBuf{
			hdr: &tar.Header{
				Typeflag: tar.TypeReg,
				Name:     v,
				Mode:     0600,
				Uid:      os.Getuid(),
				Gid:      os.Getgid(),
				Format:   tar.FormatUSTAR,
				ModTime:  modTime,
			},
			children: []string{},
			data:     []byte{},
		}, time.Time{})
		layerDesc, layerBytes, _, err := b.newLayer(true, map[string]string{f1: v})
		require.NoError(t, err, "failed to create layer")

		err = writeBlob(cs, "test-ref", *layerDesc, layerBytes)
		require.NoError(t, err, "failed to write layer")

		layers = append(layers, *layerDesc)
	}

	layerDesc, layerBytes, _, err := b.squashLayers(context.TODO(), cs, layers, time.Time{})
	err = writeBlob(cs, "test-ref", *layerDesc, layerBytes)
	require.NoError(t, err, "failed to write squashed layer")

	srcTree := newTarTree()
	err = applyLayerInMemory(context.TODO(), srcTree, cs, *layerDesc, time.Time{})
	require.NoError(t, err, "failed to apply layer")
	assert.True(t, reflect.DeepEqual(srcTree, tgtTree), fmt.Sprintf("Unexpected file tree, expected: %+v; actual: %+v", tgtTree, srcTree))
}

func TestApplyLayerInMemory(t *testing.T) {
	dir, err := ioutil.TempDir("", "tmpdir-*")
	require.NoError(t, err, "failed to create tmpdir")
	defer os.RemoveAll(dir)

	err = os.MkdirAll(path.Join(dir, "whDir"), 0700)
	require.NoError(t, err, "failed to create testdir")
	f2 := filepath.Join(dir, "whDir", "f2")
	f, err := os.OpenFile(f2, os.O_RDONLY|os.O_CREATE|os.O_TRUNC, 0600)
	require.NoError(t, err, "failed to create testfile")
	f.Close()

	f2Stat, err := os.Stat(f2)
	modTime := time.Time{}
	if err == nil {
		modTime = f2Stat.ModTime().Round(time.Second)
	}

	srcTree := tarTree{
		entries: map[string]*tarBuf{
			"whFile": &tarBuf{
				hdr: &tar.Header{
					Typeflag: tar.TypeReg,
					Name:     "whFile",
					Mode:     0644,
				},
			},
			"whDir": &tarBuf{
				hdr: &tar.Header{
					Typeflag: tar.TypeDir,
					Name:     "whDir",
					Mode:     0755,
				},
				children: []string{"whDir/f1"},
			},
			"whDir/f1": &tarBuf{
				hdr: &tar.Header{
					Typeflag: tar.TypeReg,
					Name:     "whDir/f1",
					Mode:     0644,
				},
			},
		},
	}
	newLayer := []string{
		// whiteout must be removed
		fmt.Sprintf("%swhFile", WhPrefix),
		// opaque whiteout must remove contents and preserve directory
		fmt.Sprintf("whDir/%s", WhOpaque),
		// whiteout must not affect files in current layer
		"whDir/f2",
	}
	tgtTree := tarTree{
		entries: map[string]*tarBuf{
			"whDir": &tarBuf{
				hdr: &tar.Header{
					Typeflag: tar.TypeDir,
					Name:     "whDir",
					Mode:     0755,
				},
				children: []string{"whDir/f2"},
			},
			"whDir/f2": &tarBuf{
				hdr: &tar.Header{
					Typeflag: tar.TypeReg,
					Name:     "whDir/f2",
					Mode:     0600,
					ModTime:  modTime,
					Uid:      os.Getuid(),
					Gid:      os.Getgid(),
					Format:   tar.FormatUSTAR,
				},
				children: []string{},
				data:     []byte{},
			},
		},
	}

	b := NewBuilder("")
	cs := fakestore.NewFakeContentStore()

	layerMapping := make(map[string]string)
	for _, p := range newLayer {
		layerMapping[fmt.Sprintf("%s/%s", dir, p)] = p
	}
	layerDesc, layerBytes, _, err := b.newLayer(true, layerMapping)
	require.NoError(t, err, "failed to create layer")

	err = writeBlob(cs, "test-ref", *layerDesc, layerBytes)
	require.NoError(t, err, "failed to write layer")

	err = applyLayerInMemory(context.TODO(), &srcTree, cs, *layerDesc, time.Time{})
	require.NoError(t, err, "failed to apply layer")
	assert.True(t, reflect.DeepEqual(srcTree, tgtTree), fmt.Sprintf("Unexpected file tree, expected: %+v; actual: %+v", tgtTree, srcTree))
}
func TestTreeAdd(t *testing.T) {
	tests := []struct {
		base     *tarTree
		tb       *tarBuf
		expected *tarTree
		comment  string
	}{
		{
			base: &tarTree{
				entries: map[string]*tarBuf{},
			},
			tb: &tarBuf{
				hdr: &tar.Header{Name: "d1/d2/f2", Typeflag: tar.TypeDir, Mode: 0755},
			},
			expected: &tarTree{
				entries: map[string]*tarBuf{
					"d1": &tarBuf{
						hdr:      &tar.Header{Name: "d1", Typeflag: tar.TypeDir, Mode: 0755},
						children: []string{"d1/d2"},
					},
					"d1/d2": &tarBuf{
						hdr:      &tar.Header{Name: "d1/d2", Typeflag: tar.TypeDir, Mode: 0755},
						children: []string{"d1/d2/f2"},
					},
					"d1/d2/f2": &tarBuf{
						hdr: &tar.Header{Name: "d1/d2/f2", Typeflag: tar.TypeDir, Mode: 0755},
					},
				},
			},
			comment: "Must create ancestors",
		},
		{
			base: &tarTree{
				entries: map[string]*tarBuf{
					"d1": &tarBuf{
						hdr:      &tar.Header{Name: "d1", Typeflag: tar.TypeDir, Mode: 0755},
						children: []string{"d1/f1"},
					},
					"d1/f1": &tarBuf{
						hdr: &tar.Header{Name: "d1/f1", Typeflag: tar.TypeDir, Mode: 0755},
					},
				},
			},
			tb: &tarBuf{
				hdr: &tar.Header{Name: "d1/d2/f2", Typeflag: tar.TypeDir, Mode: 0755},
			},
			expected: &tarTree{
				entries: map[string]*tarBuf{
					"d1": &tarBuf{
						hdr:      &tar.Header{Name: "d1", Typeflag: tar.TypeDir, Mode: 0755},
						children: []string{"d1/f1", "d1/d2"},
					},
					"d1/f1": &tarBuf{
						hdr: &tar.Header{Name: "d1/f1", Typeflag: tar.TypeDir, Mode: 0755},
					},
					"d1/d2": &tarBuf{
						hdr:      &tar.Header{Name: "d1/d2", Typeflag: tar.TypeDir, Mode: 0755},
						children: []string{"d1/d2/f2"},
					},
					"d1/d2/f2": &tarBuf{
						hdr: &tar.Header{Name: "d1/d2/f2", Typeflag: tar.TypeDir, Mode: 0755},
					},
				},
			},
			comment: "Must merge entries in closest ancestor directory",
		},
		{
			base: &tarTree{
				entries: map[string]*tarBuf{
					"d1": &tarBuf{
						hdr:      &tar.Header{Name: "d1", Typeflag: tar.TypeDir, Mode: 0755},
						children: []string{"d1/d2"},
					},
					"d1/d2": &tarBuf{
						hdr:      &tar.Header{Name: "d1/d2", Typeflag: tar.TypeDir, Mode: 0755},
						children: []string{"d1/d2/f2"},
					},
					"d1/d2/f2": &tarBuf{
						hdr: &tar.Header{Name: "d1/d2/f2", Typeflag: tar.TypeDir, Mode: 0755},
					},
				},
			},
			tb: &tarBuf{
				hdr: &tar.Header{
					Typeflag: tar.TypeReg,
					Name:     "d1",
					Mode:     0755,
				},
			},
			expected: &tarTree{
				entries: map[string]*tarBuf{
					"d1": &tarBuf{
						hdr: &tar.Header{Name: "d1", Typeflag: tar.TypeReg, Mode: 0755},
					},
				},
			},
			comment: "Must delete children when overwriting directory with non-directory entry",
		},
	}

	for _, tt := range tests {
		err := tt.base.add(tt.tb, time.Time{})
		require.NoError(t, err, fmt.Sprintf("failed test %s", tt.comment))
		assert.True(t, reflect.DeepEqual(*tt.base, *tt.expected), fmt.Sprintf("failed test %s", tt.comment))
	}
}

func TestTreeDelete(t *testing.T) {
	tests := []struct {
		base     *tarTree
		tb       string
		expected *tarTree
		comment  string
	}{
		{
			base: &tarTree{
				entries: map[string]*tarBuf{
					"d1": &tarBuf{
						hdr:      &tar.Header{Name: "d1", Typeflag: tar.TypeDir, Mode: 0755},
						children: []string{"d1/d2", "d1/d3"},
					},
					"d1/d2": &tarBuf{
						hdr:      &tar.Header{Name: "d1/d2", Typeflag: tar.TypeDir, Mode: 0755},
						children: []string{"d1/d2/f2"},
					},
					"d1/d2/f2": &tarBuf{
						hdr: &tar.Header{Name: "d1/d2/f2", Typeflag: tar.TypeDir, Mode: 0755},
					},
					"d1/d3": &tarBuf{
						hdr:      &tar.Header{Name: "d1/d3", Typeflag: tar.TypeDir, Mode: 0755},
						children: []string{"d1/d3/f3"},
					},
					"d1/d3/f3": &tarBuf{
						hdr: &tar.Header{Name: "d1/d3/f3", Typeflag: tar.TypeDir, Mode: 0755},
					},
				},
			},
			tb: "d1/d3",
			expected: &tarTree{
				entries: map[string]*tarBuf{
					"d1": &tarBuf{
						hdr:      &tar.Header{Name: "d1", Typeflag: tar.TypeDir, Mode: 0755},
						children: []string{"d1/d2", "d1/d3"},
					},
					"d1/d2": &tarBuf{
						hdr:      &tar.Header{Name: "d1/d2", Typeflag: tar.TypeDir, Mode: 0755},
						children: []string{"d1/d2/f2"},
					},
					"d1/d2/f2": &tarBuf{
						hdr: &tar.Header{Name: "d1/d2/f2", Typeflag: tar.TypeDir, Mode: 0755},
					},
				},
			},
			comment: "Must delete descendants",
		},
	}
	for _, tt := range tests {
		tt.base.delete(tt.tb)
		assert.True(t, reflect.DeepEqual(tt.base, tt.expected))
	}
}

func writeBlob(cs content.Store, ref string, desc ocispecv1.Descriptor, data []byte) error {
	w, err := cs.Writer(context.TODO(), content.WithRef(ref), content.WithDescriptor(desc))
	if err != nil {
		return fmt.Errorf("failed to get content writer: %v", err)
	}
	n, err := w.Write(data)
	if err != nil || n != len(data) {
		return fmt.Errorf("failed to write blob data (%d/%d bytes written): %v", n, len(data), err)
	}

	if err := w.Commit(context.TODO(), int64(n), desc.Digest); err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return fmt.Errorf("failed commit for blob: %v", err)
		}
	}
	return nil
}
