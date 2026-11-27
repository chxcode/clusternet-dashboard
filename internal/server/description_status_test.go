package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/chxcode/clusternet-dashboard/internal/config"
	"github.com/chxcode/clusternet-dashboard/internal/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

func TestDescriptionStatusVerifiesUIDAndReturnsOnlyStatus(t *testing.T) {
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.clusternet.io/v1alpha1", "kind": "Description",
		"metadata": map[string]any{"namespace": "edge", "name": "example", "uid": "current"},
		"spec":     map[string]any{"raw": "private example"},
		"status":   map[string]any{"phase": "Failure", "reason": "install timed out", "extra": "unrelated"},
	}})
	handler := New(config.Config{BasePath: "/tools/console"}, WithExplorer(kube.NewExplorer(nil, nil, client)))
	path := "/tools/console/api/description-status?namespace=edge&name=example&uid="
	for uid, code := range map[string]int{"current": http.StatusOK, "previous": http.StatusGone, "": http.StatusBadRequest} {
		response := workspaceGET(handler, path+uid)
		if response.Code != code {
			t.Fatalf("uid %q: got HTTP %d, want %d", uid, response.Code, code)
		}
		if code == http.StatusOK {
			var result map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || len(result) != 2 || result["reason"] != "install timed out" || result["phase"] != "Failure" {
				t.Fatalf("unexpected status projection: %v", result)
			}
		}
	}
}
