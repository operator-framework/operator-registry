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

	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/opencontainers/go-digest"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type tarBuf struct {
	children *[]*tarBuf
	hdr      *tar.Header
	data     []byte
}

func deleteWhiteouts(tarTree map[string]tarBuf, whFiles ...*tarBuf) {
	for _, whf := range whFiles {
		name := filepath.Clean(whf.hdr.Name)
		if f, ok := tarTree[name]; ok {
			if f.hdr.Typeflag == tar.TypeDir && len(*f.children) > 0 {
				deleteWhiteouts(tarTree, *f.children...)
			}
			delete(tarTree, name)
		}
	}
}

func addToParent(tarTree map[string]tarBuf, name string, f *tarBuf) error {
	dir, _ := filepath.Split(filepath.Clean(name))
	if len(dir) == 0 || dir[len(dir)-1] != '/' {
		dir += "/"
	}
	if _, ok := tarTree[dir]; !ok {
		if dir == "./" || dir == "/" {
			return fmt.Errorf("missing root directory entry in tar archive")
		}
		// A nil child indicates a directory which will inherit
		// its header from its closest ancestor
		addToParent(tarTree, dir, nil)
	}
	switch tarTree[dir].hdr.Typeflag {
	case tar.TypeDir:
	case tar.TypeSymlink:
		// TODO(ankitathomas): follow symlink to the end and ensure the parent
		// is a directory. Currently, we optimistically assume this.
		// we may have a symlinkA -> symlinkB -> dir, with symlinkB not created yet
		// Adding symlinkB temporarily as a dir would drop all its children once
		// the actual symlinkB header was processed.
	default:
		return fmt.Errorf("unexpected parent type in tar archive")
	}
	if f == nil {
		hdr, err := tar.FileInfoHeader(tarTree[dir].hdr.FileInfo(), "")
		if err != nil {
			return fmt.Errorf("error creating tar header: %v", err)
		}
		if len(name) == 0 || name[len(name)-1] != '/' {
			name += "/"
		}
		hdr.Name = name
		if hdr.Typeflag != tar.TypeDir {
			hdr.Linkname = ""
			hdr.Typeflag = tar.TypeDir
			hdr.Mode = int64(hdr.FileInfo().Mode() &^ 07777)
		}
		children := make([]*tarBuf, 0)
		f = &tarBuf{
			hdr:      hdr,
			children: &children,
		}
	}

	if tarTree[dir].children == nil {
		children := make([]*tarBuf, 0)
		*tarTree[dir].children = children
	}
	*tarTree[dir].children = append(*tarTree[dir].children, f)
	return nil
}

func (b *builder) squashLayers(ctx context.Context, cs content.Store, layers []ocispecv1.Descriptor) (*ocispecv1.Descriptor, []byte, digest.Digest, error) {
	tarTree := make(map[string]tarBuf)
	var diffID digest.Digest
	var buf bytes.Buffer
	for _, l := range layers {
		err := applyLayerInMemory(ctx, tarTree, cs, l)
		if err != nil {
			return nil, nil, diffID, err
		}
	}
	tarList := make([]tarBuf, 0)
	for _, tb := range tarTree {
		tarList = append(tarList, tb)
	}
	sort.SliceStable(tarList, func(i, j int) bool {
		return tarList[i].hdr.Name < tarList[j].hdr.Name
	})
	// convert the file into descriptor + diffID

	gzipWriter := gzip.NewWriter(&buf)

	tarWriter := tar.NewWriter(io.MultiWriter(gzipWriter, b.digester.Hash()))

	for _, tb := range tarList {
		name := filepath.Clean(tb.hdr.Name)
		if strings.HasPrefix(filepath.Base(name), whPrefix) {
			// This should never happen since we do not addd whiteouts to the final file set
			return nil, nil, diffID, fmt.Errorf("error adding %s to merged layer: disallowed whiteout prefix %s", name, whPrefix)
		}
		hdr := tb.hdr
		hdr.Uname = ""
		hdr.Gname = ""
		name = strings.TrimPrefix(name, "/")
		if hdr.Typeflag == tar.TypeDir {
			name += "/"
		}
		hdr.Name = name

		if err := tarWriter.WriteHeader(hdr); err != nil {
			return nil, nil, diffID, fmt.Errorf("error writing file header for %s to archive: %v", name, err)
		}

		if hdr.Typeflag == tar.TypeReg {
			n, err := tarWriter.Write(tb.data)
			if err != nil || int64(n) != hdr.Size {
				return nil, nil, diffID, fmt.Errorf("error copying %s to archive (%d/%d bytes written): %v", name, n, hdr.Size, err)
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

func applyLayerInMemory(ctx context.Context, tarTree map[string]tarBuf, cs content.Store, layer ocispecv1.Descriptor) error {
	ra, err := cs.ReaderAt(ctx, layer)
	if err != nil {
		return err
	}
	defer ra.Close()

	// TODO(njhale): Chunk layer reading
	decompressed, err := compression.DecompressStream(io.NewSectionReader(ra, 0, ra.Size()))
	if err != nil {
		return err
	}

	tr := tar.NewReader(decompressed)

	newFiles := make([]tarBuf, 0)
	var size int64
	// Read all the headers
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return err
		}

		size += hdr.Size
		dir, base := filepath.Split(filepath.Clean(hdr.Name))
		if len(dir) == 0 || dir[len(dir)-1] != '/' {
			dir += "/" // trailing slashes for compatibility with older tar formats
		}

		if base == whOpaque {
			if whDir, ok := tarTree[dir]; ok && whDir.hdr.Typeflag == tar.TypeDir && len(*whDir.children) > 0 {
				deleteWhiteouts(tarTree, *whDir.children...)
			}
			continue
		}
		if strings.HasPrefix(base, whPrefix) {
			if whFile, ok := tarTree[filepath.Clean(hdr.Name)]; ok {
				deleteWhiteouts(tarTree, &whFile)
			}
			continue
		}

		children := make([]*tarBuf, 0)
		tbuf := &tarBuf{
			hdr:      hdr,
			children: &children,
		}

		if hdr.Typeflag == tar.TypeReg || hdr.Typeflag == tar.TypeRegA {
			buf := new(bytes.Buffer)
			n, err := io.Copy(buf, tr)
			if err != nil || n != hdr.Size {
				return fmt.Errorf("error reading tar archive (%d/%d bytes read): %v", n, hdr.Size, err)
			}
			tbuf.data = buf.Bytes()
		}
		newFiles = append(newFiles, *tbuf)
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
		namei := filepath.Clean(newFiles[i].hdr.Name)
		namej := filepath.Clean(newFiles[j].hdr.Name)
		pathi := strings.Split(namei, "/")
		pathj := strings.Split(namej, "/")
		if len(pathi) != len(pathj) {
			// order based on directory depth
			return len(pathi) < len(pathj)
		}
		return namei < namej
	})
	// done processing whiteouts, now we can apply the new files
	for _, newFile := range newFiles {
		name := filepath.Clean(newFile.hdr.Name)
		if oldFile, ok := tarTree[name]; ok {
			// the new entry overwrites an entry from one of the lower layers
			if oldFile.hdr.Typeflag != newFile.hdr.Typeflag || newFile.hdr.Typeflag != tar.TypeDir {
				// preserve children only when old and new entries are both directories
				if len(*oldFile.children) > 0 {
					deleteWhiteouts(tarTree, *oldFile.children...)
				}
			}
			tarTree[name] = newFile
			addToParent(tarTree, name, &newFile)
		}
	}
	return nil
}
