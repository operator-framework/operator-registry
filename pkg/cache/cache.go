package cache

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

type Cache interface {
	registry.GRPCQuery

	CheckIntegrity(fbc fs.FS) error
	Build(fbc fs.FS) error
	Load() error
}

func LoadOrRebuild(c Cache, fbc fs.FS) error {
	if err := c.CheckIntegrity(fbc); err != nil {
		if err := c.Build(fbc); err != nil {
			return err
		}
	}
	return c.Load()
}

// New creates a new Cache. It chooses a cache implementation based
// on the files it finds in the cache directory, with a preference for the
// latest iteration of the cache implementation. It returns an error if
// cacheDir exists and contains unexpected files.
func New(cacheDir string) (Cache, error) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("detect cache format: read cache directory: %v", err)
	}
	jsonCache := sets.NewString(jsonDir, jsonDigestFile)

	found := sets.NewString()
	for _, e := range entries {
		found.Insert(e.Name())
	}

	// Preference (and currently only supported) is the JSON-based cache implementation.
	if found.IsSuperset(jsonCache) || len(entries) == 0 {
		return NewJSON(cacheDir), nil
	}

	// Anything else is unexpected.
	return nil, fmt.Errorf("cache directory has unexpected contents")
}

func ensureEmptyDir(dir string, mode os.FileMode) error {
	if err := os.MkdirAll(dir, mode); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func doesBundleProvide(ctx context.Context, c Cache, pkgName, chName, bundleName, group, version, kind string) (bool, error) {
	apiBundle, err := c.GetBundle(ctx, pkgName, chName, bundleName)
	if err != nil {
		return false, fmt.Errorf("get bundle %q: %v", bundleName, err)
	}
	for _, gvk := range apiBundle.ProvidedApis {
		if gvk.Group == group && gvk.Version == version && gvk.Kind == kind {
			return true, nil
		}
	}
	return false, nil
}

type sliceBundleSender []*api.Bundle

func (s *sliceBundleSender) Send(b *api.Bundle) error {
	*s = append(*s, b)
	return nil
}

func listBundles(ctx context.Context, c Cache) ([]*api.Bundle, error) {
	var bundleSender sliceBundleSender

	err := c.SendBundles(ctx, &bundleSender)
	if err != nil {
		return nil, err
	}

	return bundleSender, nil
}
