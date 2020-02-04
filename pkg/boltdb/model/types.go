package model

const OperatorsNamespace = "io.operators."

type OperatorBundle struct {
	Name         string `storm:"id"`
	Version      string
	Replaces     string
	SkipRange    string
	Skips        []string
	CSV          []byte
	Bundle       []byte
	BundlePath   string
	Capabilities []Capability
	Requirements []Requirement
}

type Capability struct {
	Name  string `storm:"id"`
	Value interface{}
}

type Requirement struct {
	Optional bool
	Name     string
	Selector interface{}
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
	BundleName string
	Replaces   string
}

type ChannelEntry struct {
	ID                 int `storm:"id,increment"`
	ChannelReplacement `storm:"unique,inline"`
}

type ImageUser struct {
	OperatorBundleName string
	Image              string
}

type RelatedImage struct {
	ID        int `storm:"id,increment"`
	ImageUser `storm:"unique,inline"`
}
