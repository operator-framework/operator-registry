package server

import (
	"github.com/operator-framework/api/pkg/registry/api"
	"github.com/operator-framework/api/pkg/registry"

	"golang.org/x/net/context"
)

type RegistryServer struct {
	store registry.Query
}

var _ api.RegistryServer = &RegistryServer{}

func NewRegistryServer(store registry.Query) *RegistryServer {
	return &RegistryServer{store}
}

func (s *RegistryServer) ListPackages(req *api.ListPackageRequest, stream api.Registry_ListPackagesServer) error {
	packageNames, err := s.store.ListPackages(stream.Context())
	if err != nil {
		return err
	}
	for _, p := range packageNames {
		if err := stream.Send(&api.PackageName{Name: p}); err != nil {
			return err
		}
	}

	return nil
}

func (s *RegistryServer) GetPackage(ctx context.Context, req *api.GetPackageRequest) (*api.Package, error) {
	packageManifest, err := s.store.GetPackage(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	return registry.PackageManifestToAPIPackage(packageManifest), nil
}

func (s *RegistryServer) GetBundle(ctx context.Context, req *api.GetBundleRequest) (*api.Bundle, error) {
	return s.store.GetBundle(ctx, req.GetPkgName(), req.GetChannelName(), req.GetCsvName())
}

func (s *RegistryServer) GetBundleForChannel(ctx context.Context, req *api.GetBundleInChannelRequest) (*api.Bundle, error) {
	return s.store.GetBundleForChannel(ctx, req.GetPkgName(), req.GetChannelName())
}

func (s *RegistryServer) GetChannelEntriesThatReplace(req *api.GetAllReplacementsRequest, stream api.Registry_GetChannelEntriesThatReplaceServer) error {
	channelEntries, err := s.store.GetChannelEntriesThatReplace(stream.Context(), req.GetCsvName())
	if err != nil {
		return err
	}
	for _, e := range channelEntries {
		if err := stream.Send(registry.ChannelEntryToAPIChannelEntry(e)); err != nil {
			return err
		}
	}
	return nil
}

func (s *RegistryServer) GetBundleThatReplaces(ctx context.Context, req *api.GetReplacementRequest) (*api.Bundle, error) {
	return s.store.GetBundleThatReplaces(ctx, req.GetCsvName(), req.GetPkgName(), req.GetChannelName())
}

func (s *RegistryServer) GetChannelEntriesThatProvide(req *api.GetAllProvidersRequest, stream api.Registry_GetChannelEntriesThatProvideServer) error {
	channelEntries, err := s.store.GetChannelEntriesThatProvide(stream.Context(), req.GetGroup(), req.GetVersion(), req.GetKind())
	if err != nil {
		return err
	}
	for _, e := range channelEntries {
		if err := stream.Send(registry.ChannelEntryToAPIChannelEntry(e)); err != nil {
			return err
		}
	}
	return nil
}

func (s *RegistryServer) GetLatestChannelEntriesThatProvide(req *api.GetLatestProvidersRequest, stream api.Registry_GetLatestChannelEntriesThatProvideServer) error {
	channelEntries, err := s.store.GetLatestChannelEntriesThatProvide(stream.Context(), req.GetGroup(), req.GetVersion(), req.GetKind())
	if err != nil {
		return err
	}
	for _, e := range channelEntries {
		if err := stream.Send(registry.ChannelEntryToAPIChannelEntry(e)); err != nil {
			return err
		}
	}
	return nil
}

func (s *RegistryServer) GetDefaultBundleThatProvides(ctx context.Context, req *api.GetDefaultProviderRequest) (*api.Bundle, error) {
	return s.store.GetBundleThatProvides(ctx, req.GetGroup(), req.GetVersion(), req.GetKind())
}
