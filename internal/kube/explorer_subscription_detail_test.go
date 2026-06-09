package kube

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestExplorerSubscriptionDetailReturnsTargetsWithClusterStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		subscriptionGVR:    "SubscriptionList",
		managedClusterGVR: "ManagedClusterList",
	},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apps.clusternet.io/v1alpha1",
			"kind":       "Subscription",
			"metadata":  map[string]any{"name": "demo-sub", "namespace": "default"},
			"status": map[string]any{
				"bindingClusters": []any{"clusters/child-a"},
				"aggregatedStatuses": []any{map[string]any{"feedStatusDetails": []any{
					map[string]any{"clusterName": "child-a", "replicaStatus": map[string]any{}},
				}}},
			},
		}},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "clusters.clusternet.io/v1beta1",
			"kind":       "ManagedCluster",
			"metadata":  map[string]any{"name": "child-a", "namespace": "clusters"},
			"status":    map[string]any{"readyz": true},
		}},
	)
	explorer := NewExplorer(nil, nil, client)

	detail, err := explorer.SubscriptionDetail(context.Background(), "default", "demo-sub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Summary.Name != "demo-sub" || len(detail.TargetClusters) != 1 {
		t.Fatalf("unexpected detail: %#v", detail)
	}
	if !detail.TargetClusters[0].Observed || !detail.TargetClusters[0].Online {
		t.Fatalf("expected observed online target: %#v", detail.TargetClusters[0])
	}
}
