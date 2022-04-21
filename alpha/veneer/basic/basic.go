package basic

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/pkg/image"
)

type Veneer struct {
	Registry image.Registry
}

func (v Veneer) Render(ctx context.Context, ref string) (*declcfg.DeclarativeConfig, error) {
	// only taking first argument as file
	stat, serr := os.Stat(ref)
	if serr != nil {
		return nil, serr
	}

	if stat.IsDir() {
		return nil, errors.New("cannot render veneers by directory reference")
	}
	return v.renderFile(ctx, ref)
}

func (v Veneer) renderFile(ctx context.Context, ref string) (*declcfg.DeclarativeConfig, error) {
	// xform any relative to absolute paths
	abspath, err := filepath.Abs(ref)
	if err != nil {
		return nil, err
	}
	// xform to break apart dir/file elements
	rpath, fname := filepath.Split(abspath)
	root := os.DirFS(rpath)

	cfg, err := declcfg.LoadFile(root, fname)
	if err != nil {
		return cfg, err
	}

	outb := cfg.Bundles[:0] // allocate based on max size of input, but empty slice
	// populate registry, incl any flags from CLI, and enforce only rendering bundle images
	r := action.Render{
		Registry:       v.Registry,
		AllowedRefMask: action.RefBundleImage,
	}

	for _, b := range cfg.Bundles {
		if !isBundleVeneer(&b) {
			outb = append(outb, b)
			continue
		}
		r.Refs = []string{b.Image}
		contributor, err := r.Run(ctx)
		if err != nil {
			return nil, err
		}
		outb = append(outb, contributor.Bundles...)
	}

	cfg.Bundles = outb
	return cfg, nil
}

// isBundleVeneer identifies loaded partial Bundle data from YAML/JSON veneer source as having no properties,
func isBundleVeneer(b *declcfg.Bundle) bool {
	return len(b.Properties) == 0
}
