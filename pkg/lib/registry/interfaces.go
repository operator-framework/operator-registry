//go:generate counterfeiter registry.go RegistryAdder
//go:generate counterfeiter registry.go RegistryDeleter
package registry

import (
	"github.com/sirupsen/logrus"
)

type RegistryAdder interface {
	AddToRegistry(AddToRegistryRequest) error
}

func NewRegistryAdder(logger *logrus.Entry) RegistryAdder {
	return RegistryUpdater{
		Logger: logger,
	}
}

type RegistryDeleter interface {
	DeleteFromRegistry(DeleteFromRegistryRequest) error
}

func NewRegistryDeleter(logger *logrus.Entry) RegistryDeleter {
	return RegistryUpdater{
		Logger: logger,
	}
}
