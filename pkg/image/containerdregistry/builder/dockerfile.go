package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"os"
	"strings"
)

func (i *imageBuilder) GenerateDockerfile(ctx context.Context, prefix string) error {
	ctx = ensureNamespace(ctx)
	generateList := map[string]digest.Digest{}
	err := images.Walk(ctx, images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		descBlob, err := content.ReadBlob(ctx, i.registry.Content(), desc)
		if err != nil {
			return nil, fmt.Errorf("failed to read descriptor %v", err)
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

			if len(i.actions[desc.Digest]) == 0 {
				return nil, nil
			}
			confBlob, err := content.ReadBlob(ctx, i.registry.Content(), manifest.Config)
			if err != nil {
				return nil, fmt.Errorf("failed to read descriptor %v", err)
			}
			config := ocispec.Image{}
			if err := json.Unmarshal(confBlob, &config); err != nil {
				return nil, err
			}

			generateList[fmt.Sprintf("%s-%s", config.OS, config.Architecture)] = desc.Digest
		}
		return nil, nil
	}), *i.head)

	if err != nil {
		return err
	}

	for p, dgst := range generateList {
		dockerfile := prefix
		if len(generateList) > 1 {
			if strings.HasSuffix(dockerfile, ".Dockerfile") {
				dockerfile = dockerfile[:11]
			}
			if strings.HasSuffix(dockerfile, "Dockerfile") {
				dockerfile = dockerfile[:10]
			}
			dockerfile = fmt.Sprintf("%s.%s.%s.Dockerfile", dockerfile, p, dgst.Encoded()[:6])
			strings.TrimPrefix(dockerfile, ".")
		}

		f, err := os.Create(dockerfile)
		if err != nil {
			return fmt.Errorf("failed to create new dockerfile %s: %v", dockerfile, err)
		}
		defer f.Close()
		_, err = f.Write([]byte(fmt.Sprintf("FROM %s\n\n", i.fromImage.String())))
		if err != nil {
			return err
		}
		for _, a := range i.actions[dgst] {
			_, err = f.Write([]byte(fmt.Sprintf("%s\n\n", a)))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
