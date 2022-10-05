package basic

import (
	"context"
	"fmt"
	"io"

	"github.com/grokspawn/api/pkg/lib/declcfg"
	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/pkg/image"
)

type Veneer struct {
	Registry image.Registry
}

func (v Veneer) Render(ctx context.Context, reader io.Reader) (*declcfg.DeclarativeConfig, error) {
	cfg, err := declcfg.LoadReader(reader)
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
			return nil, fmt.Errorf("unexpected fields present in basic veneer bundle")
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

// isBundleVeneer identifies a Bundle veneer source as having a Schema and Image defined
// but no Properties, RelatedImages or Package defined
func isBundleVeneer(b *declcfg.Bundle) bool {
	return b.Schema != "" && b.Image != "" && b.Package == "" && len(b.Properties) == 0 && len(b.RelatedImages) == 0
}
