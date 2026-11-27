package kube

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func versionChart() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": helmChartGVR.GroupVersion().String(), "kind": "HelmChart",
		"metadata": map[string]any{"namespace": "charts", "name": "web", "uid": "chart-uid", "resourceVersion": "17"},
		"spec":     map[string]any{"version": "1.2.3", "repo": "https://example.test", "chart": "web"},
	}}
}

func versionSub(namespace, name, feedNamespace string, targets ...string) *unstructured.Unstructured {
	sub := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": subscriptionGVR.GroupVersion().String(), "kind": "Subscription",
		"metadata": map[string]any{"namespace": namespace, "name": name, "uid": "uid-" + namespace + "-" + name},
		"spec": map[string]any{"feeds": []any{map[string]any{
			"apiVersion": helmChartGVR.GroupVersion().String(), "kind": "HelmChart", "namespace": feedNamespace, "name": "web",
		}}},
	}}
	if targets != nil {
		_ = unstructured.SetNestedStringSlice(sub.Object, targets, "status", "bindingClusters")
	}
	return sub
}

func versionExplorer(objects ...runtime.Object) (*fake.FakeDynamicClient, *Explorer) {
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		subscriptionGVR: "SubscriptionList",
	}, objects...)
	return client, NewExplorer(nil, nil, client)
}

func TestChartVersionImpactAndConditionalPatch(t *testing.T) {
	chart := versionChart()
	first := versionSub("a", "first", "charts", "p/2", "p/1", "p/1", "")
	second := versionSub("b", "second", "charts", "p/2", "p/3")
	third := versionSub("c", "pending", "charts")
	other := versionSub("c", "other", "elsewhere", "p/4")
	// v0.14 selects by kind/namespace/name even for a nonstandard apiVersion.
	second.Object["spec"].(map[string]any)["feeds"].([]any)[0].(map[string]any)["apiVersion"] = "example.test/v2"
	client, e := versionExplorer(chart, other, third, second, first)
	ctx := context.Background()
	impact, err := e.HelmChartVersionImpact(ctx, "charts", "web")
	if err != nil {
		t.Fatal(err)
	}
	if impact.BoundTargets != 3 || !reflect.DeepEqual(impact.Targets, []string{"p/1", "p/2", "p/3"}) ||
		!reflect.DeepEqual(impact.Subscriptions, []ChartVersionSubscription{{"a", "first", 2}, {"b", "second", 2}, {"c", "pending", 0}}) {
		t.Fatalf("incorrect targets or subscriptions: %+v", impact)
	}
	// A status update and reordering the same binding set do not change scope.
	first.SetResourceVersion("22")
	_ = unstructured.SetNestedStringSlice(first.Object, []string{"p/1", "p/2"}, "status", "bindingClusters")
	_ = unstructured.SetNestedField(first.Object, int64(2), "status", "completedReleases")
	if err := client.Tracker().Update(subscriptionGVR, first, "a"); err != nil {
		t.Fatal(err)
	}
	chart.SetResourceVersion("18")
	if err := client.Tracker().Update(helmChartGVR, chart, "charts"); err != nil {
		t.Fatal(err)
	}
	patches := 0
	client.PrependReactor("patch", "helmcharts", func(action clienttesting.Action) (bool, runtime.Object, error) {
		patches++
		var patch map[string]any
		if err := json.Unmarshal(action.(clienttesting.PatchAction).GetPatch(), &patch); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"metadata": map[string]any{"resourceVersion": "18"}, "spec": map[string]any{"version": "v2.0.0-rc.1+ci.10"}}
		if !reflect.DeepEqual(patch, want) {
			t.Fatalf("unexpected write: %v", patch)
		}
		return false, nil, nil
	})
	change := func() (string, error) {
		return e.ChangeHelmChartVersion(ctx, "charts", "web", impact.UID, impact.CurrentVersion, "v2.0.0-rc.1+ci.10", impact.Fingerprint)
	}
	if got, err := change(); err != nil || got != "v2.0.0-rc.1+ci.10" {
		t.Fatalf("change: got %q, err %v", got, err)
	}
	if _, err := change(); !apierrors.IsConflict(err) || patches != 1 {
		t.Fatalf("replayed request should require a new preview, patches=%d err=%v", patches, err)
	}
}

func TestChartVersionRejectsStaleScope(t *testing.T) {
	for _, scenario := range []string{"different bound ID", "new subscription", "changed selector", "recreated chart", "version changed", "list forbidden", "conflict during patch"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			sub, chart := versionSub("a", "first", "charts", "p/1"), versionChart()
			client, e := versionExplorer(chart, sub)
			impact, err := e.HelmChartVersionImpact(ctx, "charts", "web")
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "different bound ID":
				_ = unstructured.SetNestedStringSlice(sub.Object, []string{"p/2"}, "status", "bindingClusters")
			case "new subscription":
				err = client.Tracker().Add(versionSub("b", "first", "charts", "p/1"))
			case "changed selector":
				_ = unstructured.SetNestedSlice(sub.Object, []any{map[string]any{"clusterAffinity": map[string]any{"matchLabels": map[string]any{"region": "west"}}}}, "spec", "subscribers")
			case "recreated chart":
				chart.SetUID("new-identity")
			case "version changed":
				_ = unstructured.SetNestedField(chart.Object, "1.5.0", "spec", "version")
			case "list forbidden":
				client.PrependReactor("list", "subscriptions", func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewForbidden(subscriptionGVR.GroupResource(), "", errors.New("forbidden"))
				})
			case "conflict during patch":
				client.PrependReactor("patch", "helmcharts", func(clienttesting.Action) (bool, runtime.Object, error) {
					_ = unstructured.SetNestedStringSlice(sub.Object, []string{"p/2"}, "status", "bindingClusters")
					if err := client.Tracker().Update(subscriptionGVR, sub, "a"); err != nil {
						t.Fatal(err)
					}
					return true, nil, apierrors.NewConflict(helmChartGVR.GroupResource(), "web", errors.New("status changed"))
				})
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := client.Tracker().Update(subscriptionGVR, sub, "a"); err != nil {
				t.Fatal(err)
			}
			if err := client.Tracker().Update(helmChartGVR, chart, "charts"); err != nil {
				t.Fatal(err)
			}
			_, err = e.ChangeHelmChartVersion(ctx, "charts", "web", impact.UID, impact.CurrentVersion, "2.0.0", impact.Fingerprint)
			if scenario == "list forbidden" {
				if !apierrors.IsForbidden(err) {
					t.Fatalf("unexpected error: %v", err)
				}
			} else if !apierrors.IsConflict(err) {
				t.Fatalf("expected conflict, got %v", err)
			}
			patches := 0
			for _, action := range client.Actions() {
				if action.GetVerb() == "patch" {
					patches++
				}
			}
			if patches != 0 && !(scenario == "conflict during patch" && patches == 1) {
				t.Fatalf("unexpected writes: %d", patches)
			}
		})
	}
}

func TestChartVersionRequiresDifferentExactVersion(t *testing.T) {
	e := NewExplorer(nil, nil, nil)
	for _, value := range []string{"", "latest", "1.2", "^2.3.0", "1.2.x", "1.2.3", "v1.2.3", " 1.2.4", "1.2.4\n", "01.0.1", "2.0.0-01", "1.2.5+foo.", "vv2.0.1"} {
		if _, err := e.ChangeHelmChartVersion(context.Background(), "charts", "web", "uid", "1.2.3", value,
			"0000000000000000000000000000000000000000000000000000000000000000"); !apierrors.IsBadRequest(err) {
			t.Errorf("accepted %q, err=%v", value, err)
		}
	}
	for _, value := range []string{"0.0.0", "1.2.3-0", "1.2.3-01a", "v1.2.4", "1.2.3-rc.1+build.01"} {
		if !exactChartVersion.MatchString(value) {
			t.Errorf("rejected exact version %q", value)
		}
	}
}

func TestChartVersionCanPinUnversionedUnreferencedChart(t *testing.T) {
	chart := versionChart()
	unstructured.RemoveNestedField(chart.Object, "spec", "version")
	_, e := versionExplorer(chart)
	impact, err := e.HelmChartVersionImpact(context.Background(), "charts", "web")
	if err != nil || impact.CurrentVersion != "" || impact.Targets == nil || impact.Subscriptions == nil || impact.BoundTargets != 0 {
		t.Fatalf("unexpected preview: %+v, err %v", impact, err)
	}
	got, err := e.ChangeHelmChartVersion(context.Background(), "charts", "web", impact.UID, "", "1.0.0", impact.Fingerprint)
	if got != "1.0.0" || err != nil {
		t.Fatalf("pin failed: %q, %v", got, err)
	}
}
