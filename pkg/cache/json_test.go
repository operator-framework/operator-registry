package cache

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSON_StableDigest(t *testing.T) {
	cacheDir := t.TempDir()
	c := NewJSON(cacheDir)
	require.NoError(t, c.Build(context.Background(), validFS))

	actualDigest, err := c.existingDigest()
	require.NoError(t, err)

	// NOTE: The entire purpose of this test is to ensure that we don't change the cache
	// implementation and inadvertantly invalidate existing caches.
	//
	// Therefore, DO NOT CHANGE the expected digest value here unless validFS also
	// changes.
	//
	// If validFS needs to change DO NOT CHANGE the json cache implementation
	// in the same pull request.
	require.Equal(t, "9adad9ff6cf54e4f", actualDigest)
}

func TestJSON_CheckIntegrity(t *testing.T) {
	type testCase struct {
		name   string
		build  bool
		fbcFS  fs.FS
		mod    func(tc *testCase, cacheDir string) error
		expect func(t *testing.T, err error)
	}
	testCases := []testCase{
		{
			name:  "non-existent cache dir",
			fbcFS: validFS,
			mod: func(tc *testCase, cacheDir string) error {
				return os.RemoveAll(cacheDir)
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
			mod: func(tc *testCase, _ string) error {
				tc.fbcFS = badBundleFS
				return nil
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
			mod: func(tc *testCase, cacheDir string) error {
				return os.WriteFile(filepath.Join(cacheDir, jsonDir, "foo"), []byte("bar"), jsonCacheModeFile)
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
			c := NewJSON(cacheDir)

			if tc.build {
				require.NoError(t, c.Build(context.Background(), tc.fbcFS))
			}
			if tc.mod != nil {
				require.NoError(t, tc.mod(&tc, cacheDir))
			}
			tc.expect(t, c.CheckIntegrity(tc.fbcFS))
		})
	}
}
