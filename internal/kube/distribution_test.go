package kube

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestOverrideSummaryCountsOverrideEntries(t *testing.T) {
	item := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "global-demo", "namespace": "default"},
		"spec": map[string]any{
			"overrides": []any{
				map[string]any{"name": "a"},
				map[string]any{"name": "b"},
			},
		},
	}}

	summary := NewOverrideSummary(item, "Globalization")

	if summary.Name != "global-demo" || summary.Kind != "Globalization" {
		t.Fatalf("unexpected summary identity: %#v", summary)
	}
	if summary.OverrideCount != 2 {
		t.Fatalf("expected 2 overrides, got %d", summary.OverrideCount)
	}
}

func TestFeedInventorySummaryCountsFeeds(t *testing.T) {
	item := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "feed-demo", "namespace": "default"},
		"spec": map[string]any{
			"feeds": []any{
				map[string]any{"name": "feed-a"},
				map[string]any{"name": "feed-b"},
				map[string]any{"name": "feed-c"},
			},
		},
	}}

	summary := NewFeedInventorySummary(item)

	if summary.Name != "feed-demo" || summary.Namespace != "default" {
		t.Fatalf("unexpected feed inventory identity: %#v", summary)
	}
	if summary.FeedCount != 3 {
		t.Fatalf("expected 3 feeds, got %d", summary.FeedCount)
	}
}
