package appregistry

import (
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

type Input struct {
	// Sources is the set of namespaced name(s) of OperatorSource objects from
	// which we need to pull packages.
	Sources []*types.NamespacedName

	// Packages is the set of package name(s) specified.
	Packages []string
}

func (i *Input) PackagesToMap() map[string]bool {
	packages := map[string]bool{}

	for _, pkg := range i.Packages {
		packages[pkg] = false
	}

	return packages
}

type inputParser struct {
}

// Parse parses the raw input provided, sanitizes it and returns an instance of
// Input.
//
// csvSources is a comma separated list of namespaced name that specifies
// the operator source(s), it is expected to comply to the following format -
// {namespace}/{name},{namespace}/{name},
//
// csvPackages is a comma separated list of packages. It is expected to have
// the following format.
// etcd,prometheus,descheduler
func (p *inputParser) Parse(csvSources string, csvPackages string) (*Input, error) {
	sources, err := parseSources(csvSources)
	if err != nil {
		return nil, err
	}

	packages := sanitizePackageList(strings.Split(csvPackages, ","))

	return &Input{
		Sources:  sources,
		Packages: packages,
	}, nil
}

func parseSources(csvSources string) ([]*types.NamespacedName, error) {
	values := strings.Split(csvSources, ",")
	if len(values) == 0 {
		return nil, errors.New(fmt.Sprintf("No OperatorSource(s) has been specified"))
	}

	names := make([]*types.NamespacedName, 0)
	for _, v := range values {
		name, err := split(v)
		if err != nil {
			return nil, err
		}

		names = append(names, name)
	}

	return names, nil
}

// sanitizePackageList sanitizes the set of package(s) specified. It removes
// duplicates and ignores empty string.
func sanitizePackageList(in []string) []string {
	out := make([]string, 0)

	inMap := map[string]bool{}
	for _, item := range in {
		if _, ok := inMap[item]; ok || item == "" {
			continue
		}

		out = append(out, item)
	}

	return out
}

func split(sourceName string) (*types.NamespacedName, error) {
	split := strings.Split(sourceName, "/")
	if len(split) != 2 {
		return nil, errors.New(fmt.Sprintf("OperatorSource name should be specified in this format {namespace}/{name}"))
	}

	return &types.NamespacedName{
		Namespace: split[0],
		Name:      split[1],
	}, nil
}
