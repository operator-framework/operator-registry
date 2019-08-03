package sqlite

import (
	"github.com/pkg/errors"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

type emptySQLLoader struct {
	*registry.EmptyLoad

	addLoadError func(*registry.LoadError)
}

func (e *emptySQLLoader) AddLoadError(err *registry.LoadError) {
	if e.addLoadError != nil {
		e.addLoadError(err)
	}
}

func (e *emptySQLLoader) LoadErrors() []registry.LoadError {
	return nil
}

func (e *emptySQLLoader) LoadErrorsByType(errType registry.LoadErrorType) []registry.LoadError {
	return nil
}

func (emptySQLLoader) Close() error {
	return errors.New("empty loader: cannot close")
}

func newEmptySQLLoader(addLoadError func(*registry.LoadError)) *emptySQLLoader {
	return &emptySQLLoader{
		EmptyLoad: registry.NewEmptyLoad(),

		addLoadError: addLoadError,
	}
}

type commonLoader interface {
	AddOperatorBundle(bundle *registry.Bundle) error
	AddPackageChannels(manifest registry.PackageManifest) error
	AddLoadError(err *registry.LoadError)
	LoadErrors() []registry.LoadError
	LoadErrorsByType(errType registry.LoadErrorType) []registry.LoadError
	Close() error
}

type ErrorSupressingSQLLoader struct {
	errs   *registry.ErrorCache
	loader commonLoader
}

var _ registry.Load = &ErrorSupressingSQLLoader{}

func NewErrorSupressingSQLLoader(outFilename string) *ErrorSupressingSQLLoader {
	errCache := registry.NewErrorCache()
	supLoader := &ErrorSupressingSQLLoader{
		errs:   errCache,
		loader: newEmptySQLLoader(errCache.AddLoadError),
	}

	sqlLoader, err := NewSQLLiteLoader(outFilename)
	if sqlLoader != nil {
		supLoader.loader = sqlLoader
	}

	if err != nil {
		supLoader.AddLoadError(newSQLLoadError(err))
	}

	return supLoader
}

func (e *ErrorSupressingSQLLoader) AddOperatorBundle(bundle *registry.Bundle) error {
	defer func() {
		if r := recover(); r != nil {
			e.errs.AddLoadError(registry.NewGenericLoadError(errors.Errorf("%v", r)))
		}
	}()

	e.loader.AddLoadError(newSQLLoadError(e.loader.AddOperatorBundle(bundle)))

	return nil
}

func (e *ErrorSupressingSQLLoader) AddPackageChannels(manifest registry.PackageManifest) error {
	defer func() {
		if r := recover(); r != nil {
			e.errs.AddLoadError(registry.NewGenericLoadError(errors.Errorf("%v", r)))
		}
	}()

	e.loader.AddLoadError(newSQLLoadError(e.loader.AddPackageChannels(manifest)))

	return nil
}

func (e *ErrorSupressingSQLLoader) Close() error {
	defer func() {
		if r := recover(); r != nil {
			e.errs.AddLoadError(registry.NewGenericLoadError(errors.Errorf("%v", r)))
		}
	}()

	e.loader.AddLoadError(newSQLLoadError(e.loader.Close()))

	return nil
}

func (e *ErrorSupressingSQLLoader) LoadErrors() []registry.LoadError {
	defer func() {
		if r := recover(); r != nil {
			e.errs.AddLoadError(registry.NewGenericLoadError(errors.Errorf("%v", r)))
		}
	}()

	// Return the union of both load errors.
	var errs []registry.LoadError
	if e.loader != nil {
		errs = append(errs, e.loader.LoadErrors()...)
	}
	errs = append(errs, e.errs.LoadErrors()...)

	return errs
}

func (e *ErrorSupressingSQLLoader) LoadErrorsByType(errType registry.LoadErrorType) []registry.LoadError {
	defer func() {
		if r := recover(); r != nil {
			e.errs.AddLoadError(registry.NewGenericLoadError(errors.Errorf("%v", r)))
		}
	}()

	// Return the union of both load errors.
	var errs []registry.LoadError
	if e.loader != nil {
		errs = append(errs, e.loader.LoadErrorsByType(errType)...)
	}
	errs = append(errs, e.errs.LoadErrorsByType(errType)...)

	return errs
}

func (e *ErrorSupressingSQLLoader) AddLoadError(err *registry.LoadError) {
	defer func() {
		if r := recover(); r != nil {
			// Panic can occur due to programmer error here.
			// Make an attempt to store as a generic error in the top-level error cache.
			e.errs.AddLoadError(registry.NewGenericLoadError(errors.Errorf("%v", r)))
		}
	}()

	e.loader.AddLoadError(err)
}
