package kube

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestExplorerSubscriptionsListsSubscriptions(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		subscriptionGVR: "SubscriptionList",
	}, &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.clusternet.io/v1alpha1",
		"kind":       "Subscription",
		"metadata": map[string]any{
			"name":      "demo-sub",
			"namespace": "default",
		},
		"status": map[string]any{"desiredReleases": int64(1)},
	}})
	explorer := NewExplorer(nil, nil, client)

	subscriptions, err := explorer.Subscriptions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subscriptions) != 1 || subscriptions[0].Name != "demo-sub" {
		t.Fatalf("unexpected subscriptions: %#v", subscriptions)
	}
}

func TestExplorerManifestsListsManifests(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		manifestGVR: "ManifestList",
	}, &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.clusternet.io/v1alpha1",
		"kind":       "Manifest",
		"metadata": map[string]any{
			"name":      "demo-manifest",
			"namespace": "default",
		},
		"template": map[string]any{"kind": "Deployment"},
	}})
	explorer := NewExplorer(nil, nil, client)

	manifests, err := explorer.Manifests(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(manifests) != 1 || manifests[0].TemplateKind != "Deployment" {
		t.Fatalf("unexpected manifests: %#v", manifests)
	}
}
