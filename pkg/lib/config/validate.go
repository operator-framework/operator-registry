package config

import (
	"context"
	"io/fs"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

// Validate loads and validates declarative config file(s) from the given filesystem.
// Validates schema conformance, replaces chain integrity, and channel graph structure.
// Returns a hierarchical error tree describing all validation issues found.
func Validate(ctx context.Context, root fs.FS) error {
	cfg, err := declcfg.LoadFS(ctx, root)
	if err != nil {
		return err
	}
	return declcfg.Validate(*cfg)
}
