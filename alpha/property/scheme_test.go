package property

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddToScheme(t *testing.T) {
	type custom struct {
		Name string `json:"name"`
	}

	type spec struct {
		name      string
		typ       string
		val       interface{}
		assertion func(assert.TestingT, assert.PanicTestFunc, ...interface{}) bool
	}
	specs := []spec{
		{
			name:      "Success/CustomTypeValue",
			typ:       "custom1",
			val:       &custom{},
			assertion: assert.NotPanics,
		},
		{
			name:      "Panic/MustBeAPointer",
			typ:       TypePackage,
			val:       custom{},
			assertion: assert.Panics,
		},
		{
			name:      "Panic/AlreadyRegistered",
			typ:       TypePackage,
			val:       &custom{},
			assertion: assert.Panics,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			f := func() { AddToScheme(s.typ, s.val) }
			s.assertion(t, f)
		})
	}
}
