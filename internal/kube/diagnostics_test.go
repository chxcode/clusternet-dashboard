package kube

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		"metadata":  map[string]any{"name": "demo", "namespace": "default"},
		"status":    map[string]any{"desiredReleases": int64(1), "completedReleases": int64(1)},
	}})

	client := k8sfake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "clusternet-hub", Namespace: "clusternet-system"},
		Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "hub", Image: "ghcr.io/clusternet/clusternet-hub:v0.18.1"}}}}},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1, Replicas: 1},
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
}
