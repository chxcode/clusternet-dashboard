package kube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/util/retry"
)

type LabelChange struct {
	Key    string
	Before *string
	After  *string
}

func (e *Explorer) ChangeClusterLabels(ctx context.Context, namespace, name, uid string, changes []LabelChange) (map[string]string, error) {
	if e.dynamic == nil {
		return nil, apierrors.NewServiceUnavailable("Kubernetes client unavailable")
	}
	if uid == "" || len(changes) == 0 {
		return nil, apierrors.NewBadRequest("a cluster UID and label changes are required")
	}
	seen := make(map[string]bool, len(changes))
	for _, change := range changes {
		if seen[change.Key] || len(validation.IsQualifiedName(change.Key)) != 0 {
			return nil, apierrors.NewBadRequest("invalid or duplicate Kubernetes label key")
		}
		seen[change.Key] = true
		for _, value := range []*string{change.Before, change.After} {
			if value != nil && len(validation.IsValidLabelValue(*value)) != 0 {
				return nil, apierrors.NewBadRequest("invalid Kubernetes label value")
			}
		}
	}

	resource := e.dynamic.Resource(managedClusterGVR).Namespace(namespace)
	var updated map[string]string
	// Compare only edited keys. A heartbeat or unrelated label may change the
	// resourceVersion between GET and PATCH; retry that race against fresh data.
	labelConflict := errors.New("cluster identity or edited labels changed")
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cluster, err := resource.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if string(cluster.GetUID()) != uid {
			return labelConflict
		}
		labels := cluster.GetLabels()
		patchLabels := map[string]*string{}
		for _, change := range changes {
			current, exists := labels[change.Key]
			if matchesLabel(current, exists, change.After) {
				continue // also makes retrying an already applied request safe.
			}
			if !matchesLabel(current, exists, change.Before) {
				return labelConflict
			}
			patchLabels[change.Key] = change.After
		}
		if len(patchLabels) == 0 {
			updated = labels
			return nil
		}
		if cluster.GetResourceVersion() == "" {
			return fmt.Errorf("cluster lacks a resourceVersion for conditional patch")
		}
		body, err := json.Marshal(map[string]any{"metadata": map[string]any{
			"resourceVersion": cluster.GetResourceVersion(), "labels": patchLabels,
		}})
		if err != nil {
			return err
		}
		saved, err := resource.Patch(ctx, name, types.MergePatchType, body, metav1.PatchOptions{})
		if err == nil {
			updated = saved.GetLabels()
		}
		return err
	})
	if errors.Is(err, labelConflict) {
		return nil, apierrors.NewConflict(managedClusterGVR.GroupResource(), name, labelConflict)
	}
	return updated, err
}

func matchesLabel(current string, exists bool, expected *string) bool {
	return (!exists && expected == nil) || (exists && expected != nil && current == *expected)
}
