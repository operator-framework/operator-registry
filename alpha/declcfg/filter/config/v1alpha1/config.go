package v1alpha1

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// FilterConfigurationV1 is a configuration for filtering a set of packages and channels from a catalog.
// It supports selecting specific packages and specific channels and/or versions within those packages.
// The configuration is intended to be used with the `opm render` command to generate a filtered catalog.
type FilterConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// Packages is a list of packages to include in the filtered catalog.
	Packages []Package `json:"packages"`
}

type Package struct {
	// Name is the name of the package to filter.
	Name string `json:"name"`

	// DefaultChannel is the new default channel to use for the package.
	// If not set, the default channel will be the same as the original default channel.
	// If the original default channel is not in the filtered catalog, this field must be set.
	DefaultChannel string `json:"defaultChannel,omitempty"`

	// Channels is a list of channels to include in the filtered catalog.
	// If not set, all channels will be included.
	Channels []Channel `json:"channels,omitempty"`
}

type Channel struct {
	// Name is the name of the channel to include in the filtered catalog.
	Name string `json:"name"`

	// VersionRange is a semver range to filter the versions of the channel.
	// If not set, all versions will be included.
	VersionRange string `json:"versionRange,omitempty"`
}

func LoadFilterConfiguration(data []byte) (*FilterConfiguration, error) {
	cfg := &FilterConfiguration{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (f *FilterConfiguration) Validate() error {
	var errs []error
	if f.APIVersion != "olm.operatorframework.io/v1alpha1" {
		errs = append(errs, fmt.Errorf("unexpected API version %q", f.APIVersion))
	}
	if f.Kind != "FilterConfiguration" {
		errs = append(errs, fmt.Errorf("unexpected kind %q", f.Kind))
	}
	if len(f.Packages) == 0 {
		errs = append(errs, errors.New("at least one package must be specified"))
	}
	for i, pkg := range f.Packages {
		if pkg.Name == "" {
			errs = append(errs, fmt.Errorf("package %q at index [%d] is invalid: name must be specified", pkg.Name, i))
		}
		for j, channel := range pkg.Channels {
			if channel.Name == "" {
				errs = append(errs, fmt.Errorf("package %q at index [%d] is invalid: channel %q at index [%d] is invalid: name must be specified", pkg.Name, i, channel.Name, j))
			}
		}
	}
	return errors.Join(errs...)
}
