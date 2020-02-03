package boltdb

import (
	"context"
	"fmt"

	"github.com/asdine/storm/v3"
	"github.com/asdine/storm/v3/q"

	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

// StormQuerier queries over operatory registry data overa a BoltDB database.
type StormQuerier struct {
	db *storm.DB
}

var _ registry.Query = &StormQuerier{}

func NewStormQuerier(db *storm.DB) *StormQuerier {
	return &StormQuerier{
		db: db,
	}
}

func (*StormQuerier) ListTables(ctx context.Context) ([]string, error) {
	panic("implement me")
}

func (s *StormQuerier) ListPackages(ctx context.Context) ([]string, error) {
	var pkgs []Package
	if err := s.db.All(&pkgs); err != nil {
		return nil, err
	}

	var names []string
	for _, pkg := range pkgs {
		names = append(names, pkg.Name)
	}

	return names, nil
}

func (s *StormQuerier) GetPackage(ctx context.Context, name string) (*registry.PackageManifest, error) {
	var pkg Package
	if err := s.db.One("Name", name, &pkg); err != nil {
		// TODO(njhale): Check behavior of sqlite querier when no package is found -- do we return an error for this case?
		return nil, err
	}

	var channels []Channel
	if err := s.db.Find("PackageName", name, &channels); err != nil {
		// TODO(njhale): Check behavior of sqlite querier when no package channels are found -- do we return an error for this case?
		return nil, err
	}

	pkgManifest := &registry.PackageManifest{
		PackageName:        pkg.Name,
		DefaultChannelName: pkg.DefaultChannel,
	}
	for _, channel := range channels {
		pkgManifest.Channels = append(pkgManifest.Channels, registry.PackageChannel{
			Name:           channel.ChannelName,
			CurrentCSVName: channel.HeadOperatorBundleName,
		})
	}

	return pkgManifest, nil
}

func (s *StormQuerier) GetBundle(ctx context.Context, pkgName, channelName, csvName string) (*api.Bundle, error) {
	// We only need the csvName to query for the OperatorBundle, since their names are 1:1
	var opBundle OperatorBundle
	if err := s.db.One("Name", csvName, &opBundle); err != nil {
		// TODO(njhale): Check behavior of sqlite querier when no bundle is found -- do we return an error for this case?
		return nil, err
	}

	// Convert raw bytes into individual bundle JSON strings
	objs, err := registry.BundleStringToObjectStrings(string(opBundle.Bundle))
	if err != nil {
		return nil, err
	}
	bundle := &api.Bundle{
		CsvName:    opBundle.Name,
		CsvJson:    string(opBundle.CSV),
		Object:     objs,
		BundlePath: opBundle.BundlePath,
		Version:    opBundle.Version,
		SkipRange:  opBundle.SkipRange,
	}

	// Collect provided and required APIs
	err = s.db.Select().Each(new(RelatedAPI), func(record interface{}) error {
		related, ok := record.(RelatedAPI)
		if !ok {
			return fmt.Errorf("bad related api record")
		}

		gvk := &api.GroupVersionKind{
			Group:   related.Group,
			Version: related.Version,
			Kind:    related.Kind,
			Plural:  related.Plural,
		}
		if related.Provides {
			bundle.ProvidedApis = append(bundle.ProvidedApis, gvk)
		} else {
			bundle.RequiredApis = append(bundle.RequiredApis, gvk)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return bundle, nil
}

func (s *StormQuerier) GetBundleForChannel(ctx context.Context, pkgName string, channelName string) (*api.Bundle, error) {
	var channel Channel
	if err := s.db.Select(q.Eq("PackageName", pkgName), q.Eq("ChannelName", channelName)).First(&channel); err != nil {
		return nil, err
	}

	return s.GetBundle(ctx, pkgName, channelName, channel.HeadOperatorBundleName)
}

func (s *StormQuerier) GetChannelEntriesThatReplace(ctx context.Context, name string) (entries []*registry.ChannelEntry, err error) {
	if err = s.db.Find("Replaces", name, &entries); err != nil {
		return
	}

	return
}

func (s *StormQuerier) GetBundleThatReplaces(ctx context.Context, name, pkgName, channelName string) (*api.Bundle, error) {
	var entry ChannelEntry
	if err := s.db.Select(q.Eq("PackageName", pkgName), q.Eq("ChannelName", channelName), q.Eq("Replaces", name)).First(&entry); err != nil {
		return nil, err
	}

	return s.GetBundle(ctx, pkgName, channelName, name)
}

func (s *StormQuerier) GetChannelEntriesThatProvide(ctx context.Context, group, version, kind string) (entries []*registry.ChannelEntry, err error) {
	var providers []RelatedAPI
	err = s.db.Select(
		q.Eq("GVK", GVK{Group: group, Version: version, Kind: kind}),
		q.Eq("Provides", true),
	).Find(&providers)
	if err != nil {
		return nil, err
	}

	for _, provider := range providers {
		var providerEntries []ChannelEntry
		if err = s.db.Find("OperatorBundleName", provider.OperatorBundleName, &providerEntries); err != nil {
			return
		}

		for _, entry := range providerEntries {
			entries = append(entries, &registry.ChannelEntry{
				PackageName: entry.PackageName,
				ChannelName: entry.ChannelName,
				BundleName:  entry.OperatorBundleName,
				Replaces:    entry.OperatorBundleName,
			})
		}
	}

	return
}

// Get latest channel entries that provide an API.
func (s *StormQuerier) GetLatestChannelEntriesThatProvide(ctx context.Context, group, version, kind string) (entries []*registry.ChannelEntry, err error) {
	var latest LatestGVKProvider
	if err = s.db.One("GVK", GVK{Group: group, Version: version, Kind: kind}, &latest); err != nil {
		return
	}

	var providerEntries []ChannelEntry
	if err = s.db.Find("OperatorBundleName", latest.OperatorBundleName, &providerEntries); err != nil {
		return
	}

	for _, entry := range providerEntries {
		entries = append(entries, &registry.ChannelEntry{
			PackageName: entry.PackageName,
			ChannelName: entry.ChannelName,
			BundleName:  entry.OperatorBundleName,
			Replaces:    entry.Replaces,
		})
	}

	return
}

// Get the latest bundle that provides the API in a default channel, error unless there is ONLY one.
func (s *StormQuerier) GetBundleThatProvides(ctx context.Context, group, version, kind string) (*api.Bundle, error) {
	entries, err := s.GetLatestChannelEntriesThatProvide(ctx, group, version, kind)
	if err != nil {
		return nil, err
	}

	// Map the default PackageChannels
	var pkgs []Package
	if err = s.db.All(&pkgs); err != nil {
		return nil, err
	}

	defaultPkgChannels := map[PackageChannel]struct{}{}
	for _, pkg := range pkgs {
		defaultPkgChannels[PackageChannel{PackageName: pkg.Name, ChannelName: pkg.DefaultChannel}] = struct{}{}
	}

	// Get the entry for the latest default provider
	var provider *registry.ChannelEntry
	for i, entry := range entries {
		if _, ok := defaultPkgChannels[PackageChannel{PackageName: entry.PackageName, ChannelName: entry.ChannelName}]; !ok {
			// Not a default channel, skip
			continue
		}
		if provider != nil {
			return nil, fmt.Errorf("more than one entry found that provides %s %s %s", group, version, kind)
		}
		provider = entries[i]
	}

	if provider == nil {
		return nil, fmt.Errorf("no entry found that provides %s %s %s", group, version, kind)
	}

	return s.GetBundle(ctx, provider.BundleName, provider.ChannelName, provider.PackageName)
}

func (s *StormQuerier) ListImages(ctx context.Context) ([]string, error) {
	var related []RelatedImage
	if err := s.db.All(&related); err != nil {
		return nil, err
	}

	var images []string
	visited := map[string]struct{}{}
	for _, relation := range related {
		if _, ok := visited[relation.Image]; ok {
			continue
		}
		images = append(images, relation.Image)
		visited[relation.Image] = struct{}{}
	}

	return images, nil
}

func (s *StormQuerier) GetImagesForBundle(ctx context.Context, bundleName string) ([]string, error) {
	var related []RelatedImage
	if err := s.db.Find("OperatorBundleName", bundleName, related); err != nil {
		return nil, err
	}

	var images []string
	for _, relation := range related {
		images = append(images, relation.Image)
	}

	return images, nil
}

func (*StormQuerier) GetApisForEntry(ctx context.Context, entryId int64) (provided []*api.GroupVersionKind, required []*api.GroupVersionKind, err error) {
	panic("implement me")
}
