package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"k8s.io/apimachinery/pkg/util/sets"
)

var _ Cache = &JSON{}

type JSON struct {
	baseDir string

	packageIndex
	apiBundles map[apiBundleKey]string
}

const (
	jsonCacheModeDir  = 0750
	jsonCacheModeFile = 0640
)

type apiBundleKey struct {
	pkgName string
	chName  string
	name    string
}

func (q *JSON) loadAPIBundle(k apiBundleKey) (*api.Bundle, error) {
	filename, ok := q.apiBundles[k]
	if !ok {
		return nil, fmt.Errorf("package %q, channel %q, bundle %q not found", k.pkgName, k.chName, k.name)
	}
	d, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var b api.Bundle
	if err := json.Unmarshal(d, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (q *JSON) ListBundles(ctx context.Context) ([]*api.Bundle, error) {
	return listBundles(ctx, q)
}

func (q *JSON) SendBundles(_ context.Context, s registry.BundleSender) error {
	for _, pkg := range q.packageIndex {
		channels := sets.KeySet(pkg.Channels)
		for _, chName := range sets.List(channels) {
			ch := pkg.Channels[chName]

			bundles := sets.KeySet(ch.Bundles)
			for _, bName := range sets.List(bundles) {
				b := ch.Bundles[bName]
				apiBundle, err := q.loadAPIBundle(apiBundleKey{pkg.Name, ch.Name, b.Name})
				if err != nil {
					return fmt.Errorf("convert bundle %q: %v", b.Name, err)
				}
				if apiBundle.BundlePath != "" {
					// The SQLite-based server
					// configures its querier to
					// omit these fields when
					// bundle path is set.
					apiBundle.CsvJson = ""
					apiBundle.Object = nil
				}
				if err := s.Send(apiBundle); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (q *JSON) GetBundle(_ context.Context, pkgName, channelName, csvName string) (*api.Bundle, error) {
	pkg, ok := q.packageIndex[pkgName]
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
	apiBundle, err := q.loadAPIBundle(apiBundleKey{pkg.Name, ch.Name, b.Name})
	if err != nil {
		return nil, fmt.Errorf("convert bundle %q: %v", b.Name, err)
	}

	// unset Replaces and Skips (sqlite query does not populate these fields)
	apiBundle.Replaces = ""
	apiBundle.Skips = nil
	return apiBundle, nil
}

func (q *JSON) GetBundleForChannel(ctx context.Context, pkgName string, channelName string) (*api.Bundle, error) {
	return q.packageIndex.GetBundleForChannel(ctx, q, pkgName, channelName)
}

func (q *JSON) GetBundleThatReplaces(ctx context.Context, name, pkgName, channelName string) (*api.Bundle, error) {
	return q.packageIndex.GetBundleThatReplaces(ctx, q, name, pkgName, channelName)
}

func (q *JSON) GetChannelEntriesThatProvide(ctx context.Context, group, version, kind string) ([]*registry.ChannelEntry, error) {
	return q.packageIndex.GetChannelEntriesThatProvide(ctx, q, group, version, kind)
}

func (q *JSON) GetLatestChannelEntriesThatProvide(ctx context.Context, group, version, kind string) ([]*registry.ChannelEntry, error) {
	return q.packageIndex.GetLatestChannelEntriesThatProvide(ctx, q, group, version, kind)
}

func (q *JSON) GetBundleThatProvides(ctx context.Context, group, version, kind string) (*api.Bundle, error) {
	return q.packageIndex.GetBundleThatProvides(ctx, q, group, version, kind)
}

func NewJSON(baseDir string) *JSON {
	return &JSON{baseDir: baseDir}
}

const (
	jsonDigestFile = "digest"
	jsonDir        = "cache"
	packagesFile   = jsonDir + string(filepath.Separator) + "packages.json"
)

func (q *JSON) CheckIntegrity(fbcFsys fs.FS) error {
	existingDigest, err := q.existingDigest()
	if err != nil {
		return fmt.Errorf("read existing cache digest: %v", err)
	}
	computedDigest, err := q.computeDigest(fbcFsys)
	if err != nil {
		return fmt.Errorf("compute digest: %v", err)
	}
	if existingDigest != computedDigest {
		return fmt.Errorf("cache requires rebuild: cache reports digest as %q, but computed digest is %q", existingDigest, computedDigest)
	}
	return nil
}

func (q *JSON) existingDigest() (string, error) {
	existingDigestBytes, err := os.ReadFile(filepath.Join(q.baseDir, jsonDigestFile))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(existingDigestBytes)), nil
}

func (q *JSON) computeDigest(fbcFsys fs.FS) (string, error) {
	computedHasher := fnv.New64a()
	if err := fsToTar(computedHasher, fbcFsys); err != nil {
		return "", err
	}

	if cacheFS, err := fs.Sub(os.DirFS(q.baseDir), jsonDir); err == nil {
		if err := fsToTar(computedHasher, cacheFS); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("compute hash: %v", err)
		}
	}
	return fmt.Sprintf("%x", computedHasher.Sum(nil)), nil
}

func (q *JSON) Build(ctx context.Context, fbcFsys fs.FS) error {
	// ensure that generated cache is available to all future users
	oldUmask := umask(000)
	defer umask(oldUmask)

	if err := ensureEmptyDir(q.baseDir, jsonCacheModeDir); err != nil {
		return fmt.Errorf("ensure clean base directory: %v", err)
	}
	if err := ensureEmptyDir(filepath.Join(q.baseDir, jsonDir), jsonCacheModeDir); err != nil {
		return fmt.Errorf("ensure clean base directory: %v", err)
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

	packageJson, err := json.Marshal(pkgs)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(q.baseDir, packagesFile), packageJson, jsonCacheModeFile); err != nil {
		return err
	}

	q.apiBundles = map[apiBundleKey]string{}
	for _, p := range fbcModel {
		for _, ch := range p.Channels {
			for _, b := range ch.Bundles {
				apiBundle, err := api.ConvertModelBundleToAPIBundle(*b)
				if err != nil {
					return err
				}
				jsonBundle, err := json.Marshal(apiBundle)
				if err != nil {
					return err
				}
				filename := filepath.Join(q.baseDir, jsonDir, fmt.Sprintf("%s_%s_%s.json", p.Name, ch.Name, b.Name))
				if err := os.WriteFile(filename, jsonBundle, jsonCacheModeFile); err != nil {
					return err
				}
				q.apiBundles[apiBundleKey{p.Name, ch.Name, b.Name}] = filename
			}
		}
	}
	digest, err := q.computeDigest(fbcFsys)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(q.baseDir, jsonDigestFile), []byte(digest), jsonCacheModeFile); err != nil {
		return err
	}
	return nil
}

func (q *JSON) Load() error {
	packagesData, err := os.ReadFile(filepath.Join(q.baseDir, packagesFile))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(packagesData, &q.packageIndex); err != nil {
		return err
	}
	q.apiBundles = map[apiBundleKey]string{}
	for _, p := range q.packageIndex {
		for _, ch := range p.Channels {
			for _, b := range ch.Bundles {
				filename := filepath.Join(q.baseDir, jsonDir, fmt.Sprintf("%s_%s_%s.json", p.Name, ch.Name, b.Name))
				q.apiBundles[apiBundleKey{pkgName: p.Name, chName: ch.Name, name: b.Name}] = filename
			}
		}
	}
	return nil
}
