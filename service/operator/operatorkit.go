package operator

import (
	microerror "github.com/giantswarm/microkit/error"
)

// Operator implements the building blocks of any operator business logic being
// reconciled when observing TPRs. This interface provides a guideline for a
// more easy way to follow the rather complex intentions of operators in
// general.
type Operator interface {
	// GetCurrentState receives the custom object observed during TPR watches. Its
	// purpose is to return the current state of the resources being managed by
	// the operator. This can e.g. be some actual data within a configmap as
	// provided by the Kubernetes API. This is not limited to Kubernetes resources
	// though. Another example would be to fetch and return information about
	// Flannel bridges.
	GetCurrentState(obj interface{}) (interface{}, error)
	// GetDesiredState receives the custom object observed during TPR watches. Its
	// purpose is to return the desired state of the resources being managed by
	// the operator. The desired state should always be able to be made up using
	// the information provided by the TPO. This can e.g. be some data within a
	// configmap, how it should be provided by the Kubernetes API. This is not
	// limited to Kubernetes resources though. Another example would be to make up
	// and return information about Flannel bridges, how they should be look like
	// on a server host.
	GetDesiredState(obj interface{}) (interface{}, error)
	// GetEmptyState is only to return the specific zero value the operator
	// expects when reconciling delete operations. So this returns the desired
	// state for delete operations. On create and delete events the operator might
	// need to reconcile resources by removing them. GetDeleteState will receive
	// as desiredState. This is to align to the general concept of reconciliation
	// regardless creation or deletion events.
	GetEmptyState() interface{}
	// GetCreateState receives the custom object observed during TPR watches. It
	// also receives the current state as provided by GetCurrentState and the
	// desired state as provided by GetDesiredState. GetCreateState analyses the
	// current and desired state and returns the state intended to be created by
	// ProcessCreateState.
	GetCreateState(obj, currentState, desiredState interface{}) (interface{}, error)
	// GetDeleteState receives the custom object observed during TPR watches. It
	// also receives the current state as provided by GetCurrentState and the
	// desired state as provided by GetDesiredState. GetDeleteState analyses the
	// current and desired state and returns the state intended to be deleted by
	// ProcessDeleteState.
	GetDeleteState(obj, currentState, desiredState interface{}) (interface{}, error)
	// ProcessCreateState receives the custom object observed during TPR watches.
	// It also receives the state intended to be created as provided by
	// GetCreateState. ProcessCreateState only has to create resources based on
	// its provided input. All other reconciliation logic and state transformation
	// is already done at this point of the reconciliation loop.
	ProcessCreateState(obj, createState interface{}) error
	// ProcessDeleteState receives the custom object observed during TPR watches.
	// It also receives the state intended to be deleted as provided by
	// GetDeleteState. ProcessDeleteState only has to delete resources based on
	// its provided input. All other reconciliation logic and state transformation
	// is already done at this point of the reconciliation loop.
	ProcessDeleteState(obj, deleteState interface{}) error
}

// ProcessCreate is a drop-in for an informer's AddFunc. It receives the custom
// object observed during TPR watches and anything that implements Operator.
// ProcessCreate takes care about all necessary reconciliation logic for create
// events.
//
//     func addFunc(obj interface{}) {
//         err := ProcessCreate(obj, operator)
//         if err != nil {
//             // error handling here
//         }
//     }
//
//     newResourceEventHandler := &cache.ResourceEventHandlerFuncs{
//         AddFunc:    addFunc,
//     }
//
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

// ProcessDelete is a drop-in for an informer's DeleteFunc. It receives the
// custom object observed during TPR watches and anything that implements
// Operator. ProcessDelete takes care about all necessary reconciliation logic
// for delete events.
//
//     func deleteFunc(obj interface{}) {
//         err := ProcessDelete(obj, operator)
//         if err != nil {
//             // error handling here
//         }
//     }
//
//     newResourceEventHandler := &cache.ResourceEventHandlerFuncs{
//         DeleteFunc:    deleteFunc,
//     }
//
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
