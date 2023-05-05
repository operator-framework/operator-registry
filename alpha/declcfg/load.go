package declcfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/joelanford/ignore"
	"github.com/operator-framework/api/pkg/operators"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/alpha/property"
)

const (
	indexIgnoreFilename = ".indexignore"
)

type WalkMetasFSFunc func(path string, meta *Meta, err error) error

func WalkMetasFS(root fs.FS, walkFn WalkMetasFSFunc) error {
	return walkFiles(root, func(root fs.FS, path string, err error) error {
		if err != nil {
			return walkFn(path, nil, err)
		}

		f, err := root.Open(path)
		if err != nil {
			return walkFn(path, nil, err)
		}
		defer f.Close()

		return WalkMetasReader(f, func(meta *Meta, err error) error {
			return walkFn(path, meta, err)
		})
	})
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

// LoadFS loads a declarative config from the provided root FS. LoadFS walks the
// filesystem from root and uses a gitignore-style filename matcher to skip files
// that match patterns found in .indexignore files found throughout the filesystem.
// If LoadFS encounters an error loading or parsing any file, the error will be
// immediately returned.
func LoadFS(root fs.FS) (*DeclarativeConfig, error) {
	cfg := &DeclarativeConfig{}
	if err := WalkFS(root, func(path string, fcfg *DeclarativeConfig, err error) error {
		if err != nil {
			return err
		}
		cfg.Packages = append(cfg.Packages, fcfg.Packages...)
		cfg.Channels = append(cfg.Channels, fcfg.Channels...)
		cfg.Bundles = append(cfg.Bundles, fcfg.Bundles...)
		cfg.Others = append(cfg.Others, fcfg.Others...)
		return nil
	}); err != nil {
		return nil, err
	}
	return cfg, nil
}

func readBundleObjects(bundles []Bundle, root fs.FS, path string) error {
	for bi, b := range bundles {
		props, err := property.Parse(b.Properties)
		if err != nil {
			return fmt.Errorf("package %q, bundle %q: parse properties: %v", b.Package, b.Name, err)
		}
		for oi, obj := range props.BundleObjects {
			objID := fmt.Sprintf(" %q", obj.GetRef())
			if !obj.IsRef() {
				objID = fmt.Sprintf("[%d]", oi)
			}

			d, err := obj.GetData(root, filepath.Dir(path))
			if err != nil {
				return fmt.Errorf("package %q, bundle %q: get data for bundle object%s: %v", b.Package, b.Name, objID, err)
			}
			objJson, err := yaml.ToJSON(d)
			if err != nil {
				return fmt.Errorf("package %q, bundle %q: convert object%s to JSON: %v", b.Package, b.Name, objID, err)
			}
			bundles[bi].Objects = append(bundles[bi].Objects, string(objJson))
		}
		bundles[bi].CsvJSON = extractCSV(bundles[bi].Objects)
	}
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
// Path references will not be de-referenced so callers are responsible for de-referencing if necessary.
func LoadReader(r io.Reader) (*DeclarativeConfig, error) {
	cfg := &DeclarativeConfig{}

	if err := WalkMetasReader(r, func(in *Meta, err error) error {
		if err != nil {
			return err
		}
		switch in.Schema {
		case SchemaPackage:
			var p Package
			if err := json.Unmarshal(in.Blob, &p); err != nil {
				return fmt.Errorf("parse package: %v", err)
			}
			cfg.Packages = append(cfg.Packages, p)
		case SchemaChannel:
			var c Channel
			if err := json.Unmarshal(in.Blob, &c); err != nil {
				return fmt.Errorf("parse channel: %v", err)
			}
			cfg.Channels = append(cfg.Channels, c)
		case SchemaBundle:
			var b Bundle
			if err := json.Unmarshal(in.Blob, &b); err != nil {
				return fmt.Errorf("parse bundle: %v", err)
			}
			cfg.Bundles = append(cfg.Bundles, b)
		case "":
			return fmt.Errorf("object '%s' is missing root schema field", string(in.Blob))
		default:
			cfg.Others = append(cfg.Others, *in)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return cfg, nil
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

	if err := readBundleObjects(cfg.Bundles, root, path); err != nil {
		return nil, fmt.Errorf("read bundle objects: %v", err)
	}

	return cfg, nil
}
