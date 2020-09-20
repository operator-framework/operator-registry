package containerdregistry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/operator-framework/operator-registry/pkg/image"
)

const (
	emptyBaseImage   = "scratch"
	rootFSTypeLayers = "layers"
	ociSchemaVersion = 2
	// WhPrefix is the reserved prefix for whiteout files
	WhPrefix = ".wh."
	// WhOpaque is the reserved name for an opaque whiteout.
	WhOpaque = ".wh..wh..opq"
)

// NewImage creates a new image from the given tag and build options.
// If the image already exists, NewImage applies the build opts to the image config
func (r *Registry) NewImage(ctx context.Context, imageTag image.Reference, opts ...BuildOpt) error {
	ctx = ensureNamespace(ctx)

	if len(imageTag.String()) == 0 {
		return fmt.Errorf("imageTag must not be empty")
	}
	buildConfig := DefaultBuildConfig()
	for _, opt := range opts {
		opt(buildConfig)
	}
	var baseDesc *ocispecv1.Descriptor
	var err error
	if len(buildConfig.BaseImage.String()) != 0 && buildConfig.BaseImage.String() != emptyBaseImage {
		// pull the base image
		if err := r.Pull(ctx, buildConfig.BaseImage); err != nil {
			return fmt.Errorf("failed to pull base image %s: %v", buildConfig.BaseImage.String(), err)
		}

		img, err := r.Images().Get(ctx, buildConfig.BaseImage.String())
		if err != nil {
			return fmt.Errorf("unable to find pulled base image %s: %v", buildConfig.BaseImage.String(), err)
		}
		baseDesc = &img.Target
		// nil manifest is not accepted by dockerhub, so create an empty one if it doesn't exist
		_, err = r.getManifest(ctx, buildConfig.BaseImage)
	} else {
		var img images.Image
		// if the image
		img, err = r.Images().Get(ctx, imageTag.String())
		if err == nil {
			baseDesc = &img.Target
			_, err = r.getManifest(ctx, imageTag)
		}
	}

	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}

	var indexDesc *ocispecv1.Descriptor
	if err == nil {
		// update manifests and config with any options provided
		indexDesc, err = r.builder(imageTag).updateManifests(ctx, r.Content(), *baseDesc, *baseDesc, imageTag, r.platform, func(manifest *ocispecv1.Manifest) error {
			return r.builder(imageTag).updateImageConfig(ctx, r.Content(), imageTag, manifest, *buildConfig)
		})
		if err != nil {
			return err
		}
		if indexDesc == nil {
			indexDesc = baseDesc
		}
	} else {
		// manifest is missing, create an empty one and update the index with it.
		indexDesc, err = r.builder(imageTag).addNewManifest(ctx, r.Content(), imageTag, baseDesc, *buildConfig)
		if err != nil {
			return err
		}
	}

	if indexDesc == nil {
		return fmt.Errorf("could not generate index descriptor")
	}
	newImg := images.Image{
		Name:   imageTag.String(),
		Target: *indexDesc,
	}

	if _, err = r.Images().Create(ctx, newImg); err != nil {
		if errdefs.IsAlreadyExists(err) {
			_, err = r.Images().Update(ctx, newImg)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to update image descriptor: %v", err)
	}
	return nil

}

// addNewManifest creates a new manifest and adds it to the index. If no index descriptor is given, a new index will be created
func (b *builder) addNewManifest(ctx context.Context, cs content.Store, imageTag image.Reference, indexDesc *ocispecv1.Descriptor, buildConfig BuildConfig) (*ocispecv1.Descriptor, error) {
	ctx = ensureNamespace(ctx)
	configDesc, configBytes, err := b.newConfig(buildConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %v", err)
	}
	if err := content.WriteBlob(ctx, cs, imageTag.String(), bytes.NewBuffer(configBytes), *configDesc); err != nil {
		return nil, fmt.Errorf("failed to write config: %v", err)
	}

	manifestDesc, manifestBytes, err := b.newManifest(imageTag, configDesc, buildConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create manifest: %v", err)
	}
	if err := content.WriteBlob(ctx, cs, imageTag.String(), bytes.NewBuffer(manifestBytes), *manifestDesc); err != nil {
		return nil, fmt.Errorf("failed to write manifest: %v", err)
	}

	var idx ocispecv1.Index
	var indexBytes []byte
	if indexDesc != nil {
		p, err := content.ReadBlob(ctx, cs, *indexDesc)
		if err != nil {
			return nil, fmt.Errorf("failed to read index blob: %v", err)
		}

		if err := json.Unmarshal(p, &idx); err != nil {
			return nil, fmt.Errorf("failed to unmarshal index blob: %v", err)
		}
		idx.Manifests = append(idx.Manifests, *manifestDesc)

		indexDesc, indexBytes, err = b.getDescriptorAndJSONBytes(idx, indexDesc.Annotations, nil)
	} else {
		indexDesc, indexBytes, err = b.newIndex([]ocispecv1.Descriptor{*manifestDesc})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to update index: %v", err)
	}
	if err := content.WriteBlob(ctx, cs, imageTag.String(), bytes.NewBuffer(indexBytes), *indexDesc); err != nil {
		return nil, fmt.Errorf("failed to write index: %v", err)
	}
	return indexDesc, nil
}

// HistoryEntry creates a new entry for the image history
func historyEntry(emptyLayer bool, creationCommand string, b BuildConfig) ocispecv1.History {
	historyEntry := ocispecv1.History{
		EmptyLayer: emptyLayer,
		CreatedBy:  "operator-registry",
	}

	if b.Author != nil {
		historyEntry.Author = *b.Author
	}

	if b.Comment != nil {
		historyEntry.Comment = *b.Comment
	}

	var created time.Time
	if !b.OmitTimestamp {
		created = time.Now()
		if b.CreationTimestamp != nil {
			created = *b.CreationTimestamp
		}
	}
	historyEntry.Created = &created
	return historyEntry
}

// NewConfig initializes a new empty config
func (b *builder) newConfig(co BuildConfig) (*ocispecv1.Descriptor, []byte, error) {
	historyEntry := historyEntry(true, "", co)
	diffIDs := []digest.Digest{}
	if len(co.Layers) > 0 {
		for _, l := range co.Layers {
			diffIDs = append(diffIDs, l.diffID)
		}
	}
	config := ocispecv1.Image{
		Created:      historyEntry.Created,
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		RootFS: ocispecv1.RootFS{
			Type:    rootFSTypeLayers,
			DiffIDs: diffIDs,
		},
		History: []ocispecv1.History{historyEntry},
	}
	if co.Author != nil {
		config.Author = *co.Author
	}
	if co.Platform != nil {
		p := platforms.Normalize(*co.Platform)
		config.OS = p.OS
		config.Architecture = p.Architecture
	}
	return b.getDescriptorAndJSONBytes(config, map[string]string{}, nil)
}

// NewManifest initializes a new manifest with the provided config
func (b *builder) newManifest(tag image.Reference, config *ocispecv1.Descriptor, co BuildConfig) (*ocispecv1.Descriptor, []byte, error) {
	layers := []ocispecv1.Descriptor{}
	if len(co.Layers) > 0 {
		for _, l := range co.Layers {
			layers = append(layers, *l.descriptor)
		}
	}
	manifest := ocispecv1.Manifest{
		Versioned: ocispec.Versioned{
			SchemaVersion: ociSchemaVersion,
		},
		Config:      *config,
		Layers:      layers,
		Annotations: map[string]string{},
	}

	descAnnotations := map[string]string{}
	if len(tag.String()) != 0 {
		descAnnotations[ocispecv1.AnnotationRefName] = tag.String()
	}
	return b.getDescriptorAndJSONBytes(manifest, descAnnotations, co.Platform)
}

// NewIndex initializes an index with the provided manifests
func (b *builder) newIndex(manifests []ocispecv1.Descriptor) (*ocispecv1.Descriptor, []byte, error) {
	index := ocispecv1.Index{
		Versioned: ocispec.Versioned{
			SchemaVersion: ociSchemaVersion,
		},
		Manifests:   manifests,
		Annotations: map[string]string{},
	}

	return b.getDescriptorAndJSONBytes(index, map[string]string{}, nil)
}

// newLayer creates a new layer from the given src-dst mapping
func (b *builder) newLayer(allowWhiteouts bool, srcs map[string]string) (*ocispecv1.Descriptor, []byte, digest.Digest, error) {
	var diffID digest.Digest
	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)

	tarWriter := tar.NewWriter(io.MultiWriter(gzipWriter, b.digester.Hash()))

	for src, dst := range srcs {
		src = strings.TrimRight(filepath.Clean(src), "/")
		dst = strings.Trim(filepath.Clean(dst), "/")
		if name := filepath.Base(dst); !allowWhiteouts || !strings.HasPrefix(name, WhPrefix) {
			if _, err := os.Stat(src); err != nil {
				return nil, nil, diffID, err
			}
		}

		filepath.Walk(src, func(name string, info os.FileInfo, err error) error {
			// TODO(ankitathomas): This can be abused if dst is outside of the tar root. ensure that name
			// is always a subdirectory of the tar root
			name = strings.TrimRight(name, "/")
			if src != dst {
				relPath, err := filepath.Rel(src, name)
				if err != nil {
					return err
				}
				name = filepath.Join(dst, relPath)
			}
			if strings.HasPrefix(filepath.Base(name), WhPrefix) {
				if !allowWhiteouts {
					return fmt.Errorf("error adding file %s to layer: file has disallowed whiteout prefix %s", name, WhPrefix)
				}
				hdr := &tar.Header{
					Name: name,
					Size: 0,
				}
				if err := tarWriter.WriteHeader(hdr); err != nil {
					return fmt.Errorf("error writing whiteout header for %s to archive: %v", name, err)
				}
				return nil
			}

			linkname := info.Name()
			if info.Mode()&os.ModeSymlink == os.ModeSymlink {
				linkname, err = os.Readlink(name)
				if err != nil {
					return fmt.Errorf("error following symlink %s: %v", name, err)
				}
				relPath, err := filepath.Rel(src, linkname)
				if err == nil {
					linkname = filepath.Join(dst, relPath)
				}
			}

			hdr, err := tar.FileInfoHeader(info, linkname)
			if err != nil {
				return fmt.Errorf("error creating tar header for %s: %v", name, err)
			}
			hdr.Uname = ""
			hdr.Gname = ""

			hdr.Name = name

			if err := tarWriter.WriteHeader(hdr); err != nil {
				return fmt.Errorf("error writing file header for %s to archive: %v", name, err)
			}

			if hdr.Typeflag == tar.TypeReg {
				fh, err := os.Open(name)
				if err != nil {
					return fmt.Errorf("error opening file %s: %v", name, err)
				}
				defer fh.Close()

				n, err := io.Copy(tarWriter, fh)
				if err != nil || n != hdr.Size {
					return fmt.Errorf("error copying %s to archive (%d/%d bytes written): %v", name, n, hdr.Size, err)
				}
			}

			return nil

		})
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

// getDescriptorAndJSONBytes returns the OCI descriptor and marshalled JSON for OCI indexes, manifests and configs
func (b *builder) getDescriptorAndJSONBytes(data interface{}, annotations map[string]string, platform *ocispecv1.Platform) (*ocispecv1.Descriptor, []byte, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, nil, err
	}

	size, err := b.digester.Hash().Write(jsonBytes)
	defer b.digester.Hash().Reset()
	if err != nil {
		return nil, nil, fmt.Errorf("error calculating index digest: %v", err)
	}

	digest := b.digester.Digest()

	desc := &ocispecv1.Descriptor{
		Digest:      digest,
		Size:        int64(size),
		Annotations: annotations,
	}

	switch descType := data.(type) {
	case ocispecv1.Index:
		desc.MediaType = ocispecv1.MediaTypeImageIndex
	case ocispecv1.Manifest:
		desc.MediaType = ocispecv1.MediaTypeImageManifest
		var descPlatform ocispecv1.Platform
		if platform == nil {
			descPlatform = ocispecv1.Platform{
				Architecture: runtime.GOARCH,
				OS:           runtime.GOOS,
			}
		} else {
			descPlatform = *platform
		}
		descPlatform = platforms.Normalize(descPlatform)
		desc.Platform = &descPlatform
	case ocispecv1.Image:
		desc.MediaType = ocispecv1.MediaTypeImageConfig
	default:
		// TODO: reading mediatype from pointer interfaces fails currently
		// enforce data to be either a non-pointer type or extract underlying type.
		return nil, nil, fmt.Errorf("unrecognized blob type %+v", descType)
	}
	return desc, jsonBytes, nil
}

// walk through the image and update all manifests for the platform matcher
func (b *builder) updateManifests(ctx context.Context, cs content.Store, root, desc ocispecv1.Descriptor, ref image.Reference, platform platforms.MatchComparer, updatefunc func(*ocispecv1.Manifest) error) (*ocispecv1.Descriptor, error) {
	var data interface{}
	p, err := content.ReadBlob(ctx, cs, desc)
	if err != nil {
		return nil, err
	}
	switch desc.MediaType {
	case images.MediaTypeDockerSchema2ManifestList, ocispecv1.MediaTypeImageIndex:
		var idx ocispecv1.Index
		if err := json.Unmarshal(p, &idx); err != nil {
			return nil, err
		}

		manifests := make([]ocispecv1.Descriptor, 0)
		for _, d := range idx.Manifests {
			if platform != nil && d.Platform != nil && !platform.Match(*d.Platform) {
				manifests = append(manifests, d)
				continue
			}
			manifestDesc, err := b.updateManifests(ctx, cs, root, d, ref, platform, updatefunc)
			if err != nil {
				return nil, err
			}
			if manifestDesc != nil {
				manifests = append(manifests, *manifestDesc)
				continue
			}
			manifests = append(manifests, d)
		}
		idx.Manifests = manifests
		data = idx
	case images.MediaTypeDockerSchema2Manifest, ocispecv1.MediaTypeImageManifest:
		var manifest ocispecv1.Manifest
		if desc.Digest.String() != root.Digest.String() && platform != nil {
			var descPlatform ocispecv1.Platform
			if desc.Platform == nil {
				if err := json.Unmarshal(p, &manifest); err != nil {
					return nil, err
				}
				p, err := content.ReadBlob(ctx, cs, manifest.Config)
				if err != nil {
					return nil, err
				}
				var image ocispecv1.Image
				if err := json.Unmarshal(p, &image); err != nil {
					return nil, err
				}
				descPlatform = platforms.Normalize(ocispecv1.Platform{OS: image.OS, Architecture: image.Architecture})
			} else {
				descPlatform = *desc.Platform
			}
			if !platform.Match(descPlatform) {
				return nil, nil
			}
		}

		if err := json.Unmarshal(p, &manifest); err != nil {
			return nil, err
		}

		err := updatefunc(&manifest)
		if err != nil {
			return nil, err
		}
		data = manifest
	default:
		// noop for anything that isn't a manifest or index
		return nil, nil
	}

	// Update the changed file
	newDesc, newDescBytes, err := b.getDescriptorAndJSONBytes(data, desc.Annotations, desc.Platform)
	if err != nil {
		return nil, err
	}
	if newDesc.Digest.String() == desc.Digest.String() {
		// update func made no change
		return nil, nil
	}

	if err := content.WriteBlob(ctx, cs, ref.String(), bytes.NewBuffer(newDescBytes), *newDesc); err != nil {
		return nil, err
	}
	return newDesc, nil
}

// make changes to the image config in a manifest according to options. Also update the manifest if layers are added.
// This should be used as an argument to updateManifests, or the manifest will not have the updated config or layer information.
func (b *builder) updateImageConfig(ctx context.Context, cs content.Store, ref image.Reference, manifest *ocispecv1.Manifest, options BuildConfig) error {
	configBytes, err := content.ReadBlob(ctx, cs, manifest.Config)
	if err != nil {
		return fmt.Errorf("error reading config blob %+v: %v", manifest.Config, err)
	}
	var config ocispecv1.Image
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return fmt.Errorf("error unmarshaling config blob: %v", err)
	}

	if options.User != nil {
		config.Config.User = *options.User
	}

	for p, types := range options.Ports {
		if types == nil {
			delete(config.Config.ExposedPorts, fmt.Sprintf("%d", p))
			delete(config.Config.ExposedPorts, fmt.Sprintf("%d/%s", p, TCPType))
			delete(config.Config.ExposedPorts, fmt.Sprintf("%d/%s", p, UDPType))
			continue
		}
		for _, t := range types {
			switch t {
			case TCPType, UDPType:
				config.Config.ExposedPorts[fmt.Sprintf("%d/%s", p, t)] = struct{}{}
			case "":
				config.Config.ExposedPorts[fmt.Sprintf("%d", p)] = struct{}{}
			default:
				return fmt.Errorf("unrecognized port type %d/%s", p, t)
			}
		}
	}

	if len(options.Env) > 0 {
		config.Config.Env = append(config.Config.Env, options.Env...)
	}

	if options.Entrypoint != nil {
		config.Config.Entrypoint = *options.Entrypoint
	}

	if options.WorkingDir != nil {
		config.Config.WorkingDir = *options.WorkingDir
	}

	for v, keep := range options.Volumes {
		if keep == nil {
			delete(config.Config.Volumes, v)
			continue
		}
		config.Config.Volumes[v] = struct{}{}
	}

	if options.Cmd != nil {
		config.Config.Cmd = *options.Cmd
	}

	for k, v := range options.Labels {
		if len(v) == 0 {
			delete(config.Config.Labels, k)
			continue
		}
		config.Config.Labels[k] = v
	}

	if options.StopSignal != nil {
		config.Config.StopSignal = *options.StopSignal
	}

	if len(options.Layers) > 0 {
		if config.RootFS.DiffIDs == nil {
			config.RootFS.DiffIDs = make([]digest.Digest, 0)
		}
		if manifest.Layers == nil {
			manifest.Layers = make([]ocispecv1.Descriptor, 0)
		}

		if options.SquashLayers && len(manifest.Layers)+len(options.Layers) > 1 {
			layers := manifest.Layers
			for _, l := range options.Layers {
				layers = append(layers, *l.descriptor)
			}
			var modTime time.Time
			if !options.OmitTimestamp {
				modTime = time.Now()
				if options.CreationTimestamp != nil {
					modTime = *options.CreationTimestamp
				}
			}
			layerDesc, layerBytes, diffID, err := b.squashLayers(ctx, cs, layers, modTime)
			if err != nil {
				return fmt.Errorf("error squashing layers: %v", err)
			}
			if err := content.WriteBlob(ctx, cs, ref.String(), bytes.NewBuffer(layerBytes), *layerDesc); err != nil {
				return fmt.Errorf("error writing updated config blob: %v", err)
			}

			config.RootFS.DiffIDs = []digest.Digest{diffID}
			manifest.Layers = []ocispecv1.Descriptor{*layerDesc}
			options.Layers = []layer{}
		}

		for _, l := range options.Layers {
			config.RootFS.DiffIDs = append(config.RootFS.DiffIDs, l.diffID)
			manifest.Layers = append(manifest.Layers, *l.descriptor)
		}
	}

	historyEntry := historyEntry(len(options.Layers) == 0, "", options)
	newConfigBytes, _ := json.Marshal(&config)
	historyEntryBytes, _ := json.Marshal(&historyEntry)
	lastHistoryEntryBytes, _ := json.Marshal(config.History[len(config.History)-1])
	if len(config.History) == 0 || string(lastHistoryEntryBytes) != string(historyEntryBytes) || string(configBytes) != string(newConfigBytes) {
		config.History = append(config.History, historyEntry)
	}

	configDesc, configBytes, err := b.getDescriptorAndJSONBytes(config, manifest.Config.Annotations, nil)
	if err != nil {
		return fmt.Errorf("error updating config descriptor: %v", err)
	}
	if err := content.WriteBlob(ctx, cs, ref.String(), bytes.NewBuffer(configBytes), *configDesc); err != nil {
		return fmt.Errorf("error writing updated config blob: %v", err)
	}
	manifest.Config = *configDesc
	return nil
}
