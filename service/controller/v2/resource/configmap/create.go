package configmap

import (
	"context"
)

// ApplyCreateChange is a no-op. The ingress-operator manages a config map that
// already exists. Thus only update proceedures are done. The creation of the
// service being maintained is ensured by another component.
func (r *Resource) ApplyCreateChange(ctx context.Context, obj, createChange interface{}) error {
	return nil
}
