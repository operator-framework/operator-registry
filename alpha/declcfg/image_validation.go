package declcfg

import (
	"fmt"

	"go.podman.io/image/v5/docker/reference"
)

// validateImagePullSpec checks that a non-empty image pull spec is valid
// Empty pull specs are not validated.
func validateImagePullSpec(pullSpec, errFormat string, errArgs ...interface{}) error {
	if pullSpec == "" {
		return nil
	}
	if _, err := reference.ParseNormalizedNamed(pullSpec); err != nil {
		return fmt.Errorf(errFormat+": invalid image pull spec %q: %w", append(errArgs, pullSpec, err)...)
	}
	return nil
}
