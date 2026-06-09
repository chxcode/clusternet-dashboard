package kube

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewSubscriptionDetailShowsObservedAndMissingTargets(t *testing.T) {
	subscription := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "demo-app", "namespace": "demo-app"},
		"spec": map[string]any{"feeds": []any{map[string]any{"apiVersion": "apps.clusternet.io/v1alpha1", "kind": "HelmChart", "name": "demo-app", "namespace": "demo-app"}}},
		"status": map[string]any{
			"desiredReleases": int64(2),
			"bindingClusters": []any{"clusters/child-a", "clusters/child-b"},
			"aggregatedStatuses": []any{map[string]any{"feedStatusDetails": []any{
				map[string]any{"clusterName": "child-a", "available": true, "replicaStatus": map[string]any{}},
			}}},
		},
	}}
	clusters := []ManagedClusterSummary{
		{Name: "child-a", Namespace: "clusters", Online: true, Status: "Online"},
		{Name: "child-b", Namespace: "clusters", Online: false, Status: "Offline"},
	}

	detail := NewSubscriptionDetail(subscription, clusters)

	if detail.Summary.Name != "demo-app" || len(detail.Feeds) != 1 {
		t.Fatalf("unexpected detail summary/feed: %#v", detail)
	}
	if len(detail.TargetClusters) != 2 {
		t.Fatalf("expected 2 target clusters, got %#v", detail.TargetClusters)
	}
	if !detail.TargetClusters[0].Observed || !detail.TargetClusters[0].Online || !detail.TargetClusters[0].Available {
		t.Fatalf("expected child-a observed, available, and online: %#v", detail.TargetClusters[0])
	}
	if detail.TargetClusters[1].Observed || detail.TargetClusters[1].Online {
		t.Fatalf("expected child-b not observed and offline: %#v", detail.TargetClusters[1])
	}
}
