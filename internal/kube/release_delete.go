package kube

import (
	"context"
	"errors"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func (e *Explorer) DeleteFailedRelease(ctx context.Context, namespace, name, uid, resourceVersion string) error {
	if uid == "" || resourceVersion == "" {
		return apierrors.NewBadRequest("HelmRelease UID and resourceVersion are required")
	}
	if e.dynamic == nil {
		return apierrors.NewServiceUnavailable("Kubernetes client unavailable")
	}
	resource := e.dynamic.Resource(helmReleaseGVR).Namespace(namespace)
	release, err := resource.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	expectedUID := types.UID(uid)
	if release.GetUID() != expectedUID || release.GetResourceVersion() != resourceVersion ||
		release.GetDeletionTimestamp() != nil || !strings.EqualFold(getNestedString(release, "status", "phase"), "failed") {
		return apierrors.NewConflict(helmReleaseGVR.GroupResource(), name, errors.New("release changed, is terminating or is no longer failed; reload before deleting"))
	}
	ref := controllingDescriptionRef(release)
	if ref == nil {
		return apierrors.NewConflict(helmReleaseGVR.GroupResource(), name, errors.New("release has no controlling Clusternet Description"))
	}
	owner, err := e.dynamic.Resource(descriptionGVR).Namespace(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if apierrors.IsNotFound(err) || owner.GetUID() != ref.UID || owner.GetDeletionTimestamp() != nil {
		return apierrors.NewConflict(helmReleaseGVR.GroupResource(), name, errors.New("controlling Description is missing, replaced or terminating"))
	}
	if !descriptionWantsRelease(owner, name) {
		return apierrors.NewConflict(helmReleaseGVR.GroupResource(), name, errors.New("controlling Description no longer declares this HelmRelease"))
	}
	// Preserve normal deletion and finalizer processing. Both values come
	// from the operator's selected release, including on a repeated request.
	return resource.Delete(ctx, name, metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{UID: &expectedUID, ResourceVersion: &resourceVersion},
	})
}

func descriptionWantsRelease(owner *unstructured.Unstructured, name string) bool {
	if getNestedString(owner, "spec", "deployer") != "Helm" {
		return false
	}
	charts, _, _ := unstructured.NestedSlice(owner.Object, "spec", "charts")
	for _, value := range charts {
		chart, ok := value.(map[string]any)
		if !ok {
			continue
		}
		ns, chartName := stringValue(chart["namespace"]), stringValue(chart["name"])
		// Matches Clusternet's utils.GenerateHelmReleaseName.
		if ns != "" && chartName != "" && owner.GetName()+"-"+ns+"-"+chartName == name {
			return true
		}
	}
	return false
}
