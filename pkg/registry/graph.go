package registry

// see https://github.com/operator-framework/enhancements/blob/master/enhancements/implicit-catalog-versioning.md#tooling

type Package struct {
	Name           string
	DefaultChannel string
	Channels       []Channel
}

type Channel struct {
	Name           string
	OperatorBundle []OperatorBundle
	Head           string // csv name of head of channel
}

type OperatorBundle struct {
	Version    string // semver string
	Name       string // csv name of bundle
	BundlePath string
	Replaces   []Replace
	//Replacements not implemented
}

type Replace struct {
	Version string //semver string
	Name    string //csv name
}
