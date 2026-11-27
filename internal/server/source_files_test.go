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
	kt "k8s.io/client-go/testing"
)

func TestChartSourceGuardAndCreate(t *testing.T) {
	apps := schema.GroupVersion{Group: "apps.clusternet.io", Version: "v1alpha1"}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		apps.WithResource("subscriptions"): "SubscriptionList",
	})
	client.PrependReactor("create", "helmcharts", func(a kt.Action) (bool, runtime.Object, error) {
		obj := a.(kt.CreateAction).GetObject().(*unstructured.Unstructured)
		obj.SetUID("created-uid")
		obj.SetResourceVersion("3")
		return false, nil, nil
	})
	explorer := WithExplorer(kube.NewExplorer(nil, nil, client))
	cfg := config.Config{BasePath: "/dash"}
	off := New(cfg, explorer, WithChartDeployment(true))
	on := New(cfg, explorer, WithChartSourceEditing(true))
	post := func(h http.Handler, path, origin, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "http://localhost/dash/api/helmcharts/apps/web/"+path, strings.NewReader(body))
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		return res
	}
	const origin = "http://localhost"
	for _, tc := range []struct {
		h                  http.Handler
		path, origin, body string
		code               int
	}{
		{off, "source", origin, "{}", 403}, {off, "source-save", origin, "{}", 403}, {off, "source-preview", origin, "{}", 403},
		{on, "source", "https://elsewhere.invalid", "{}", 403}, {on, "source-preview", origin, `{"namespace":"other"}`, 400},
		{on, "source-save", origin, `{"name":"other"}`, 400}, {on, "source-preview", origin, `{"secret":"extra"}`, 400},
	} {
		if res := post(tc.h, tc.path, tc.origin, tc.body); res.Code != tc.code {
			t.Fatalf("%s: got %d want %d", tc.path, res.Code, tc.code)
		}
	}
	if len(client.Actions()) != 0 {
		t.Fatal("rejected requests reached Kubernetes")
	}
	payload := map[string]string{"mode": "create", "yaml": "apiVersion: apps.clusternet.io/v1alpha1\nkind: HelmChart\nmetadata: {namespace: apps, name: web}\nspec: {repo: https://charts.example.com, chart: web, version: 1.0.0, targetNamespace: shop}\n"}
	body, _ := json.Marshal(payload)
	plan := post(on, "source-preview", origin, string(body))
	var p kube.SourceFilePreview
	if plan.Code != 200 || json.Unmarshal(plan.Body.Bytes(), &p) != nil || p.Fingerprint == "" {
		t.Fatalf("preview: %d %s", plan.Code, plan.Body.String())
	}
	for _, a := range client.Actions() {
		if a.GetVerb() != "get" && a.GetVerb() != "list" {
			t.Fatalf("preview used %s", a.GetVerb())
		}
	}
	payload["fingerprint"] = p.Fingerprint
	body, _ = json.Marshal(payload)
	if res := post(on, "source-save", origin, string(body)); res.Code != 201 {
		t.Fatalf("save: %d %s", res.Code, res.Body.String())
	}
	doc := post(on, "source", origin, "{}")
	var d kube.SourceFileDocument
	if doc.Code != 200 || json.Unmarshal(doc.Body.Bytes(), &d) != nil || d.UID != "created-uid" || !strings.Contains(d.YAML, "charts.example.com") {
		t.Fatalf("source doc: %d %s", doc.Code, doc.Body.String())
	}
	if res := post(on, "source-save", origin, string(body)); res.Code != 409 {
		t.Fatalf("replay: %d", res.Code)
	}
}
