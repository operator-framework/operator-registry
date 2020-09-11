package image

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/opencontainers/go-digest"
)

// Reference describes a reference to a container image.
type Reference interface {
	fmt.Stringer
	Tag() string
	Digest() digest.Digest
	WithTag(string) error
	WithDigest(digest.Digest) error
}

// SimpleReference is a reference backed by a string with no additional validation.
type simpleReference string

func (s *simpleReference) String() string {
	ref := string(*s)
	return ref
}

// SimpleReference creates a new reference backed by a string.
func SimpleReference(rawRef string) *simpleReference {
	ref := simpleReference(rawRef)
	return &ref
}

// Tag returns the tag portion of the reference
func (s *simpleReference) Tag() string {
	ref := s.String()
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		ref = ref[:idx]
	}
	if idx := strings.LastIndex(ref, ":"); idx != -1 && isTag(ref[idx+1:]) {
		return ref[idx+1:]
	}
	return ""
}

// Digest returns the digest portion of the reference
func (s *simpleReference) Digest() digest.Digest {
	ref := s.String()
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		dgst, _ := digest.Parse(ref[idx+1:])
		return dgst
	}
	return ""
}

// WithTag replaces the tag for the reference with the provided one
func (s *simpleReference) WithTag(tag string) error {
	ref := s.String()
	if len(tag) > 0 {
		if !isTag(tag) {
			return fmt.Errorf("Invalid tag: %s", tag)
		}
		tag = ":" + tag
	}
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		// Preserve any existing digests
		tag += ref[idx:]
		ref = ref[:idx]
	}
	if idx := strings.LastIndex(ref, ":"); idx != -1 && isTag(ref[idx+1:]) {
		ref = ref[:idx]
	}
	*s = *SimpleReference(ref + tag)
	return nil
}

// WithDigest replaces the digest for the reference with the provided one
func (s *simpleReference) WithDigest(dgst digest.Digest) error {
	ref := s.String()
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		ref = ref[:idx]
	}
	dgstStr := dgst.String()
	if len(dgstStr) > 0 {
		dgstStr = "@" + dgstStr
	}
	*s = *SimpleReference(ref + dgstStr)
	return nil
}

// TaggedReference is a reference that supports a digest and tag.
type TaggedReference struct {
	tag    string
	ref    string
	digest digest.Digest
}

// String provides a string corresponding to the ref.
func (r *TaggedReference) String() string {
	ref := r.ref
	if len(r.tag) != 0 {
		ref += ":" + r.tag
	}
	if len(r.digest) != 0 {
		ref += "@" + r.digest.String()
	}
	return ref
}

// Tag returns the tag portion of the reference
func (r *TaggedReference) Tag() string {
	return r.tag
}

// Digest returns the digest portion of the reference
func (r TaggedReference) Digest() digest.Digest {
	return r.digest
}

// WithTag replaces the tag for the reference with the provided one
func (r *TaggedReference) WithTag(tag string) error {
	if len(tag) > 0 && !isTag(tag) {
		return fmt.Errorf("Invalid tag: %s", tag)
	}
	r.tag = tag
	return nil
}

// WithDigest replaces the digest for the reference with the provided one
func (r *TaggedReference) WithDigest(dgst digest.Digest) error {
	r.digest = dgst
	return nil
}

var isTag = regexp.MustCompile(`^[A-Za-z0-9_][-.A-Za-z0-9_]{0,127}$`).MatchString

// ParseReference converts a string into a TaggedReference
func ParseReference(ref string) (*TaggedReference, error) {
	tagref := TaggedReference{}
	if digestIndex := strings.LastIndex(ref, "@"); digestIndex != -1 {
		dgst, err := digest.Parse(ref[digestIndex+1:])
		if err != nil {
			return nil, err
		}
		tagref.digest = dgst
		ref = ref[:digestIndex]
	}
	if tagIndex := strings.LastIndex(ref, ":"); tagIndex != -1 {
		if isTag(ref[tagIndex+1:]) {
			tagref.tag = ref[tagIndex+1:]
			ref = ref[:tagIndex]
		}
	}
	// TODO: Validate domain name and path
	tagref.ref = ref
	return &tagref, nil
}
