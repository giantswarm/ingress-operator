package operator

import (
	microerror "github.com/giantswarm/microkit/error"
)

type Operator interface {
	GetCurrentState(obj interface{}) (interface{}, error)
	GetDesiredState(obj interface{}) (interface{}, error)
	GetEmptyState() interface{}

	GetCreateState(obj, currentState, desiredState interface{}) (interface{}, error)
	GetDeleteState(obj, currentState, desiredState interface{}) (interface{}, error)

	ProcessCreateState(obj, createState interface{}) error
	ProcessDeleteState(obj, deleteState interface{}) error
}

func ProcessCreate(obj interface{}, operator Operator) error {
	currentState, err := operator.GetCurrentState(obj)
	if err != nil {
		return microerror.MaskAny(err)
	}

	desiredState, err := operator.GetDesiredState(obj)
	if err != nil {
		return microerror.MaskAny(err)
	}

	createState, err := operator.GetCreateState(obj, currentState, desiredState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	err = operator.ProcessCreateState(obj, createState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	deleteState, err := operator.GetDeleteState(obj, currentState, desiredState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	err = operator.ProcessDeleteState(obj, deleteState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	return nil
}

func ProcessDelete(obj interface{}, operator Operator) error {
	currentState, err := operator.GetCurrentState(obj)
	if err != nil {
		return microerror.MaskAny(err)
	}

	desiredState := operator.GetEmptyState()

	deleteState, err := operator.GetDeleteState(obj, currentState, desiredState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	err = operator.ProcessDeleteState(obj, deleteState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	return nil
}
