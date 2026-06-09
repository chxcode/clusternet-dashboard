package kube

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestExplorerClustersListsManagedClusters(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "clusters.clusternet.io", Version: "v1beta1", Resource: "managedclusters"}
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "clusters.clusternet.io/v1beta1",
		"kind":       "ManagedCluster",
		"metadata": map[string]any{
			"name":      "child-a",
			"namespace": "clusternet-system",
		},
		"status": map[string]any{"readyz": true},
	}}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "ManagedClusterList",
	}, cluster)
	explorer := NewExplorer(nil, nil, client)

	clusters, err := explorer.ManagedClusters(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if clusters[0].Name != "child-a" || !clusters[0].Online {
		t.Fatalf("unexpected cluster summary: %#v", clusters[0])
	}
}

func TestExplorerClustersReturnsEmptyWhenNoManagedClustersExist(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "clusters.clusternet.io", Version: "v1beta1", Resource: "managedclusters"}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "ManagedClusterList",
	}, &unstructured.UnstructuredList{Object: map[string]any{"metadata": metav1.ListMeta{}}})
	explorer := NewExplorer(nil, nil, client)

	clusters, err := explorer.ManagedClusters(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clusters) != 0 {
		t.Fatalf("expected no clusters, got %#v", clusters)
	}
}
