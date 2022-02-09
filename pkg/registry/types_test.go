package registry

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/operator-framework/api/pkg/constraints"
	"github.com/stretchr/testify/assert"
)

func TestCelConstraintValidation(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		errs       []error
	}{
		{
			name:       "ValidCelConstraint",
			constraint: `{"cel":{"rule":"properties.exists(p, p.type == 'olm.test' && (semver_compare(p.value, '1.0.0') == 0))"}}`,
		},
		{
			name:       "InvalidCelConstraint/MissingRule",
			constraint: `{"cel":{"rule":""}}`,
			errs: []error{
				fmt.Errorf("The CEL expression is missing"),
			},
		},
		{
			name:       "InvalidCelConstraint/NotExistedFunc",
			constraint: `{"cel":{"rule":"properties.exists(p, p.type == 'olm.test' && (doesnt_exist(p.value, '1.0.0') == 0))"}}`,
			errs: []error{
				fmt.Errorf("Invalid CEL expression"),
			},
		},
		{
			name:       "InvalidCelExpression/NonBoolReturn",
			constraint: `{"cel":{"rule":"1"}}`,
			errs: []error{
				fmt.Errorf("Invalid CEL expression"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dep constraints.Constraint
			err := json.Unmarshal([]byte(tt.constraint), &dep)
			assert.NoError(t, err)
			errs := ValidateCEL(dep)
			if len(tt.errs) > 0 {
				assert.Error(t, errs[0])
				assert.Contains(t, errs[0].Error(), tt.errs[0].Error())
			} else {
				assert.Equal(t, len(errs), 0)
			}
		})
	}
}
