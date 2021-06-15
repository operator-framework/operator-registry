package declcfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/joelanford/ignore"
	"github.com/operator-framework/api/pkg/operators"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/internal/property"
)

// LoadFS loads a declarative config from the provided root FS. LoadFS walks the
// filesystem from root and uses a gitignore-style filename matcher to skip files
// that match patterns found in .indexignore files found throughout the filesystem.
// If LoadFS encounters an error loading or parsing any file, the error will be
// immedidately returned.
func LoadFS(root fs.FS) (*DeclarativeConfig, error) {
	if root == nil {
		return nil, fmt.Errorf("no declarative config filesystem provided")
	}
	cfg := &DeclarativeConfig{}

	matcher, err := ignore.NewMatcher(root, ".indexignore")
	if err != nil {
		return nil, err
	}

	if err := walkFiles(root, func(path string, r io.Reader) error {
		if matcher.Match(path, false) {
			return nil
		}
		fileCfg, err := readYAMLOrJSON(r)
		if err != nil {
			return fmt.Errorf("could not load config file %q: %v", path, err)
		}
		if err := readBundleObjects(fileCfg.Bundles, root, path); err != nil {
			return fmt.Errorf("read bundle objects: %v", err)
		}
		cfg.Packages = append(cfg.Packages, fileCfg.Packages...)
		cfg.Bundles = append(cfg.Bundles, fileCfg.Bundles...)
		cfg.Others = append(cfg.Others, fileCfg.Others...)

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to read declarative configs dir: %v", err)
	}
	return cfg, nil
}

func readBundleObjects(bundles []Bundle, root fs.FS, path string) error {
	for bi, b := range bundles {
		props, err := property.Parse(b.Properties)
		if err != nil {
			return fmt.Errorf("parse properties for bundle %q: %v", b.Name, err)
		}
		for oi, obj := range props.BundleObjects {
			d, err := obj.GetData(root, filepath.Dir(path))
			if err != nil {
				return fmt.Errorf("get data for bundle object[%d]: %v", oi, err)
			}
			bundles[bi].Objects = append(bundles[bi].Objects, string(d))
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

func readYAMLOrJSON(r io.Reader) (*DeclarativeConfig, error) {
	cfg := &DeclarativeConfig{}
	dec := yaml.NewYAMLOrJSONDecoder(r, 4096)
	for {
		doc := json.RawMessage{}
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		doc = []byte(strings.NewReplacer(`\u003c`, "<", `\u003e`, ">", `\u0026`, "&").Replace(string(doc)))

		var in Meta
		if err := json.Unmarshal(doc, &in); err != nil {
			return nil, err
		}

		switch in.Schema {
		case schemaPackage:
			var p Package
			if err := json.Unmarshal(doc, &p); err != nil {
				return nil, fmt.Errorf("parse package: %v", err)
			}
			cfg.Packages = append(cfg.Packages, p)
		case schemaBundle:
			var b Bundle
			if err := json.Unmarshal(doc, &b); err != nil {
				return nil, fmt.Errorf("parse bundle: %v", err)
			}
			cfg.Bundles = append(cfg.Bundles, b)
		case "":
			return nil, fmt.Errorf("object '%s' is missing root schema field", string(doc))
		default:
			cfg.Others = append(cfg.Others, in)
		}
	}
	return cfg, nil
}

func walkFiles(root fs.FS, f func(string, io.Reader) error) error {
	return fs.WalkDir(root, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := root.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		return f(path, file)
	})
}
