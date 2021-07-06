package action

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListPackages(t *testing.T) {
	type spec struct {
		name     string
		list     ListPackages
		expected string
	}

	specs := []spec{
		{
			name: "Success",
			list: ListPackages{IndexReference: "testdata/foo-index-v0.2.0-declcfg"},
			expected: `NAME  DISPLAY NAME  DEFAULT CHANNEL
foo   Foo Operator  beta
`,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			res, err := s.list.Run(context.Background())
			require.NoError(t, err)

			buf := &bytes.Buffer{}
			err = res.WriteColumns(buf)
			require.NoError(t, err)

			require.Equal(t, s.expected, buf.String())
		})
	}
}

func TestListChannels(t *testing.T) {
	type spec struct {
		name     string
		list     ListChannels
		expected string
	}
	specs := []spec{
		{
			name: "Success/WithPackage",
			list: ListChannels{IndexReference: "testdata/foo-index-v0.2.0-declcfg", PackageName: "foo"},
			expected: `PACKAGE  CHANNEL  HEAD
foo      beta     foo.v0.2.0
foo      stable   foo.v0.2.0
`,
		},
		{
			name: "Success/WithoutPackage",
			list: ListChannels{IndexReference: "testdata/foo-index-v0.2.0-declcfg"},
			expected: `PACKAGE  CHANNEL  HEAD
foo      beta     foo.v0.2.0
foo      stable   foo.v0.2.0
`,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			res, err := s.list.Run(context.Background())
			require.NoError(t, err)

			buf := &bytes.Buffer{}
			err = res.WriteColumns(buf)
			require.NoError(t, err)

			require.Equal(t, s.expected, buf.String())
		})
	}
}

func TestListBundles(t *testing.T) {
	type spec struct {
		name     string
		list     ListBundles
		expected string
	}
	specs := []spec{
		{
			name: "Success/WithPackage",
			list: ListBundles{IndexReference: "testdata/foo-index-v0.2.0-declcfg", PackageName: "foo"},
			expected: `PACKAGE  CHANNEL  BUNDLE      REPLACES    SKIPS                  SKIP RANGE  IMAGE
foo      beta     foo.v0.1.0                                     <0.1.0      test.registry/foo-operator/foo-bundle:v0.1.0
foo      beta     foo.v0.2.0  foo.v0.1.0  foo.v0.1.1,foo.v0.1.2  <0.2.0      test.registry/foo-operator/foo-bundle:v0.2.0
foo      stable   foo.v0.2.0  foo.v0.1.0  foo.v0.1.1,foo.v0.1.2  <0.2.0      test.registry/foo-operator/foo-bundle:v0.2.0
`,
		},
		{
			name: "Success/WithoutPackage",
			list: ListBundles{IndexReference: "testdata/foo-index-v0.2.0-declcfg"},
			expected: `PACKAGE  CHANNEL  BUNDLE      REPLACES    SKIPS                  SKIP RANGE  IMAGE
foo      beta     foo.v0.1.0                                     <0.1.0      test.registry/foo-operator/foo-bundle:v0.1.0
foo      beta     foo.v0.2.0  foo.v0.1.0  foo.v0.1.1,foo.v0.1.2  <0.2.0      test.registry/foo-operator/foo-bundle:v0.2.0
foo      stable   foo.v0.2.0  foo.v0.1.0  foo.v0.1.1,foo.v0.1.2  <0.2.0      test.registry/foo-operator/foo-bundle:v0.2.0
`,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			res, err := s.list.Run(context.Background())
			require.NoError(t, err)

			buf := &bytes.Buffer{}
			err = res.WriteColumns(buf)
			require.NoError(t, err)

			require.Equal(t, s.expected, buf.String())
		})
	}
}
