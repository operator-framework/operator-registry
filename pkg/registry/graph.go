package registry

import (
	"github.com/blang/semver"
)

type Package struct {
	Name           string
	DefaultChannel string
	Channels       []Channel
}

type Channel struct {
	Name            string
	OperatorBundles []OperatorBundle
	Head            BundleRef
}

type OperatorBundle struct {
	BundlePath      string
	Version         semver.Version
	CsvName         string
	ReplacesBundles []OperatorBundle
	Replaces        []BundleRef
}

type BundleRef struct {
	BundlePath string
	Version    semver.Version
	CsvName    string
}

func (b *BundleRef) IsEmptyRef() bool {
	emptyVersion, _ := semver.Make("")
	if b.BundlePath == "" && b.Version.Equals(emptyVersion) && b.CsvName == "" {
		return true
	}
	return false
}
