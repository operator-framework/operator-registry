package containerdregistry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/operator-framework/operator-registry/pkg/image"
)

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

// Repack calculates the diff of a directory from the recorded values, makes a new layer from this diff
// and updates the referenced image. The diff is calculated based on the file headers and hashes stored
// during an unpack. Attempting a Repack on a directory without running Unpack on it first will fail.
func (r *Registry) Repack(ctx context.Context, ref image.Reference, dir string, opts ...BuildOpt) error {
	ctx = ensureNamespace(ctx)

	srcs, err := r.diff(dir, ref)
	if err != nil {
		return err
	}

	if len(srcs) == 0 {
		return nil
	}

	layerDesc, layerBytes, layerDiffID, err := r.builder.newLayer(true, srcs...)
	if err != nil {
		return fmt.Errorf("could not create new layer for repack: %v", err)
	}

	if err := content.WriteBlob(ctx, r.Content(), ref.String(), bytes.NewBuffer(layerBytes), *layerDesc); err != nil {
		return fmt.Errorf("error writing blob: %v", err)
	}

	opts = append(opts, addLayer(layerDesc, layerDiffID))
	buildConfig := DefaultBuildConfig()
	for _, opt := range opts {
		opt(buildConfig)
	}

	indexImg, err := r.Images().Get(ctx, ref.String())
	if err != nil {
		return fmt.Errorf("error fetching image %s: %v", ref, err)
	}

	indexDesc, err := r.builder.updateManifests(ctx, r.Content(), indexImg.Target, indexImg.Target, ref, r.platform, func(manifest *ocispec.Manifest) error {
		return r.builder.updateImageConfig(ctx, r.Content(), ref, manifest, *buildConfig)
	})
	if err != nil {
		return fmt.Errorf("error updating manifests: %v", err)
	}

	newImg := images.Image{
		Name:   indexImg.Name,
		Target: *indexDesc,
	}

	if _, err = r.Images().Create(ctx, newImg); err != nil {
		if errdefs.IsAlreadyExists(err) {
			_, err = r.Images().Update(ctx, newImg)
		}
	}
	if err != nil {
		return fmt.Errorf("error updating image for %s: %v", ref, err)
	}
	return nil
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
func (r *Registry) diff(dir string, ref image.Reference) ([]string, error) {
	srcs := make([]string, 0)
	cmpRoot, isUnpacked := r.builder.buildRoot[ref]
	if !isUnpacked {
		return srcs, fmt.Errorf("repack on a non-unpacked directory")
	}
	found := make(map[string]bool)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		found[path] = true
		f, ok := cmpRoot[path]
		if !ok {
			// new
			srcs = append(srcs, path)
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !os.SameFile(info, f.info) {
			// modified
			srcs = append(srcs, path)
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

		if fmt.Sprintf("%x", h.Sum(nil)) != f.hash {
			srcs = append(srcs, path)
		}
		h.Reset()
		return nil
	})
	// add whiteouts:
	for p := range cmpRoot {
		if !found[p] {
			dir, f := filepath.Split(filepath.Clean(p))
			if len(f) == 0 || f == "." {
				srcs = append(srcs, filepath.Join(dir, whOpaque))
				continue
			}
			f = fmt.Sprintf("%s%s", whPrefix, f)
			if f == whOpaque {
				return srcs, fmt.Errorf("cannot add whiteout for file %s: resulting opaque whiteout %s will delete directory contents", p, filepath.Join(dir, f))
			}
			srcs = append(srcs, filepath.Join(dir, f))
		}
	}
	return srcs, err
}
