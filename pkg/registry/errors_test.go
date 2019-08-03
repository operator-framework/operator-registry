package registry

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestLoadErrors(t *testing.T) {
	semantic := errors.New("semantic error")
	generic := errors.New("generic error")

	tests := []struct {
		description string
		errs        []*LoadError
		expected    []LoadError
	}{
		{
			description: "NoLoadErrors",
			errs:        nil,
			expected:    nil,
		},
		{
			description: "MultipleLoadErrors",
			errs: []*LoadError{
				NewSemanticLoadError(semantic),
				NewGenericLoadError(generic),
			},
			expected: []LoadError{
				*NewSemanticLoadError(semantic),
				*NewGenericLoadError(generic),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			errCache := NewErrorCache()
			for _, loadErr := range tt.errs {
				errCache.AddLoadError(loadErr)
			}

			require.ElementsMatch(t, tt.expected, errCache.LoadErrors())
		})
	}
}

func TestLoadErrorsByType(t *testing.T) {
	semantic := errors.New("semantic error")
	generic := errors.New("generic error")

	tests := []struct {
		description string
		errs        []*LoadError
		byType      LoadErrorType
		expected    []LoadError
	}{
		{
			description: "NoLoadErrors",
			errs:        nil,
			expected:    nil,
		},
		{
			description: "MultipleLoadErrors/ByCustom",
			errs: []*LoadError{
				NewSemanticLoadError(semantic),
				NewGenericLoadError(generic),
			},
			byType:   "custom",
			expected: nil,
		},
		{
			description: "MultipleLoadErrors/BySemantic",
			errs: []*LoadError{
				NewSemanticLoadError(semantic),
				NewGenericLoadError(generic),
			},
			byType: SemanticLoadErrorType,
			expected: []LoadError{
				*NewSemanticLoadError(semantic),
			},
		},
		{
			description: "MultipleLoadErrors/ByGeneric",
			errs: []*LoadError{
				NewSemanticLoadError(semantic),
				NewGenericLoadError(generic),
			},
			byType: GenericLoadErrorType,
			expected: []LoadError{
				*NewGenericLoadError(generic),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			errCache := NewErrorCache()
			for _, loadErr := range tt.errs {
				errCache.AddLoadError(loadErr)
			}

			require.ElementsMatch(t, tt.expected, errCache.LoadErrorsByType(tt.byType))
		})
	}
}
