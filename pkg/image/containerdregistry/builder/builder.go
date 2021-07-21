package builder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/remotes"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"time"
)
func (i *imageBuilder) updateConfig(ctx context.Context, platformMatcher platforms.MatchComparer, f func(*ocispec.Image) (bool, string, error)) error {
	ctx = ensureNamespace(ctx)
	return i.updateManifest(ctx, platformMatcher, func(m *ocispec.Manifest) (bool, string, error) {
		confBlob, err := content.ReadBlob(ctx, i.registry.Content(), m.Config)
		if err != nil {
			return false, "", err
		}

		config := ocispec.Image{}
		if err := json.Unmarshal(confBlob, &config); err != nil {
			return false, "", fmt.Errorf("failed to get imageConfig from manifest %v", err)
		}

		changed, action, err := f(&config)
		if !changed {
			return false, "", nil
		}

		if !i.NoHistory && len(action) > 0 {
			historyEntry := ocispec.History{
				Created:    nil,
				CreatedBy:  "opm generate",
				Comment:    action,
				EmptyLayer: true,
			}
			if !i.OmitTimestamp {
				historyEntry.Created = i.WithTimestamp
				if historyEntry.Created == nil {
					ts := time.Now()
					historyEntry.Created = &ts
				}
			}
			if len(config.History) == 0 {
				config.History = []ocispec.History{}
			}
			config.History = append(config.History, historyEntry)
		}

		configDesc, err := i.newDescriptor(ctx, config, images.MediaTypeDockerSchema2Config)
		if err != nil {
			return false, "", fmt.Errorf("error creating config descriptor: %v", err)
		}
		m.Config = configDesc
		return true, action, nil
	})
}

func (i *imageBuilder) updateImage(ctx context.Context, platformMatcher platforms.MatchComparer, f func(*ocispec.Manifest) (bool, string, error), desc *ocispec.Descriptor) (*ocispec.Descriptor, error) {
	if platformMatcher == nil {
		platformMatcher = platforms.Default()
	}
	p, err := content.ReadBlob(ctx, i.registry.Content(), *desc)
	if err != nil {
		return nil, fmt.Errorf("failed to read descriptor %v: %v", desc.Digest, err)
	}
	switch desc.MediaType {
	case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
		var manifest ocispec.Manifest
		if err := json.Unmarshal(p, &manifest); err != nil {
			return nil, err
		}
		p, err = content.ReadBlob(ctx, i.registry.Content(), manifest.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to read config descriptor %v", err)
		}

		var config ocispec.Image
		if err := json.Unmarshal(p, &config); err != nil {
			return nil, err
		}
		if !platformMatcher.Match(platforms.Normalize(ocispec.Platform{OS: config.OS, Architecture: config.Architecture})) {
			return desc, nil
		}

		changed, action, err := f(&manifest)
		if err != nil {
			return nil, err
		}

		if !changed {
			return desc, nil
		}

		newDesc, err := i.newDescriptor(ctx, manifest, desc.MediaType)
		if err != nil {
			return nil, err
		}
		actionSet := i.actions[desc.Digest]
		if len(actionSet) == 0 {
			actionSet = []string{}
		}
		i.actions[newDesc.Digest] = append(actionSet, action)
		return &newDesc, nil
	case images.MediaTypeDockerSchema2ManifestList, ocispec.MediaTypeImageIndex:
		var idx ocispec.Index
		if err := json.Unmarshal(p, &idx); err != nil {
			return nil, err
		}

		var changed bool
		var manifestDescs []ocispec.Descriptor
		for _, mDesc := range idx.Manifests {
			newDesc, err := i.updateImage(ctx, platformMatcher, f, &mDesc)
			if err != nil {
				return nil, err
			}
			if newDesc != &mDesc {
				changed = true
			}
			manifestDescs = append(manifestDescs, *newDesc)
		}
		if changed {
			idx.Manifests = manifestDescs
			newDesc, err := i.newDescriptor(ctx, idx, desc.MediaType)
			if err != nil {
				return nil, err
			}
			return &newDesc, nil
		}
		return desc, nil
	}
	return desc, nil
}

func (i *imageBuilder) updateManifest(ctx context.Context, platformMatcher platforms.MatchComparer, f func(*ocispec.Manifest) (bool, string, error)) error {
	ctx = ensureNamespace(ctx)
	newDesc, err := i.updateImage(ctx, platformMatcher, f, i.head)
	if err != nil {
		return fmt.Errorf("failed to update image %s: %v", i.head.Digest, err)
	}
	if newDesc != i.head && newDesc != nil {
		newImg := images.Image{
			Name: i.tag,
			Target: *newDesc,
		}
		_, err := i.registry.Images().Create(ctx, newImg)

		if err != nil && errdefs.IsAlreadyExists(err) {
			_, err = i.registry.Images().Update(ctx, newImg)
		}
		if err != nil {
			return fmt.Errorf("failed to create image: %v", err)
		}
		i.head = newDesc
	}
	return nil
}

func (i *imageBuilder) newDescriptor(ctx context.Context, obj interface{}, mediaType string) (ocispec.Descriptor, error) {
	ctx = ensureNamespace(ctx)
	data, err := json.Marshal(obj)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("error marshaling %s descriptor: %v", mediaType, err)
	}
	return i.descriptorFromBytes(ctx, data, mediaType)
}

func (i *imageBuilder) descriptorFromBytes(ctx context.Context, data []byte, mediaType string) (ocispec.Descriptor, error) {
	ctx = ensureNamespace(ctx)
	desc := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(data),
		Size:      int64(len(data)),
	}
	if _, err := i.registry.Content().Info(ctx, desc.Digest); err == nil {
		return desc, nil
	}

	buf := bytes.NewReader(data)

	err := content.WriteBlob(ctx, i.registry.Content(), remotes.MakeRefKey(ctx, desc), buf, desc)
	if err != nil {
		return desc, fmt.Errorf("error writing descriptor %s: %v", desc.Digest, err)
	}

	return desc, nil
}

func ensureNamespace(ctx context.Context) context.Context {
	if _, namespaced := namespaces.Namespace(ctx); !namespaced {
		return namespaces.WithNamespace(ctx, namespaces.Default)
	}
	return ctx
}