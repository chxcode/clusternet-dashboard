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

func TestSubscriptionDetailEndpointReturnsTargetClusters(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "apps.clusternet.io", Version: "v1alpha1", Resource: "subscriptions"}:     "SubscriptionList",
		{Group: "clusters.clusternet.io", Version: "v1beta1", Resource: "managedclusters"}: "ManagedClusterList",
	},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apps.clusternet.io/v1alpha1",
			"kind":       "Subscription",
			"metadata":  map[string]any{"name": "demo-sub", "namespace": "default"},
			"status": map[string]any{
				"bindingClusters": []any{"clusters/child-a"},
				"aggregatedStatuses": []any{map[string]any{"feedStatusDetails": []any{
					map[string]any{"clusterName": "child-a", "replicaStatus": map[string]any{}},
				}}},
			},
		}},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "clusters.clusternet.io/v1beta1",
			"kind":       "ManagedCluster",
			"metadata":  map[string]any{"name": "child-a", "namespace": "clusters"},
			"status":    map[string]any{"readyz": true},
		}},
	)
	explorer := kube.NewExplorer(nil, nil, dynamicClient)
	handler := New(config.Config{BasePath: "/clusternet"}, WithExplorer(explorer))

	req := httptest.NewRequest(http.MethodGet, "/clusternet/api/subscriptions/default/demo-sub", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"targetClusters"`) || !strings.Contains(res.Body.String(), `"observed":true`) {
		t.Fatalf("expected target cluster detail, got %s", res.Body.String())
	}
}
