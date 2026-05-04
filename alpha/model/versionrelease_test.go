package model

import (
	"encoding/json"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionRelease_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		vr       *VersionRelease
		expected string
	}{
		{
			name: "version only",
			vr: &VersionRelease{
				Version: semver.MustParse("1.2.3"),
				Release: nil,
			},
			expected: `{"version":"1.2.3","release":""}`,
		},
		{
			name: "version with release",
			vr: &VersionRelease{
				Version: semver.MustParse("1.2.3"),
				Release: Release{
					semver.PRVersion{VersionStr: "alpha"},
					semver.PRVersion{VersionNum: 1, IsNum: true},
				},
			},
			expected: `{"version":"1.2.3","release":"alpha.1"}`,
		},
		{
			name: "version with empty release",
			vr: &VersionRelease{
				Version: semver.MustParse("0.1.0"),
				Release: nil,
			},
			expected: `{"version":"0.1.0","release":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.vr)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))
		})
	}
}

func TestVersionRelease_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *VersionRelease
		wantErr  bool
	}{
		{
			name:  "version only",
			input: `{"version":"1.2.3"}`,
			expected: &VersionRelease{
				Version: semver.MustParse("1.2.3"),
				Release: nil,
			},
		},
		{
			name:  "version with release",
			input: `{"version":"1.2.3","release":"alpha.1"}`,
			expected: &VersionRelease{
				Version: semver.MustParse("1.2.3"),
				Release: Release{
					semver.PRVersion{VersionStr: "alpha"},
					semver.PRVersion{VersionNum: 1, IsNum: true},
				},
			},
		},
		{
			name:  "version with empty release",
			input: `{"version":"0.1.0","release":""}`,
			expected: &VersionRelease{
				Version: semver.MustParse("0.1.0"),
				Release: nil,
			},
		},
		{
			name:    "invalid json",
			input:   `{invalid}`,
			wantErr: true,
		},
		{
			name:    "invalid version",
			input:   `{"version":"not-a-version"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var vr VersionRelease
			err := json.Unmarshal([]byte(tt.input), &vr)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected.Version, vr.Version)
			assert.Equal(t, tt.expected.Release, vr.Release)
		})
	}
}

func TestVersionRelease_MarshalUnmarshalRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		vr   *VersionRelease
	}{
		{
			name: "version only",
			vr: &VersionRelease{
				Version: semver.MustParse("2.5.1"),
				Release: nil,
			},
		},
		{
			name: "version with release",
			vr: &VersionRelease{
				Version: semver.MustParse("1.0.0"),
				Release: Release{
					semver.PRVersion{VersionStr: "beta"},
					semver.PRVersion{VersionNum: 2, IsNum: true},
				},
			},
		},
		{
			name: "complex version with metadata",
			vr: &VersionRelease{
				Version: semver.Version{
					Major: 3,
					Minor: 4,
					Patch: 5,
					Pre: []semver.PRVersion{
						{VersionStr: "rc"},
						{VersionNum: 1, IsNum: true},
					},
					Build: []string{"build", "123"},
				},
				Release: Release{
					semver.PRVersion{VersionStr: "rel"},
					semver.PRVersion{VersionNum: 42, IsNum: true},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := json.Marshal(tt.vr)
			require.NoError(t, err)

			// Unmarshal
			var result VersionRelease
			err = json.Unmarshal(data, &result)
			require.NoError(t, err)

			// Compare
			assert.Equal(t, tt.vr.Version, result.Version)
			assert.Equal(t, tt.vr.Release, result.Release)
			assert.Equal(t, 0, tt.vr.Compare(&result), "round-tripped VersionRelease should compare equal")
		})
	}
}

func TestNewRelease(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Release
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
			wantErr:  false,
		},
		{
			name:  "single alphanumeric segment",
			input: "alpha",
			expected: Release{
				semver.PRVersion{VersionStr: "alpha"},
			},
		},
		{
			name:  "single numeric segment",
			input: "1",
			expected: Release{
				semver.PRVersion{VersionNum: 1, IsNum: true},
			},
		},
		{
			name:  "multiple segments",
			input: "alpha.1.beta.2",
			expected: Release{
				semver.PRVersion{VersionStr: "alpha"},
				semver.PRVersion{VersionNum: 1, IsNum: true},
				semver.PRVersion{VersionStr: "beta"},
				semver.PRVersion{VersionNum: 2, IsNum: true},
			},
		},
		{
			name:  "hyphens allowed",
			input: "rc-1.beta-2",
			expected: Release{
				semver.PRVersion{VersionStr: "rc-1"},
				semver.PRVersion{VersionStr: "beta-2"},
			},
		},
		{
			name:  "max length 20 characters",
			input: "12345678901234567890",
			expected: Release{
				semver.PRVersion{VersionNum: 12345678901234567890, IsNum: true},
			},
		},
		{
			name:    "exceeds max length",
			input:   "123456789012345678901",
			wantErr: true,
			errMsg:  "exceeds maximum length of 20 characters",
		},
		{
			name:    "leading zeros in numeric segment",
			input:   "01",
			wantErr: true,
			errMsg:  "Numeric PreRelease version must not contain leading zeroes",
		},
		{
			name:    "leading zeros in multiple digit numeric segment",
			input:   "001",
			wantErr: true,
			errMsg:  "Numeric PreRelease version must not contain leading zeroes",
		},
		{
			name:  "zero without leading zeros is valid",
			input: "0",
			expected: Release{
				semver.PRVersion{VersionNum: 0, IsNum: true},
			},
		},
		{
			name:  "alphanumeric starting with zero is valid",
			input: "0alpha",
			expected: Release{
				semver.PRVersion{VersionStr: "0alpha"},
			},
		},
		{
			name:    "invalid characters",
			input:   "alpha_beta",
			wantErr: true,
			errMsg:  "Invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewRelease(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
