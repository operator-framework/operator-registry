package containerdregistry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/opencontainers/go-digest"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type tarBuf struct {
	children []string
	hdr      *tar.Header
	data     []byte
}

type tarTree struct {
	entries map[string]*tarBuf
}

func newTarTree() *tarTree {
	return &tarTree{
		entries: make(map[string]*tarBuf),
	}
}

func (t *tarTree) add(tb *tarBuf, modTime time.Time) error {
	if tb == nil {
		return nil
	}
	name := filepath.Clean(tb.hdr.Name)
	if old := t.get(name); old != nil && len(old.children) > 0 {
		// preserve children only when old and new entries are both directories
		if old.hdr.Typeflag == tar.TypeDir && tb.hdr.Typeflag == tar.TypeDir {
			tb.children = append(tb.children, old.children...)
		} else {
			t.delete(old.children...)
		}
	}
	t.entries[name] = tb

	dir := filepath.Clean(filepath.Dir(name))
	for len(dir) > 0 && dir != "/" && dir != "." {
		if dirEntry := t.get(dir); dirEntry != nil {
			if dirEntry.hdr.Typeflag != tar.TypeDir {
				return fmt.Errorf("unexpected parent type in tar archive")
			}
			if dirEntry.children == nil {
				dirEntry.children = make([]string, 0)
			}
			dirEntry.children = append(dirEntry.children, name)
			return nil
		}
		t.entries[dir] = &tarBuf{
			hdr: &tar.Header{
				Typeflag: tar.TypeDir,
				Name:     dir,
				Mode:     0755,
				Uid:      tb.hdr.Uid,
				Gid:      tb.hdr.Gid,
				ModTime:  modTime,
			},
			children: []string{name},
		}
		name = dir
		dir = filepath.Dir(name)
	}
	return nil
}

func (t *tarTree) delete(files ...string) {
	count := len(files)
	for i := 0; i < count; i++ {
		name := filepath.Clean(files[i])
		if f := t.get(name); f != nil {
			delete(t.entries, name)
			if f.hdr.Typeflag == tar.TypeDir && len(f.children) > 0 {
				files = append(files, f.children...)
				count = len(files)
			}
		}
	}
}

func (t *tarTree) get(name string) *tarBuf {
	return t.entries[name]
}

func (b *builder) squashLayers(ctx context.Context, cs content.Store, layers []ocispecv1.Descriptor, modTime time.Time) (*ocispecv1.Descriptor, []byte, digest.Digest, error) {
	tree := newTarTree()
	var diffID digest.Digest
	var buf bytes.Buffer
	for _, l := range layers {
		err := applyLayerInMemory(ctx, tree, cs, l, modTime)
		if err != nil {
			return nil, nil, diffID, err
		}
	}
	tarList := make([]*tarBuf, 0)
	for _, tb := range tree.entries {
		tarList = append(tarList, tb)
	}
	sort.SliceStable(tarList, func(i, j int) bool {
		return tarList[i].hdr.Name < tarList[j].hdr.Name
	})

	// convert the file into descriptor + diffID
	gzipWriter := gzip.NewWriter(&buf)

	tarWriter := tar.NewWriter(io.MultiWriter(gzipWriter, b.digester.Hash()))
	for _, tb := range tarList {
		if strings.HasPrefix(filepath.Base(filepath.Clean(tb.hdr.Name)), WhPrefix) {
			// This should never happen since we do not add whiteouts to the final file set
			return nil, nil, diffID, fmt.Errorf("error adding %s to merged layer: disallowed whiteout prefix %s", tb.hdr.Name, WhPrefix)
		}
		hdr := tb.hdr
		hdr.Uname = ""
		hdr.Gname = ""
		if err := tarWriter.WriteHeader(hdr); err != nil {
			return nil, nil, diffID, fmt.Errorf("error writing file header for %s to archive: %v", tb.hdr.Name, err)
		}

		if hdr.Typeflag == tar.TypeReg {
			n, err := tarWriter.Write(tb.data)
			if err != nil || int64(n) != hdr.Size {
				return nil, nil, diffID, fmt.Errorf("error copying %s to archive (%d/%d bytes written): %v", tb.hdr.Name, n, hdr.Size, err)
			}
		}
	}

	if err := tarWriter.Close(); err != nil {
		return nil, nil, diffID, err
	}

	if err := gzipWriter.Close(); err != nil {
		return nil, nil, diffID, err
	}

	// DiffID is the digest over the uncompressed tar archive
	diffID = b.digester.Digest()
	b.digester.Hash().Reset()

	layerBytes := buf.Bytes()
	size, err := b.digester.Hash().Write(layerBytes)
	if err != nil {
		return nil, nil, diffID, err
	}

	// digest is done over the final layer's contents. This may or may not be the same as diffID depending on compression used.
	digest := b.digester.Digest()
	b.digester.Hash().Reset()

	return &ocispecv1.Descriptor{
		MediaType:   ocispecv1.MediaTypeImageLayerGzip,
		Digest:      digest,
		Size:        int64(size),
		Annotations: map[string]string{},
		Platform: &ocispecv1.Platform{
			Architecture: runtime.GOARCH,
			OS:           runtime.GOOS,
		},
	}, layerBytes, diffID, nil
}

func applyLayerInMemory(ctx context.Context, tree *tarTree, cs content.Store, layer ocispecv1.Descriptor, modTime time.Time) error {
	ra, err := cs.ReaderAt(ctx, layer)
	if err != nil {
		return fmt.Errorf("reader error: %v", err)
	}
	defer ra.Close()

	// TODO(njhale): Chunk layer reading
	decompressed, err := compression.DecompressStream(io.NewSectionReader(ra, 0, ra.Size()))
	if err != nil {
		return fmt.Errorf(": %v", err)
	}

	tr := tar.NewReader(decompressed)

	newFiles := make([]*tarBuf, 0)
	var size int64
	// Read all the headers
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context error: %v", ctx.Err())
		default:
		}

		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return fmt.Errorf("error reading archive: %v", err)
		}

		size += hdr.Size
		dir, base := filepath.Split(filepath.Clean(hdr.Name))
		if base == WhOpaque {
			if whDir := tree.get(filepath.Clean(dir)); whDir != nil && whDir.hdr.Typeflag == tar.TypeDir && len(whDir.children) > 0 {
				tree.delete(whDir.children...)
				whDir.children = []string{}
			}
			continue
		}
		if strings.HasPrefix(base, WhPrefix) {
			tree.delete(filepath.Join(dir, strings.TrimPrefix(base, WhPrefix)))
			continue
		}

		tbuf := &tarBuf{
			hdr:      hdr,
			children: make([]string, 0),
		}

		if hdr.Typeflag == tar.TypeReg || hdr.Typeflag == tar.TypeRegA {
			buf := new(bytes.Buffer)
			n, err := io.Copy(buf, tr)
			if err != nil || n != hdr.Size {
				return fmt.Errorf("error reading tar archive (%d/%d bytes read): %v", n, hdr.Size, err)
			}
			tbuf.data = buf.Bytes()
		}
		newFiles = append(newFiles, tbuf)
	}
	hdrOrder := map[byte]int{
		// create directories first to ensure parents
		tar.TypeDir: -2,
		// links processed last to ensure their source exists (except in the symlink->symlink case)
		tar.TypeSymlink: 1,
		tar.TypeLink:    2,
	}
	sort.SliceStable(newFiles, func(i, j int) bool {
		if hdrOrder[newFiles[i].hdr.Typeflag] != hdrOrder[newFiles[j].hdr.Typeflag] {
			return hdrOrder[newFiles[i].hdr.Typeflag] < hdrOrder[newFiles[j].hdr.Typeflag]
		}
		pathi := strings.Split(newFiles[i].hdr.Name, "/")
		pathj := strings.Split(newFiles[j].hdr.Name, "/")
		if len(pathi) != len(pathj) {
			// order based on directory depth
			return len(pathi) < len(pathj)
		}
		return newFiles[i].hdr.Name < newFiles[j].hdr.Name
	})
	// done processing whiteouts, now we can apply the new files
	for _, newFile := range newFiles {
		tree.add(newFile, modTime)
	}
	return nil
}
