package declcfg

import (
	"context"
)

type CatalogFilter interface {
	FilterCatalog(ctx context.Context, fbc *DeclarativeConfig) (*DeclarativeConfig, error)
}

type MetaFilter interface {
	KeepMeta(meta *Meta) bool
}

type MetaFilterFunc func(meta *Meta) bool

func (f MetaFilterFunc) KeepMeta(meta *Meta) bool { return f(meta) }
