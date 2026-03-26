package declcfg

import (
	"errors"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"

	"github.com/operator-framework/operator-registry/alpha/property"
)

const (
	versionReleaseMaxLength = 20
)

type Release []semver.PRVersion

// String returns the string representation of the release version.
func (r Release) String() string {
	if len(r) == 0 {
		return ""
	}
	pres := make([]string, 0, len(r))
	for _, pre := range r {
		pres = append(pres, pre.String())
	}
	return strings.Join(pres, ".")
}

func (r Release) Compare(other Release) int {
	if len(r) == 0 && len(other) > 0 {
		return -1
	}
	if len(other) == 0 && len(r) > 0 {
		return 1
	}
	a := semver.Version{Pre: r}
	b := semver.Version{Pre: other}
	return a.Compare(b)
}

func NewRelease(relStr string) (Release, error) {
	// empty input is not an error, but results in an empty release slice
	if relStr == "" {
		return nil, nil
	}

	// Validate against CRD constraint from operators.coreos.com/v1alpha1 ClusterServiceVersion
	if len(relStr) > versionReleaseMaxLength {
		return nil, fmt.Errorf("invalid release %q: exceeds maximum length of %d characters", relStr, versionReleaseMaxLength)
	}

	var (
		segments = strings.Split(relStr, ".")
		r        = make(Release, 0, len(segments))
		errs     []error
	)
	for i, segment := range segments {
		// semver.NewPRVersion validates:
		// - Pattern: alphanumerics and hyphens only
		// - No leading zeros in numeric identifiers
		prVer, err := semver.NewPRVersion(segment)
		if err != nil {
			errs = append(errs, fmt.Errorf("segment %d: %v", i, err))
			continue
		}
		r = append(r, prVer)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("invalid release %q: %v", relStr, err)
	}
	return r, nil
}

// MustBuildRelease builds a Release from a dot-separated string, panicking on error.
func MustBuildRelease(relStr string) Release {
	r, err := NewRelease(relStr)
	if err != nil {
		panic(fmt.Sprintf("MustBuildRelease(%q): %v", relStr, err))
	}
	return r
}

type VersionRelease struct {
	Version semver.Version `json:"version"`
	Release Release        `json:"release,omitempty"`
}

// String returns the string representation of the version release.
func (vr *VersionRelease) String() string {
	if len(vr.Release) > 0 {
		return vr.Version.String() + "-" + vr.Release.String()
	}
	return vr.Version.String()
}

func (vr *VersionRelease) Compare(other *VersionRelease) int {
	if cmp := vr.Version.Compare(other.Version); cmp != 0 {
		return cmp
	}
	return vr.Release.Compare(other.Release)
}

// MustBuildVersionRelease builds a VersionRelease from version and optional release strings, panicking on error.
// This is intended for use in tests.
func MustBuildVersionRelease(version string, release ...string) *VersionRelease {
	v, err := semver.Parse(version)
	if err != nil {
		panic(fmt.Sprintf("MustBuildVersionRelease: invalid version %q: %v", version, err))
	}

	vr := &VersionRelease{Version: v}

	if len(release) > 0 && release[0] != "" {
		r, err := NewRelease(release[0])
		if err != nil {
			panic(fmt.Sprintf("MustBuildVersionRelease: invalid release %q: %v", release[0], err))
		}
		vr.Release = r
	}

	return vr
}

// order by version, then
// release, if present
func (b *Bundle) Compare(other *Bundle) int {
	if b.Name == other.Name {
		return 0
	}
	avr, err := b.VersionRelease()
	if err != nil {
		return 0
	}
	otherVr, err := other.VersionRelease()
	if err != nil {
		return 0
	}
	return avr.Compare(otherVr)
}

func (b *Bundle) VersionRelease() (*VersionRelease, error) {
	props, err := property.Parse(b.Properties)
	if err != nil {
		return nil, fmt.Errorf("parse properties for bundle %q: %v", b.Name, err)
	}
	if len(props.Packages) != 1 {
		return nil, fmt.Errorf("bundle %q must have exactly 1 \"olm.package\" property, found %v", b.Name, len(props.Packages))
	}
	v, err := semver.Parse(props.Packages[0].Version)
	if err != nil {
		return nil, fmt.Errorf("bundle %q has invalid version %q: %v", b.Name, props.Packages[0].Version, err)
	}

	var r Release
	if props.Packages[0].Release != "" {
		r, err = NewRelease(props.Packages[0].Release)
		if err != nil {
			return nil, fmt.Errorf("error parsing bundle %q release %q: %v", b.Name, props.Packages[0].Release, err)
		}
	}

	return &VersionRelease{
		Version: v,
		Release: r,
	}, nil
}
