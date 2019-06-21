package appregistry

import (
	"fmt"
	"strings"
)

type Package struct {
	Name    string
	Release string
}

func (p *Package) String() string {
	return fmt.Sprintf("%s/%s", p.Name, p.Release)
}

// packageFromString will generate a Package from a string
// The in parameter is expected to match the following format:
// <package name>:<release>
func packageFromString(in string) *Package {
	split := strings.Split(in, ":")
	release := ""

	if len(split) == 2 {
		release = split[1]
	}
	return &Package{Name: split[0], Release: release}
}
