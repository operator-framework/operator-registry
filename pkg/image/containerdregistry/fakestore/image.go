package fakestore

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/filters"
	"github.com/containerd/containerd/images"
	"github.com/pkg/errors"
)

type FakeImageStore map[string]images.Image

func NewFakeImageStore() *FakeImageStore {
	var s FakeImageStore = make(map[string]images.Image)
	return &s
}

func (s *FakeImageStore) Get(ctx context.Context, name string) (images.Image, error) {
	if im, ok := (*s)[name]; ok {
		return im, nil
	}
	return images.Image{}, errdefs.ErrNotFound
}

func (s *FakeImageStore) List(ctx context.Context, fs ...string) ([]images.Image, error) {
	filter, err := filters.ParseAll(fs...)
	if err != nil {
		return nil, errors.Wrap(errdefs.ErrInvalidArgument, err.Error())
	}
	imgs := make([]images.Image, 0)
	for _, obj := range *s {
		if filter.Match(adaptImage(obj)) {
			imgs = append(imgs, obj)
		}
	}
	return imgs, nil
}

func (s *FakeImageStore) Create(ctx context.Context, image images.Image) (images.Image, error) {
	(*s)[image.Name] = image
	return image, nil
}

func (s *FakeImageStore) Delete(ctx context.Context, name string, opts ...images.DeleteOpt) error {
	delete(*s, name)
	return nil
}

func (s *FakeImageStore) Update(ctx context.Context, image images.Image, fieldpaths ...string) (images.Image, error) {
	if len(image.Name) == 0 {
		return images.Image{}, fmt.Errorf("required field Name missing in image")
	}

	img := images.Image{Name: image.Name}
	if len(fieldpaths) == 0 {
		img = image
	}
	for _, fieldpath := range fieldpaths {
		switch fieldpath {
		case "labels":
			img.Labels = image.Labels
		case "target":
			img.Target = image.Target
		case "annotations":
			img.Target.Annotations = image.Target.Annotations
		case "urls":
			img.Target.URLs = image.Target.URLs
		default:
			prefix := strings.SplitN(fieldpath, ".", 2)
			if len(prefix) != 2 {
				return images.Image{}, fmt.Errorf("invalid fieldpath %s", fieldpath)
			}
			switch prefix[0] {
			case "labels":
				img.Labels[prefix[1]] = image.Labels[prefix[1]]
			case "annotations":
				img.Target.Annotations[prefix[1]] = image.Target.Annotations[prefix[1]]
			}
		}
	}
	(*s)[img.Name] = img
	return img, nil
}

func adaptImage(obj images.Image) filters.Adaptor {
	return filters.AdapterFunc(func(fieldpath []string) (string, bool) {
		if len(fieldpath) == 0 {
			return "", false
		}

		switch fieldpath[0] {
		case "name":
			return obj.Name, len(obj.Name) > 0
		case "target":
			if len(fieldpath) < 2 {
				return "", false
			}

			switch fieldpath[1] {
			case "digest":
				return obj.Target.Digest.String(), len(obj.Target.Digest) > 0
			case "mediatype":
				return obj.Target.MediaType, len(obj.Target.MediaType) > 0
			}
		case "labels":
			if len(obj.Labels) == 0 {
				return "", false
			}
			value, ok := obj.Labels[strings.Join(fieldpath[1:], ".")]
			return value, ok
		case "annotations":
			if len(obj.Target.Annotations) == 0 {
				return "", false
			}
			value, ok := obj.Target.Annotations[strings.Join(fieldpath[1:], ".")]
			return value, ok
		}

		return "", false
	})
}
