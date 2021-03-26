package semver

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"
)

func TestBuildIdCompare(t *testing.T) {
	type args struct {
		b string
		v string
	}
	type expected struct {
		r int
	}
	tests := []struct {
		description string
		args        args
		expected    expected
	}{
		{
			description: "Equal",
			args: args{
				b: "1.0.0",
				v: "1.0.0",
			},
			expected: expected{
				r: 0,
			},
		},
		{
			description: "LessThan",
			args: args{
				b: "1.0.0",
				v: "1.0.1",
			},
			expected: expected{
				r: -1,
			},
		},
		{
			description: "GreaterThan",
			args: args{
				b: "1.0.1",
				v: "1.0.0",
			},
			expected: expected{
				r: 1,
			},
		},
		{
			description: "BuildIdLessThan",
			args: args{
				b: "1.0.0+1",
				v: "1.0.0+2",
			},
			expected: expected{
				r: -1,
			},
		},
		{
			description: "PatchBuildIdLessThan",
			args: args{
				b: "1.0.0+1",
				v: "1.0.1+2",
			},
			expected: expected{
				r: -1,
			},
		},
		{
			description: "MinorBuildIdLessThan",
			args: args{
				b: "1.0.0+1",
				v: "1.1.0+2",
			},
			expected: expected{
				r: -1,
			},
		},
		{
			description: "MajorBuildIdLessThan",
			args: args{
				b: "1.0.0+1",
				v: "2.0.0+2",
			},
			expected: expected{
				r: -1,
			},
		},
		{
			description: "PreReleaseBuildIdLessThan",
			args: args{
				b: "1.0.0+1",
				v: "1.0.1-1",
			},
			expected: expected{
				r: -1,
			},
		},
		{
			description: "BuildIdGreaterThan",
			args: args{
				b: "1.0.0+2",
				v: "1.0.0+1",
			},
			expected: expected{
				r: 1,
			},
		},
		{
			description: "BuildIdEqual",
			args: args{
				b: "1.0.0+1",
				v: "1.0.0+1",
			},
			expected: expected{
				r: 0,
			},
		},
		{
			description: "OneBuildIdLessThan",
			args: args{
				b: "1.0.0",
				v: "1.0.0+1",
			},
			expected: expected{
				r: -1,
			},
		},
		{
			description: "OneBuildIdGreaterThan",
			args: args{
				b: "1.0.0+1",
				v: "1.0.0",
			},
			expected: expected{
				r: 1,
			},
		},
		{
			description: "ZeroBuildIdGreaterThan",
			args: args{
				b: "1.0.0+0.123",
				v: "1.0.0",
			},
			expected: expected{
				r: 1,
			},
		},
		{
			description: "ZeroBuildIdGreaterThanPreRelease",
			args: args{
				b: "1.0.0-1+0.123",
				v: "1.0.0-1",
			},
			expected: expected{
				r: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			b, err := semver.Parse(tt.args.b)
			require.NoError(t, err)
			v, err := semver.Parse(tt.args.v)
			require.NoError(t, err)
			result, err := BuildIdCompare(b, v)
			require.NoError(t, err)
			require.Equal(t, tt.expected.r, result)
		})
	}
}
