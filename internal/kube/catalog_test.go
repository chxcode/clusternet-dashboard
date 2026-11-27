package kube

import (
	"encoding/json"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCatalogProjectsRegistrationWithoutCredentials(t *testing.T) {
	r := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "request-x", "creationTimestamp": "2026-11-01T12:00:00Z", "annotations": map[string]any{"opaque": "private-field"}},
		"spec":     map[string]any{"clusterName": "edge", "clusterId": "identity", "syncMode": "Pull", "clusterLabels": map[string]any{"opaque": "private-field"}},
		"status":   map[string]any{"result": "Approved", "managedClusterName": "edge-42", "dedicatedNamespace": "tenant", "token": "private-field", "caCertificate": "private-field", "errorMessage": "private-field"},
	}}
	got := registrationSummary(r)
	if got.Result != "Approved" || got.ManagedCluster != "edge-42" || got.Created == "" || got.ClusterID != "identity" || got.DedicatedNamespace != "tenant" {
		t.Fatalf("unexpected projected identities: %#v", got)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "private-field") {
		t.Fatal("private registration payload in projection")
	}
	delete(r.Object, "status")
	if v := registrationSummary(r).Result; v != "" {
		t.Fatalf("fabricated result %q", v)
	}
}

func TestCatalogOverridesSelectTargetsWithoutLeakingPatchContent(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{"clusterAffinity": map[string]any{"matchLabels": map[string]any{"region": "west"}}, "priority": int64(0), "overridePolicy": "ApplyLater",
			"feed":      map[string]any{"kind": "HelmChart", "name": "web"},
			"overrides": []any{map[string]any{"name": "set-replicas", "type": "MergePatch", "value": "private-field", "overrideChart": map[string]any{"repo": "private-field"}}}},
	}}
	got := NewOverrideSummary(obj, "Globalization")
	if got.ClusterSelector != "region=west" || got.Feed.Name != "web" || got.Priority == nil || *got.Priority != 0 || len(got.Rules) != 1 || got.Rules[0].Name != "set-replicas" {
		t.Fatalf("unexpected override: %#v", got)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "private-field") {
		t.Fatal("raw override or chart settings leaked")
	}
	_ = unstructured.SetNestedMap(obj.Object, map[string]any{"matchExpressions": []any{map[string]any{"key": "a", "operator": "invalid"}}}, "spec", "clusterAffinity")
	if !NewOverrideSummary(obj, "Globalization").InvalidSelector {
		t.Fatal("expected unsupported selector to remain unknown")
	}
}

func TestCatalogProjectsHelmChartFlagsAndBaseFeedReferences(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "example", "labels": map[string]any{"apps.clusternet.io/subs.name": "sub", "clusters.clusternet.io/cluster-name": "edge"}},
		"spec":     map[string]any{"chart": "https://private-field/repo", "chartPullSecret": "private-field", "repo": "private-field", "wait": false, "timeoutSeconds": int64(0), "version": "1.2.3", "feeds": []any{map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "name": "settings", "replicaRequirements": "private-field"}}},
	}}
	c := helmChartSummary(obj)
	if c.Chart != "" || c.Version != "1.2.3" || c.TimeoutSeconds == nil || *c.TimeoutSeconds != 0 || len(c.Options) != 1 || c.Options["wait"] {
		t.Fatalf("wrong chart projection: %#v", c)
	}
	obj.Object["spec"].(map[string]any)["upgradeAtomic"] = false
	if upgrade, reported := helmChartSummary(obj).Options["upgradeAtomic"]; !reported || upgrade {
		t.Fatal("explicitly disabled upgradeAtomic must be visible")
	}
	b := baseSummary(obj)
	if b.ClusterName != "edge" || b.SubscriptionName != "sub" || len(b.Feeds) != 1 || b.Feeds[0].Name != "settings" {
		t.Fatalf("wrong base: %#v", b)
	}
	raw, _ := json.Marshal([]any{c, b, NewFeedInventorySummary(obj)})
	if strings.Contains(string(raw), "private-field") {
		t.Fatal("private material in projection")
	}
}
