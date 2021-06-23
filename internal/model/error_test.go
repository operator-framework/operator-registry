package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidationError_Error(t *testing.T) {
	type spec struct {
		name   string
		err    *validationError
		expect string
	}

	recursiveErr := &validationError{
		message: "l1",
	}
	recursiveErr.subErrors = []error{
		fmt.Errorf("err1"),
		&validationError{
			message: "l2",
			subErrors: []error{
				fmt.Errorf("err3"),
				recursiveErr,
				fmt.Errorf("err4"),
			},
		},
		fmt.Errorf("err2"),
	}

	specs := []spec{
		{
			name:   "Nil",
			err:    nil,
			expect: "",
		},
		{
			name:   "Empty",
			err:    &validationError{},
			expect: "",
		},
		{
			name: "RecursiveError",
			err:  recursiveErr,
			expect: `l1:
├── err1
├── l2:
│   ├── err3
│   └── err4
└── err2`,
		},
		{
			name:   "MessageOnly",
			err:    &validationError{message: "hello"},
			expect: "hello",
		},
		{
			name: "WithSubErrors",
			err: &validationError{
				message: "hello",
				subErrors: []error{
					fmt.Errorf("world"),
					fmt.Errorf("foobar"),
				}},
			expect: `hello:
├── world
└── foobar`,
		},
		{
			name: "WithEmptyLeafSubErrors",
			err: &validationError{
				message: "hello",
				subErrors: []error{
					&validationError{
						message:   "foo",
						subErrors: []error{},
					},
					&validationError{
						message: "bar",
						subErrors: []error{
							fmt.Errorf("bar1"),
							fmt.Errorf("bar2"),
						},
					},
				}},
			expect: `hello:
├── foo
└── bar:
    ├── bar1
    └── bar2`,
		},
		{
			name: "WithSubSubErrors",
			err: &validationError{
				message: "hello",
				subErrors: []error{
					&validationError{
						message: "foo",
						subErrors: []error{
							fmt.Errorf("foo1"),
							fmt.Errorf("foo2"),
						},
					},
					&validationError{
						message: "bar",
						subErrors: []error{
							fmt.Errorf("bar1"),
							fmt.Errorf("bar2"),
						},
					},
				}},
			expect: `hello:
├── foo:
│   ├── foo1
│   └── foo2
└── bar:
    ├── bar1
    └── bar2`,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			require.Equal(t, s.expect, s.err.Error())
		})
	}
}
