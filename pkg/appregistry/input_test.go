package appregistry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testInput struct {
	csvSources       string
	expectedPackages []*Package
}

var testGoodInput = []testInput{
	{
		"Jaeger,Jaeger",
		[]*Package{
			&Package{Name: "Jaeger", Release: ""},
		},
	},
	{
		"Jaeger,Kubevirt:10.0.0",
		[]*Package{
			&Package{Name: "Jaeger", Release: ""},
			&Package{Name: "Kubevirt", Release: "10.0.0"},
		},
	},
	{
		"Jaeger,Kubevirt:10.0.0,Kubevirt",
		[]*Package{
			&Package{Name: "Jaeger", Release: ""},
			&Package{Name: "Kubevirt", Release: "10.0.0"},
		},
	},
	{
		"Jaeger :2.0.0,  Kubevirt: 10.0.0, Kubevirt",
		[]*Package{
			&Package{Name: "Jaeger", Release: "2.0.0"},
			&Package{Name: "Kubevirt", Release: "10.0.0"},
		},
	},
	{
		"",
		[]*Package{},
	},
}

func TestGoodInput(t *testing.T) {
	parser := inputParser{sourceSpecifier: &registrySpecifier{}}

	for _, goodInput := range testGoodInput {
		actual, err := parser.Parse(
			[]string{"https://quay.io/cnr|community-operator|"},
			goodInput.csvSources,
		)
		assert.NoError(t, err)
		assert.Equal(t, goodInput.expectedPackages, actual.Packages)
	}
}

var testFaultyInput = []testInput{
	{
		"Jaeger,Kubevirt:10.0.0:11.0.0",
		nil,
	},
}

func TestFaultyInput(t *testing.T) {
	parser := inputParser{sourceSpecifier: &registrySpecifier{}}

	for _, mfInput := range testFaultyInput {
		_, err := parser.Parse(
			[]string{"https://quay.io/cnr|community-operator|"},
			mfInput.csvSources,
		)
		assert.Error(t, err)
	}
}
