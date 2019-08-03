package appregistry

import (
	"github.com/operator-framework/operator-registry/pkg/registry"
)

const AppRegistryLoadErrorType registry.LoadErrorType = "appregistry"

func newAppRegistryLoadError(err error) *registry.LoadError {
	return registry.NewLoadError(AppRegistryLoadErrorType, err)
}
