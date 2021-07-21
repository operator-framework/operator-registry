package builder

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"io"
	"os"
	"path/filepath"
	"time"
)

func writeToTar(w *tar.Writer, name string, data []byte, ts time.Time, isDir bool) error {
	name = filepath.Clean(name)
	var fType byte = tar.TypeReg
	var perm int64 = 0644
	if isDir {
		fType = tar.TypeDir
		perm = 0755
		name = fmt.Sprintf("%s%c", name,os.PathSeparator)
	}
	hdr := tar.Header{
		Typeflag:   fType,
		Name:       name,
		Size:       int64(len(data)),
		Mode:       perm,
		ModTime:    ts,
		AccessTime: ts,
		ChangeTime: ts,
		Format:     tar.FormatPAX,
	}
	err := w.WriteHeader(&hdr)
	if err != nil {
		return err
	}
	if fType == tar.TypeReg {
		_, err := io.Copy(w, bytes.NewReader(data))
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *imageBuilder) ExportDescToOCIArchive(ctx context.Context, desc ocispec.Descriptor, dst string) error {
	ts := time.Now()
	var buf bytes.Buffer
	tarWriter := tar.NewWriter(&buf)

	err := writeToTar(tarWriter, "blobs", nil, ts, true)
	if err != nil {
		return err
	}
	layoutFile := ocispec.ImageLayout{
		Version: ocispec.ImageLayoutVersion,
	}
	layoutData, err := json.Marshal(layoutFile)
	if err != nil {
		return err
	}
	err = writeToTar(tarWriter, ocispec.ImageLayoutFile, layoutData, ts, false)
	if err != nil {
		return err
	}

	indexJSON := ocispec.Index{
		Versioned:   specs.Versioned{
			SchemaVersion: 2,
		},
		Manifests: []ocispec.Descriptor{desc},
	}
	indexJSONData, err := json.Marshal(indexJSON)
	if err != nil {
		return err
	}
	err = writeToTar(tarWriter, filepath.Join("index.json"), indexJSONData, ts, false)

	paths := map[string]struct{}{}
	if err := images.Walk(ctx, images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		algDir := filepath.Join("blobs", desc.Digest.Algorithm().String())
		if _, ok := paths[algDir]; !ok {
			err := writeToTar(tarWriter, algDir, nil, ts, true)
			if err != nil {
				return nil, err
			}
			paths[algDir] = struct{}{}
		}

		descFile := filepath.Join(algDir, desc.Digest.Encoded())
		descBlob, err := content.ReadBlob(ctx, i.registry.Content(), desc)
		if err != nil {
			return nil, fmt.Errorf("failed to read descriptor %v", err)
		}

		err = writeToTar(tarWriter, descFile, descBlob, ts, false)
		if err != nil {
			return nil, err
		}

		switch desc.MediaType {
		case images.MediaTypeDockerSchema2ManifestList, ocispec.MediaTypeImageIndex:
			index := ocispec.Index{}
			if err := json.Unmarshal(descBlob, &index); err != nil {
				return nil, err
			}
			return index.Manifests, nil
		case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
			manifest := ocispec.Manifest{}
			if err := json.Unmarshal(descBlob, &manifest); err != nil {
				return nil, err
			}
			return append(manifest.Layers, manifest.Config), nil
		}
		return nil, nil
	}), desc); err != nil {
		return err
	}
	err = tarWriter.Close()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(filepath.Join(filepath.Dir(dst), fmt.Sprintf("%s.tar", filepath.Base(dst))), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	defer f.Close()
	if err != nil {
		return err
	}
	_, err = io.Copy(f, &buf)
	return err
}

func (i *imageBuilder) ExportImageToOCIArchive(ctx context.Context, dst string) error {
	ctx = ensureNamespace(ctx)
	return i.ExportDescToOCIArchive(ctx, *i.head, dst)
}

