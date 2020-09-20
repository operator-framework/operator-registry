package fakestore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/opencontainers/go-digest"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type FakeContentStore struct {
	store  map[digest.Digest]content.Info
	status map[string]content.Status
	data   map[digest.Digest][]byte
}

func NewFakeContentStore() *FakeContentStore {
	return &FakeContentStore{
		store:  make(map[digest.Digest]content.Info),
		status: make(map[string]content.Status),
		data:   make(map[digest.Digest][]byte),
	}
}

func (s *FakeContentStore) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	return s.store[dgst], nil
}

func (s *FakeContentStore) Delete(ctx context.Context, dgst digest.Digest) error {
	delete(s.store, dgst)
	return nil
}

func (s *FakeContentStore) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	t := time.Now()
	info.UpdatedAt = t
	if _, ok := s.store[info.Digest]; !ok {
		info.CreatedAt = t
	}
	if len(fieldpaths) != 0 {
		keep := make(map[string]bool)
		for _, field := range fieldpaths {
			if field == "labels" {
				// use all of info.labels
				keep = nil
				break
			}
			if !strings.HasPrefix(field, "labels.") {
				return content.Info{}, fmt.Errorf("invalid path")
			}
			keep[field[len("labels."):]] = true
		}
		if keep != nil {
			for l := range info.Labels {
				if !keep[l] {
					delete(info.Labels, l)
				}
			}
			for l := range s.store[info.Digest].Labels {
				if _, ok := info.Labels[l]; !ok {
					info.Labels[l] = s.store[info.Digest].Labels[l]
				}
			}
		}
	}
	info.UpdatedAt = time.Now()

	s.store[info.Digest] = info
	return info, nil
}

func (s *FakeContentStore) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	for dgst := range s.store {
		if err := fn(s.store[dgst]); err != nil {
			return err
		}
	}
	return nil
}

func (s *FakeContentStore) Status(ctx context.Context, ref string) (content.Status, error) {
	if refStat, ok := s.status[ref]; ok {
		return refStat, nil
	}
	return content.Status{}, errdefs.ErrNotFound
}

func (s *FakeContentStore) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	statuses := make([]content.Status, 0)
	for _, status := range s.status {
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (f *FakeContentStore) Abort(ctx context.Context, ref string) error {
	return nil
}

func (s *FakeContentStore) ReaderAt(ctx context.Context, desc ocispecv1.Descriptor) (content.ReaderAt, error) {
	if _, ok := s.data[desc.Digest]; !ok {
		return nil, errdefs.ErrNotFound
	}
	return &FakeReader{
		buf: s.data[desc.Digest],
	}, nil
}

func (f *FakeContentStore) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	wrOpts := content.WriterOpts{}
	for _, opt := range opts {
		if err := opt(&wrOpts); err != nil {
			return nil, err
		}
	}
	now := time.Now()
	return &FakeWriter{
		ref:       wrOpts.Ref,
		desc:      wrOpts.Desc,
		data:      new([]byte),
		cs:        f,
		createdAt: now,
		updatedAt: now,
	}, nil
}
