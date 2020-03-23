package registry

// see https://github.com/operator-framework/enhancements/blob/master/enhancements/implicit-catalog-versioning.md#tooling

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
	Version         string // semver string
	CsvName         string
	ReplacesBundles []OperatorBundle
	Replaces        []BundleRef
}

type BundleRef struct {
	BundlePath string
	Version    string //semver string
	CsvName    string //csv name
}
