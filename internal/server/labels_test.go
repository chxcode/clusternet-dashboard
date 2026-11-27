package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chxcode/clusternet-dashboard/internal/config"
	"github.com/chxcode/clusternet-dashboard/internal/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

const labelChanges = `{"uid":"object-uid","changes":[{"key":"example.com/rack","before":"old","after":"new"},{"key":"delete","before":"","after":null},{"key":"fresh","before":null,"after":""}]}`

func labelsFixture() (*fake.FakeDynamicClient, *kube.Explorer) {
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "clusters.clusternet.io/v1beta1", "kind": "ManagedCluster",
		"metadata": map[string]any{"name": "edge", "namespace": "test", "uid": "object-uid", "resourceVersion": "10",
			"labels": map[string]any{"example.com/rack": "old", "delete": "", "keep": "original"}},
		"spec": map[string]any{"clusterId": "registration-id"},
	}})
	return client, kube.NewExplorer(nil, nil, client)
}

func requestLabels(h http.Handler, body, host, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("PATCH", "http://"+host+"/dash/api/clusters/test/edge/labels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", origin)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, req)
	return response
}

func TestLabelWriteBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name               string
		enabled            bool
		host, origin, body string
		want               int
	}{
		{"disabled", false, "localhost", "http://localhost", labelChanges, 403},
		{"foreign origin", true, "localhost", "http://example.org", labelChanges, 403},
		{"untrusted host", true, "example.org", "http://example.org", labelChanges, 403},
		{"no origin", true, "localhost", "", labelChanges, 403},
		{"outside labels", true, "localhost", "http://localhost", `{"uid":"object-uid","spec":{}}`, 400},
		{"missing after", true, "localhost", "http://localhost", `{"uid":"object-uid","changes":[{"key":"delete","before":""}]}`, 400},
		{"invalid key", true, "localhost", "http://localhost", strings.ReplaceAll(labelChanges, "example.com/rack", "invalid//rack"), 400},
		{"invalid value", true, "localhost", "http://localhost", strings.ReplaceAll(labelChanges, `"new"`, `"bad value"`), 400},
		{"duplicate key", true, "localhost", "http://localhost", strings.ReplaceAll(labelChanges, `"fresh"`, `"delete"`), 400},
		{"replaced object", true, "localhost", "http://localhost", strings.ReplaceAll(labelChanges, "object-uid", "previous-uid"), 409},
		{"edited concurrently", true, "localhost", "http://localhost", strings.ReplaceAll(labelChanges, `"old"`, `"stale"`), 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, explorer := labelsFixture()
			h := New(config.Config{BasePath: "/dash"}, WithExplorer(explorer), WithLabelEditing(tc.enabled))
			response := requestLabels(h, tc.body, tc.host, tc.origin)
			if response.Code != tc.want {
				t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
			}
			for _, action := range client.Actions() {
				if action.GetVerb() == "patch" {
					t.Fatal("invalid request touched cluster labels")
				}
			}
		})
	}
}

func TestLabelChangesAtomicAndRetryHeartbeat(t *testing.T) {
	client, explorer := labelsFixture()
	gvr := schema.GroupVersionResource{Group: "clusters.clusternet.io", Version: "v1beta1", Resource: "managedclusters"}
	patches := 0
	client.PrependReactor("patch", "managedclusters", func(action clienttesting.Action) (bool, runtime.Object, error) {
		patches++
		var patch map[string]map[string]json.RawMessage
		if err := json.Unmarshal(action.(clienttesting.PatchAction).GetPatch(), &patch); err != nil {
			t.Fatal(err)
		}
		meta := patch["metadata"]
		if len(patch) != 1 || len(meta) != 2 || len(meta["resourceVersion"]) == 0 || len(meta["labels"]) == 0 {
			t.Fatal("patch must only set labels and guard resourceVersion")
		}
		if patches == 1 {
			obj, err := client.Tracker().Get(gvr, "test", "edge")
			if err != nil {
				t.Fatal(err)
			}
			cluster := obj.(*unstructured.Unstructured)
			cluster.SetResourceVersion("11")
			labels := cluster.GetLabels()
			labels["keep"] = "updated-concurrently"
			cluster.SetLabels(labels)
			if err := client.Tracker().Update(gvr, cluster, "test"); err != nil {
				t.Fatal(err)
			}
			return true, nil, apierrors.NewConflict(gvr.GroupResource(), "edge", errors.New("stale resourceVersion"))
		}
		if string(meta["resourceVersion"]) != `"11"` {
			t.Fatal("retry did not read current version")
		}
		return false, nil, nil
	})
	h := New(config.Config{BasePath: "/dash"}, WithExplorer(explorer), WithLabelEditing(true, "dashboard.example.test"))
	for i := 0; i < 2; i++ {
		response := requestLabels(h, labelChanges, "dashboard.example.test", "https://dashboard.example.test")
		if response.Code != 200 {
			t.Fatalf("save failed: %d %s", response.Code, response.Body.String())
		}
	}
	if patches != 2 {
		t.Fatalf("expected one heartbeat conflict, one successful patch and idempotent retry, got %d patches", patches)
	}
	obj, err := client.Resource(gvr).Namespace("test").Get(context.Background(), "edge", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	labels := obj.GetLabels()
	if len(labels) != 3 || labels["example.com/rack"] != "new" || labels["keep"] != "updated-concurrently" {
		t.Fatalf("incorrect labels: %v", labels)
	}
	if value, ok := labels["fresh"]; !ok || value != "" {
		t.Fatal("expected explicitly empty value")
	}
}
