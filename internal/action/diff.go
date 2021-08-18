package action

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/operator-framework/operator-registry/internal/declcfg"
	"github.com/operator-framework/operator-registry/internal/model"
	"github.com/operator-framework/operator-registry/pkg/image"
)

type Diff struct {
	Registry image.Registry

	OldRefs []string
	NewRefs []string
	// SkipDependencies directs Run() to not include dependencies
	// of bundles included in the diff if true.
	SkipDependencies bool

	Logger *logrus.Entry
}

func (a Diff) Run(ctx context.Context) (*declcfg.DeclarativeConfig, error) {
	if err := a.validate(); err != nil {
		return nil, err
	}

	// Disallow bundle refs.
	mask := RefDCDir | RefDCImage | RefSqliteFile | RefSqliteImage

	// Heads-only mode does not require an old ref, so there may be nothing to render.
	var oldModel model.Model
	if len(a.OldRefs) != 0 {
		oldRender := Render{Refs: a.OldRefs, Registry: a.Registry, AllowedRefMask: mask}
		oldCfg, err := oldRender.Run(ctx)
		if err != nil {
			if errors.Is(err, ErrNotAllowed) {
				return nil, fmt.Errorf("%w (diff does not permit direct bundle references)", err)
			}
			return nil, fmt.Errorf("error rendering old refs: %v", err)
		}
		oldModel, err = declcfg.ConvertToModel(*oldCfg)
		if err != nil {
			return nil, fmt.Errorf("error converting old declarative config to model: %v", err)
		}
	}

	newRender := Render{Refs: a.NewRefs, Registry: a.Registry, AllowedRefMask: mask}
	newCfg, err := newRender.Run(ctx)
	if err != nil {
		if errors.Is(err, ErrNotAllowed) {
			return nil, fmt.Errorf("%w (diff does not permit direct bundle references)", err)
		}
		return nil, fmt.Errorf("error rendering new refs: %v", err)
	}
	newModel, err := declcfg.ConvertToModel(*newCfg)
	if err != nil {
		return nil, fmt.Errorf("error converting new declarative config to model: %v", err)
	}

	g := &declcfg.DiffGenerator{
		Logger:           a.Logger,
		SkipDependencies: a.SkipDependencies,
	}
	diffModel, err := g.Run(oldModel, newModel)
	if err != nil {
		return nil, fmt.Errorf("error generating diff: %v", err)
	}

	cfg := declcfg.ConvertFromModel(diffModel)
	return &cfg, nil
}

func (p Diff) validate() error {
	if len(p.NewRefs) == 0 {
		return fmt.Errorf("no new refs to diff")
	}
	return nil
}
