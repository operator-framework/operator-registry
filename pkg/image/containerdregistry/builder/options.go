package builder

import (
	"context"
	"fmt"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"time"
)

type imageBuilder struct {
	registry containerdregistry.Registry
	fromImage image.Reference
	NoHistory bool
	OmitTimestamp bool
	WithTimestamp *time.Time
	tag string
	head *ocispec.Descriptor
	actions map[digest.Digest][]string
}

type ImageBuilderOption func(*imageBuilder)

func FromImage(from image.Reference) ImageBuilderOption {
	return func(i *imageBuilder) {
		i.fromImage = from
	}
}

func NoHistory() ImageBuilderOption {
	return func(i *imageBuilder) {
		i.NoHistory = true
	}
}

func WithTimestamp(t time.Time) ImageBuilderOption {
	return func(i *imageBuilder) {
		i.WithTimestamp = &t
		i.OmitTimestamp = false
	}
}

func OmitTimestamp() ImageBuilderOption {
	return func(i *imageBuilder) {
		i.OmitTimestamp = true
		i.WithTimestamp = nil
	}
}

func NewImageBuilder(ctx context.Context, tag string, r containerdregistry.Registry, opts...ImageBuilderOption) (*imageBuilder, error) {
	ctx = ensureNamespace(ctx)
	builder := &imageBuilder{
		registry: r,
		tag: tag,
		actions: map[digest.Digest][]string{},
	}
	for _, o := range opts {
		o(builder)
	}
	if builder.fromImage == nil || builder.fromImage.String() == "scratch" {
		// handle this separately
		return nil, fmt.Errorf("empty base image")
	} else {
		err := builder.registry.Pull(ctx, builder.fromImage)
		if err != nil {
			return nil, fmt.Errorf("error pulling base image: %v", err)
		}
	}

	img, err := builder.registry.Images().Get(ctx, builder.fromImage.String())
	if err != nil {
		return nil, fmt.Errorf("error getting index for %v: %v", builder.fromImage.String(), err)
	}
	builder.head = &img.Target

	return builder, nil
}
