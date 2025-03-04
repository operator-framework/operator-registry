package declcfg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"runtime"
	"sync"

	"github.com/joelanford/ignore"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/api/pkg/operators"

	"github.com/operator-framework/operator-registry/alpha/property"
)

const (
	indexIgnoreFilename = ".indexignore"
)

type WalkMetasFSFunc func(path string, meta *Meta, err error) error

// WalkMetasFS walks the filesystem rooted at root and calls walkFn for each individual meta object found in the root.
// By default, WalkMetasFS is not thread-safe because it invokes walkFn concurrently. In order to make it thread-safe,
// use the WithConcurrency(1) to avoid concurrent invocations of walkFn.
func WalkMetasFS(ctx context.Context, root fs.FS, walkFn WalkMetasFSFunc, opts ...LoadOption) error {
	if root == nil {
		return fmt.Errorf("no declarative config filesystem provided")
	}

	options := LoadOptions{
		concurrency: runtime.NumCPU(),
	}
	for _, opt := range opts {
		opt(&options)
	}

	pathChan := make(chan string, options.concurrency)

	// Create an errgroup to manage goroutines. The context is closed when any
	// goroutine returns an error. Goroutines should check the context
	// to see if they should return early (in the case of another goroutine
	// returning an error).
	eg, ctx := errgroup.WithContext(ctx)

	// Walk the FS and send paths to a channel for parsing.
	eg.Go(func() error {
		return sendPaths(ctx, root, pathChan)
	})

	// Parse paths concurrently. The waitgroup ensures that all paths are parsed
	// before the cfgChan is closed.
	for i := 0; i < options.concurrency; i++ {
		eg.Go(func() error {
			return parseMetaPaths(ctx, root, pathChan, walkFn, options)
		})
	}
	return eg.Wait()
}

type WalkMetasReaderFunc func(meta *Meta, err error) error

func WalkMetasReader(r io.Reader, walkFn WalkMetasReaderFunc) error {
	dec := yaml.NewYAMLOrJSONDecoder(r, 4096)
	for {
		var in Meta
		if err := dec.Decode(&in); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return walkFn(nil, err)
		}

		if err := walkFn(&in, nil); err != nil {
			return err
		}
	}
	return nil
}

type WalkFunc func(path string, cfg *DeclarativeConfig, err error) error

// WalkFS walks root using a gitignore-style filename matcher to skip files
// that match patterns found in .indexignore files found throughout the filesystem.
// It calls walkFn for each declarative config file it finds. If WalkFS encounters
// an error loading or parsing any file, the error will be immediately returned.
func WalkFS(root fs.FS, walkFn WalkFunc) error {
	return walkFiles(root, func(root fs.FS, path string, err error) error {
		if err != nil {
			return walkFn(path, nil, err)
		}

		cfg, err := LoadFile(root, path)
		if err != nil {
			return walkFn(path, cfg, err)
		}

		return walkFn(path, cfg, nil)
	})
}

func walkFiles(root fs.FS, fn func(root fs.FS, path string, err error) error) error {
	if root == nil {
		return fmt.Errorf("no declarative config filesystem provided")
	}

	matcher, err := ignore.NewMatcher(root, indexIgnoreFilename)
	if err != nil {
		return err
	}

	return fs.WalkDir(root, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return fn(root, path, err)
		}
		// avoid validating a directory, an .indexignore file, or any file that matches
		// an ignore pattern outlined in a .indexignore file.
		if info.IsDir() || info.Name() == indexIgnoreFilename || matcher.Match(path, false) {
			return nil
		}

		return fn(root, path, nil)
	})
}

type LoadOptions struct {
	concurrency int
}

type LoadOption func(*LoadOptions)

func WithConcurrency(concurrency int) LoadOption {
	return func(opts *LoadOptions) {
		opts.concurrency = concurrency
	}
}

// LoadFS loads a declarative config from the provided root FS. LoadFS walks the
// filesystem from root and uses a gitignore-style filename matcher to skip files
// that match patterns found in .indexignore files found throughout the filesystem.
// If LoadFS encounters an error loading or parsing any file, the error will be
// immediately returned.
func LoadFS(ctx context.Context, root fs.FS, opts ...LoadOption) (*DeclarativeConfig, error) {
	builder := fbcBuilder{}
	if err := WalkMetasFS(ctx, root, func(path string, meta *Meta, err error) error {
		if err != nil {
			return err
		}
		return builder.addMeta(meta)
	}, opts...); err != nil {
		return nil, err
	}
	return &builder.cfg, nil
}

func sendPaths(ctx context.Context, root fs.FS, pathChan chan<- string) error {
	defer close(pathChan)
	return walkFiles(root, func(_ fs.FS, path string, err error) error {
		if err != nil {
			return err
		}
		select {
		case pathChan <- path:
		case <-ctx.Done(): // don't block on sending to pathChan
			return ctx.Err()
		}
		return nil
	})
}

func parseMetaPaths(ctx context.Context, root fs.FS, pathChan <-chan string, walkFn WalkMetasFSFunc, options LoadOptions) error {
	for {
		select {
		case <-ctx.Done(): // don't block on receiving from pathChan
			return ctx.Err()
		case path, ok := <-pathChan:
			if !ok {
				return nil
			}
			file, err := root.Open(path)
			if err != nil {
				return err
			}
			if err := WalkMetasReader(file, func(meta *Meta, err error) error {
				return walkFn(path, meta, err)
			}); err != nil {
				return err
			}
		}
	}
}

func readBundleObjects(b *Bundle) error {
	var obj property.BundleObject
	for i, props := range b.Properties {
		if props.Type != property.TypeBundleObject {
			continue
		}
		if err := json.Unmarshal(props.Value, &obj); err != nil {
			return fmt.Errorf("package %q, bundle %q: parse property at index %d as bundle object: %v", b.Package, b.Name, i, err)
		}
		objJson, err := yaml.ToJSON(obj.Data)
		if err != nil {
			return fmt.Errorf("package %q, bundle %q: convert bundle object property at index %d to JSON: %v", b.Package, b.Name, i, err)
		}
		b.Objects = append(b.Objects, string(objJson))
	}
	b.CsvJSON = extractCSV(b.Objects)
	return nil
}

func extractCSV(objs []string) string {
	for _, obj := range objs {
		u := unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(obj), &u); err != nil {
			continue
		}
		if u.GetKind() == operators.ClusterServiceVersionKind {
			return obj
		}
	}
	return ""
}

// LoadReader reads yaml or json from the passed in io.Reader and unmarshals it into a DeclarativeConfig struct.
func LoadReader(r io.Reader) (*DeclarativeConfig, error) {
	builder := fbcBuilder{}
	if err := WalkMetasReader(r, func(meta *Meta, err error) error {
		if err != nil {
			return err
		}
		return builder.addMeta(meta)
	}); err != nil {
		return nil, err
	}
	return &builder.cfg, nil
}

// LoadFile will unmarshall declarative config components from a single filename provided in 'path'
// located at a filesystem hierarchy 'root'
func LoadFile(root fs.FS, path string) (*DeclarativeConfig, error) {
	file, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cfg, err := LoadReader(file)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadSlice will compose declarative config components from a slice of Meta objects
func LoadSlice(metas []*Meta) (*DeclarativeConfig, error) {
	builder := fbcBuilder{}
	for _, meta := range metas {
		if err := builder.addMeta(meta); err != nil {
			return nil, err
		}
	}
	return &builder.cfg, nil
}

type fbcBuilder struct {
	cfg DeclarativeConfig

	packagesMu                  sync.Mutex
	channelsMu                  sync.Mutex
	bundlesMu                   sync.Mutex
	packageV2sMu                sync.Mutex
	packageV2IconsMu            sync.Mutex
	packageV2MetadatasMu        sync.Mutex
	channelV2sMu                sync.Mutex
	bundleV2sMu                 sync.Mutex
	bundleV2MetadatasMu         sync.Mutex
	bundleV2RelatedReferencesMu sync.Mutex
	deprecationsMu              sync.Mutex
	othersMu                    sync.Mutex
}

func (c *fbcBuilder) addMeta(in *Meta) error {
	switch in.Schema {
	case SchemaPackage:
		var p Package
		if err := json.Unmarshal(in.Blob, &p); err != nil {
			return fmt.Errorf("parse package: %v", err)
		}
		c.packagesMu.Lock()
		c.cfg.Packages = append(c.cfg.Packages, p)
		c.packagesMu.Unlock()
	case SchemaChannel:
		var ch Channel
		if err := json.Unmarshal(in.Blob, &ch); err != nil {
			return fmt.Errorf("parse channel: %v", err)
		}
		c.channelsMu.Lock()
		c.cfg.Channels = append(c.cfg.Channels, ch)
		c.channelsMu.Unlock()
	case SchemaBundle:
		var b Bundle
		if err := json.Unmarshal(in.Blob, &b); err != nil {
			return fmt.Errorf("parse bundle: %v", err)
		}
		if err := readBundleObjects(&b); err != nil {
			return fmt.Errorf("read bundle objects: %v", err)
		}
		c.bundlesMu.Lock()
		c.cfg.Bundles = append(c.cfg.Bundles, b)
		c.bundlesMu.Unlock()
	case SchemaPackageV2:
		var p PackageV2
		if err := json.Unmarshal(in.Blob, &p); err != nil {
			return fmt.Errorf("parse package: %v", err)
		}
		c.packageV2sMu.Lock()
		c.cfg.PackageV2s = append(c.cfg.PackageV2s, p)
		c.packageV2sMu.Unlock()
	case SchemaPackageV2Icon:
		var p PackageV2Icon
		if err := json.Unmarshal(in.Blob, &p); err != nil {
			return fmt.Errorf("parse package icon: %v", err)
		}
		c.packageV2IconsMu.Lock()
		c.cfg.PackageV2Icons = append(c.cfg.PackageV2Icons, p)
		c.packageV2IconsMu.Unlock()
	case SchemaPackageV2Metadata:
		var p PackageV2Metadata
		if err := json.Unmarshal(in.Blob, &p); err != nil {
			return fmt.Errorf("parse package metadata: %v", err)
		}
		c.packageV2MetadatasMu.Lock()
		c.cfg.PackageV2Metadatas = append(c.cfg.PackageV2Metadatas, p)
		c.packageV2MetadatasMu.Unlock()
	case SchemaChannelV2:
		var u ChannelV2
		if err := json.Unmarshal(in.Blob, &u); err != nil {
			return fmt.Errorf("parse channel v2: %v", err)
		}
		c.channelV2sMu.Lock()
		c.cfg.ChannelV2s = append(c.cfg.ChannelV2s, u)
		c.channelV2sMu.Unlock()
	case SchemaBundleV2:
		var b BundleV2
		if err := json.Unmarshal(in.Blob, &b); err != nil {
			return fmt.Errorf("parse bundle: %v", err)
		}
		c.bundleV2sMu.Lock()
		c.cfg.BundleV2s = append(c.cfg.BundleV2s, b)
		c.bundleV2sMu.Unlock()
	case SchemaBundleV2Metadata:
		var b BundleV2Metadata
		if err := json.Unmarshal(in.Blob, &b); err != nil {
			return fmt.Errorf("parse bundle metadata: %v", err)
		}
		c.bundleV2MetadatasMu.Lock()
		c.cfg.BundleV2Metadatas = append(c.cfg.BundleV2Metadatas, b)
		c.bundleV2MetadatasMu.Unlock()
	case SchemaBundleV2RelatedReferences:
		var b BundleV2RelatedReferences
		if err := json.Unmarshal(in.Blob, &b); err != nil {
			return fmt.Errorf("parse bundle related references: %v", err)
		}
		c.bundleV2RelatedReferencesMu.Lock()
		c.cfg.BundleV2RelatedReferences = append(c.cfg.BundleV2RelatedReferences, b)
		c.bundleV2RelatedReferencesMu.Unlock()
	case SchemaDeprecation:
		var d Deprecation
		if err := json.Unmarshal(in.Blob, &d); err != nil {
			return fmt.Errorf("parse deprecation: %w", err)
		}
		c.deprecationsMu.Lock()
		c.cfg.Deprecations = append(c.cfg.Deprecations, d)
		c.deprecationsMu.Unlock()
	case "":
		return fmt.Errorf("object '%s' is missing root schema field", string(in.Blob))
	default:
		c.othersMu.Lock()
		c.cfg.Others = append(c.cfg.Others, *in)
		c.othersMu.Unlock()
	}
	return nil
}
