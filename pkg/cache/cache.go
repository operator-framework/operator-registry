package cache

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

type Cache interface {
	registry.GRPCQuery

	CheckIntegrity(ctx context.Context, fbc fs.FS) error
	Build(ctx context.Context, fbc fs.FS) error
	Load(ctc context.Context) error
	Close() error
}

type backend interface {
	IsCachePresent() bool

	Init() error
	Open() error
	Close() error

	GetPackageIndex(context.Context) (packageIndex, error)
	PutPackageIndex(context.Context, packageIndex) error

	SendBundles(context.Context, registry.BundleSender) error
	GetBundle(context.Context, bundleKey) (*api.Bundle, error)
	PutBundle(context.Context, bundleKey, *api.Bundle) error

	GetDigest(context.Context) (string, error)
	ComputeDigest(context.Context, fs.FS) (string, error)
	PutDigest(context.Context, string) error
}

// New creates a new Cache. It chooses a cache implementation based
// on the files it finds in the cache directory, with a preference for the
// latest iteration of the cache implementation. If the cache directory
// is non-empty and a supported cache format is not found, an error is returned.
func New(cacheDir string) (Cache, error) {
	cacheBackend, err := getDefaultBackend(cacheDir)
	if err != nil {
		return nil, err
	}
	if err := cacheBackend.Open(); err != nil {
		return nil, fmt.Errorf("open cache: %v", err)
	}
	return &cache{backend: cacheBackend}, nil
}

func getDefaultBackend(cacheDir string) (backend, error) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("detect cache format: read cache directory: %v", err)
	}

	backends := []backend{
		newPogrebV1Backend(cacheDir),
		newJSONBackend(cacheDir),
	}

	if len(entries) == 0 {
		return backends[0], nil
	}

	for _, backend := range backends {
		if backend.IsCachePresent() {
			return backend, nil
		}
	}

	// Anything else is unexpected.
	return nil, fmt.Errorf("cache directory has unexpected contents")
}

func LoadOrRebuild(ctx context.Context, c Cache, fbc fs.FS) error {
	if err := c.CheckIntegrity(ctx, fbc); err != nil {
		if err := c.Build(ctx, fbc); err != nil {
			return fmt.Errorf("failed to rebuild cache: %v", err)
		}
	}
	return c.Load(ctx)
}

var _ Cache = &cache{}

type cache struct {
	backend backend
	packageIndex
}

type bundleStreamTransformer func(*api.Bundle)
type transformingBundleSender struct {
	stream      registry.BundleSender
	transformer bundleStreamTransformer
}

func (t *transformingBundleSender) Send(b *api.Bundle) error {
	t.transformer(b)
	return t.stream.Send(b)
}

type sliceBundleSender []*api.Bundle

func (s *sliceBundleSender) Send(b *api.Bundle) error {
	*s = append(*s, b)
	return nil
}

func (c *cache) SendBundles(ctx context.Context, stream registry.BundleSender) error {
	transform := func(bundle *api.Bundle) {
		if bundle.BundlePath != "" {
			// The SQLite-based server
			// configures its querier to
			// omit these fields when
			// key path is set.
			bundle.CsvJson = ""
			bundle.Object = nil
		}
	}
	return c.backend.SendBundles(ctx, &transformingBundleSender{stream, transform})
}

func (c *cache) ListBundles(ctx context.Context) ([]*api.Bundle, error) {
	var bundleSender sliceBundleSender
	if err := c.SendBundles(ctx, &bundleSender); err != nil {
		return nil, err
	}
	return bundleSender, nil
}

func (c *cache) getTrimmedBundle(ctx context.Context, key bundleKey) (*api.Bundle, error) {
	apiBundle, err := c.backend.GetBundle(ctx, key)
	if err != nil {
		return nil, err
	}
	apiBundle.Replaces = ""
	apiBundle.Skips = nil
	return apiBundle, nil
}

func (c *cache) GetBundle(ctx context.Context, pkgName, channelName, csvName string) (*api.Bundle, error) {
	pkg, ok := c.packageIndex[pkgName]
	if !ok {
		return nil, fmt.Errorf("package %q not found", pkgName)
	}
	ch, ok := pkg.Channels[channelName]
	if !ok {
		return nil, fmt.Errorf("package %q, channel %q not found", pkgName, channelName)
	}
	b, ok := ch.Bundles[csvName]
	if !ok {
		return nil, fmt.Errorf("package %q, channel %q, bundle %q not found", pkgName, channelName, csvName)
	}
	return c.getTrimmedBundle(ctx, bundleKey{pkg.Name, ch.Name, b.Name})
}

func (c *cache) GetBundleForChannel(ctx context.Context, pkgName string, channelName string) (*api.Bundle, error) {
	return c.packageIndex.GetBundleForChannel(ctx, c.getTrimmedBundle, pkgName, channelName)
}

func (c *cache) GetBundleThatReplaces(ctx context.Context, name, pkgName, channelName string) (*api.Bundle, error) {
	return c.packageIndex.GetBundleThatReplaces(ctx, c.getTrimmedBundle, name, pkgName, channelName)
}

func (c *cache) GetChannelEntriesThatProvide(ctx context.Context, group, version, kind string) ([]*registry.ChannelEntry, error) {
	return c.packageIndex.GetChannelEntriesThatProvide(ctx, c.backend.GetBundle, group, version, kind)
}

func (c *cache) GetLatestChannelEntriesThatProvide(ctx context.Context, group, version, kind string) ([]*registry.ChannelEntry, error) {
	return c.packageIndex.GetLatestChannelEntriesThatProvide(ctx, c.backend.GetBundle, group, version, kind)
}

func (c *cache) GetBundleThatProvides(ctx context.Context, group, version, kind string) (*api.Bundle, error) {
	return c.packageIndex.GetBundleThatProvides(ctx, c, group, version, kind)
}

func (c *cache) CheckIntegrity(ctx context.Context, fbc fs.FS) error {
	existingDigest, err := c.backend.GetDigest(ctx)
	if err != nil {
		return fmt.Errorf("read existing cache digest: %v", err)
	}
	computedDigest, err := c.backend.ComputeDigest(ctx, fbc)
	if err != nil {
		return fmt.Errorf("compute digest: %v", err)
	}
	if existingDigest != computedDigest {
		return fmt.Errorf("cache requires rebuild: cache reports digest as %q, but computed digest is %q", existingDigest, computedDigest)
	}
	return nil
}

func (c *cache) Build(ctx context.Context, fbcFsys fs.FS) error {
	// ensure that generated cache is available to all future users
	oldUmask := umask(000)
	defer umask(oldUmask)

	if err := c.backend.Init(); err != nil {
		return fmt.Errorf("init cache: %v", err)
	}

	fbc, err := declcfg.LoadFS(ctx, fbcFsys)
	if err != nil {
		return err
	}
	fbcModel, err := declcfg.ConvertToModel(*fbc)
	if err != nil {
		return err
	}

	pkgs, err := packagesFromModel(fbcModel)
	if err != nil {
		return err
	}

	if err := c.backend.PutPackageIndex(ctx, pkgs); err != nil {
		return fmt.Errorf("store package index: %v", err)
	}

	for _, p := range fbcModel {
		for _, ch := range p.Channels {
			for _, b := range ch.Bundles {
				apiBundle, err := api.ConvertModelBundleToAPIBundle(*b)
				if err != nil {
					return err
				}
				if err := c.backend.PutBundle(ctx, bundleKey{p.Name, ch.Name, b.Name}, apiBundle); err != nil {
					return fmt.Errorf("store bundle %q: %v", b.Name, err)
				}
			}
		}
	}
	digest, err := c.backend.ComputeDigest(ctx, fbcFsys)
	if err != nil {
		return fmt.Errorf("compute digest: %v", err)
	}
	if err := c.backend.PutDigest(ctx, digest); err != nil {
		return fmt.Errorf("store digest: %v", err)
	}
	return nil
}

func (c *cache) Load(ctx context.Context) error {
	pi, err := c.backend.GetPackageIndex(ctx)
	if err != nil {
		return fmt.Errorf("get package index: %v", err)
	}
	c.packageIndex = pi
	return nil
}

func (c *cache) Close() error {
	return c.backend.Close()
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

func readDigestFile(digestFile string) (string, error) {
	existingDigestBytes, err := os.ReadFile(digestFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(existingDigestBytes)), nil
}

func writeDigestFile(file string, digest string, mode os.FileMode) error {
	return os.WriteFile(file, []byte(digest), mode)
}

func doesBundleProvide(ctx context.Context, getBundle getBundleFunc, pkgName, chName, bundleName, group, version, kind string) (bool, error) {
	apiBundle, err := getBundle(ctx, bundleKey{pkgName, chName, bundleName})
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
