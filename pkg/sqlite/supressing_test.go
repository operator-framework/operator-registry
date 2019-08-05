package sqlite

import (
	"testing"

	"github.com/pkg/errors"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

func TestErrorSupressingSQLLoader(t *testing.T) {
	suite := loaderTestSuite{
		{
			description: "EmptySQLLoader",
			loader: func() commonLoader {
				errCache := registry.NewErrorCache()
				return &ErrorSupressingSQLLoader{
					errs:   errCache,
					loader: newEmptySQLLoader(errCache.AddLoadError),
				}

			}(),
			addOperatorBundlesCases: []addOperatorBundleTestCase{
				{
					bundle:      registry.Bundle{},
					expectedErr: nil,
				},
			},
			addPackageChannelsCases: []addPackageChannelsTestCase{
				{
					manifest:    registry.PackageManifest{},
					expectedErr: nil,
				},
			},
			expectedLoadErrs: []registry.LoadError{
				*newSQLLoadError(errors.New("empty loader: cannot add operator bundle")),
				*newSQLLoadError(errors.New("empty loader: cannot add package channels")),
			},
		},
		{
			description: "RecoversFromPanic",
			loader: func() commonLoader {
				errCache := registry.NewErrorCache()
				return &ErrorSupressingSQLLoader{
					errs:   errCache,
					loader: nil,
				}

			}(),
			addOperatorBundlesCases: []addOperatorBundleTestCase{
				{
					bundle:      registry.Bundle{},
					expectedErr: nil,
				},
			},
			addPackageChannelsCases: []addPackageChannelsTestCase{
				{
					manifest:    registry.PackageManifest{},
					expectedErr: nil,
				},
			},
			expectedLoadErrs: []registry.LoadError{
				*registry.NewGenericLoadError(errors.New("runtime error: invalid memory address or nil pointer dereference")),
				*registry.NewGenericLoadError(errors.New("runtime error: invalid memory address or nil pointer dereference")),
			},
		},
	}

	suite.run(t)
}
