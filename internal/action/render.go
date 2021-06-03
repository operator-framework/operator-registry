package action

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/operator-framework/operator-registry/internal/declcfg"
	"github.com/operator-framework/operator-registry/internal/property"
	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

type Render struct {
	Refs     []string
	Registry image.Registry
}

func nullLogger() *logrus.Entry {
	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)
	return logrus.NewEntry(logger)
}

func (r Render) Run(ctx context.Context) (*declcfg.DeclarativeConfig, error) {
	if r.Registry == nil {
		reg, err := r.createRegistry()
		if err != nil {
			return nil, fmt.Errorf("create registry: %v", err)
		}
		defer reg.Destroy()
		r.Registry = reg
	}

	var cfgs []declcfg.DeclarativeConfig
	for _, ref := range r.Refs {
		var (
			cfg *declcfg.DeclarativeConfig
			err error
		)
		// TODO(joelanford): Add support for detecting and rendering sqlite files.
		if stat, serr := os.Stat(ref); serr == nil && stat.IsDir() {
			cfg, err = declcfg.LoadDir(ref)
		} else {
			cfg, err = r.imageToDeclcfg(ctx, ref)
		}
		if err != nil {
			return nil, fmt.Errorf("render reference %q: %v", ref, err)
		}
		renderBundleObjects(cfg)
		cfgs = append(cfgs, *cfg)
	}

	return combineConfigs(cfgs), nil
}

func (r Render) createRegistry() (*containerdregistry.Registry, error) {
	cacheDir, err := os.MkdirTemp("", "render-registry-")
	if err != nil {
		return nil, fmt.Errorf("create tempdir: %v", err)
	}

	reg, err := containerdregistry.NewRegistry(
		containerdregistry.WithCacheDir(cacheDir),

		// The containerd registry impl is somewhat verbose, even on the happy path,
		// so discard all logger logs. Any important failures will be returned from
		// registry methods and eventually logged as fatal errors.
		containerdregistry.WithLog(nullLogger()),
	)
	if err != nil {
		return nil, err
	}
	return reg, nil
}

func (r Render) imageToDeclcfg(ctx context.Context, imageRef string) (*declcfg.DeclarativeConfig, error) {
	ref := image.SimpleReference(imageRef)
	if err := r.Registry.Pull(ctx, ref); err != nil {
		return nil, err
	}
	labels, err := r.Registry.Labels(ctx, ref)
	if err != nil {
		return nil, err
	}
	tmpDir, err := ioutil.TempDir("", "render-unpack-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)
	if err := r.Registry.Unpack(ctx, ref, tmpDir); err != nil {
		return nil, err
	}

	var cfg *declcfg.DeclarativeConfig
	if dbFile, ok := labels[containertools.DbLocationLabel]; ok {
		cfg, err = sqliteToDeclcfg(ctx, filepath.Join(tmpDir, dbFile))
		if err != nil {
			return nil, err
		}
	} else if configsDir, ok := labels["operators.operatorframework.io.index.configs.v1"]; ok {
		// TODO(joelanford): Make a constant for above configs location label
		cfg, err = declcfg.LoadDir(filepath.Join(tmpDir, configsDir))
		if err != nil {
			return nil, err
		}
	} else if _, ok := labels[bundle.PackageLabel]; ok {
		img, err := registry.NewImageInput(ref, tmpDir)
		if err != nil {
			return nil, err
		}

		cfg, err = bundleToDeclcfg(img.Bundle)
		if err != nil {
			return nil, err
		}
	} else {
		labelKeys := sets.StringKeySet(labels)
		labelVals := []string{}
		for _, k := range labelKeys.List() {
			labelVals = append(labelVals, fmt.Sprintf("  %s=%s", k, labels[k]))
		}
		if len(labelVals) > 0 {
			return nil, fmt.Errorf("render %q: image type could not be determined, found labels\n%s", ref, strings.Join(labelVals, "\n"))
		} else {
			return nil, fmt.Errorf("render %q: image type could not be determined: image has no labels", ref)
		}
	}
	return cfg, nil
}

func sqliteToDeclcfg(ctx context.Context, dbFile string) (*declcfg.DeclarativeConfig, error) {
	db, err := sqlite.Open(dbFile)
	if err != nil {
		return nil, err
	}

	migrator, err := sqlite.NewSQLLiteMigrator(db)
	if err != nil {
		return nil, err
	}
	if migrator == nil {
		return nil, fmt.Errorf("failed to load migrator")
	}

	if err := migrator.Migrate(ctx); err != nil {
		return nil, err
	}

	q := sqlite.NewSQLLiteQuerierFromDb(db)
	m, err := sqlite.ToModel(ctx, q)
	if err != nil {
		return nil, err
	}

	cfg := declcfg.ConvertFromModel(m)
	return &cfg, nil
}

func bundleToDeclcfg(bundle *registry.Bundle) (*declcfg.DeclarativeConfig, error) {
	bundleProperties, err := registry.PropertiesFromBundle(bundle)
	if err != nil {
		return nil, fmt.Errorf("get properties for bundle %q: %v", bundle.Name, err)
	}
	relatedImages, err := getRelatedImages(bundle)
	if err != nil {
		return nil, fmt.Errorf("get related images for bundle %q: %v", bundle.Name, err)
	}

	dBundle := declcfg.Bundle{
		Schema:        "olm.bundle",
		Name:          bundle.Name,
		Package:       bundle.Package,
		Image:         bundle.BundleImage,
		Properties:    bundleProperties,
		RelatedImages: relatedImages,
	}

	return &declcfg.DeclarativeConfig{Bundles: []declcfg.Bundle{dBundle}}, nil
}

func getRelatedImages(b *registry.Bundle) ([]declcfg.RelatedImage, error) {
	csv, err := b.ClusterServiceVersion()
	if err != nil {
		return nil, err
	}

	var objmap map[string]*json.RawMessage
	if err = json.Unmarshal(csv.Spec, &objmap); err != nil {
		return nil, err
	}

	rawValue, ok := objmap["relatedImages"]
	if !ok || rawValue == nil {
		return nil, err
	}

	var relatedImages []declcfg.RelatedImage
	if err = json.Unmarshal(*rawValue, &relatedImages); err != nil {
		return nil, err
	}
	return relatedImages, nil
}

func renderBundleObjects(cfg *declcfg.DeclarativeConfig) {
	for bi, b := range cfg.Bundles {
		props := b.Properties[:0]
		for _, p := range b.Properties {
			if p.Type != property.TypeBundleObject {
				props = append(props, p)
			}
		}

		for _, obj := range b.Objects {
			props = append(props, property.MustBuildBundleObjectData([]byte(obj)))
		}
		cfg.Bundles[bi].Properties = props
	}
}

func combineConfigs(cfgs []declcfg.DeclarativeConfig) *declcfg.DeclarativeConfig {
	out := &declcfg.DeclarativeConfig{}
	for _, in := range cfgs {
		out.Packages = append(out.Packages, in.Packages...)
		out.Bundles = append(out.Bundles, in.Bundles...)
		out.Others = append(out.Others, in.Others...)
	}
	return out
}
