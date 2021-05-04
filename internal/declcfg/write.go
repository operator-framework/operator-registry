package declcfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	"github.com/operator-framework/operator-registry/internal/property"
)

const (
	globalName = "__global"
)

func WriteDir(cfg DeclarativeConfig, configDir string) error {
	entries, err := ioutil.ReadDir(configDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("config dir %q must be empty", configDir)
	}

	return writeToFS(cfg, &diskWriter{}, configDir)
}

type fsWriter interface {
	MkdirAll(path string, mode os.FileMode) error
	WriteFile(path string, data []byte, mode os.FileMode) error
}

var _ fsWriter = &diskWriter{}

type diskWriter struct{}

func (w diskWriter) MkdirAll(path string, mode os.FileMode) error {
	return os.MkdirAll(path, mode)
}

func (w diskWriter) WriteFile(path string, data []byte, mode os.FileMode) error {
	return ioutil.WriteFile(path, data, mode)
}

func writeToFS(cfg DeclarativeConfig, w fsWriter, rootDir string) error {
	bundlesByPackage := map[string][]Bundle{}
	for _, b := range cfg.Bundles {
		bundlesByPackage[b.Package] = append(bundlesByPackage[b.Package], b)
	}
	othersByPackage := map[string][]Meta{}
	for _, o := range cfg.Others {
		pkgName := o.Package
		if pkgName == "" {
			pkgName = globalName
		}
		othersByPackage[pkgName] = append(othersByPackage[pkgName], o)
	}

	if err := w.MkdirAll(rootDir, 0777); err != nil {
		return fmt.Errorf("mkdir %q: %v", rootDir, err)
	}

	for _, p := range cfg.Packages {
		fcfg := DeclarativeConfig{
			Packages: []Package{p},
			Bundles:  bundlesByPackage[p.Name],
			Others:   othersByPackage[p.Name],
		}
		pkgDir := filepath.Join(rootDir, p.Name)
		if err := w.MkdirAll(pkgDir, 0777); err != nil {
			return err
		}
		filename := filepath.Join(pkgDir, fmt.Sprintf("%s.json", p.Name))
		if err := writeFile(fcfg, w, filename); err != nil {
			return err
		}

		for _, b := range fcfg.Bundles {
			if err := writeObjectFiles(b, w, pkgDir); err != nil {
				return fmt.Errorf("write object files for bundle %q: %v", b.Name, err)
			}
		}
	}

	if globals, ok := othersByPackage[globalName]; ok {
		gcfg := DeclarativeConfig{
			Others: globals,
		}
		filename := filepath.Join(rootDir, fmt.Sprintf("%s.json", globalName))
		if err := writeFile(gcfg, w, filename); err != nil {
			return err
		}
	}
	return nil
}

func writeObjectFiles(b Bundle, w fsWriter, baseDir string) error {
	props, err := property.Parse(b.Properties)
	if err != nil {
		return fmt.Errorf("parse properties: %v", err)
	}
	if len(props.BundleObjects) != len(b.Objects) {
		return fmt.Errorf("expected %d properties of type %q, found %d", len(b.Objects), property.TypeBundleObject, len(props.BundleObjects))
	}
	for i, p := range props.BundleObjects {
		if p.IsRef() {
			objPath := filepath.Join(baseDir, p.GetRef())
			objDir := filepath.Dir(objPath)
			if err := w.MkdirAll(objDir, 0777); err != nil {
				return fmt.Errorf("create directory %q for bundle object ref %q: %v", objDir, p.GetRef(), err)
			}
			if err := w.WriteFile(objPath, []byte(b.Objects[i]), 0666); err != nil {
				return fmt.Errorf("write bundle object for ref %q: %v", p.GetRef(), err)
			}
		}
	}
	return nil
}

func writeFile(cfg DeclarativeConfig, w fsWriter, filename string) error {
	buf := &bytes.Buffer{}
	if err := WriteJSON(cfg, buf); err != nil {
		return fmt.Errorf("write to buffer for %q: %v", filename, err)
	}
	if err := w.WriteFile(filename, buf.Bytes(), 0666); err != nil {
		return fmt.Errorf("write file %q: %v", filename, err)
	}
	return nil
}

func WriteJSON(cfg DeclarativeConfig, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)
	return writeToEncoder(cfg, enc)
}

func WriteYAML(cfg DeclarativeConfig, w io.Writer) error {
	enc := newYAMLEncoder(w)
	return writeToEncoder(cfg, enc)
}

type yamlEncoder struct {
	encodedFirst bool
	w            io.Writer
}

func newYAMLEncoder(w io.Writer) *yamlEncoder {
	return &yamlEncoder{false, w}
}

func (e *yamlEncoder) Encode(v interface{}) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	yamlData, err := yaml.JSONToYAML(buf.Bytes())
	if err != nil {
		return err
	}
	if e.encodedFirst {
		yamlData = append([]byte("---\n"), yamlData...)
	}
	e.encodedFirst = true
	_, err = e.w.Write(yamlData)
	return err
}

type encoder interface {
	Encode(interface{}) error
}

func writeToEncoder(cfg DeclarativeConfig, enc encoder) error {
	pkgNames := sets.NewString()

	packagesByName := map[string][]Package{}
	for _, p := range cfg.Packages {
		pkgName := p.Name
		pkgNames.Insert(pkgName)
		packagesByName[pkgName] = append(packagesByName[pkgName], p)
	}
	bundlesByPackage := map[string][]Bundle{}
	for _, b := range cfg.Bundles {
		pkgName := b.Package
		pkgNames.Insert(pkgName)
		bundlesByPackage[pkgName] = append(bundlesByPackage[pkgName], b)
	}
	othersByPackage := map[string][]Meta{}
	for _, o := range cfg.Others {
		pkgName := o.Package
		if pkgName == "" {
			pkgName = globalName
		}
		pkgNames.Insert(pkgName)
		othersByPackage[pkgName] = append(othersByPackage[pkgName], o)
	}

	for _, pName := range pkgNames.List() {
		if pName == globalName {
			continue
		}
		pkgs := packagesByName[pName]
		for _, p := range pkgs {
			if err := enc.Encode(p); err != nil {
				return err
			}
		}

		bundles := bundlesByPackage[pName]
		sort.Slice(bundles, func(i, j int) bool {
			return bundles[i].Name < bundles[j].Name
		})
		for _, b := range bundles {
			if err := enc.Encode(b); err != nil {
				return err
			}
		}

		others := othersByPackage[pName]
		sort.SliceStable(others, func(i, j int) bool {
			return others[i].Schema < others[j].Schema
		})
		for _, o := range others {
			if err := enc.Encode(o); err != nil {
				return err
			}
		}
	}

	for _, o := range othersByPackage[globalName] {
		if err := enc.Encode(o); err != nil {
			return err
		}
	}
	return nil
}
