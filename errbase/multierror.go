package errbase

import (
	goErr "errors"
)

// MultiError represents an error type with multiple causes introduced in go
// 1.20. This type is used to encode/decode such errors.
type MultiError struct {
	errs []error
}

func NewMultiError(errs []error) *MultiError {
	return &MultiError{
		errs: errs,
	}
}

func (e *MultiError) Error() string {
	// TODO(davidh): should we implement our own here?
	return goErr.Join(e.errs...).Error()
}

// GetErrors returns the inner array of error values. Not naming this `Unwrap()`
// because then the UnwrapOnce implementation will loop forever.
func (e *MultiError) GetErrors() []error {
	return e.errs
}
