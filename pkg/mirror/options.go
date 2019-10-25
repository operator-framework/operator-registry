package mirror

import (
	"fmt"
)

type IndexImageMirrorerOptions struct {
	ImageMirrorer     ImageMirrorer
	DatabaseExtractor DatabaseExtractor

	Source, Dest            string
	ManifestDir            string
}

func (o *IndexImageMirrorerOptions) Validate() error {
	// TODO: better validation

	if o.ImageMirrorer == nil {
		return fmt.Errorf("can't mirror without a mirrorer configured")
	}
	if o.DatabaseExtractor == nil {
		return fmt.Errorf("can't mirror without a database extractor configured")
	}
	if o.Source == "" {
		return fmt.Errorf("source image required")
	}

	if o.Dest == "" {
		return fmt.Errorf("destination registry required")
	}

	if o.ManifestDir == "" {
		return fmt.Errorf("must have directory to write manifests to")
	}

	return nil
}

func (o *IndexImageMirrorerOptions) Complete() error {
	if o.ManifestDir == "" {
		o.ManifestDir = "./manifests"
	}
	return nil
}

// Apply sequentially applies the given options to the config.
func (c *IndexImageMirrorerOptions) Apply(options []ImageIndexMirrorOption) {
	for _, option := range options {
		option(c)
	}
}

type ImageIndexMirrorOption func(*IndexImageMirrorerOptions)

func DefaultImageIndexMirrorerOptions() *IndexImageMirrorerOptions {
	return &IndexImageMirrorerOptions{
		ManifestDir: "./manifests",
	}
}

func WithMirrorer(i ImageMirrorer) ImageIndexMirrorOption {
	return func(o *IndexImageMirrorerOptions) {
		o.ImageMirrorer = i
	}
}

func WithExtractor(e DatabaseExtractor ) ImageIndexMirrorOption {
	return func(o *IndexImageMirrorerOptions) {
		o.DatabaseExtractor = e
	}
}

func WithSource(s string) ImageIndexMirrorOption {
	return func(o *IndexImageMirrorerOptions) {
		o.Source = s
	}
}


func WithDest(d string) ImageIndexMirrorOption {
	return func(o *IndexImageMirrorerOptions) {
		o.Dest = d
	}
}

func WithManifestDir(d string) ImageIndexMirrorOption {
	return func(o *IndexImageMirrorerOptions) {
		o.ManifestDir = d
	}
}
