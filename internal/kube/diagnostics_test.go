package kube

import (
	"context"
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestExplorerDiagnosticsReportsResourcesComponentsAndCompatibility(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		subscriptionGVR: "SubscriptionList",
	}, &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.clusternet.io/v1alpha1",
		"kind":       "Subscription",
		"metadata":   map[string]any{"name": "demo", "namespace": "default"},
		"status":     map[string]any{"desiredReleases": int64(1), "completedReleases": int64(1)},
	}})

	client := k8sfake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "clusternet-hub", Namespace: "clusternet-system"},
		Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "hub", Image: "ghcr.io/clusternet/clusternet-hub:v0.18.1"}}}}},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1, Replicas: 1, AvailableReplicas: 1, UpdatedReplicas: 1, ObservedGeneration: 3,
			Conditions: []appsv1.DeploymentCondition{{
				Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue, Reason: "NewReplicaSetAvailable",
				Message: "New replica set is available", LastTransitionTime: metav1.NewTime(time.Unix(0, 0).UTC()),
			}},
		},
	})
	client.Fake.Resources = []*metav1.APIResourceList{
		{GroupVersion: "apps.clusternet.io/v1alpha1", APIResources: []metav1.APIResource{{Name: "subscriptions", Kind: "Subscription", Namespaced: true, Verbs: []string{"get", "list"}}}},
		{GroupVersion: "clusters.clusternet.io/v1beta1", APIResources: []metav1.APIResource{{Name: "managedclusters", Kind: "ManagedCluster", Namespaced: false, Verbs: []string{"get", "list"}}}},
	}
	discoveryClient := &discoveryfake.FakeDiscovery{Fake: &client.Fake}
	discoveryClient.Resources = client.Fake.Resources
	client.Fake.PrependReactor("create", "selfsubjectaccessreviews", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})

	explorer := NewExplorer(client, discoveryClient, dynamicClient)
	diagnostics, err := explorer.Diagnostics(context.Background())
	if err != nil {
		t.Fatalf("unexpected diagnostics error: %v", err)
	}

	if diagnostics.CompletedReleasesSupport != "supported" {
		t.Fatalf("expected completedReleases support, got %#v", diagnostics)
	}
	if diagnostics.Resources == nil {
		t.Fatalf("expected resources slice to be initialized")
	}
	if len(diagnostics.Components) != 1 || diagnostics.Components[0].Image != "ghcr.io/clusternet/clusternet-hub:v0.18.1" {
		t.Fatalf("expected component image, got %#v", diagnostics.Components)
	}
	component := diagnostics.Components[0]
	if component.AvailableReplicas != 1 || component.UpdatedReplicas != 1 || component.UnavailableReplicas != 0 || component.ObservedGeneration != 3 {
		t.Fatalf("incorrect deployment rollout counts: %#v", component)
	}
	wantCondition := ResourceCondition{
		Type: "Progressing", Status: "True", Reason: "NewReplicaSetAvailable",
		Message: "New replica set is available", LastTransitionTime: "1970-01-01T00:00:00Z",
	}
	if len(component.Conditions) != 1 || component.Conditions[0] != wantCondition {
		t.Fatalf("incorrect deployment conditions: %#v", component.Conditions)
	}
}

func TestDiagnosticsDegradedSectionsKeepPermissionChecks(t *testing.T) {
	client := k8sfake.NewSimpleClientset()
	client.PrependReactor("list", "deployments", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "deployments"}, "", errors.New("sensitive diagnostic detail"))
	})
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		subscriptionGVR: "SubscriptionList",
	})
	dynamicClient.PrependReactor("list", "subscriptions", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewServiceUnavailable("sensitive connection detail")
	})
	client.PrependReactor("create", "selfsubjectaccessreviews", func(action clienttesting.Action) (bool, runtime.Object, error) {
		review := action.(clienttesting.CreateAction).GetObject().(*authv1.SelfSubjectAccessReview).DeepCopy()
		review.Status.Allowed = review.Spec.ResourceAttributes.Resource != "deployments"
		return true, review, nil
	})
	diagnostics, err := NewExplorer(client, nil, dynamicClient).Diagnostics(t.Context())
	if err != nil || diagnostics.SectionErrors["components"] != "Forbidden" || diagnostics.SectionErrors["subscriptions"] != "ServiceUnavailable" {
		t.Fatalf("expected isolated errors with safe reasons, got %#v, %v", diagnostics.SectionErrors, err)
	}
	if diagnostics.CompletedReleasesSupport != "unknown" || len(diagnostics.ReadOnlyRBACChecks) == 0 {
		t.Fatalf("expected unknown compatibility and available RBAC results: %#v", diagnostics)
	}
}

type partialResourceDiscovery struct {
	discovery.DiscoveryInterface
	failedGroup string
}

func (d partialResourceDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	resources := []*metav1.APIResourceList{{
		GroupVersion: "apps.clusternet.io/v1alpha1",
		APIResources: []metav1.APIResource{{Name: "subscriptions"}},
	}}
	return resources, &discovery.ErrGroupDiscoveryFailed{Groups: map[schema.GroupVersion]error{
		{Group: d.failedGroup, Version: "v1"}: errors.New("unavailable API group"),
	}}
}

func TestClusternetDiscoveryRetainsPartialEvidence(t *testing.T) {
	for group, wantError := range map[string]bool{"clusters.clusternet.io": true, "metrics.k8s.io": false} {
		explorer := NewExplorer(nil, partialResourceDiscovery{failedGroup: group}, nil)
		resources, err := explorer.discoverClusternetResources()
		if (err != nil) != wantError || len(resources) != 1 || resources[0].Name != "subscriptions" {
			t.Errorf("%s: resources %#v, error %v, want partial discovery flag %t", group, resources, err, wantError)
		}
		if listed, err := explorer.ClusternetResources(); err != nil || len(listed) != 1 {
			t.Errorf("resource list lost readable groups: %#v, %v", listed, err)
		}
	}
}
