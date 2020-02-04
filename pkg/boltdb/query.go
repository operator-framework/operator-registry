package boltdb

import (
	"context"
	"fmt"
	"github.com/asdine/storm/v3"
	"github.com/asdine/storm/v3/q"
	"github.com/operator-framework/operator-registry/pkg/boltdb/model"

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
	var pkgs []model.Package
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
	var pkg model.Package
	if err := s.db.One("Name", name, &pkg); err != nil {
		return nil, err
	}

	var channels []model.Channel
	if err := s.db.Find("PackageName", name, &channels); err != nil {
		return nil, fmt.Errorf("couldn't get channels for package %s: %v", name, err)
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
	var opBundle model.OperatorBundle
	if err := s.db.One("Name", csvName, &opBundle); err != nil {
		return nil, fmt.Errorf("couldn't find bundle for %s, %s, %s: %v", pkgName, channelName, csvName, err)
	}

	// Convert raw bytes into individual bundle JSON strings
	objs, err := registry.BundleStringToObjectStrings(string(opBundle.Bundle))
	if err != nil {
		return nil, err
	}
	bundle := &api.Bundle{
		CsvName:      opBundle.Name,
		CsvJson:      string(opBundle.CSV),
		Object:       objs,
		BundlePath:   opBundle.BundlePath,
		Version:      opBundle.Version,
		SkipRange:    opBundle.SkipRange,
		PackageName:  pkgName,
		ChannelName:  channelName,
		ProvidedApis: []*api.GroupVersionKind{},
		RequiredApis: []*api.GroupVersionKind{},
	}

	// provided apis
	for _, cap := range opBundle.Capabilities {
		fmt.Printf("%#v", cap)

		if cap.Name != model.GvkCapability {
			continue
		}
		gvk, ok := cap.Value.(*model.Api)
		if !ok {
			continue
		}
		if err != nil {
			return nil, err
		}
		bundle.ProvidedApis = append(bundle.ProvidedApis, &api.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
			Plural:  gvk.Plural,
		})
	}

	// required apis
	for _, req := range opBundle.Requirements {
		if req.Name != model.GvkCapability {
			continue
		}
		gvk, ok := req.Selector.(*model.ApiEqualitySelector)
		if !ok {
			continue
		}
		if err != nil {
			return nil, err
		}
		bundle.RequiredApis = append(bundle.RequiredApis, &api.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
			Plural:  gvk.Plural,
		})
	}

	return bundle, nil
}

func (s *StormQuerier) GetBundleForChannel(ctx context.Context, pkgName string, channelName string) (*api.Bundle, error) {
	var channel model.Channel
	if err := s.db.Select(q.Eq("PackageName", pkgName), q.Eq("ChannelName", channelName)).First(&channel); err != nil {
		return nil, fmt.Errorf("couldn't fetch bundle for %s %s: %v", pkgName, channelName, err)
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
	var entry model.ChannelEntry
	if err := s.db.Select(q.Eq("PackageName", pkgName), q.Eq("ChannelName", channelName), q.Eq("Replaces", name)).First(&entry); err != nil {
		return nil, err
	}

	return s.GetBundle(ctx, entry.PackageName, entry.ChannelName, entry.BundleName)
}

type GVKMatcher struct {
	Group string
	Version string
	Kind string
}

func (m GVKMatcher) MatchField(v interface{}) (bool, error) {
	caps, ok := v.([]model.Capability)
	if !ok {
		return false, fmt.Errorf("not a capability list")
	}

	for _, c := range caps {
		if c.Name != model.GvkCapability {
			continue
		}
		api, ok := c.Value.(*model.Api)
		if !ok {
			continue
		}
		if api.Group == m.Group && api.Kind == m.Kind && api.Version == m.Version {
			return true, nil
		}
	}
	return false, nil
}

func (s *StormQuerier) GetChannelEntriesThatProvide(ctx context.Context, group, version, kind string) (entries []*registry.ChannelEntry, err error) {
	var bundles []model.OperatorBundle
	if err := s.db.Select(q.NewFieldMatcher("Capabilities", GVKMatcher{
		Group:   group,
		Version: version,
		Kind:    kind,
	})).Find(&bundles); err != nil {
		return nil, err
	}

	bundleNames := make([]string, len(bundles))
	for i, b := range bundles {
		bundleNames[i] = b.Name
	}
	var ents []model.ChannelEntry
	if err := s.db.Select(q.In("BundleName", bundleNames)).Find(&ents); err != nil {
		return nil, err
	}

	entries = make([]*registry.ChannelEntry, 0)
	for _, e := range ents {
		entries = append(entries, &registry.ChannelEntry{
			PackageName: e.PackageName,
			ChannelName: e.ChannelName,
			BundleName:  e.BundleName,
			Replaces:    e.Replaces,
		})
	}
	return
}

// Get latest channel entries that provide an API.
func (s *StormQuerier) GetLatestChannelEntriesThatProvide(ctx context.Context, group, version, kind string) (entries []*registry.ChannelEntry, err error) {
	var bundles []model.OperatorBundle
	if err := s.db.Select(q.NewFieldMatcher("Capabilities", GVKMatcher{
		Group:   group,
		Version: version,
		Kind:    kind,
	})).Find(&bundles); err != nil {
		return nil, err
	}

	bundleNames := make([]string, len(bundles))
	for i, b := range bundles {
		bundleNames[i] = b.Name
	}
	var ents []model.ChannelEntry
	if err := s.db.Select(q.In("BundleName", bundleNames)).Find(&ents); err != nil {
		return nil, err
	}

	pkgChannelToLatest := map[model.PackageChannel]*registry.ChannelEntry{}
	// record which packagechannels we have entries in
	for _, e := range ents {
		pkgChannelToLatest[model.PackageChannel{
			PackageName: e.PackageName,
			ChannelName: e.ChannelName,
		}] = nil
	}

	// for each package channel, find the latest entry
	for key := range pkgChannelToLatest {
		// TODO: do better
		for j := 0; j < len(ents)*len(pkgChannelToLatest); j++ {
			for _, e := range ents {
				if e.PackageName != key.PackageName || e.ChannelName != key.ChannelName {
					continue
				}
				if pkgChannelToLatest[key] == nil {
					pkgChannelToLatest[key] = &registry.ChannelEntry{
						PackageName: e.PackageName,
						ChannelName: e.ChannelName,
						BundleName:  e.BundleName,
						Replaces:    e.Replaces,
					}
					continue
				}
				if e.Replaces == pkgChannelToLatest[key].BundleName {
					pkgChannelToLatest[key] = &registry.ChannelEntry{
						PackageName: e.PackageName,
						ChannelName: e.ChannelName,
						BundleName:  e.BundleName,
						Replaces:    e.Replaces,
					}
					continue
				}
			}
		}
	}

	entries = make([]*registry.ChannelEntry, 0)
	for _, e := range pkgChannelToLatest {
		entries = append(entries, e)
	}

	return
}

// Get the latest bundle that provides the API in a default channel, error unless there is ONLY one.
func (s *StormQuerier) GetBundleThatProvides(ctx context.Context, group, version, kind string) (*api.Bundle, error) {
	entries, err := s.GetLatestChannelEntriesThatProvide(ctx, group, version, kind)
	if err != nil {
		return nil, err
	}

	// We will have 1 entry per package/channel
	pkgChannelToLatest := map[model.PackageChannel]*registry.ChannelEntry{}
	pkgName := ""
	// record which packagechannels we have entries in
	for _, e := range entries {
		if pkgName == "" {
			pkgName = e.PackageName
		} else if pkgName != e.PackageName {
			return nil, fmt.Errorf("more than one entry found that provides %s %s %s", group, version, kind)
		}
		pkgChannelToLatest[model.PackageChannel{
			PackageName: e.PackageName,
			ChannelName: e.ChannelName,
		}] = e
	}

	pkg, err := s.GetPackage(ctx, pkgName)
	if err != nil {
		return nil, err
	}
	provider, ok := pkgChannelToLatest[model.PackageChannel{
		PackageName: pkgName,
		ChannelName: pkg.GetDefaultChannel(),
	}]
	if !ok {
		return nil, fmt.Errorf("no entry found that provides %s %s %s", group, version, kind)
	}

	return s.GetBundle(ctx, pkgName, pkg.GetDefaultChannel(), provider.BundleName)
}

func (s *StormQuerier) ListImages(ctx context.Context) ([]string, error) {
	var related []model.RelatedImage
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
	var related []model.RelatedImage
	if err := s.db.Find("OperatorBundleName", bundleName, &related); err != nil {
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

func (s *StormQuerier) GetBundleVersion(ctx context.Context, image string) (string, error) {
	panic("implement me")
}

func (s *StormQuerier) GetBundlePathsForPackage(ctx context.Context, pkgName string) ([]string, error) {
	panic("implement me")
}

func (s *StormQuerier) GetDefaultChannelForPackage(ctx context.Context, pkgName string) (string, error) {
	panic("implement me")
}

func (s *StormQuerier) ListChannels(ctx context.Context, pkgName string) ([]string, error) {
	panic("implement me")
}

func (s *StormQuerier) GetCurrentCSVNameForChannel(ctx context.Context, pkgName, channel string) (string, error) {
	panic("implement me")
}