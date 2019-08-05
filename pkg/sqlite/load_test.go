package sqlite

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

type addOperatorBundleTestCase struct {
	bundle      registry.Bundle
	expectedErr error
}

type addPackageChannelsTestCase struct {
	manifest    registry.PackageManifest
	expectedErr error
}

type loaderTestCase struct {
	description string
	loader      commonLoader

	// Test cases for individual operations
	addOperatorBundlesCases []addOperatorBundleTestCase
	addPackageChannelsCases []addPackageChannelsTestCase

	// Expected load errors for all operations
	expectedLoadErrs []registry.LoadError
}

func (e loaderTestCase) run(t *testing.T) {
	// Run inidividual test cases for each operation
	for _, tc := range e.addOperatorBundlesCases {
		require.Equal(t, tc.expectedErr, e.loader.AddOperatorBundle(&tc.bundle))
	}
	for _, tc := range e.addPackageChannelsCases {
		require.Equal(t, tc.expectedErr, e.loader.AddPackageChannels(tc.manifest))
	}

	// Check load errors accumulated by the loader from operation test cases
	loadErrs := e.loader.LoadErrors()
	require.Len(t, e.loader.LoadErrors(), len(e.expectedLoadErrs))

	unvisited := make(map[int]registry.LoadError, len(e.expectedLoadErrs))
	for i, expected := range e.expectedLoadErrs {
		unvisited[i] = expected
	}

	for _, actual := range loadErrs {
		for i, expected := range unvisited {
			if actual.Error() == expected.Error() {
				fmt.Printf("%s = %s\n", actual, expected)
				delete(unvisited, i)
			}
		}
	}

	require.Zero(t, len(unvisited), "failed to find all expected load errors")

}

type loaderTestSuite []loaderTestCase

func (l loaderTestSuite) run(t *testing.T) {
	for _, tt := range l {
		t.Run(tt.description, tt.run)
	}
}
