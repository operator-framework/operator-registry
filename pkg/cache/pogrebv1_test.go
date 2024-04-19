package cache

import (
	"context"
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/api"
)

func TestPogrebV1_StableDigest(t *testing.T) {
	cacheDir := t.TempDir()
	c := &cache{backend: newPogrebV1Backend(cacheDir)}
	require.NoError(t, c.Build(context.Background(), validFS))

	actualDigest, err := c.backend.GetDigest(context.Background())
	require.NoError(t, err)

	// NOTE: The entire purpose of this test is to ensure that we don't change the cache
	// implementation and inadvertantly invalidate existing caches.
	//
	// Therefore, DO NOT CHANGE the expected digest value here unless validFS also
	// changes.
	//
	// If validFS needs to change DO NOT CHANGE the json cache implementation
	// in the same pull request.
	require.Equal(t, "485a767449dd66d4", actualDigest)
}

func TestPogrebV1_CheckIntegrity(t *testing.T) {
	type testCase struct {
		name   string
		build  bool
		fbcFS  fs.FS
		mod    func(t *testing.T, tc *testCase, cacheDir string, backend backend)
		expect func(t *testing.T, err error)
	}
	testCases := []testCase{
		{
			name:  "non-existent cache dir",
			fbcFS: validFS,
			mod: func(t *testing.T, tc *testCase, cacheDir string, _ backend) {
				require.NoError(t, os.RemoveAll(cacheDir))
			},
			expect: func(t *testing.T, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "read existing cache digest")
			},
		},
		{
			name:  "empty cache dir",
			fbcFS: validFS,
			expect: func(t *testing.T, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "read existing cache digest")
			},
		},
		{
			name:  "valid cache dir",
			build: true,
			fbcFS: validFS,
			expect: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:  "different FBC",
			build: true,
			fbcFS: validFS,
			mod: func(t *testing.T, tc *testCase, _ string, _ backend) {
				tc.fbcFS = badBundleFS
			},
			expect: func(t *testing.T, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "cache requires rebuild")
			},
		},
		{
			name:  "different cache",
			build: true,
			fbcFS: validFS,
			mod: func(t *testing.T, tc *testCase, cacheDir string, b backend) {
				require.NoError(t, b.PutBundle(context.Background(), bundleKey{"foo", "bar", "baz"}, &api.Bundle{PackageName: "foo", ChannelName: "bar", CsvName: "baz"}))
			},
			expect: func(t *testing.T, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "cache requires rebuild")
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cacheDir := t.TempDir()
			c := &cache{backend: newPogrebV1Backend(cacheDir)}

			if tc.build {
				require.NoError(t, c.Build(context.Background(), tc.fbcFS))
			}
			if tc.mod != nil {
				tc.mod(t, &tc, cacheDir, c.backend)
			}
			tc.expect(t, c.CheckIntegrity(context.Background(), tc.fbcFS))
		})
	}
}
