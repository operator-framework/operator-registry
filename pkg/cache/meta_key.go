package cache

import (
	"sync"

	"github.com/tidwall/btree"
)

type metaKey struct {
	Schema      string
	PackageName string
	Name        string
}

func metaKeyComparator(a, b metaKey) bool {
	if a.Schema != b.Schema {
		return a.Schema < b.Schema
	}
	if a.PackageName != b.PackageName {
		return a.PackageName < b.PackageName
	}
	return a.Name < b.Name
}

type metaKeys struct {
	mu sync.Mutex
	t  *btree.BTreeG[metaKey]
}

func newMetaKeys() metaKeys {
	return metaKeys{t: btree.NewBTreeG[metaKey](metaKeyComparator)}
}

func (m *metaKeys) Set(k metaKey) {
	m.mu.Lock()
	m.t.Set(k)
	m.mu.Unlock()
}

func (m *metaKeys) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.t.Len()
}

func (m *metaKeys) Walk(f func(k metaKey) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	it := m.t.Iter()
	for it.Next() {
		if err := f(it.Item()); err != nil {
			return err
		}
	}
	return nil
}
