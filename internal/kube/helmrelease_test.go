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

func TestReleaseDescriptionOwnerMatchesGroupKindAndUID(t *testing.T) {
	release := &unstructured.Unstructured{}
	release.SetOwnerReferences([]metav1.OwnerReference{
		{APIVersion: "other.example/v1", Kind: "Description", Name: "wrong", UID: "foreign"},
		{APIVersion: "apps.clusternet.io/v1alpha1", Kind: "Description", Name: "without-uid"},
		{APIVersion: "apps.clusternet.io/v1alpha1", Kind: "Description", Name: "expected", UID: "original"},
	})
	if ref := NewHelmReleaseSummary(release).DescriptionOwner; ref == nil || ref.Name != "expected" || ref.UID != "original" {
		t.Errorf("wrong owner: %#v", ref)
	}
	if NewHelmReleaseSummary(release).ControlledByDescription {
		t.Fatal("a non-controlling owner cannot authorize deletion")
	}
	controller := true
	release.SetOwnerReferences(append(release.GetOwnerReferences(), metav1.OwnerReference{
		APIVersion: "apps.clusternet.io/v1alpha1", Kind: "Description", Name: "controller", UID: "owner", Controller: &controller,
	}))
	release.SetResourceVersion("27")
	now := metav1.Now()
	release.SetDeletionTimestamp(&now)
	summary := NewHelmReleaseSummary(release)
	if !summary.ControlledByDescription || summary.DescriptionOwner == nil || summary.DescriptionOwner.UID != "owner" || summary.ResourceVersion != "27" || !summary.Deleting {
		t.Fatalf("unexpected controlling owner or deletion identity: %#v", summary)
	}
}

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
	if summary.Atomic != nil || summary.UpgradeAtomic != nil {
		t.Fatal("an absent Helm option must remain unreported")
	}
	release.Object["spec"].(map[string]any)["atomic"] = false
	release.Object["spec"].(map[string]any)["upgradeAtomic"] = true
	summary = NewHelmReleaseSummary(release)
	if summary.Atomic == nil || *summary.Atomic || summary.UpgradeAtomic == nil || !*summary.UpgradeAtomic {
		t.Fatal("expected separately configured installation and upgrade policies")
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
