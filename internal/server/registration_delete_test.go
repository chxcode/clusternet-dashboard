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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestRegistrationWithdrawalHTTP(t *testing.T) {
	const path = "/dash/api/registrations/held"
	const body = `{"uid":"old","resourceVersion":"7","confirmName":"held"}`
	for _, s := range []struct {
		name                     string
		enabled                  bool
		origin, content, payload string
		want                     int
	}{
		{"disabled", false, "http://localhost", "application/json", body, 403},
		{"cross origin", true, "https://foreign.example", "application/json", body, 403},
		{"media type", true, "http://localhost", "text/plain", body, 415},
		{"extra payload", true, "http://localhost", "application/json", body + "{}", 400},
		{"unknown parameter", true, "http://localhost", "application/json", `{"uid":"old","resourceVersion":"7","confirmName":"held","force":true}`, 400},
		{"wrong typed name", true, "http://localhost", "application/json", `{"uid":"old","resourceVersion":"7","confirmName":"other"}`, 400},
		{"stale", true, "http://localhost", "application/json", `{"uid":"old","resourceVersion":"1","confirmName":"held"}`, 409},
		{"accepted", true, "http://localhost", "application/json", body, 202},
	} {
		t.Run(s.name, func(t *testing.T) {
			gv := schema.GroupVersion{Group: "clusters.clusternet.io", Version: "v1beta1"}
			c := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
				gv.WithResource("managedclusters"): "ManagedClusterList", {Version: "v1", Resource: "namespaces"}: "NamespaceList",
			}, &unstructured.Unstructured{Object: map[string]any{"apiVersion": gv.String(), "kind": "ClusterRegistrationRequest", "metadata": map[string]any{"name": "held", "uid": "old", "resourceVersion": "7"}, "spec": map[string]any{"clusterId": "aaaa0000-1234-abcd-0000-000000000001"}, "status": map[string]any{"result": "Failed", "errorMessage": "secret-marker"}}})
			h := New(config.Config{BasePath: "/dash"}, WithExplorer(kube.NewExplorer(nil, nil, c)), WithReleaseDeletion(true), WithFailedRegistrationDeletion(s.enabled))
			r := httptest.NewRequest(http.MethodDelete, "http://localhost"+path, strings.NewReader(s.payload))
			r.Header.Set("Content-Type", s.content)
			r.Header.Set("Origin", s.origin)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != s.want || strings.Contains(w.Body.String(), "secret-marker") {
				t.Fatalf("unexpected response %d %s", w.Code, w.Body.String())
			}
			if s.want == 202 && strings.TrimSpace(w.Body.String()) != `{"status":"deletionRequested"}` {
				t.Fatalf("unexpected accepted response %s", w.Body.String())
			}
			if s.want != 202 {
				for _, a := range c.Actions() {
					if a.GetVerb() == "delete" {
						t.Fatal("rejected request mutated the cluster")
					}
				}
			}
		})
	}
}
