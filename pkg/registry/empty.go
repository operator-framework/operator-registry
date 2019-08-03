package registry

import (
	"context"
	
	"github.com/pkg/errors"
)

type EmptyLoad struct{}

func (EmptyLoad) AddOperatorBundle(bundle *Bundle) error {
	return errors.New("empty loader: cannot add operator bundle")
}

func (EmptyLoad) AddPackageChannels(manifest PackageManifest) error {
	return errors.New("empty loader: cannot add package channels")
}

func (EmptyLoad) AddLoadError(err *LoadError) {}

func NewEmptyLoad() *EmptyLoad {
	return &EmptyLoad{}
}

type EmptyQuery struct{}

func (EmptyQuery) ListTables(ctx context.Context) ([]string, error) {
	return nil, errors.New("empty querier: cannot list tables")
}

func (EmptyQuery) ListPackages(ctx context.Context) ([]string, error) {
	return nil, errors.New("empty querier: cannot list packages")
}

func (EmptyQuery) GetPackage(ctx context.Context, name string) (*PackageManifest, error) {
	return nil, errors.New("empty querier: cannot get package")
}

func (EmptyQuery) GetBundle(ctx context.Context, pkgName, channelName, csvName string) (string, error) {
	return "", errors.New("empty querier: cannot get bundle")
}

func (EmptyQuery) GetBundleForChannel(ctx context.Context, pkgName string, channelName string) (string, error) {
	return "", errors.New("empty querier: cannot get bundle for channel")
}

func (EmptyQuery) GetChannelEntriesThatReplace(ctx context.Context, name string) (entries []*ChannelEntry, err error) {
	return nil, errors.New("empty querier: cannot get channel entries that replace")
}

func (EmptyQuery) GetBundleThatReplaces(ctx context.Context, name, pkgName, channelName string) (string, error) {
	return "", errors.New("empty querier: cannot get bundle that replaces")
}

func (EmptyQuery) GetChannelEntriesThatProvide(ctx context.Context, group, version, kind string) (entries []*ChannelEntry, err error) {
	return nil, errors.New("empty querier: cannot get channel entries that provide")
}

func (EmptyQuery) GetLatestChannelEntriesThatProvide(ctx context.Context, group, version, kind string) (entries []*ChannelEntry, err error) {
	return nil, errors.New("empty querier: cannot get latest channel entries that provide")
}

func (EmptyQuery) GetBundleThatProvides(ctx context.Context, group, version, kind string) (string, *ChannelEntry, error) {
	return "", nil, errors.New("empty querier: cannot get bundle that provides")
}

func NewEmptyQuerier() *EmptyQuery {
	return &EmptyQuery{}
}
