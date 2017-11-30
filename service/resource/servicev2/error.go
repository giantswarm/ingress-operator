package servicev2

import (
	"github.com/giantswarm/microerror"
)

var invalidConfigError = microerror.New("invalid config")

// IsInvalidConfig asserts invalidConfigError.
func IsInvalidConfig(err error) bool {
	return microerror.Cause(err) == invalidConfigError
}

var servicePortNotFoundError = microerror.New("service port not found")

// IsServicePortNotFound asserts servicePortNotFoundError.
func IsServicePortNotFound(err error) bool {
	return microerror.Cause(err) == servicePortNotFoundError
}

var wrongTypeError = microerror.New("wrong type")

// IsWrongType asserts wrongTypeError.
func IsWrongType(err error) bool {
	return microerror.Cause(err) == wrongTypeError
}
