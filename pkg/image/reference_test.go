package image_test

import (
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/operator-framework/operator-registry/pkg/image"
	"gotest.tools/assert"
)

func TestSimpleReference(t *testing.T) {
	tests := []struct {
		ref       string
		tag       string
		digest    string
		expectref string
	}{
		{
			ref:       "a.b:1234/c/repo",
			tag:       "new",
			digest:    "sha256:494414ded24da13c451b13b424928821351c78fce49f93d9e1b55f102790c206",
			expectref: "a.b:1234/c/repo:new@sha256:494414ded24da13c451b13b424928821351c78fce49f93d9e1b55f102790c206",
		},
		{
			ref:       "a.b/c/repo:old",
			tag:       "new",
			expectref: "a.b/c/repo:new",
		},
		{
			ref:       "a.b/c/repo@sha256:5555555555555555555555555555555555555555555555555555555555555555",
			digest:    "sha256:494414ded24da13c451b13b424928821351c78fce49f93d9e1b55f102790c206",
			expectref: "a.b/c/repo@sha256:494414ded24da13c451b13b424928821351c78fce49f93d9e1b55f102790c206",
		},
		{
			ref:       "a.b:1234/c/repo:old@sha256:5555555555555555555555555555555555555555555555555555555555555555",
			tag:       "new",
			digest:    "sha256:494414ded24da13c451b13b424928821351c78fce49f93d9e1b55f102790c206",
			expectref: "a.b:1234/c/repo:new@sha256:494414ded24da13c451b13b424928821351c78fce49f93d9e1b55f102790c206",
		},
	}

	for _, tt := range tests {
		ref := image.SimpleReference(tt.ref)
		if len(tt.tag) != 0 {
			ref.WithTag(tt.tag)
		}
		if len(tt.digest) != 0 {
			dgst, _ := digest.Parse(tt.digest)
			ref.WithDigest(dgst)
		}
		assert.Equal(t, tt.expectref, ref.String())
	}
}

func TestTaggedReference(t *testing.T) {
	tests := []struct {
		ref       string
		tag       string
		digest    string
		expectref string
	}{
		{
			ref:       "a.b:1234/c/repo",
			tag:       "new",
			digest:    "sha256:494414ded24da13c451b13b424928821351c78fce49f93d9e1b55f102790c206",
			expectref: "a.b:1234/c/repo:new@sha256:494414ded24da13c451b13b424928821351c78fce49f93d9e1b55f102790c206",
		},
		{
			ref:       "a.b/c/repo:old",
			tag:       "new",
			expectref: "a.b/c/repo:new",
		},
		{
			ref:       "a.b/c/repo@sha256:5555555555555555555555555555555555555555555555555555555555555555",
			digest:    "sha256:494414ded24da13c451b13b424928821351c78fce49f93d9e1b55f102790c206",
			expectref: "a.b/c/repo@sha256:494414ded24da13c451b13b424928821351c78fce49f93d9e1b55f102790c206",
		},
		{
			ref:       "a.b:1234/c/repo:old@sha256:5555555555555555555555555555555555555555555555555555555555555555",
			tag:       "new",
			digest:    "sha256:494414ded24da13c451b13b424928821351c78fce49f93d9e1b55f102790c206",
			expectref: "a.b:1234/c/repo:new@sha256:494414ded24da13c451b13b424928821351c78fce49f93d9e1b55f102790c206",
		},
	}

	for _, tt := range tests {
		ref, err := image.ParseReference(tt.ref)
		assert.NilError(t, err)
		if len(tt.tag) != 0 {
			ref.WithTag(tt.tag)
		}
		if len(tt.digest) != 0 {
			dgst, _ := digest.Parse(tt.digest)
			ref.WithDigest(dgst)
		}
		assert.Equal(t, tt.expectref, ref.String())
	}
}
