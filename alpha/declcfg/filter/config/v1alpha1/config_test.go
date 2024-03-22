package v1alpha1

import (
	"bytes"
	"embed"
	"errors"
	"io"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFilterConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		openFile  func() (io.Reader, error)
		assertion func(*testing.T, *FilterConfiguration, error)
	}{
		{
			name:     "ReadFailure",
			openFile: func() (io.Reader, error) { return iotest.ErrReader(errors.New("read failure")), nil },
			assertion: func(t *testing.T, cfg *FilterConfiguration, err error) {
				assert.Nil(t, cfg)
				require.Error(t, err)
				assert.ErrorContains(t, err, "read failure")
			},
		},
		{
			name:     "ParseFailure",
			openFile: func() (io.Reader, error) { return bytes.NewReader([]byte(`{`)), nil },
			assertion: func(t *testing.T, cfg *FilterConfiguration, err error) {
				assert.Nil(t, cfg)
				require.Error(t, err)
				assert.ErrorContains(t, err, "yaml: line 1: did not find expected node content")
			},
		},
		{
			name:     "WrongAPIVersion",
			openFile: func() (io.Reader, error) { return configsFS.Open("testdata/configs/invalid_apiversion.yaml") },
			assertion: func(t *testing.T, cfg *FilterConfiguration, err error) {
				assert.Nil(t, cfg)
				require.Error(t, err)
				assert.ErrorContains(t, err, "unexpected API version")
			},
		},
		{
			name:     "WrongKind",
			openFile: func() (io.Reader, error) { return configsFS.Open("testdata/configs/invalid_kind.yaml") },
			assertion: func(t *testing.T, cfg *FilterConfiguration, err error) {
				assert.Nil(t, cfg)
				require.Error(t, err)
				assert.ErrorContains(t, err, "unexpected kind")
			},
		},
		{
			name:     "NoPackages",
			openFile: func() (io.Reader, error) { return configsFS.Open("testdata/configs/invalid_nopackages.yaml") },
			assertion: func(t *testing.T, cfg *FilterConfiguration, err error) {
				assert.Nil(t, cfg)
				require.Error(t, err)
				assert.ErrorContains(t, err, "at least one package must be specified")
			},
		},
		{
			name:     "MissingPackageName",
			openFile: func() (io.Reader, error) { return configsFS.Open("testdata/configs/invalid_missingpackagename.yaml") },
			assertion: func(t *testing.T, cfg *FilterConfiguration, err error) {
				assert.Nil(t, cfg)
				require.Error(t, err)
				assert.ErrorContains(t, err, `package "" at index [1] is invalid: name must be specified`)
			},
		},
		{
			name:     "MissingChannelName",
			openFile: func() (io.Reader, error) { return configsFS.Open("testdata/configs/invalid_missingchannelname.yaml") },
			assertion: func(t *testing.T, cfg *FilterConfiguration, err error) {
				assert.Nil(t, cfg)
				require.Error(t, err)
				assert.ErrorContains(t, err, `package "bar" at index [1] is invalid: channel "" at index [1] is invalid: name must be specified`)
			},
		},
		{
			name:     "MultipleErrors",
			openFile: func() (io.Reader, error) { return configsFS.Open("testdata/configs/invalid_multipleerrors.yaml") },
			assertion: func(t *testing.T, cfg *FilterConfiguration, err error) {
				assert.Nil(t, cfg)
				require.Error(t, err)
				assert.ErrorContains(t, err, `unexpected API version`)
				assert.ErrorContains(t, err, "unexpected kind")
				assert.ErrorContains(t, err, `package "" at index [2] is invalid: name must be specified`)
				assert.ErrorContains(t, err, `package "" at index [2] is invalid: channel "" at index [0] is invalid: name must be specified`)
				assert.ErrorContains(t, err, `package "" at index [2] is invalid: channel "" at index [1] is invalid: name must be specified`)
				assert.ErrorContains(t, err, `package "" at index [3] is invalid: name must be specified`)
				assert.ErrorContains(t, err, `package "" at index [3] is invalid: channel "" at index [0] is invalid: name must be specified`)
				assert.ErrorContains(t, err, `package "" at index [3] is invalid: channel "" at index [1] is invalid: name must be specified`)
				assert.ErrorContains(t, err, `package "baz" at index [4] is invalid: channel "" at index [0] is invalid: name must be specified`)
			},
		},
		{
			name:     "Valid",
			openFile: func() (io.Reader, error) { return configsFS.Open("testdata/configs/valid.yaml") },
			assertion: func(t *testing.T, cfg *FilterConfiguration, err error) {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				assert.Equal(t, "olm.operatorframework.io/v1alpha1", cfg.APIVersion)
				assert.Equal(t, "FilterConfiguration", cfg.Kind)
				assert.Len(t, cfg.Packages, 2)
				assert.Equal(t, "foo", cfg.Packages[0].Name)
				assert.Len(t, cfg.Packages[0].Channels, 0)
				assert.Equal(t, "", cfg.Packages[0].DefaultChannel)
				assert.Equal(t, "bar", cfg.Packages[1].Name)
				assert.Len(t, cfg.Packages[1].Channels, 2)
				assert.Equal(t, "bar-channel1", cfg.Packages[1].DefaultChannel)
				assert.Equal(t, "bar-channel1", cfg.Packages[1].Channels[0].Name)
				assert.Equal(t, ">=1.0.0 <2.0.0", cfg.Packages[1].Channels[0].VersionRange)
				assert.Equal(t, "bar-channel2", cfg.Packages[1].Channels[1].Name)
				assert.Equal(t, ">=2.0.0 <3.0.0", cfg.Packages[1].Channels[1].VersionRange)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := tt.openFile()
			if err != nil {
				tt.assertion(t, nil, err)
				return
			}
			cfg, err := LoadFilterConfiguration(f)
			tt.assertion(t, cfg, err)
		})
	}
}

//go:embed testdata/configs/*
var configsFS embed.FS
