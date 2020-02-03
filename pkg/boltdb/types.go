package boltdb

import (
	"fmt"
	"strings"
)

const (
	KubeApiNamespace = "io.operators."
	GvkCapability = KubeApiNamespace+"gvk"
)

type OperatorBundle struct {
	Name       string `storm:"id"`
	Version    string
	Replaces   string
	SkipRange  string
	Skips      []string
	CSV        []byte
	Bundle     []byte
	BundlePath string
	Capabilities []Capability
	Requirements []Requirement
}

type Api struct {
	Group string
	Version string
	Kind string
	Plural string
}

func (a Api) String() string {
	return fmt.Sprintf("%s/%s/%s/%s", a.Group, a.Version, a.Kind, a.Plural)
}

func ApiFromString(s string) (*Api, error) {
	split := strings.Split(s, "/")
	if len(split) < 4 {
		return nil, fmt.Errorf("invalid gvk encoding")
	}
	return &Api{
		Group:   split[0],
		Version: split[1],
		Kind:    split[2],
		Plural:  split[3],
	}, nil
}

type Capability struct {
	Name  string           `storm:"id"`
	Value string
}

type Requirement struct {
	Optional bool
	Name string
	Selector string
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

type ImageUser struct {
	OperatorBundleName string
	Image              string
}

type RelatedImage struct {
	ID        int `storm:"id,increment"`
	ImageUser `storm:"unique,inline"`
}
