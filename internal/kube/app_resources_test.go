package kube

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSubscriptionSummaryCalculatesProgress(t *testing.T) {
	subscription := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name":      "demo-sub",
			"namespace": "default",
		},
		"status": map[string]any{
			"desiredReleases":   int64(3),
			"completedReleases": int64(2),
			"bindingClusters": []any{
				"clusternet-system/child-a",
				"clusternet-system/child-b",
			},
		},
	}}

	summary := NewSubscriptionSummary(subscription)

	if summary.Name != "demo-sub" || summary.Namespace != "default" {
		t.Fatalf("unexpected subscription identity: %#v", summary)
	}
	if summary.DesiredReleases != 3 || summary.CompletedReleases != 2 {
		t.Fatalf("unexpected release progress: %#v", summary)
	}
	if !summary.CompletionKnown {
		t.Fatalf("expected completion to be known")
	}
	if summary.Status != "Progressing" {
		t.Fatalf("expected Progressing, got %q", summary.Status)
	}
	if summary.BindingClusterCount != 2 {
		t.Fatalf("expected 2 binding clusters, got %d", summary.BindingClusterCount)
	}
}

func TestSubscriptionSummaryIncludesSchedulingAndFeedSummary(t *testing.T) {
	subscription := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "demo-sub"},
		"spec": map[string]any{
			"schedulerName":      "default-scheduler",
			"schedulingStrategy": "Dividing",
			"subscribers": []any{
				map[string]any{"clusterAffinity": map[string]any{}},
			},
			"feeds": []any{
				map[string]any{"kind": "HelmChart", "name": "demo-chart", "namespace": "demo"},
			},
		},
	}}

	summary := NewSubscriptionSummary(subscription)

	if summary.SchedulerName != "default-scheduler" || summary.SchedulingStrategy != "Dividing" {
		t.Fatalf("unexpected scheduling info: %#v", summary)
	}
	if summary.SubscriberCount != 1 || summary.FeedCount != 1 || summary.FeedKinds[0] != "HelmChart" {
		t.Fatalf("unexpected feed/subscriber summary: %#v", summary)
	}
}

func TestSubscriptionSummaryMarksCompleted(t *testing.T) {
	subscription := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "done-sub"},
		"status": map[string]any{
			"desiredReleases":   int64(2),
			"completedReleases": int64(2),
		},
	}}

	summary := NewSubscriptionSummary(subscription)

	if summary.Status != "Completed" {
		t.Fatalf("expected Completed, got %q", summary.Status)
	}
}

func TestSubscriptionSummaryHandlesOlderStatusWithoutCompletedReleases(t *testing.T) {
	subscription := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "legacy-sub"},
		"status": map[string]any{
			"desiredReleases": int64(18),
			"bindingClusters": []any{"ns-a/cluster-a", "ns-b/cluster-b"},
			"aggregatedStatuses": []any{
				map[string]any{
					"feedStatusDetails": []any{
						map[string]any{"clusterName": "cluster-a", "replicaStatus": map[string]any{}},
						map[string]any{"clusterName": "cluster-b", "replicaStatus": map[string]any{}},
					},
				},
			},
		},
	}}

	summary := NewSubscriptionSummary(subscription)

	if summary.CompletionKnown {
		t.Fatalf("expected completionKnown=false for old status without completedReleases")
	}
	if summary.CompletedReleases != 0 || summary.ObservedReleases != 2 {
		t.Fatalf("unexpected progress counters: %#v", summary)
	}
	if summary.Status != "Progressing" {
		t.Fatalf("expected Progressing, got %q", summary.Status)
	}
}

func TestManifestSummaryExtractsTemplateIdentity(t *testing.T) {
	manifest := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name":      "demo-manifest",
			"namespace": "default",
		},
		"template": map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "nginx",
				"namespace": "demo",
			},
		},
	}}

	summary := NewManifestSummary(manifest)

	if summary.Name != "demo-manifest" || summary.Namespace != "default" {
		t.Fatalf("unexpected manifest identity: %#v", summary)
	}
	if summary.TemplateKind != "Deployment" || summary.TemplateName != "nginx" || summary.TemplateNamespace != "demo" {
		t.Fatalf("unexpected template identity: %#v", summary)
	}
}
