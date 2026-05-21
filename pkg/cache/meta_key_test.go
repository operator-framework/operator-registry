package cache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateMetaKeyComponent(t *testing.T) {
	for _, tt := range []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid simple", "my-schema", false},
		{"valid dotted", "custom.operator.io", false},
		{"empty", "", true},
		{"dot-dot", "..", true},
		{"dot-dot prefix", "../foo", true},
		{"dot-dot suffix", "foo/..", true},
		{"forward slash", "foo/bar", true},
		{"backslash", "foo\\bar", true},
		{"dot-dot-prefix no slash", "..foo", true},
		{"dot-dot-suffix no slash", "foo..", false},
		{"single dot", ".", true},
		{"triple dot", "...", true},
		{"starts with dot", ".hidden", true},
		{"starts with dash", "-flag", true},
		{"starts with underscore", "_private", true},
		{"contains underscore", "my_schema", false},
		{"numeric", "123", false},
		{"contains plus", "v1+beta", true},
		{"contains colon", "custom:v1", true},
		{"contains space", "has space", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMetaKeyComponent("test", tt.value)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewValidatedMetaKey(t *testing.T) {
	for _, tt := range []struct {
		name        string
		schema      string
		packageName string
		wantErr     bool
		wantKey     metaKey
	}{
		{
			name:        "both populated",
			schema:      "custom.operator.io",
			packageName: "testpkg",
			wantKey:     metaKey{Schema: "custom.operator.io", PackageName: "testpkg"},
		},
		{
			name:        "empty packageName allowed",
			schema:      "custom.operator.io",
			packageName: "",
			wantKey:     metaKey{Schema: "custom.operator.io", PackageName: ""},
		},
		{
			name:    "empty schema rejected",
			schema:  "",
			wantErr: true,
		},
		{
			name:        "schema with slash rejected",
			schema:      "foo/bar",
			packageName: "pkg",
			wantErr:     true,
		},
		{
			name:        "packageName with slash rejected",
			schema:      "custom.io",
			packageName: "foo/bar",
			wantErr:     true,
		},
		{
			name:        "schema dot-dot rejected",
			schema:      "..",
			packageName: "pkg",
			wantErr:     true,
		},
		{
			name:        "packageName dot-dot rejected",
			schema:      "custom.io",
			packageName: "..",
			wantErr:     true,
		},
		{
			name:        "schema dot rejected",
			schema:      ".",
			packageName: "pkg",
			wantErr:     true,
		},
		{
			name:        "packageName dot rejected",
			schema:      "custom.io",
			packageName: ".",
			wantErr:     true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newValidatedMetaKey(tt.schema, tt.packageName)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantKey, got)
			}
		})
	}
}
