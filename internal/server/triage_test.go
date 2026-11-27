package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/chxcode/clusternet-dashboard/internal/config"
	"github.com/chxcode/clusternet-dashboard/internal/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientfake "k8s.io/client-go/kubernetes/fake"
)

const followUpPath = "/tools/dash/api/helmreleases/child/app/triage"
const followUpList = "/tools/dash/api/release-triage"

func TestTriageHTTPBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name               string
		enabled            bool
		host, origin, body string
		want               int
	}{
		{"disabled", false, "localhost", "http://localhost", "{}", 403},
		{"untrusted host", true, "other.example.test", "http://other.example.test", "{}", 403},
		{"label-only host", true, "labels.example.test", "http://labels.example.test", "{}", 403},
		{"release host reaches validation", true, "releases.example.test", "http://releases.example.test", "{}", 400},
		{"cross origin", true, "localhost", "http://other.example.test", "{}", 403},
		{"missing origin", true, "localhost", "", "{}", 403},
		{"unknown fields", true, "localhost", "http://localhost", `{"spec":{}}`, 400},
		{"invalid state", true, "localhost", "http://localhost", `{"uid":"id","incident":"one","before":"open","state":"deployed"}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := clientfake.NewSimpleClientset()
			d := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
			handler := New(config.Config{BasePath: "/tools/dash"}, WithExplorer(kube.NewExplorer(c, nil, d)), WithLabelEditing(true, "labels.example.test"),
				WithReleaseOperations(tc.enabled, true, kube.TriageStore{Namespace: "console", Name: "records", CreateOnFirstWrite: true}, "releases.example.test"))
			req := httptest.NewRequest(http.MethodPost, "http://"+tc.host+followUpPath, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
			if !tc.enabled {
				if got := workspaceGET(handler, followUpList); got.Code != http.StatusForbidden {
					t.Errorf("disabled list: %d", got.Code)
				}
			}
			if len(c.Actions()) != 0 || len(d.Actions()) != 0 {
				t.Fatalf("rejected request used cluster clients: %v / %v", c.Actions(), d.Actions())
			}
		})
	}
}

func TestTriageHTTPStoreRoundTrip(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.clusternet.io/v1alpha1", "kind": "HelmRelease",
		"metadata": map[string]any{"name": "app", "namespace": "child", "uid": "current"},
		"status":   map[string]any{"phase": "failed"},
	}}
	incident := kube.NewHelmReleaseSummary(obj).TriageIncident
	if incident == "" {
		t.Fatal("expected an incident on a failed release")
	}
	c := clientfake.NewSimpleClientset()
	d := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), obj)
	handler := New(config.Config{BasePath: "/tools/dash"}, WithExplorer(kube.NewExplorer(c, nil, d)),
		WithReleaseOperations(true, false, kube.TriageStore{Namespace: "console", Name: "records", CreateOnFirstWrite: true}))
	initial := workspaceGET(handler, followUpList)
	if initial.Code != http.StatusOK || strings.TrimSpace(initial.Body.String()) != "{}" || len(c.Actions()) != 1 || !c.Actions()[0].Matches("get", "configmaps") {
		t.Fatalf("initial GET = %d %s; actions %v", initial.Code, initial.Body.String(), c.Actions())
	}
	post := func(uid, before, state string) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(map[string]string{"uid": uid, "incident": incident, "before": before, "state": state})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1"+followUpPath, strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://127.0.0.1")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}
	if res := post("old-uid", "open", "investigating"); res.Code != http.StatusConflict {
		t.Fatalf("stale identity = %d", res.Code)
	}
	if res := post("current", "open", "investigating"); res.Code != http.StatusOK {
		t.Fatalf("save = %d %s", res.Code, res.Body.String())
	}
	if res := post("current", "open", "resolved"); res.Code != http.StatusConflict {
		t.Fatalf("stale state = %d", res.Code)
	}
	res := workspaceGET(handler, followUpList)
	var states map[string]string
	if res.Code != http.StatusOK || json.Unmarshal(res.Body.Bytes(), &states) != nil || !reflect.DeepEqual(states, map[string]string{incident: "investigating"}) {
		t.Fatalf("list = %d %s", res.Code, res.Body.String())
	}
}
