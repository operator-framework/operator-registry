package registry

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"
)

func TestBundleVersionCompare(t *testing.T) {
	type fields struct {
		version string
	}
	type args struct {
		version string
	}

	type order func(t *testing.T, val int)
	var (
		lt order = func(t *testing.T, val int) {
			require.Less(t, val, 0)
		}
		gt order = func(t *testing.T, val int) {
			require.Greater(t, val, 0)
		}
		eq order = func(t *testing.T, val int) {
			require.Equal(t, val, 0)
		}
	)
	type expect struct {
		order order
	}

	for _, tt := range []struct {
		description string
		fields      fields
		args        args
		expect      expect
	}{
		{
			description: "BuildMetaAscendingLT",
			fields: fields{
				version: "1.0.0+1",
			},
			args: args{
				version: "1.0.0+2",
			},
			expect: expect{
				order: lt,
			},
		},
		{
			description: "BuildMetaDescendingGT",
			fields: fields{
				version: "1.0.0+2",
			},
			args: args{
				version: "1.0.0+1",
			},
			expect: expect{
				order: gt,
			},
		},
		{
			description: "BuildMetaGT",
			fields: fields{
				version: "1.0.0+1",
			},
			args: args{
				version: "1.0.0",
			},
			expect: expect{
				order: gt,
			},
		},
		{
			description: "BuildMetaEQ",
			fields: fields{
				version: "1.0.0+1",
			},
			args: args{
				version: "1.0.0+1",
			},
			expect: expect{
				order: eq,
			},
		},
		{
			description: "BuildMetaZeroGT",
			fields: fields{
				version: "1.0.0+0",
			},
			args: args{
				version: "1.0.0",
			},
			expect: expect{
				order: gt,
			},
		},
		{
			description: "BuildMetaMultipartLT",
			fields: fields{
				version: "1.0.0+0",
			},
			args: args{
				version: "1.0.0+1.2.3",
			},
			expect: expect{
				order: lt,
			},
		},
		{
			description: "PatchAscendingLT",
			fields: fields{
				version: "1.0.0",
			},
			args: args{
				version: "1.0.1",
			},
			expect: expect{
				order: lt,
			},
		},
		{
			description: "PatchDescendingGT",
			fields: fields{
				version: "1.0.1",
			},
			args: args{
				version: "1.0.0",
			},
			expect: expect{
				order: gt,
			},
		},
		{
			description: "MinorAscendingLT",
			fields: fields{
				version: "1.0.0",
			},
			args: args{
				version: "1.1.0",
			},
			expect: expect{
				order: lt,
			},
		},
		{
			description: "MinorDescendingGT",
			fields: fields{
				version: "1.1.0",
			},
			args: args{
				version: "1.0.0",
			},
			expect: expect{
				order: gt,
			},
		},
		{
			description: "MajorAscendingLT",
			fields: fields{
				version: "1.0.0",
			},
			args: args{
				version: "2.0.0",
			},
			expect: expect{
				order: lt,
			},
		},
		{
			description: "MajorDescendingGT",
			fields: fields{
				version: "2.0.0",
			},
			args: args{
				version: "1.0.0",
			},
			expect: expect{
				order: gt,
			},
		},
		{
			description: "PrereleaseLT",
			fields: fields{
				version: "1.0.0-pre",
			},
			args: args{
				version: "1.0.0",
			},
			expect: expect{
				order: lt,
			},
		},
		{
			description: "PrereleaseGT",
			fields: fields{
				version: "2.0.0-pre",
			},
			args: args{
				version: "1.0.0",
			},
			expect: expect{
				order: gt,
			},
		},
	} {
		t.Run(tt.description, func(t *testing.T) {
			fieldVersion, err := semver.Parse(tt.fields.version)
			require.NoErrorf(t, err, "bad testcase")

			argVersion, err := semver.Parse(tt.args.version)
			require.NoErrorf(t, err, "bad testcase")

			c, err := bundleVersion{version: fieldVersion}.compare(bundleVersion{version: argVersion})
			require.NoError(t, err)

			tt.expect.order(t, c)
		})
	}
}
