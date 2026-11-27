package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chxcode/clusternet-dashboard/internal/config"
	"github.com/chxcode/clusternet-dashboard/internal/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestChartDeploymentHTTP(t *testing.T) {
	apps := schema.GroupVersion{Group: "apps.clusternet.io", Version: "v1alpha1"}
	clusters := schema.GroupVersion{Group: "clusters.clusternet.io", Version: "v1beta1"}
	const clusterID = "fdae5f8c-c5de-42f0-a0bf-1d21b49272e3"
	chart := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apps.String(), "kind": "HelmChart",
		"metadata": map[string]any{"name": "web", "namespace": "sources", "uid": "chart-uid", "resourceVersion": "10", "generation": int64(1)},
		"spec":     map[string]any{"chart": "web", "targetNamespace": "workloads", "version": "1.2.3"},
		"status":   map[string]any{"phase": "Found"},
	}}
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": clusters.String(), "kind": "ManagedCluster",
		"metadata": map[string]any{"name": "edge", "namespace": "region", "uid": "cluster-uid", "resourceVersion": "2",
			"labels": map[string]any{"clusters.clusternet.io/cluster-id": clusterID}},
		"spec": map[string]any{"clusterId": clusterID},
	}}
	namespace := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Namespace",
		"metadata": map[string]any{"name": "region", "labels": map[string]any{"clusters.clusternet.io/cluster-id": clusterID}},
	}}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		apps.WithResource("subscriptions"): "SubscriptionList", apps.WithResource("helmcharts"): "HelmChartList",
		apps.WithResource("bases"): "BaseList", apps.WithResource("helmreleases"): "HelmReleaseList",
		{Version: "v1", Resource: "namespaces"}:  "NamespaceList",
		clusters.WithResource("managedclusters"): "ManagedClusterList",
	}, chart, cluster, namespace)
	// The production API server assigns the UID on creation.
	client.PrependReactor("create", "subscriptions", func(action clienttesting.Action) (bool, runtime.Object, error) {
		action.(clienttesting.CreateAction).GetObject().(*unstructured.Unstructured).SetUID("created-uid")
		return false, nil, nil
	})
	cfg := config.Config{BasePath: "/dash"}
	explorer := WithExplorer(kube.NewExplorer(nil, nil, client))
	disabled := New(cfg, explorer, WithLabelEditing(true), WithReleaseOperations(true, true, kube.TriageStore{}))
	enabled := New(cfg, explorer, WithChartDeployment(true))
	const prefix = "/dash/api/helmcharts/sources/web/"
	post := func(h http.Handler, path, origin, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://localhost"+prefix+path, strings.NewReader(body))
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	const preview, create = "deployment-preview", "deployments"
	const origin = "http://localhost"
	const input = `{"name":"test-app","targets":[{"namespace":"region","name":"edge","uid":"cluster-uid"}]}`
	for _, tc := range []struct {
		h                http.Handler
		path, host, body string
		code             int
	}{
		{disabled, preview, origin, input, 403}, {disabled, create, origin, input, 403},
		{enabled, create, "https://other.test", input, 403},
		{enabled, create, origin, `{"metadata":{"name":"arbitrary"},"spec":{}}`, 400},
		{enabled, preview, origin, input + input, 400},
	} {
		if res := post(tc.h, tc.path, tc.host, tc.body); res.Code != tc.code {
			t.Fatalf("%s: got %d, want %d", tc.path, res.Code, tc.code)
		}
	}
	if len(client.Actions()) != 0 {
		t.Fatal("a rejected HTTP request reached Kubernetes")
	}
	res := post(enabled, preview, origin, input)
	var plan struct {
		Fingerprint string
		Conflicts   []string
	}
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &plan) != nil || plan.Fingerprint == "" || len(plan.Conflicts) != 0 {
		t.Fatalf("preview failed: %d %s", res.Code, res.Body.String())
	}
	for _, action := range client.Actions() {
		if action.GetVerb() != "get" && action.GetVerb() != "list" {
			t.Fatalf("preview used %s", action.GetVerb())
		}
	}
	body := strings.TrimSuffix(input, "}") + `,"fingerprint":"` + plan.Fingerprint + `"}`
	res = post(enabled, create, origin, body)
	var selected struct{ Name, Namespace, UID string }
	if res.Code != 201 || json.Unmarshal(res.Body.Bytes(), &selected) != nil || selected.Name != "test-app" || selected.Namespace != "sources" || selected.UID != "created-uid" {
		t.Fatalf("create failed: %d %s", res.Code, res.Body.String())
	}
	if res = post(enabled, create, origin, body); res.Code != 409 {
		t.Fatalf("replay should require a new preview, got %d", res.Code)
	}
}
