package buildahregistry

import (
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"

	//"sync"
	//
	//contentlocal "github.com/containerd/containerd/content/local"
	//"github.com/containerd/containerd/metadata"
	//"github.com/containerd/containerd/platforms"
	"github.com/sirupsen/logrus"
	//bolt "go.etcd.io/bbolt"
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

	var (
		once   sync.Once
		closed bool
	)
	closeFunc := func() error {
		defer func() {
			once.Do(func() {
				closed = true
			})
		}()
		if closed {
			// Already closed, no-op
			return nil
		}

		if config.PreserveCache {
			return nil
		}
		return os.RemoveAll(config.CacheDir)
	}

	// TODO: at this point we've overwritten all the defaults, may as well not use this
	storeOpts, err := storage.DefaultStoreOptionsAutoDetectUID()
	if err != nil {
		return nil, err
	}
	storeOpts.RootlessStoragePath = config.CacheDir
	storeOpts.RunRoot = config.CacheDir
	storeOpts.GraphRoot = config.CacheDir
	storeOpts.GraphDriverName = "vfs"
	storeOpts.UIDMap = []idtools.IDMap{
		{ContainerID: 0, HostID: os.Getuid()},
	}
	storeOpts.GIDMap = []idtools.IDMap{
		{ContainerID: 0, HostID: os.Getgid()},
	}

	store, err := storage.GetStore(storeOpts)
	if err != nil {
		return nil, err
	}

	// TODO: probably don't want the signature policy to be here
	ioutil.WriteFile(path.Join(config.CacheDir, "policy.json"), []byte(`
{
    "default": [
        {
            "type": "insecureAcceptAnything"
        }
    ],
    "transports":
        {
            "docker-daemon":
                {
                    "": [{"type":"insecureAcceptAnything"}]
                }
        }
}
`), os.ModePerm)

	r := &Registry{
		Store:    store,
		CacheDir: config.CacheDir,
		log:      config.Log,
		close:    closeFunc,
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

func PreserveCache() RegistryOption {
	return func(config *RegistryConfig) {
		config.PreserveCache = true
	}
}

func SkipTLS() RegistryOption {
	return func(config *RegistryConfig) {
		config.SkipTLS = true
	}
}
