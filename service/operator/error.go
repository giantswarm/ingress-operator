package operator

import (
	"github.com/juju/errgo"
)

var capacityReachedError = errgo.New("capacity reached")

// IsCapacityReached asserts capacityReachedError.
func IsCapacityReached(err error) bool {
	return errgo.Cause(err) == capacityReachedError
}

var invalidConfigError = errgo.New("invalid config")

// IsInvalidConfig asserts invalidConfigError.
func IsInvalidConfig(err error) bool {
	return errgo.Cause(err) == invalidConfigError
}

var wrongTypeError = errgo.New("wrong type")

// IsWrongType asserts wrongTypeError.
func IsWrongType(err error) bool {
	return errgo.Cause(err) == wrongTypeError
}
