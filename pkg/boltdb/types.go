package boltdb

type OperatorBundle struct {
	Name       string `storm:"id"`
	Version    string
	Replaces   string
	SkipRange  string
	Skips      []string
	CSV        []byte `storm:"unique"`
	Bundle     []byte
	BundlePath string
}

type Package struct {
	Name           string `storm:"id"`
	DefaultChannel string
}

type PackageChannel struct {
	PackageName string
	ChannelName string
}

type Channel struct {
	ID                     int `storm:"id,increment"`
	PackageChannel         `storm:"unique,inline"`
	HeadOperatorBundleName string `storm:"index"`
}

type ChannelReplacement struct {
	PackageChannel
	OperatorBundleName string
	Replaces           string
}

type ChannelEntry struct {
	ID                 int `storm:"id,increment"`
	ChannelReplacement `storm:"unique,inline"`
}

type GVK struct {
	Group   string
	Version string
	Kind    string
}

type GVKUser struct {
	GVK
	OperatorBundleName string
}

type RelatedAPI struct {
	ID       int `storm:"id,increment"`
	GVKUser  `storm:"unique,inline"`
	Plural   string
	Provides bool
}

type ImageUser struct {
	OperatorBundleName string
	Image              string
}

type RelatedImage struct {
	ID        int `storm:"id,increment"`
	ImageUser `storm:"unique,inline"`
}

type PackageChannelGVK struct {
	PackageChannel
	GVK
}

type LatestGVKProvider struct {
	ID                 string `storm:"id,increment"`
	PackageChannelGVK  `storm:"unique,inline"`
	OperatorBundleName string `storm:"index"`
}
