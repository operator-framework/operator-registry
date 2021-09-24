package registry

import (
	"context"
	"fmt"
	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDeprecated(t *testing.T) {
	deprecated := map[string]bool{
		"deprecatedBundle": true,
		"otherBundle":      false,
	}

	q := querierFromMap{
		Data: map[string]map[string]map[string]*api.Bundle{"testPkg": {"alpha": {"deprecatedBundle": {
			BundlePath: "deprecatedBundle",
			Properties: []*api.Property{{
				Type:  DeprecatedType,
				Value: "{}",
			}}}, "otherBundle": {BundlePath: "otherBundle"}}}},
	}

	_, err := isDeprecated(context.TODO(), &q, BundleKey{BundlePath: "missingBundle"})
	require.Error(t, err)

	for b := range deprecated {
		isDeprecated, err := isDeprecated(context.TODO(), &q, BundleKey{CsvName: b})
		require.NoError(t, err)
		require.Equal(t, deprecated[b], isDeprecated)
	}
}

type querierFromMap struct {
	Query
	Data map[string]map[string]map[string]*api.Bundle
}

func (q *querierFromMap) GetChannelEntriesFromPackage(ctx context.Context, pkg string) ([]ChannelEntryAnnotated, error) {
	if _, ok := q.Data[pkg]; !ok {
		return nil, ErrPackageNotInDatabase
	}
	entries := []ChannelEntryAnnotated{}
	for channelName, bundles := range q.Data[pkg] {
		for bundleName, b := range bundles {
			entry := ChannelEntryAnnotated{
				PackageName: pkg,
				ChannelName: channelName,
				BundleName:  bundleName,
				BundlePath:  b.BundlePath,
				Version:     b.Version,
				Replaces:    b.Replaces,
			}
			if len(b.Replaces) > 0 {
				if replaced, ok := q.Data[pkg][channelName][b.Replaces]; ok {
					entry.ReplacesVersion = replaced.Version
					entry.ReplacesBundlePath = replaced.BundlePath
				}
			}
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func (q *querierFromMap) GetBundle(ctx context.Context, csvName, version, bundlePath string) (*api.Bundle, error) {
	for _, p := range q.Data {
		for _, c := range p {
			for name, b := range c {
				if len(name) > 0 && len(csvName) > 0 && name != csvName {
					continue
				}
				if len(version) > 0 && len(b.Version) > 0 && version != b.Version {
					continue
				}
				if len(bundlePath) > 0 && len(b.BundlePath) > 0 && bundlePath != b.BundlePath {
					continue
				}
				return b, nil
			}
		}
	}
	return nil, ErrBundleImageNotInDatabase
}

func TestExpectedGraphBundles(t *testing.T) {
	testBundle, err := NewBundleFromStrings("testBundle", "0.0.1", "testPkg", "default", "default", "")
	require.NoError(t, err)
	testBundle.BundleImage = "testImage"
	testBundleKey := BundleKey{
		BundlePath: testBundle.BundleImage,
		Version:    "0.0.1",
		CsvName:    testBundle.Name,
	}
	newTestPackage := func(name string, channelEntries map[string]BundleKey) *Package {
		channels := map[string]Channel{}
		for channelName, node := range channelEntries {
			if _, ok := channels[channelName]; !ok {
				channels[channelName] = Channel{
					Nodes: map[BundleKey]map[BundleKey]struct{}{},
				}
			}
			channels[channelName].Nodes[node] = nil
		}
		return &Package{
			Name:     name,
			Channels: channels,
		}
	}

	tests := []struct {
		description      string
		graphLoader      GraphLoader
		querier          Query
		bundles          []*Bundle
		overwrite        bool
		wantErr          error
		wantGraphBundles map[string]*Package
	}{
		{
			description: "NewPackage",
			querier:     &querierFromMap{},
			bundles:     []*Bundle{testBundle},
			wantGraphBundles: map[string]*Package{
				"testPkg": newTestPackage("testPkg", map[string]BundleKey{"default": testBundleKey}),
			},
		},
		{
			description: "OverwriteWithoutFlag",
			querier: &querierFromMap{Data: map[string]map[string]map[string]*api.Bundle{"testPkg": {"alpha": {testBundleKey.CsvName: &api.Bundle{
				CsvName:     testBundle.Name,
				PackageName: "testPkg",
				ChannelName: "alpha",
				BundlePath:  testBundle.BundleImage,
				Version:     testBundleKey.Version,
			}}}}},
			bundles: []*Bundle{testBundle},
			wantErr: BundleImageAlreadyAddedErr{ErrorString: fmt.Sprintf("Bundle %s already exists", testBundle.BundleImage)},
		},
		{
			description: "OverwriteWithFlag",
			querier: &querierFromMap{Data: map[string]map[string]map[string]*api.Bundle{"testPkg": {"alpha": {testBundleKey.CsvName: &api.Bundle{
				CsvName:     testBundle.Name,
				PackageName: "testPkg",
				ChannelName: "alpha",
				BundlePath:  testBundle.BundleImage,
				Version:     testBundleKey.Version,
			}}}}},
			bundles:   []*Bundle{testBundle},
			overwrite: true,
			wantGraphBundles: map[string]*Package{
				"testPkg": newTestPackage("testPkg", map[string]BundleKey{"default": testBundleKey}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			graphBundles, err := expectedGraphBundles(context.TODO(), tt.bundles, tt.querier, tt.overwrite)
			if tt.wantErr != nil {
				require.EqualError(t, err, tt.wantErr.Error())
				return
			}
			require.NoError(t, err)

			require.EqualValues(t, graphBundles, tt.wantGraphBundles)
		})
	}
}
