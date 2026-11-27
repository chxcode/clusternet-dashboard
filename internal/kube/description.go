package kube

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var descriptionGVR = helmReleaseGVR.GroupVersion().WithResource("descriptions")

type DescriptionReference struct {
	Name string `json:"name"`
	UID  string `json:"uid"`
}

type DescriptionStatus struct {
	Phase  string `json:"phase,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func controllingDescriptionRef(release *unstructured.Unstructured) *metav1.OwnerReference {
	ref := metav1.GetControllerOf(release)
	if ref == nil || ref.APIVersion != descriptionGVR.GroupVersion().String() ||
		ref.Kind != "Description" || ref.Name == "" || ref.UID == "" {
		return nil
	}
	return ref
}

func releaseDescriptionOwner(release *unstructured.Unstructured) (*DescriptionReference, bool) {
	if ref := controllingDescriptionRef(release); ref != nil {
		return &DescriptionReference{Name: ref.Name, UID: string(ref.UID)}, true
	}
	for _, ref := range release.GetOwnerReferences() {
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err == nil && gv.Group == descriptionGVR.Group && ref.Kind == "Description" && ref.Name != "" && ref.UID != "" {
			return &DescriptionReference{Name: ref.Name, UID: string(ref.UID)}, false
		}
	}
	return nil, false
}

// DescriptionStatus reads only the status of the referenced object incarnation.
func (e *Explorer) DescriptionStatus(ctx context.Context, namespace, name, uid string) (DescriptionStatus, error) {
	if e.dynamic == nil {
		return DescriptionStatus{}, apierrors.NewServiceUnavailable("Kubernetes client unavailable")
	}
	obj, err := e.dynamic.Resource(descriptionGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return DescriptionStatus{}, err
	}
	if string(obj.GetUID()) != uid {
		return DescriptionStatus{}, apierrors.NewGone("the referenced Description has been replaced")
	}
	return DescriptionStatus{
		Phase: getNestedString(obj, "status", "phase"), Reason: getNestedString(obj, "status", "reason"),
	}, nil
}
