package cache

import (
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func Test_fsToTar(t *testing.T) {
	type testCase struct {
		name   string
		fsys   func() fs.FS
		expect func(*testing.T, []byte, error)
	}
	testCases := []testCase{
		{
			name: "non-existent fs path",
			fsys: func() fs.FS {
				notExist, _ := fs.Sub(fstest.MapFS{}, "sub")
				return notExist
			},
			expect: func(t *testing.T, bytes []byte, err error) {
				require.True(t, errors.Is(err, fs.ErrNotExist))
			},
		},
		{
			// NOTE: The entire purpose of this test is to ensure that the fsToTar implementation
			// is stable over time
			//
			// Therefore, DO NOT CHANGE the expected digest value here unless validFS also
			// changes.
			//
			// If validFS needs to change DO NOT CHANGE the fsToTar implementation in the same
			// pull request.
			name: "stable hash output",
			fsys: func() fs.FS { return validFS },
			expect: func(t *testing.T, i []byte, err error) {
				require.NoError(t, err)
				hasher := fnv.New64a()
				hasher.Write(i)
				require.Equal(t, "6f9eec5b366c557f", fmt.Sprintf("%x", hasher.Sum(nil)))
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := bytes.Buffer{}
			err := fsToTar(&w, tc.fsys(), nil)
			tc.expect(t, w.Bytes(), err)
		})
	}
}
