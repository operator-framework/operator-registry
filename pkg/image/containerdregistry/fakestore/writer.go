package fakestore

import (
	"context"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/opencontainers/go-digest"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type FakeWriter struct {
	ref       string
	desc      ocispecv1.Descriptor
	data      *[]byte
	cs        *FakeContentStore
	createdAt time.Time
	updatedAt time.Time
}

func (w *FakeWriter) Write(p []byte) (n int, err error) {
	w.updatedAt = time.Now()
	if w.data == nil {
		w.data = new([]byte)
	}
	*w.data = append(*w.data, p...)
	return len(p), nil
}

func (w *FakeWriter) Close() error {
	w.data = nil
	return nil
}

func (w *FakeWriter) Digest() digest.Digest {
	return digest.FromBytes(*w.data)
}

func (w *FakeWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	i := content.Info{
		Digest:    digest.FromBytes(*w.data),
		Size:      int64(len(*w.data)),
		CreatedAt: w.createdAt,
		UpdatedAt: w.updatedAt,
	}
	for _, opt := range opts {
		if err := opt(&i); err != nil {
			return err
		}
	}
	w.cs.store[i.Digest] = i
	if (w.cs.data) == nil {
		w.cs.data = make(map[digest.Digest][]byte)
	}
	w.cs.data[w.desc.Digest] = make([]byte, len(*w.data))
	copy(w.cs.data[w.desc.Digest], *w.data)
	status, err := w.Status()
	if err != nil {
		return err
	}
	w.cs.status[w.ref] = status
	w.Close()
	return nil
}

func (w *FakeWriter) Status() (content.Status, error) {
	return content.Status{
		Ref:       w.ref,
		Offset:    int64(len(*w.data)),
		Total:     w.desc.Size,
		Expected:  w.desc.Digest,
		StartedAt: w.createdAt,
		UpdatedAt: w.updatedAt,
	}, nil
}

func (w *FakeWriter) Truncate(size int64) error {
	if len(*w.data) < int(size) {
		return nil
	}
	*w.data = (*w.data)[:size]
	return nil
}
