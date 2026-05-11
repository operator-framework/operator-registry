package cache

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tidwall/btree"
)

func validateMetaKeyComponent(name, value string) error {
	if strings.ContainsAny(value, "/\\") || value == ".." || strings.HasPrefix(value, "../") || strings.HasSuffix(value, "/..") {
		return fmt.Errorf("invalid %s %q: must not contain path separators or '..'", name, value)
	}
	return nil
}

func newValidatedMetaKey(schema, packageName, name string) (metaKey, error) {
	if err := validateMetaKeyComponent("schema", schema); err != nil {
		return metaKey{}, err
	}
	if err := validateMetaKeyComponent("packageName", packageName); err != nil {
		return metaKey{}, err
	}
	if err := validateMetaKeyComponent("name", name); err != nil {
		return metaKey{}, err
	}
	return metaKey{Schema: schema, PackageName: packageName, Name: name}, nil
}

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
	keys := make([]metaKey, 0, m.t.Len())
	it := m.t.Iter()
	for it.Next() {
		keys = append(keys, it.Item())
	}
	m.mu.Unlock()

	for _, k := range keys {
		if err := f(k); err != nil {
			return err
		}
	}
	return nil
}
