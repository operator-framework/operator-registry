package containerdregistry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	r.init(dir, ref)
	return nil
}

// Pack calculates the diff of a directory from the recorded values, makes a new layer from this diff
// and updates the referenced image. The diff is calculated based on the file headers and hashes stored
// during an unpack. Attempting a Repack on a directory without running unpack first will add the directory
// to the image as a new layer.
func (r *Registry) Pack(ctx context.Context, ref image.Reference, dir string, opts ...BuildOpt) error {
	ctx = ensureNamespace(ctx)

	srcs, err := r.diff(dir, ref)
	if err != nil && err.Error() != errNotUnpacked.Error() {
		return err
	}

	if len(srcs) > 0 {
		layerDesc, layerBytes, layerDiffID, err := r.builder.NewLayer(true, srcs)
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
func (r *Registry) init(root string, ref image.Reference) {
	paths := make(map[string]*fileInfo)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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

		h := r.builder.digester.Hash()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}

		paths[path].hash = fmt.Sprintf("%x", h.Sum(nil))
		h.Reset()
		return nil
	})
	r.builder.buildRoot[ref] = paths
}

// compare a directory with the file header and hash information recorded during unpack
func (r *Registry) diff(dir string, ref image.Reference) (map[string]string, error) {
	srcs := make(map[string]string)
	cmpRoot, isUnpacked := r.builder.buildRoot[ref]
	if !isUnpacked {
		srcs[dir] = dir
		return srcs, errNotUnpacked
	}
	found := make(map[string]bool)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		found[path] = true
		f, ok := cmpRoot[path]
		if !ok {
			// new
			srcs[path] = path
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !os.SameFile(info, f.info) {
			// modified
			srcs[path] = path
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
		h := r.builder.digester.Hash()
		if _, err := io.Copy(h, f2); err != nil {
			return err
		}

		// different content
		if fmt.Sprintf("%x", h.Sum(nil)) != f.hash {
			srcs[path] = path
		}
		h.Reset()
		return nil
	})
	// add whiteouts:
	for p := range cmpRoot {
		if !found[p] {
			dir, f := filepath.Split(filepath.Clean(p))
			if len(f) == 0 || f == "." {
				whFile := filepath.Join(dir, whOpaque)
				srcs[whFile] = whFile
				continue
			}
			f = fmt.Sprintf("%s%s", whPrefix, f)
			if f == whOpaque {
				return srcs, fmt.Errorf("cannot add whiteout for file %s: resulting opaque whiteout %s will delete directory contents", p, filepath.Join(dir, f))
			}
			whFile := filepath.Join(dir, f)
			srcs[whFile] = whFile
		}
	}
	return srcs, err
}
