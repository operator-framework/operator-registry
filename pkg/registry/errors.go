package registry

import (
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// LoadErrorType represents a type of load error.
// Primarily used as a key for storing related errors.
type LoadErrorType string

// LoadError associates an error with a LoadErrorType.
type LoadError struct {
	Type LoadErrorType
	Err  error
}

func (l LoadError) Error() string {
	if l.Err != nil {
		return l.Err.Error()
	}
	return ""
}

// NewLoadError returns a new LoadError initialized with the given arguments.
func NewLoadError(errType LoadErrorType, err error) *LoadError {
	return &LoadError{
		Type: errType,
		Err:  err,
	}
}

const GenericLoadErrorType LoadErrorType = "generic"

func NewGenericLoadError(err error) *LoadError {
	return NewLoadError(GenericLoadErrorType, err)
}

const SemanticLoadErrorType LoadErrorType = "semantic"

func NewSemanticLoadError(err error) *LoadError {
	return NewLoadError(SemanticLoadErrorType, err)
}

type ErrorCache struct {
	size int
	errs map[LoadErrorType][]LoadError
}

var _ LoadErrors = &ErrorCache{}

func (e *ErrorCache) LoadErrors() []LoadError {
	var errList []LoadError
	for _, errs := range e.errs {
		errList = append(errList, errs...)
	}

	return errList
}

func (e *ErrorCache) LoadErrorsByType(errType LoadErrorType) []LoadError {
	if e.errs == nil {
		return nil
	}

	return e.errs[errType]
}

func (e *ErrorCache) AddLoadError(err *LoadError) {
	if err == nil {
		return
	}
	if err.Err == nil {
		return
	}
	if e.errs == nil {
		e.errs = map[LoadErrorType][]LoadError{}
	}

	e.errs[err.Type] = append(e.errs[err.Type], *err)
	e.size++
}

func NewErrorCache() *ErrorCache {
	return &ErrorCache{}
}

func NewAggregate(l LoadErrors) error {
	var errs []error
	for _, loadErr := range l.LoadErrors() {
		errs = append(errs, loadErr)
	}
	return utilerrors.NewAggregate(errs)
}
