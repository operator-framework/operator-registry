package fakestore

import (
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
)

type FakeStore struct {
	is *FakeImageStore
	cs *FakeContentStore
}

func NewFakeStore() *FakeStore {
	return &FakeStore{
		cs: NewFakeContentStore(),
		is: NewFakeImageStore(),
	}
}

func (s *FakeStore) Content() content.Store {
	return s.cs
}

func (s *FakeStore) Images() images.Store {
	return s.is
}
