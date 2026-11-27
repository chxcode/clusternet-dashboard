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
)

func TestChartVersionHTTP(t *testing.T) {
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		{Group: "apps.clusternet.io", Version: "v1alpha1", Resource: "subscriptions"}: "SubscriptionList",
	}, &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.clusternet.io/v1alpha1", "kind": "HelmChart",
		"metadata": map[string]any{"name": "web", "namespace": "charts", "uid": "chart-uid", "resourceVersion": "10"},
		"spec":     map[string]any{"chart": "web"},
	}})
	explorer := WithExplorer(kube.NewExplorer(nil, nil, client))
	cfg := config.Config{BasePath: "/dash"}
	disabled := New(cfg, explorer, WithLabelEditing(true), WithReleaseOperations(true, false, kube.TriageStore{}))
	enabled := New(cfg, explorer, WithReleaseOperations(false, true, kube.TriageStore{}))
	request := func(h http.Handler, method, path, origin, contentType, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://localhost/dash/api/helmcharts/"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	const preview = "charts/web/version-impact"
	const save = "charts/web/version"
	for _, tc := range []struct {
		h                                       http.Handler
		method, path, origin, contentType, body string
		want                                    int
	}{
		{disabled, "POST", save, "http://localhost", "application/json", "{}", 403},
		{enabled, "GET", "INVALID/web/version-impact", "", "", "", 400},
		{enabled, "POST", save, "http://external.test", "application/json", "{}", 403},
		{enabled, "POST", save, "http://localhost", "text/plain", "{}", 415},
		{enabled, "POST", save, "http://localhost", "application/json", `{"uid":"chart-uid","newVersion":"2.0.0"}`, 400},
		{enabled, "POST", save, "http://localhost", "application/json", `{"currentVersion":"","spec":{}}`, 400},
		{enabled, "POST", save, "http://localhost", "application/json", `{"currentVersion":""}{}`, 400},
	} {
		got := request(tc.h, tc.method, tc.path, tc.origin, tc.contentType, tc.body)
		if got.Code != tc.want {
			t.Fatalf("%s %s: got %d, want %d", tc.method, tc.path, got.Code, tc.want)
		}
	}
	if len(client.Actions()) != 0 {
		t.Fatal("rejected requests reached Kubernetes")
	}
	got := request(disabled, "GET", preview, "", "", "")
	var impact kube.ChartVersionImpact
	if got.Code != http.StatusOK || got.Header().Get("Cache-Control") != "no-store" || json.Unmarshal(got.Body.Bytes(), &impact) != nil || impact.UID != "chart-uid" || impact.Fingerprint == "" {
		t.Fatalf("preview: %d %s", got.Code, got.Body.String())
	}
	payload, err := json.Marshal(map[string]string{"uid": impact.UID, "fingerprint": impact.Fingerprint, "currentVersion": "", "newVersion": "2.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	got = request(enabled, "POST", save, "http://localhost", "application/json", string(payload))
	if got.Code != http.StatusOK || strings.TrimSpace(got.Body.String()) != `{"version":"2.0.0"}` {
		t.Fatalf("save: %d %s", got.Code, got.Body.String())
	}
	got = request(enabled, "POST", save, "http://localhost", "application/json", string(payload))
	if got.Code != http.StatusConflict {
		t.Fatalf("duplicate save: %d %s", got.Code, got.Body.String())
	}
}
