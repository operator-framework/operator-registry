package action

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListPackages(t *testing.T) {
	type spec struct {
		name        string
		list        ListPackages
		expectedOut string
		expectedErr string
	}

	specs := []spec{
		{
			name: "Success/ValidIndex",
			list: ListPackages{IndexReference: "testdata/list-index"},
			expectedOut: `NAME  DISPLAY NAME  DEFAULT CHANNEL
bar   Bar Operator  beta
foo   Foo Operator  beta
`,
		},
		{
			name:        "Error/UnknownIndex",
			list:        ListPackages{IndexReference: "unknown-index"},
			expectedErr: `render reference "unknown-index": repository name must be canonical`,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			res, err := s.list.Run(context.Background())
			if s.expectedErr != "" {
				require.Nil(t, res)
				require.EqualError(t, err, s.expectedErr)
			} else {
				require.NoError(t, err)

				buf := &bytes.Buffer{}
				err = res.WriteColumns(buf)
				require.NoError(t, err)

				require.Equal(t, s.expectedOut, buf.String())
			}
		})
	}
}

func TestListChannels(t *testing.T) {
	type spec struct {
		name        string
		list        ListChannels
		expectedOut string
		expectedErr string
	}
	specs := []spec{
		{
			name: "Success/WithPackage",
			list: ListChannels{IndexReference: "testdata/list-index", PackageName: "foo"},
			expectedOut: `PACKAGE  CHANNEL  HEAD
foo      beta     foo.v0.2.0
foo      stable   foo.v0.2.0
`,
		},
		{
			name: "Success/WithoutPackage",
			list: ListChannels{IndexReference: "testdata/list-index"},
			expectedOut: `PACKAGE  CHANNEL  HEAD
bar      beta     bar.v0.2.0
bar      stable   bar.v0.2.0
foo      beta     foo.v0.2.0
foo      stable   foo.v0.2.0
`,
		},
		{
			name:        "Error/UnknownIndex",
			list:        ListChannels{IndexReference: "unknown-index"},
			expectedErr: `render reference "unknown-index": repository name must be canonical`,
		},
		{
			name:        "Error/UnknownPackage",
			list:        ListChannels{IndexReference: "testdata/list-index", PackageName: "unknown"},
			expectedErr: `package "unknown" not found`,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			res, err := s.list.Run(context.Background())
			if s.expectedErr != "" {
				require.Nil(t, res)
				require.EqualError(t, err, s.expectedErr)
			} else {
				require.NoError(t, err)

				buf := &bytes.Buffer{}
				err = res.WriteColumns(buf)
				require.NoError(t, err)

				require.Equal(t, s.expectedOut, buf.String())
			}
		})
	}
}

func TestListBundles(t *testing.T) {
	type spec struct {
		name        string
		list        ListBundles
		expectedOut string
		expectedErr string
	}
	specs := []spec{
		{
			name: "Success/WithPackage",
			list: ListBundles{IndexReference: "testdata/list-index", PackageName: "foo"},
			expectedOut: `PACKAGE  CHANNEL  BUNDLE      REPLACES    SKIPS                  SKIP RANGE  IMAGE
foo      beta     foo.v0.1.0                                     <0.1.0      test.registry/foo-operator/foo-bundle:v0.1.0
foo      beta     foo.v0.2.0  foo.v0.1.0  foo.v0.1.1,foo.v0.1.2  <0.2.0      test.registry/foo-operator/foo-bundle:v0.2.0
foo      stable   foo.v0.2.0  foo.v0.1.0  foo.v0.1.1,foo.v0.1.2  <0.2.0      test.registry/foo-operator/foo-bundle:v0.2.0
`,
		},
		{
			name: "Success/WithoutPackage",
			list: ListBundles{IndexReference: "testdata/list-index"},
			expectedOut: `PACKAGE  CHANNEL  BUNDLE      REPLACES    SKIPS                  SKIP RANGE  IMAGE
bar      beta     bar.v0.1.0                                     <0.1.0      test.registry/bar-operator/bar-bundle:v0.1.0
bar      beta     bar.v0.2.0  bar.v0.1.0  bar.v0.1.1,bar.v0.1.2  <0.2.0      test.registry/bar-operator/bar-bundle:v0.2.0
bar      stable   bar.v0.2.0  bar.v0.1.0  bar.v0.1.1,bar.v0.1.2  <0.2.0      test.registry/bar-operator/bar-bundle:v0.2.0
foo      beta     foo.v0.1.0                                     <0.1.0      test.registry/foo-operator/foo-bundle:v0.1.0
foo      beta     foo.v0.2.0  foo.v0.1.0  foo.v0.1.1,foo.v0.1.2  <0.2.0      test.registry/foo-operator/foo-bundle:v0.2.0
foo      stable   foo.v0.2.0  foo.v0.1.0  foo.v0.1.1,foo.v0.1.2  <0.2.0      test.registry/foo-operator/foo-bundle:v0.2.0
`,
		},
		{
			name:        "Error/UnknownIndex",
			list:        ListBundles{IndexReference: "unknown-index"},
			expectedErr: `render reference "unknown-index": repository name must be canonical`,
		},
		{
			name:        "Error/UnknownPackage",
			list:        ListBundles{IndexReference: "testdata/list-index", PackageName: "unknown"},
			expectedErr: `package "unknown" not found`,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			res, err := s.list.Run(context.Background())
			if s.expectedErr != "" {
				require.Nil(t, res)
				require.EqualError(t, err, s.expectedErr)
			} else {
				require.NoError(t, err)

				buf := &bytes.Buffer{}
				err = res.WriteColumns(buf)
				require.NoError(t, err)

				require.Equal(t, s.expectedOut, buf.String())
			}
		})
	}
}
