package declcfg

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/operator-framework/operator-registry/alpha/model"
)

func TestConvertFromModel(t *testing.T) {
	type spec struct {
		name      string
		m         model.Model
		expectCfg DeclarativeConfig
	}

	specs := []spec{
		{
			name:      "Success",
			m:         buildTestModel(),
			expectCfg: buildValidDeclarativeConfig(false),
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			s.m.Normalize()
			assert.NoError(t, s.m.Validate())
			actual := ConvertFromModel(s.m)

			removeJSONWhitespace(&s.expectCfg)
			removeJSONWhitespace(&actual)

			assert.Equal(t, s.expectCfg, actual)
		})
	}
}
