package client

import (
	"context"
	"errors"
	"testing"

	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/api/grpc_health_v1"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type RegistryClientStub struct {
	ListBundlesClient api.Registry_ListBundlesClient
	error             error
}

func (s *RegistryClientStub) ListPackages(ctx context.Context, in *api.ListPackageRequest, opts ...grpc.CallOption) (api.Registry_ListPackagesClient, error) {
	return nil, nil
}

func (s *RegistryClientStub) GetPackage(ctx context.Context, in *api.GetPackageRequest, opts ...grpc.CallOption) (*api.Package, error) {
	return nil, nil
}

func (s *RegistryClientStub) GetBundle(ctx context.Context, in *api.GetBundleRequest, opts ...grpc.CallOption) (*api.Bundle, error) {
	return nil, nil
}

func (s *RegistryClientStub) GetBundleForChannel(ctx context.Context, in *api.GetBundleInChannelRequest, opts ...grpc.CallOption) (*api.Bundle, error) {
	return nil, nil
}

func (s *RegistryClientStub) GetChannelEntriesThatReplace(ctx context.Context, in *api.GetAllReplacementsRequest, opts ...grpc.CallOption) (api.Registry_GetChannelEntriesThatReplaceClient, error) {
	return nil, nil
}

func (s *RegistryClientStub) GetBundleThatReplaces(ctx context.Context, in *api.GetReplacementRequest, opts ...grpc.CallOption) (*api.Bundle, error) {
	return nil, nil
}

func (s *RegistryClientStub) GetChannelEntriesThatProvide(ctx context.Context, in *api.GetAllProvidersRequest, opts ...grpc.CallOption) (api.Registry_GetChannelEntriesThatProvideClient, error) {
	return nil, nil
}

func (s *RegistryClientStub) GetLatestChannelEntriesThatProvide(ctx context.Context, in *api.GetLatestProvidersRequest, opts ...grpc.CallOption) (api.Registry_GetLatestChannelEntriesThatProvideClient, error) {
	return nil, nil
}

func (s *RegistryClientStub) GetDefaultBundleThatProvides(ctx context.Context, in *api.GetDefaultProviderRequest, opts ...grpc.CallOption) (*api.Bundle, error) {
	return nil, nil
}

func (s *RegistryClientStub) ListBundles(ctx context.Context, in *api.ListBundlesRequest, opts ...grpc.CallOption) (api.Registry_ListBundlesClient, error) {
	return s.ListBundlesClient, s.error
}

func (s *RegistryClientStub) Check(ctx context.Context, in *grpc_health_v1.HealthCheckRequest, opts ...grpc.CallOption) (*grpc_health_v1.HealthCheckResponse, error) {
	return nil, nil
}

type BundleReceiverStub struct {
	Bundle *api.Bundle
	error  error
	grpc.ClientStream
}

func (s *BundleReceiverStub) Recv() (*api.Bundle, error) {
	return s.Bundle, s.error
}

func TestListBundlesError(t *testing.T) {
	expected := errors.New("test error")
	stub := &RegistryClientStub{
		error: expected,
	}
	c := Client{
		Registry: stub,
		Health:   stub,
	}

	_, actual := c.ListBundles(context.TODO())
	require.Equal(t, expected, actual)
}

func TestListBundlesRecvError(t *testing.T) {
	expected := errors.New("test error")
	rstub := &BundleReceiverStub{
		error: expected,
	}
	cstub := &RegistryClientStub{
		ListBundlesClient: rstub,
	}
	c := Client{
		Registry: cstub,
		Health:   cstub,
	}

	it, err := c.ListBundles(context.TODO())
	require.NoError(t, err)

	require.Nil(t, it.Next())
	require.Equal(t, expected, it.Error())
}

func TestListBundlesNext(t *testing.T) {
	expected := &api.Bundle{CsvName: "test"}
	rstub := &BundleReceiverStub{
		Bundle: expected,
	}
	cstub := &RegistryClientStub{
		ListBundlesClient: rstub,
	}
	c := Client{
		Registry: cstub,
		Health:   cstub,
	}

	it, err := c.ListBundles(context.TODO())
	require.NoError(t, err)

	actual := it.Next()
	require.NoError(t, it.Error())
	require.Equal(t, expected, actual)
}
