package service

import (
	"github.com/giantswarm/microerror"
)

var invalidConfigError = &microerror.Error{
	Kind: "invalidConfigError",
}

// IsInvalidConfig asserts invalidConfigError.
func IsInvalidConfig(err error) bool {
	return microerror.Cause(err) == invalidConfigError
}

var servicePortNotFoundError = &microerror.Error{
	Kind: "servicePortNotFoundError",
}

// IsServicePortNotFound asserts servicePortNotFoundError.
func IsServicePortNotFound(err error) bool {
	return microerror.Cause(err) == servicePortNotFoundError
}

var wrongTypeError = &microerror.Error{
	Kind: "wrongTypeError",
}

// IsWrongType asserts wrongTypeError.
func IsWrongType(err error) bool {
	return microerror.Cause(err) == wrongTypeError
}
