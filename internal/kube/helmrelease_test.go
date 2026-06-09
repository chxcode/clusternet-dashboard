package kube

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestNewHelmReleaseSummaryExtractsStatusAndErrorDescription(t *testing.T) {
	release := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name":      "demo-helm-demo-app",
			"namespace": "child-a",
			"labels": map[string]any{
				"clusters.clusternet.io/cluster-name": "child-a",
				"apps.clusternet.io/subs.name":        "demo-app",
				"apps.clusternet.io/subs.namespace":   "demo-app",
			},
		},
		"spec": map[string]any{
			"releaseName":     "demo-app",
			"targetNamespace": "demo-app",
			"chart":           "demo-chart",
			"version":         "1.2.3",
		},
		"status": map[string]any{
			"phase":       "failed",
			"description": "rendered manifests contain a resource that already exists",
			"version":     int64(5),
		},
	}}

	summary := NewHelmReleaseSummary(release)

	if summary.Name != "demo-helm-demo-app" || summary.Namespace != "child-a" {
		t.Fatalf("unexpected identity: %#v", summary)
	}
	if summary.ClusterName != "child-a" || summary.SubscriptionName != "demo-app" || summary.SubscriptionNamespace != "demo-app" {
		t.Fatalf("unexpected labels mapped: %#v", summary)
	}
	if summary.Phase != "failed" || summary.Description == "" || summary.Revision != 5 {
		t.Fatalf("unexpected status: %#v", summary)
	}
	if summary.ReleaseName != "demo-app" || summary.TargetNamespace != "demo-app" || summary.Chart != "demo-chart" || summary.ChartVersion != "1.2.3" {
		t.Fatalf("unexpected spec fields: %#v", summary)
	}
}

func TestExplorerHelmReleasesListsAllNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{helmReleaseGVR: "HelmReleaseList"}, &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.clusternet.io/v1alpha1",
		"kind":       "HelmRelease",
		"metadata": map[string]any{
			"name":      "demo",
			"namespace": "child-a",
		},
		"status": map[string]any{"phase": "deployed"},
	}})
	explorer := NewExplorer(nil, nil, client)

	releases, err := explorer.HelmReleases(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 1 || releases[0].Phase != "deployed" {
		t.Fatalf("unexpected releases: %#v", releases)
	}
}
