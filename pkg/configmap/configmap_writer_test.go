package configmap

import (
	"testing"
)

func TestTranslateInvalidChars(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name           string
		input          string
		expectedOutput string
	}{
		{
			name:           "identity when there're no invalid characters",
			input:          "foo-bar.yaml",
			expectedOutput: "foo-bar.yaml",
		},
		{
			name:           "input having invalid character",
			input:          "foo:bar.yaml",
			expectedOutput: "foo-bar.yaml",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := TranslateInvalidChars(tc.input)
			if tc.expectedOutput != got {
				t.Errorf("expected %s, got %s", tc.expectedOutput, got)
			}

			if unallowedKeyChars.MatchString(got) {
				t.Errorf("translated output %q contains invalid characters", got)
			}
		})
	}
}
