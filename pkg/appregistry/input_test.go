package appregistry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDuplicatePackageResultHasNoRelease(t *testing.T) {
	parser := inputParser{sourceSpecifier: &registrySpecifier{}}
	expectedPackages := []*Package{&Package{Name: "Jaeger", Release: ""}}

	actual, err := parser.Parse([]string{"https://quay.io/cnr|community-operator|"}, "Jaeger,Jaeger,Jaeger:0.0.1,Jaeger:0.0.2")
	assert.NoError(t, err)
	assert.Equal(t, expectedPackages, actual.Packages)
}

func TestDuplicatePackageResultHasRelease(t *testing.T) {
	parser := inputParser{sourceSpecifier: &registrySpecifier{}}
	expectedPackages := []*Package{&Package{Name: "Jaeger", Release: "0.0.1"}}

	actual, err := parser.Parse([]string{"https://quay.io/cnr|community-operator|"}, "Jaeger:0.0.1,Jaeger,Jaeger,Jaeger:0.0.2")
	assert.NoError(t, err)
	assert.Equal(t, expectedPackages, actual.Packages)
}

func TestPackageListIncludesPackageWithAndWithoutRelease(t *testing.T) {
	parser := inputParser{sourceSpecifier: &registrySpecifier{}}
	expectedPackages := []*Package{&Package{Name: "Jaeger", Release: ""}, &Package{Name: "Syndesis", Release: "0.0.1"}}

	actual, err := parser.Parse([]string{"https://quay.io/cnr|community-operator|"}, "Jaeger,Syndesis:0.0.1")
	assert.NoError(t, err)
	assert.Equal(t, expectedPackages, actual.Packages)
}
