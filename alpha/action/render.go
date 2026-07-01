package action

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/operator-framework/operator-registry/alpha/action/migrations"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containersimageregistry"
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

type RefType uint

const (
	RefBundleImage RefType = 1 << iota
	RefDCImage
	RefDCDir
	RefBundleDir

	RefAll = 0
)

func (r RefType) Allowed(refType RefType) bool {
	return r == RefAll || r&refType == refType
}

var ErrNotAllowed = errors.New("not allowed")

type Render struct {
	Refs             []string
	Registry         image.Registry
	AllowedRefMask   RefType
	ImageRefTemplate *template.Template
	Migrations       *migrations.Migrations
}

func (r Render) Run(ctx context.Context) (*declcfg.DeclarativeConfig, error) {
	if r.Registry == nil {
		reg, err := containersimageregistry.NewDefault()
		if err != nil {
			return nil, fmt.Errorf("create registry: %v", err)
		}
		defer func() {
			_ = reg.Destroy()
		}()
		r.Registry = reg
	}

	// nolint:prealloc
	var cfgs []declcfg.DeclarativeConfig
	for _, ref := range r.Refs {
		cfg, err := r.renderReference(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("render reference %q: %w", ref, err)
		}
		moveBundleObjectsToEndOfPropertySlices(cfg)

		for _, b := range cfg.Bundles {
			sort.Slice(b.RelatedImages, func(i, j int) bool {
				return b.RelatedImages[i].Image < b.RelatedImages[j].Image
			})
		}

		if err := r.migrate(cfg); err != nil {
			return nil, fmt.Errorf("migrate: %v", err)
		}

		cfgs = append(cfgs, *cfg)
	}

	return combineConfigs(cfgs), nil
}

func (r Render) renderReference(ctx context.Context, ref string) (*declcfg.DeclarativeConfig, error) {
	stat, err := os.Stat(ref)
	if err != nil {
		return r.imageToDeclcfg(ctx, ref)
	}
	// nolint:nestif
	if stat.IsDir() {
		dirEntries, err := os.ReadDir(ref)
		if err != nil {
			return nil, err
		}
		if isBundle(dirEntries) {
			// Looks like a bundle directory
			if !r.AllowedRefMask.Allowed(RefBundleDir) {
				return nil, fmt.Errorf("cannot render bundle directory %q: %w", ref, ErrNotAllowed)
			}
			return r.renderBundleDirectory(ref)
		}

		// Otherwise, assume it is a declarative config root directory.
		if !r.AllowedRefMask.Allowed(RefDCDir) {
			return nil, fmt.Errorf("cannot render declarative config directory: %w", ErrNotAllowed)
		}
		return declcfg.LoadFS(ctx, os.DirFS(ref))
	}
	// Only directories are supported for file-based catalogs and bundles.
	return nil, fmt.Errorf("ref %q is not a directory", ref)
}

func (r Render) imageToDeclcfg(ctx context.Context, imageRef string) (*declcfg.DeclarativeConfig, error) {
	ref := image.SimpleReference(imageRef)
	if err := r.Registry.Pull(ctx, ref); err != nil {
		return nil, fmt.Errorf("failed to pull image %q: %v", ref, err)
	}
	labels, err := r.Registry.Labels(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to get labels for image %q: %v", ref, err)
	}
	tmpDir, err := os.MkdirTemp("", "render-unpack-")
	if err != nil {
		return nil, fmt.Errorf("create tempdir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	if err := r.Registry.Unpack(ctx, ref, tmpDir); err != nil {
		return nil, fmt.Errorf("failed to unpack image %q: %v", ref, err)
	}

	cfg, err := r.renderImageConfig(ctx, ref, tmpDir, labels)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (r Render) renderImageConfig(ctx context.Context, ref image.SimpleReference, tmpDir string, labels map[string]string) (*declcfg.DeclarativeConfig, error) {
	if configsDir, ok := labels[containertools.ConfigsLocationLabel]; ok {
		return r.renderDeclcfgImage(ctx, tmpDir, configsDir)
	}
	if _, ok := labels[bundle.PackageLabel]; ok {
		return r.renderBundleImage(ref, tmpDir)
	}
	return nil, r.imageTypeError(ref.String(), labels)
}

func (r Render) renderDeclcfgImage(ctx context.Context, tmpDir, configsDir string) (*declcfg.DeclarativeConfig, error) {
	if !r.AllowedRefMask.Allowed(RefDCImage) {
		return nil, fmt.Errorf("cannot render declarative config image: %w", ErrNotAllowed)
	}
	cfg, err := declcfg.LoadFS(ctx, os.DirFS(filepath.Join(tmpDir, configsDir)))
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (r Render) renderBundleImage(ref image.SimpleReference, tmpDir string) (*declcfg.DeclarativeConfig, error) {
	if !r.AllowedRefMask.Allowed(RefBundleImage) {
		return nil, fmt.Errorf("cannot render bundle image: %w", ErrNotAllowed)
	}
	img, err := registry.NewImageInput(ref, tmpDir)
	if err != nil {
		return nil, err
	}

	bundle, err := bundleToDeclcfg(img.Bundle)
	if err != nil {
		return nil, err
	}
	return &declcfg.DeclarativeConfig{Bundles: []declcfg.Bundle{*bundle}}, nil
}

func (r Render) imageTypeError(ref string, labels map[string]string) error {
	labelKeys := sets.StringKeySet(labels)
	labelList := labelKeys.List()
	labelVals := make([]string, 0, len(labelList))
	for _, k := range labelList {
		labelVals = append(labelVals, fmt.Sprintf("  %s=%s", k, labels[k]))
	}
	if len(labelVals) > 0 {
		return fmt.Errorf("render %q: image type could not be determined, found labels\n%s", ref, strings.Join(labelVals, "\n"))
	}
	return fmt.Errorf("render %q: image type could not be determined: image has no labels", ref)
}

func bundleToDeclcfg(bundle *registry.Bundle) (*declcfg.Bundle, error) {
	objs, props, err := registry.ObjectsAndPropertiesFromBundle(bundle)
	if err != nil {
		return nil, fmt.Errorf("get properties for bundle %q: %v", bundle.Name, err)
	}
	relatedImages, err := getRelatedImages(bundle)
	if err != nil {
		return nil, fmt.Errorf("get related images for bundle %q: %v", bundle.Name, err)
	}

	var csvJSON []byte
	for _, obj := range bundle.Objects {
		if obj.GetKind() == "ClusterServiceVersion" {
			csvJSON, err = json.Marshal(obj)
			if err != nil {
				return nil, fmt.Errorf("marshal CSV JSON for bundle %q: %v", bundle.Name, err)
			}
		}
	}

	return &declcfg.Bundle{
		Schema:        "olm.bundle",
		Name:          bundle.Name,
		Package:       bundle.Package,
		Image:         bundle.BundleImage,
		Properties:    props,
		RelatedImages: relatedImages,
		Objects:       objs,
		CsvJSON:       string(csvJSON),
	}, nil
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

	var relatedImages []declcfg.RelatedImage
	rawValue, ok := objmap["relatedImages"]
	if ok && rawValue != nil {
		if err = json.Unmarshal(*rawValue, &relatedImages); err != nil {
			return nil, err
		}
	}

	// Keep track of the images we've already found, so that we don't add
	// them multiple times.
	allImages := sets.NewString()
	for _, ri := range relatedImages {
		allImages = allImages.Insert(ri.Image)
	}

	if b.BundleImage != "" && !allImages.Has(b.BundleImage) {
		relatedImages = append(relatedImages, declcfg.RelatedImage{
			Image: b.BundleImage,
		})
	}

	opImages, err := csv.GetOperatorImages()
	if err != nil {
		return nil, err
	}
	for img := range opImages {
		if !allImages.Has(img) {
			relatedImages = append(relatedImages, declcfg.RelatedImage{
				Image: img,
			})
		}
		allImages = allImages.Insert(img)
	}

	return relatedImages, nil
}

func moveBundleObjectsToEndOfPropertySlices(cfg *declcfg.DeclarativeConfig) {
	for bi, b := range cfg.Bundles {
		var (
			others []property.Property
			objs   []property.Property
		)
		for _, p := range b.Properties {
			switch p.Type {
			case property.TypeBundleObject, property.TypeCSVMetadata:
				objs = append(objs, p)
			default:
				others = append(others, p)
			}
		}
		cfg.Bundles[bi].Properties = append(others, objs...)
	}
}

func (r Render) migrate(cfg *declcfg.DeclarativeConfig) error {
	// If there are no migrations, do nothing.
	if r.Migrations == nil {
		return nil
	}
	return r.Migrations.Migrate(cfg)
}

func combineConfigs(cfgs []declcfg.DeclarativeConfig) *declcfg.DeclarativeConfig {
	out := &declcfg.DeclarativeConfig{}
	for _, in := range cfgs {
		out.Merge(&in)
	}
	return out
}

func isBundle(entries []os.DirEntry) bool {
	foundManifests := false
	foundMetadata := false
	for _, e := range entries {
		if e.IsDir() {
			switch e.Name() {
			case "manifests":
				foundManifests = true
			case "metadata":
				foundMetadata = true
			}
		}
		if foundMetadata && foundManifests {
			return true
		}
	}
	return false
}

type imageReferenceTemplateData struct {
	Package string
	Name    string
	Version string
}

func (r *Render) renderBundleDirectory(ref string) (*declcfg.DeclarativeConfig, error) {
	img, err := registry.NewImageInput(image.SimpleReference(""), ref)
	if err != nil {
		return nil, err
	}
	if err := r.templateBundleImageRef(img.Bundle); err != nil {
		return nil, fmt.Errorf("failed templating image reference from bundle for %q: %v", ref, err)
	}
	fbcBundle, err := bundleToDeclcfg(img.Bundle)
	if err != nil {
		return nil, err
	}
	return &declcfg.DeclarativeConfig{Bundles: []declcfg.Bundle{*fbcBundle}}, nil
}

func (r *Render) templateBundleImageRef(bundle *registry.Bundle) error {
	if r.ImageRefTemplate == nil {
		return nil
	}

	var pkgProp property.Package
	for _, p := range bundle.Properties {
		if p.Type != property.TypePackage {
			continue
		}
		if err := json.Unmarshal(p.Value, &pkgProp); err != nil {
			return err
		}
		break
	}

	var buf strings.Builder
	tmplInput := imageReferenceTemplateData{
		Package: bundle.Package,
		Name:    bundle.Name,
		Version: pkgProp.Version,
	}
	if err := r.ImageRefTemplate.Execute(&buf, tmplInput); err != nil {
		return err
	}
	bundle.BundleImage = buf.String()
	return nil
}
