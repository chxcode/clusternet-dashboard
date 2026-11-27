package kube

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewSubscriptionDetailShowsObservedAndMissingTargets(t *testing.T) {
	subscription := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "demo-app", "namespace": "demo-app"},
		"spec":     map[string]any{"feeds": []any{map[string]any{"apiVersion": "apps.clusternet.io/v1alpha1", "kind": "HelmChart", "name": "demo-app", "namespace": "demo-app"}}},
		"status": map[string]any{
			"desiredReleases": int64(2),
			"bindingClusters": []any{"clusters/child-a", "clusters/child-b"},
			"aggregatedStatuses": []any{map[string]any{"apiVersion": "apps.clusternet.io/v1alpha1", "kind": "HelmChart", "name": "demo-app", "namespace": "demo-app", "feedStatusDetails": []any{
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
	if !detail.TargetClusters[0].Observed || !detail.TargetClusters[0].Online || detail.TargetClusters[0].Available == nil || !*detail.TargetClusters[0].Available {
		t.Fatalf("expected child-a observed, available, and online: %#v", detail.TargetClusters[0])
	}
	if detail.TargetClusters[1].Observed || detail.TargetClusters[1].Online {
		t.Fatalf("expected child-b not observed and offline: %#v", detail.TargetClusters[1])
	}
}

func TestSubscriptionReportsPreserveEachFeedAndExplicitZeroValues(t *testing.T) {
	group := func(name string, data map[string]any) any {
		data["clusterName"] = "child"
		return map[string]any{"kind": "Deployment", "name": name, "feedStatusDetails": []any{data}}
	}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"bindingClusters": []any{"clusters/child"},
			"aggregatedStatuses": []any{
				group("one", map[string]any{"available": false, "replicaStatus": map[string]any{
					"readyReplicas": int64(0), "currentReplicas": int64(2), "internal": "payload", "active": "bad type",
				}}),
				group("two", map[string]any{}),
			},
		},
	}}
	detail := NewSubscriptionDetail(obj, nil)
	target := detail.TargetClusters[0]
	if !target.Observed || target.Available != nil || detail.Summary.ObservedReleases != 1 || len(target.FeedReports) != 2 {
		t.Fatalf("unexpected target feedback: %#v", target)
	}
	first, second := target.FeedReports[0], target.FeedReports[1]
	if first.Name != "one" || first.Available == nil || *first.Available || second.Name != "two" || second.Available != nil {
		t.Fatalf("unexpected per-feed availability: %#v", target.FeedReports)
	}
	if zero, ok := first.ReplicaStatus["readyReplicas"]; !ok || zero != 0 || first.ReplicaStatus["currentReplicas"] != 2 || len(first.ReplicaStatus) != 2 {
		t.Fatalf("unexpected replica counters: %#v", first.ReplicaStatus)
	}
}

func TestSubscriptionReportsDisambiguateSameNamedTargets(t *testing.T) {
	feed := func(name, id string, available bool) any {
		return map[string]any{"name": name, "kind": "HelmChart", "feedStatusDetails": []any{
			map[string]any{"clusterName": "child", "clusterId": id, "available": available},
		}}
	}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"bindingClusters":    []any{"first/child", "second/child", "missing/child"},
			"aggregatedStatuses": []any{feed("chart", "stable-a", true), feed("chart", "stable-b", false), feed("legacy", "", true)},
		},
	}}
	clusters := []ManagedClusterSummary{
		{Name: "child", Namespace: "first", Status: "Online", Online: true, ClusterID: "stable-a"},
		{Name: "child", Namespace: "second", Status: "Offline", ClusterID: "stable-b"},
	}
	got := NewSubscriptionDetail(obj, clusters)
	if got.Summary.ObservedReleases != 2 || NewSubscriptionSummary(obj).ObservedReleases != 2 {
		t.Fatal("expected two individually identifiable reporters")
	}
	first, second, missing := got.TargetClusters[0], got.TargetClusters[1], got.TargetClusters[2]
	if len(first.FeedReports) != 1 || *first.FeedReports[0].Available != true ||
		len(second.FeedReports) != 1 || *second.FeedReports[0].Available != false {
		t.Fatalf("cross-target report: %#v", got.TargetClusters)
	}
	if len(first.UnattributedFeeds) != 1 || first.UnattributedFeeds[0].Name != "legacy" || len(second.UnattributedFeeds) != 1 {
		t.Fatalf("expected ambiguous legacy feedback: %#v", got.TargetClusters)
	}
	if missing.Observed || missing.Online || missing.Status != "Unknown" || len(missing.UnattributedFeeds) == 0 {
		t.Fatalf("missing qualified target borrowed another target's data: %#v", missing)
	}
}
func TestSubscriberSelectorsDistinguishAbsentAndEmpty(t *testing.T) {
	sub := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"subscribers": []any{
		map[string]any{"clusterAffinity": map[string]any{"matchLabels": map[string]any{"app": "sample"}}},
		map[string]any{"clusterAffinity": map[string]any{}}, map[string]any{},
	}}}}
	selectors := NewSubscriptionDetail(sub, nil).SubscriberSelectors
	if len(selectors) != 3 || selectors[0] == nil || *selectors[0] != "app=sample" || selectors[1] == nil || *selectors[1] != "" || selectors[2] != nil {
		t.Fatalf("unexpected selector output: %#v", selectors)
	}
}
