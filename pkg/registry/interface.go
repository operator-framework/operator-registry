package registry

import (
	"context"
)

type Load interface {
	AddOperatorBundle(bundle *Bundle) error
	AddPackageChannels(manifest PackageManifest) error
	AddProvidedApis() error
}

type Query interface {
	ListPackages(context context.Context) ([]string, error)
	GetPackage(context context.Context, name string) (*PackageManifest, error)
	GetBundleForChannel(context context.Context, pkgName string, channelName string) (string, error)
	GetBundleForName(context context.Context, name string) (string, error)
	// Get all channel entries that say they replace this one
	GetChannelEntriesThatReplace(context context.Context, name string) (entries []*ChannelEntry, err error)
	// Get the bundle in a package/channel that replace this one
	GetBundleThatReplaces(context context.Context, name, pkgName, channelName string) (string, error)
	// Get all channel entries that provide an api
	GetChannelEntriesThatProvide(context context.Context, groupOrName, version, kind string) (entries []*ChannelEntry, err error)
	// Get latest channel entries that provide an api
	GetLatestChannelEntriesThatProvide(context context.Context, groupOrName, version, kind string) (entries []*ChannelEntry, err error)
	// Get the the latest bundle that provides the API in a default channel
	GetBundleThatProvides(context context.Context, groupOrName, version, kind string) (string, error)
}
