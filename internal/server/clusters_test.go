package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"

	"github.com/chxcode/clusternet-dashboard/internal/config"
	"github.com/chxcode/clusternet-dashboard/internal/kube"
)

func TestClustersEndpointReturnsManagedClustersWithOnlineStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "clusters.clusternet.io", Version: "v1beta1", Resource: "managedclusters"}
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "clusters.clusternet.io/v1beta1",
		"kind":       "ManagedCluster",
		"metadata": map[string]any{
			"name":      "child-a",
			"namespace": "clusternet-system",
		},
		"status": map[string]any{"readyz": true},
	}}
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "ManagedClusterList",
	}, cluster)
	explorer := kube.NewExplorer(nil, nil, dynamicClient)
	handler := New(config.Config{BasePath: "/clusternet"}, WithExplorer(explorer))

	req := httptest.NewRequest(http.MethodGet, "/clusternet/api/clusters", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), `"name":"child-a"`) {
		t.Fatalf("expected managed cluster name, got %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"online":true`) {
		t.Fatalf("expected online cluster, got %s", res.Body.String())
	}
}
