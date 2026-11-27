package kube

import (
	"context"
	"errors"
	"slices"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
)

// Kubernetes admission may include the full CRR in a status message.
func registrationAPIError(err error) error {
	if err == nil {
		return nil
	}
	var status apierrors.APIStatus
	if errors.As(err, &status) {
		return &apierrors.StatusError{ErrStatus: metav1.Status{
			Status: metav1.StatusFailure, Code: status.Status().Code, Reason: status.Status().Reason,
			Message: "Kubernetes rejected the registration operation; check object state and permissions",
		}}
	}
	return errors.New("registration operation outcome unavailable; refresh the list")
}

func registrationIdentity(name, id string) bool {
	return clusterUUID.MatchString(id) || clusterUUID.MatchString(strings.TrimPrefix(name, "clusternet-"))
}

func registrationDeletionMode(obj *unstructured.Unstructured) string {
	if obj.GetUID() == "" || obj.GetResourceVersion() == "" || obj.GetDeletionTimestamp() != nil ||
		!registrationIdentity(obj.GetName(), getNestedString(obj, "spec", "clusterId")) {
		return ""
	}
	// A temporarily inconsistent object must not expose a withdrawal action.
	for _, key := range []string{"token", "caCertificate", "managedClusterName", "dedicatedNamespace"} {
		value, _, err := unstructured.NestedFieldNoCopy(obj.Object, "status", key)
		if err != nil || (value != nil && value != "") {
			return ""
		}
	}
	result, _, err := unstructured.NestedFieldNoCopy(obj.Object, "status", "result")
	if err != nil {
		return ""
	}
	switch result {
	case "Failed":
		return "failed"
	case "Denied":
		return "denied"
	case nil:
		// An explicit namespace may need correction in the child agent. A
		// regular bootstrap with an automatically allocated namespace continues.
		if ns := getNestedString(obj, "spec", "clusterNamespace"); ns != "" && len(validation.IsDNS1123Label(ns)) == 0 {
			return "pendingNamespace"
		}
	}
	return ""
}

// A custom approver can write arbitrary text, tokens or kubeconfig material
// into errorMessage. Return diagnostic categories only.
func registrationFailureReason(obj *unstructured.Unstructured) string {
	result := getNestedString(obj, "status", "result")
	if result == "Approved" {
		return ""
	}
	message := strings.ToLower(getNestedString(obj, "status", "errorMessage"))
	switch {
	case strings.Contains(message, "illegal update on spec.clusterid"):
		return "请求名称与声明的集群 ID 不一致"
	case strings.Contains(message, "namespace") && strings.Contains(message, "already exists"):
		return "控制器报告命名空间名称已被占用"
	case strings.Contains(message, "forbidden") || strings.Contains(message, "unauthorized"):
		return "控制器报告注册权限或凭证异常"
	case message != "":
		return "控制器已记录错误，完整原因请从受控的控制器日志核对"
	default:
		return ""
	}
}

type registrationNamespace struct {
	name, id           string
	owned, terminating bool
}

func projectRegistrationNamespace(obj *unstructured.Unstructured) registrationNamespace {
	l := obj.GetLabels()
	return registrationNamespace{obj.GetName(), l[deploymentIDLabel], l["clusternet.io/created-by"] == "clusternet-agent", obj.GetDeletionTimestamp() != nil || getNestedString(obj, "status", "phase") == "Terminating"}
}

// Reproduce the approver's precedence: an existing dedicated namespace for
// the ID is reused before it tries to create the explicitly requested name.
func (e *Explorer) addRegistrationNamespaceEvidence(ctx context.Context, rows []RegistrationSummary) {
	needed := false
	for _, r := range rows {
		if r.Result != "Approved" && r.RequestedNamespace != "" {
			needed = true
			break
		}
	}
	if !needed {
		return
	}
	namespaces, err := listCatalog(ctx, e, deploymentNamespaces, projectRegistrationNamespace)
	if err != nil {
		return
	}
	byName := make(map[string]registrationNamespace, len(namespaces))
	allocated := map[string]bool{}
	for _, ns := range namespaces {
		byName[ns.name] = ns
		if ns.owned && ns.id != "" {
			allocated[ns.id] = true
		}
	}
	for i := range rows {
		r := &rows[i]
		if r.Result == "Approved" || r.RequestedNamespace == "" {
			continue
		}
		ns, exists := byName[r.RequestedNamespace]
		switch {
		case allocated[r.ClusterID]:
			r.NamespaceEvidence = "allocated"
			r.DeletionMode = ""
		case exists && ns.terminating:
			r.NamespaceEvidence = "terminating"
		case exists:
			r.NamespaceEvidence = "occupied"
		default:
			r.NamespaceEvidence = "available"
		}
	}
}

func (e *Explorer) DeleteFailedRegistration(ctx context.Context, name, uid, version, confirmName string) error {
	if uid == "" || version == "" || name == "" || name != confirmName || len(validation.IsDNS1123Subdomain(name)) > 0 {
		return apierrors.NewBadRequest("enter the exact request name and provide its UID and resourceVersion")
	}
	if e.dynamic == nil {
		return apierrors.NewServiceUnavailable("Kubernetes client unavailable")
	}
	client := e.dynamic.Resource(registrationGVR)
	obj, err := client.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return registrationAPIError(err)
	}
	expected := types.UID(uid)
	if obj.GetUID() != expected || obj.GetResourceVersion() != version || registrationDeletionMode(obj) == "" {
		return apierrors.NewConflict(registrationGVR.GroupResource(), name, errors.New("request changed, is being processed or is ineligible; refresh the registration list"))
	}

	// A denied CRR can have different IDs in its name and spec. Check both
	// against actual allocations even when the approver has not recorded status.
	specID := getNestedString(obj, "spec", "clusterId")
	nameID := strings.TrimPrefix(name, "clusternet-")
	if !clusterUUID.MatchString(nameID) {
		nameID = ""
	}
	matches := func(id string) bool {
		return id != "" && (id == specID || id == nameID)
	}
	held, err := listCatalog(ctx, e, deploymentNamespaces, func(ns *unstructured.Unstructured) bool { return matches(ns.GetLabels()[deploymentIDLabel]) })
	if err != nil {
		return registrationAPIError(err)
	}
	if slices.Contains(held, true) {
		return apierrors.NewConflict(registrationGVR.GroupResource(), name, errors.New("a namespace is already associated with this cluster ID"))
	}
	held, err = listCatalog(ctx, e, managedClusterGVR, func(mc *unstructured.Unstructured) bool {
		return matches(getNestedString(mc, "spec", "clusterId")) || matches(mc.GetLabels()[deploymentIDLabel])
	})
	if err != nil {
		return registrationAPIError(err)
	}
	if slices.Contains(held, true) {
		return apierrors.NewConflict(registrationGVR.GroupResource(), name, errors.New("a ManagedCluster is already associated with this cluster ID"))
	}
	orphan := metav1.DeletePropagationOrphan
	return registrationAPIError(client.Delete(ctx, name, metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{UID: &expected, ResourceVersion: &version}, PropagationPolicy: &orphan,
	}))
}
