package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chxcode/clusternet-dashboard/internal/config"
	"github.com/chxcode/clusternet-dashboard/internal/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

func TestFailedReleaseDeletionHTTP(t *testing.T) {
	const gv = "apps.clusternet.io/v1alpha1"
	const path = "/dash/api/helmreleases/edge/setup-charts-web"
	const payload = `{"uid":"instance","resourceVersion":"3"}`
	for _, tc := range []struct {
		name                      string
		enabled                   bool
		host, origin, media, body string
		want                      int
	}{
		{"other operations enabled", false, "localhost", "http://localhost", "application/json", payload, 403},
		{"different origin", true, "localhost", "https://foreign.test", "application/json", payload, 403},
		{"label-only host", true, "labels.test", "https://labels.test", "application/json", payload, 403},
		{"content type", true, "localhost", "http://localhost", "text/plain", payload, 415},
		{"extra JSON", true, "localhost", "http://localhost", "application/json", payload + "{}", 400},
		{"extra parameter", true, "localhost", "http://localhost", "application/json", `{"uid":"instance","force":true,"resourceVersion":"3"}`, 400},
		{"missing version", true, "localhost", "http://localhost", "application/json", `{"uid":"instance"}`, 400},
		{"stale", true, "localhost", "http://localhost", "application/json", `{"uid":"previous","resourceVersion":"3"}`, 409},
		{"accepted", true, "ops.test", "https://ops.test", "application/json", payload, 202},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewSimpleDynamicClient(runtime.NewScheme(), &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": gv, "kind": "HelmRelease", "metadata": map[string]any{"namespace": "edge", "name": "setup-charts-web", "uid": "instance", "resourceVersion": "3", "ownerReferences": []any{
					map[string]any{"apiVersion": gv, "kind": "Description", "name": "setup", "uid": "controller", "controller": true},
				}}, "status": map[string]any{"phase": "Failed"},
			}}, &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": gv, "kind": "Description", "metadata": map[string]any{"namespace": "edge", "name": "setup", "uid": "controller"}, "spec": map[string]any{"deployer": "Helm", "charts": []any{map[string]any{"namespace": "charts", "name": "web"}}},
			}})
			h := New(config.Config{BasePath: "/dash"}, WithExplorer(kube.NewExplorer(nil, nil, client)), WithLabelEditing(true, "labels.test"), WithReleaseOperations(true, true, kube.TriageStore{}, "ops.test"), WithReleaseDeletion(tc.enabled))
			request := httptest.NewRequest(http.MethodDelete, "http://"+tc.host+path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", tc.media)
			request.Header.Set("Origin", tc.origin)
			res := httptest.NewRecorder()
			h.ServeHTTP(res, request)
			if res.Code != tc.want {
				t.Fatalf("got %d (%s), want %d", res.Code, res.Body.String(), tc.want)
			}
			if tc.want == http.StatusAccepted && (res.Header().Get("Content-Type") != "application/json" || strings.TrimSpace(res.Body.String()) != `{"status":"deletionRequested"}`) {
				t.Fatalf("unexpected accepted response: %s", res.Body.String())
			}
			for _, action := range client.Actions() {
				if action.GetVerb() != "get" && tc.want != http.StatusAccepted {
					t.Fatalf("rejected request attempted %s", action.GetVerb())
				}
			}
		})
	}
}
