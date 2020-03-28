package sqlite

import (
	"testing"

	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/stretchr/testify/require"
)

func compareBundle(t *testing.T, expected, actual *api.Bundle) {
	require.Equal(t, expected.CsvName, actual.CsvName)
	require.Equal(t, expected.PackageName, actual.PackageName)
	require.Equal(t, expected.ChannelName, actual.ChannelName)
	require.Equal(t, expected.Version, actual.Version)
	require.Equal(t, expected.SkipRange, actual.SkipRange)
	require.Equal(t, expected.CsvJson, actual.CsvJson)
	require.Equal(t, expected.Object, actual.Object)
	require.Equal(t, expected.BundlePath, actual.BundlePath)
	require.Equal(t, len(expected.ProvidedApis), len(actual.ProvidedApis))
	require.Equal(t, len(expected.RequiredApis), len(actual.RequiredApis))

	expectedProvidedAPIs := make(map[string]struct{})
	for _, v := range expected.ProvidedApis {
		expectedProvidedAPIs[v.String()] = struct{}{}
	}
	for _, v := range actual.ProvidedApis {
		_, ok := expectedProvidedAPIs[v.String()]
		require.True(t, ok, "Unable to find provided API: %s", v.String())
	}

	expectedRequiredAPIs := make(map[string]struct{})
	for _, v := range expected.RequiredApis {
		expectedRequiredAPIs[v.String()] = struct{}{}
	}
	for _, v := range actual.RequiredApis {
		_, ok := expectedRequiredAPIs[v.String()]
		require.True(t, ok, "Unable to find required API: %s", v.String())
	}
}
