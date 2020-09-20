package containerdregistry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/content"
	"github.com/operator-framework/operator-registry/pkg/image"
)

var errNotUnpacked error = fmt.Errorf("directory not unpacked")

// Unpack writes the unpackaged content of an image to a directory.
// If the referenced image does not exist in the registry, an error is returned.
func (r *Registry) Unpack(ctx context.Context, ref image.Reference, dir string) error {
	// Set the default namespace if unset
	ctx = ensureNamespace(ctx)

	manifest, err := r.getManifest(ctx, ref)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	for _, layer := range manifest.Layers {
		r.log.Infof("unpacking layer: %v", layer)
		if err := r.unpackLayer(ctx, layer, dir); err != nil {
			return err
		}
	}
	err = r.init(dir, ref)
	return err
}

// Pack calculates the diff of a directory from the recorded values, makes a new layer from this diff
// and updates the referenced image. The diff is calculated based on the file headers and hashes stored
// during an unpack. Attempting a Repack on a directory without running unpack first will add the directory
// to the image as a new layer. If Pack was previously run on the directory, only the diff will be added as
// as a layer.
//
// for instance, r.Pack(ctx, ref, "foo") will create the following addition if 'foo' is not an unpacked directory:
// └── rootfs
//     └── foo
//         └── bar
//
// if 'foo' was already unpacked, then only the contents of the diff would be added
// └── rootfs
//     :
//     ├── bar
//     :
func (r *Registry) Pack(ctx context.Context, ref image.Reference, dir string, opts ...BuildOpt) error {
	ctx = ensureNamespace(ctx)

	srcs, err := r.diff(dir, "", ref)
	if err != nil && err.Error() != errNotUnpacked.Error() {
		return err
	}

	if len(srcs) > 0 {
		layerDesc, layerBytes, layerDiffID, err := r.builder(ref).newLayer(true, srcs)
		if err != nil {
			return fmt.Errorf("could not create new layer for repack: %v", err)
		}

		if err := content.WriteBlob(ctx, r.Content(), ref.String(), bytes.NewBuffer(layerBytes), *layerDesc); err != nil {
			return fmt.Errorf("error writing blob: %v", err)
		}

		opts = append(opts, addLayer(layerDesc, layerDiffID))
	}

	return r.NewImage(ctx, ref, opts...)
}

// cache file hashes and stat after unpack for comparison when repacking the image
func (r *Registry) init(root string, ref image.Reference) error {
	paths := make(map[string]*fileInfo)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if path == root {
			return nil
		}
		if strings.HasPrefix(info.Name(), WhPrefix) {
			return fmt.Errorf("Illegal whiteout prefix for file: %v", path)
		}
		paths[path] = &fileInfo{
			info: info,
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		h := r.builder(ref).digester.Hash()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}

		paths[path].hash = fmt.Sprintf("%x", h.Sum(nil))
		h.Reset()
		return nil
	})
	r.builder(ref).buildRoot[root] = paths
	return err
}

// compare a directory with the file header and hash information recorded during unpack.
// dstRoot is the target directory to which the root will be copied, in case of a non-unpacked
// root. An empty dstRoot will always pack the directory onto the root of the image.
func (r *Registry) diff(root, dstRoot string, ref image.Reference) (map[string]string, error) {
	srcs := make(map[string]string)
	cmpRoot, isUnpacked := r.builder(ref).buildRoot[root]
	if !isUnpacked {
		dst := root
		if root != dstRoot {
			base := filepath.Base(filepath.Clean(root))
			dst = filepath.Join(dstRoot, base)
		}
		srcs[root] = dst
		return srcs, errNotUnpacked
	}
	found := make(map[string]bool)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if path == root {
			return nil
		}
		dst, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		found[path] = true
		f, ok := cmpRoot[path]
		if !ok {
			// new
			srcs[path] = dst
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !os.SameFile(info, f.info) {
			// modified
			srcs[path] = dst
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		f2, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f2.Close()
		h := r.builder(ref).digester.Hash()
		if _, err := io.Copy(h, f2); err != nil {
			return err
		}

		// different content
		if fmt.Sprintf("%x", h.Sum(nil)) != f.hash {
			srcs[path] = dst
		}
		h.Reset()
		return nil
	})
	// add whiteouts:
	for p := range cmpRoot {
		if !found[p] {
			dir, f := filepath.Split(strings.TrimRight(filepath.Clean(p), "/"))
			if len(f) == 0 || f == "." {
				whFile := filepath.Join(dir, WhOpaque)
				srcs[whFile] = whFile
				continue
			}
			f = fmt.Sprintf("%s%s", WhPrefix, f)
			if f == WhOpaque {
				return srcs, fmt.Errorf("cannot add whiteout for file %s: resulting opaque whiteout %s will delete directory contents", p, filepath.Join(dir, f))
			}
			dstDir, err := filepath.Rel(root, dir)
			if err != nil {
				return srcs, err
			}
			whFile := filepath.Join(dstDir, f)
			srcs[whFile] = whFile
		}
	}
	return srcs, err
}
