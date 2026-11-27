package kube

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestManagedClusterSummaryUsesReadyzForOnlineStatus(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name":      "child-a",
			"namespace": "clusternet-system",
		},
		"status": map[string]any{
			"readyz":       true,
			"k8sVersion":   "v1.30.0",
			"apiserverURL": "https://child-a.example.com",
		},
	}}

	summary := NewManagedClusterSummary(cluster)

	if summary.Name != "child-a" || summary.Namespace != "clusternet-system" {
		t.Fatalf("unexpected cluster identity: %#v", summary)
	}
	if !summary.Online {
		t.Fatalf("expected cluster to be online: %#v", summary)
	}
	if summary.Status != "Online" {
		t.Fatalf("expected status Online, got %q", summary.Status)
	}
	if summary.KubernetesVersion != "v1.30.0" {
		t.Fatalf("expected Kubernetes version v1.30.0, got %q", summary.KubernetesVersion)
	}
}

func TestManagedClusterSummaryIncludesLabels(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name": "child-with-labels",
			"labels": map[string]any{
				"customer": "demo-app",
				"app":      "sample-app",
			},
		},
	}}

	summary := NewManagedClusterSummary(cluster)

	if summary.Labels["customer"] != "demo-app" || summary.Labels["app"] != "sample-app" {
		t.Fatalf("expected managed cluster labels, got %#v", summary.Labels)
	}
}

func TestManagedClusterSummaryUsesReadyConditionForOnlineStatus(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "child-b"},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "True"},
			},
		},
	}}

	summary := NewManagedClusterSummary(cluster)

	if !summary.Online {
		t.Fatalf("expected Ready condition to mark cluster online: %#v", summary)
	}
}

func TestManagedClusterSummaryMarksMissingHealthUnknown(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "child-c"},
		"status":   map[string]any{},
	}}

	summary := NewManagedClusterSummary(cluster)

	if summary.Online {
		t.Fatalf("expected cluster without health signals to be offline: %#v", summary)
	}
	if summary.Status != "Unknown" {
		t.Fatalf("expected status Unknown, got %q", summary.Status)
	}
}

func TestManagedClusterHonorsReadyConditionOverProbes(t *testing.T) {
	for condition, want := range map[string]string{"False": "NotReady", "Unknown": "Unknown", "True": "Online"} {
		cluster := &unstructured.Unstructured{Object: map[string]any{"status": map[string]any{
			"livez": true, "conditions": []any{map[string]any{"type": "Ready", "status": condition}},
		}}}
		if got := NewManagedClusterSummary(cluster); got.Status != want || got.Online != (want == "Online") {
			t.Errorf("Ready=%s: got %s/online=%t", condition, got.Status, got.Online)
		}
	}
}

func TestUnreadyProbeTakesPrecedenceOverLiveProbe(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]any{"status": map[string]any{"readyz": false, "livez": true}}}
	if got := NewManagedClusterSummary(cluster); got.Online || got.Status != "NotReady" {
		t.Fatalf("readyz=false, livez=true: got %s/online=%t", got.Status, got.Online)
	}
}
