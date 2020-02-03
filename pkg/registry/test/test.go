package test

import (
	"testing"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

type Setup func(*testing.T) (loader registry.Load, querier registry.Query, teardown func(*testing.T))

type registryTest func(*testing.T, registry.Load, registry.Query)

func curryRegistryTest(rt registryTest, setup Setup) func(*testing.T) {
	return func(t *testing.T) {
		loader, querier, teardown := setup(t)
		defer teardown(t)
		rt(t, loader, querier)
	}
}

var suites = map[string]func(*testing.T, Setup){
	"GeneralLoadSuite":   RunGeneralLoadSuite,
	"DirectoryLoadSuite": RunDirectoryLoadSuite,
	"ConfigMapLoadSuite": RunConfigMapLoadSuite,
	"ImageLoadSuite":     RunImageLoadSuite,
}

func RunLoadSuite(t *testing.T, setup Setup) {
	for description, suite := range suites {
		t.Run(description, func(t *testing.T) {
			suite(t, setup)
		})
	}
}
