package containerdregistry

import (
	"os"
	"path/filepath"
	"sync"

	contentlocal "github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/platforms"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

type RegistryConfig struct {
	Log               *logrus.Entry
	ResolverConfigDir string
	DBPath            string
	CacheDir          string
	PreserveCache     bool
	SkipTLS           bool
}

func (r *RegistryConfig) apply(options []RegistryOption) {
	for _, option := range options {
		option(r)
	}
}

func (r *RegistryConfig) complete() error {
	if err := os.Mkdir(r.CacheDir, os.ModePerm); err != nil && !os.IsExist(err) {
		return err
	}

	if r.DBPath == "" {
		r.DBPath = filepath.Join(r.CacheDir, "metadata.db")
	}

	return nil
}

func defaultConfig() *RegistryConfig {
	config := &RegistryConfig{
		Log:               logrus.NewEntry(logrus.New()),
		ResolverConfigDir: "",
		CacheDir:          "cache",
	}

	return config
}

func NewRegistry(options ...RegistryOption) (*Registry, error) {
	config := defaultConfig()
	config.apply(options)
	if err := config.complete(); err != nil {
		return nil, err
	}

	cs, err := contentlocal.NewStore(config.CacheDir)
	if err != nil {
		return nil, err
	}

	bdb, err := bolt.Open(config.DBPath, 0644, nil)
	if err != nil {
		return nil, err
	}

	var (
		once   sync.Once
		closed bool
	)
	close := func() error {
		defer func() {
			once.Do(func() {
				closed = true
			})
		}()
		if closed {
			// Already closed, no-op
			return nil
		}

		if err := bdb.Close(); err != nil {
			return err
		}

		if config.PreserveCache {
			return nil
		}
		return os.RemoveAll(config.CacheDir)
	}

	resolver, err := NewResolver(config.ResolverConfigDir, config.SkipTLS)
	if err != nil {
		return nil, err
	}

	r := &Registry{
		Store: newStore(metadata.NewDB(bdb, cs, nil)),

		log:      config.Log,
		resolver: resolver,
		platform: platforms.Only(platforms.DefaultSpec()),

		close: close,
	}
	return r, nil
}

type RegistryOption func(config *RegistryConfig)

func WithLog(log *logrus.Entry) RegistryOption {
	return func(config *RegistryConfig) {
		config.Log = log
	}
}

func WithResolverConfigDir(path string) RegistryOption {
	return func(config *RegistryConfig) {
		config.ResolverConfigDir = path
	}
}

func WithCacheDir(dir string) RegistryOption {
	return func(config *RegistryConfig) {
		config.CacheDir = dir
	}
}

func PreserveCache(preserve bool) RegistryOption {
	return func(config *RegistryConfig) {
		config.PreserveCache = preserve
	}
}

func SkipTLS(skip bool) RegistryOption {
	return func(config *RegistryConfig) {
		config.SkipTLS = skip
	}
}
