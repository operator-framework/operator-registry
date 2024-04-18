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
	"sort"
	"strings"
	"sync"

	"github.com/akrylysov/pogreb"
	pogrebfs "github.com/akrylysov/pogreb/fs"
	"github.com/golang/protobuf/proto"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

var _ Cache = &PogrebV1{}

type PogrebV1 struct {
	baseDir string

	dbMu sync.RWMutex
	db   *pogreb.DB

	packageIndex
	apiBundles map[apiBundleKey]string
}

const (
	pogrebCacheModeDir  = 0750
	pogrebCacheModeFile = 0640
)

func (q *PogrebV1) loadAPIBundle(k apiBundleKey) (*api.Bundle, error) {
	key, ok := q.apiBundles[k]
	if !ok {
		return nil, fmt.Errorf("package %q, channel %q, bundle %q not found", k.pkgName, k.chName, k.name)
	}
	d, err := q.db.Get([]byte(key))
	if err != nil {
		return nil, err
	}
	var b api.Bundle
	if err := proto.Unmarshal(d, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (q *PogrebV1) ListBundles(ctx context.Context) ([]*api.Bundle, error) {
	return listBundles(ctx, q)
}

func (q *PogrebV1) SendBundles(_ context.Context, s registry.BundleSender) error {
	var keys []apiBundleKey
	for _, pkg := range q.packageIndex {
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				keys = append(keys, apiBundleKey{pkg.Name, ch.Name, b.Name})
			}
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].chName != keys[j].chName {
			return keys[i].chName < keys[j].chName
		}
		if keys[i].pkgName != keys[j].pkgName {
			return keys[i].pkgName < keys[j].pkgName
		}
		return keys[i].name < keys[j].name
	})
	index := 0
	for _, key := range keys {
		dbKey, ok := q.apiBundles[key]
		if !ok {
			return fmt.Errorf("package %q, channel %q, key %q not found", key.pkgName, key.chName, key.name)
		}

		bundleData, err := q.db.Get([]byte(dbKey))
		if err != nil {
			return fmt.Errorf("failed to open file for package %q, channel %q, key %q: %w", key.pkgName, key.chName, key.name, err)
		}
		var bundle api.Bundle
		if err := proto.Unmarshal(bundleData, &bundle); err != nil {
			return fmt.Errorf("failed to decode file for package %q, channel %q, key %q: %w", key.pkgName, key.chName, key.name, err)
		}
		if bundle.BundlePath != "" {
			// The SQLite-based server
			// configures its querier to
			// omit these fields when
			// key path is set.
			bundle.CsvJson = ""
			bundle.Object = nil
		}
		if err := s.Send(&bundle); err != nil {
			return err
		}
		index += 1
	}
	return nil
}

func (q *PogrebV1) GetBundle(_ context.Context, pkgName, channelName, csvName string) (*api.Bundle, error) {
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

func (q *PogrebV1) GetBundleForChannel(ctx context.Context, pkgName string, channelName string) (*api.Bundle, error) {
	return q.packageIndex.GetBundleForChannel(ctx, q, pkgName, channelName)
}

func (q *PogrebV1) GetBundleThatReplaces(ctx context.Context, name, pkgName, channelName string) (*api.Bundle, error) {
	return q.packageIndex.GetBundleThatReplaces(ctx, q, name, pkgName, channelName)
}

func (q *PogrebV1) GetChannelEntriesThatProvide(ctx context.Context, group, version, kind string) ([]*registry.ChannelEntry, error) {
	return q.packageIndex.GetChannelEntriesThatProvide(ctx, q, group, version, kind)
}

func (q *PogrebV1) GetLatestChannelEntriesThatProvide(ctx context.Context, group, version, kind string) ([]*registry.ChannelEntry, error) {
	return q.packageIndex.GetLatestChannelEntriesThatProvide(ctx, q, group, version, kind)
}

func (q *PogrebV1) GetBundleThatProvides(ctx context.Context, group, version, kind string) (*api.Bundle, error) {
	return q.packageIndex.GetBundleThatProvides(ctx, q, group, version, kind)
}

func NewPogrebV1(baseDir string) *PogrebV1 {
	return &PogrebV1{baseDir: baseDir}
}

const (
	pograbV1CacheDir = "pogreb.v1"
	pogrebDigestFile = pograbV1CacheDir + "/digest"
	pogrebDbDir      = pograbV1CacheDir + "/db"
)

func (q *PogrebV1) open() error {
	q.dbMu.Lock()
	defer q.dbMu.Unlock()
	if q.db == nil {
		db, err := pogreb.Open(filepath.Join(q.baseDir, pogrebDbDir), &pogreb.Options{FileSystem: pogrebfs.OS})
		if err != nil {
			return err
		}
		q.db = db
	}
	return nil
}

func (q *PogrebV1) CheckIntegrity(fbcFsys fs.FS) error {
	if err := q.open(); err != nil {
		return fmt.Errorf("open database: %v", err)
	}

	q.dbMu.RLock()
	defer q.dbMu.RUnlock()

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

func (q *PogrebV1) Close() error {
	q.dbMu.Lock()
	defer q.dbMu.Unlock()
	return q.closeNoMutex()
}

func (q *PogrebV1) closeNoMutex() error {
	if q.db == nil {
		return nil
	}
	if err := q.db.Close(); err != nil {
		return err
	}
	q.db = nil
	return nil
}

func (q *PogrebV1) existingDigest() (string, error) {
	existingDigestBytes, err := os.ReadFile(filepath.Join(q.baseDir, pogrebDigestFile))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(existingDigestBytes)), nil
}

func (q *PogrebV1) computeDigest(fbcFsys fs.FS) (string, error) {
	computedHasher := fnv.New64a()

	declcfg.WalkMetasFS(fbcFsys, func(path string, meta *declcfg.Meta, err error) error {
		if err != nil {
			return err
		}
		if _, err := computedHasher.Write(meta.Blob); err != nil {
			return err
		}
		return nil
	})
	it := q.db.Items()
	for {
		k, v, err := it.Next()
		if errors.Is(err, pogreb.ErrIterationDone) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("error iterating database items: %v", err)
		}
		if _, err := computedHasher.Write(k); err != nil {
			return "", err
		}
		if _, err := computedHasher.Write(v); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%x", computedHasher.Sum(nil)), nil
}

func (q *PogrebV1) Build(ctx context.Context, fbcFsys fs.FS) error {
	q.dbMu.Lock()
	defer q.dbMu.Unlock()
	if err := q.closeNoMutex(); err != nil {
		return fmt.Errorf("could not close existing pogreb database: %v", err)
	}
	db, err := q.buildPogrebDB(ctx, fbcFsys)
	if err != nil {
		if db != nil {
			_ = db.Close()
		}
		return fmt.Errorf("build db: %v", err)
	}
	q.db = db

	digest, err := q.computeDigest(fbcFsys)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(q.baseDir, pogrebDigestFile), []byte(digest), pogrebCacheModeFile); err != nil {
		return err
	}
	return nil

}

func (q *PogrebV1) buildPogrebDB(ctx context.Context, fbcFsys fs.FS) (*pogreb.DB, error) {
	if err := ensureEmptyDir(filepath.Join(q.baseDir, pograbV1CacheDir), pogrebCacheModeDir); err != nil {
		return nil, fmt.Errorf("ensure clean base directory: %v", err)
	}

	fbc, err := declcfg.LoadFS(ctx, fbcFsys)
	if err != nil {
		return nil, err
	}
	fbcModel, err := declcfg.ConvertToModel(*fbc)
	if err != nil {
		return nil, err
	}

	pkgs, err := packagesFromModel(fbcModel)
	if err != nil {
		return nil, err
	}

	packageJson, err := json.Marshal(pkgs)
	if err != nil {
		return nil, err
	}

	db, err := pogreb.Open(filepath.Join(q.baseDir, pogrebDbDir), &pogreb.Options{})
	if err != nil {
		return nil, err
	}
	if err := db.Put([]byte("packages.json"), packageJson); err != nil {
		return db, err
	}

	q.apiBundles = map[apiBundleKey]string{}
	for _, p := range fbcModel {
		for _, ch := range p.Channels {
			for _, b := range ch.Bundles {
				apiBundle, err := api.ConvertModelBundleToAPIBundle(*b)
				if err != nil {
					return db, err
				}
				protoBundle, err := proto.Marshal(apiBundle)
				if err != nil {
					return db, err
				}
				key := fmt.Sprintf("bundles/%s_%s_%s.pb", p.Name, ch.Name, b.Name)
				if err := db.Put([]byte(key), protoBundle); err != nil {
					return db, err
				}
				q.apiBundles[apiBundleKey{p.Name, ch.Name, b.Name}] = key
			}
		}
	}
	if err := db.Sync(); err != nil {
		return db, fmt.Errorf("failed to sync database: %v", err)
	}
	return db, nil
}

func (q *PogrebV1) Load() error {
	if err := q.open(); err != nil {
		return fmt.Errorf("open database: %v", err)
	}

	q.dbMu.RLock()
	defer q.dbMu.RUnlock()

	packagesData, err := q.db.Get([]byte("packages.json"))
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
				key := fmt.Sprintf("bundles/%s_%s_%s.json", p.Name, ch.Name, b.Name)
				q.apiBundles[apiBundleKey{pkgName: p.Name, chName: ch.Name, name: b.Name}] = key
			}
		}
	}
	return nil
}
