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

func TestSubscriptionsEndpointReturnsSubscriptions(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "apps.clusternet.io", Version: "v1alpha1", Resource: "subscriptions"}: "SubscriptionList",
	}, &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.clusternet.io/v1alpha1",
		"kind":       "Subscription",
		"metadata":  map[string]any{"name": "demo-sub", "namespace": "default"},
	}})
	explorer := kube.NewExplorer(nil, nil, dynamicClient)
	handler := New(config.Config{BasePath: "/clusternet"}, WithExplorer(explorer))

	req := httptest.NewRequest(http.MethodGet, "/clusternet/api/subscriptions", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), `"name":"demo-sub"`) {
		t.Fatalf("expected subscription response, got %s", res.Body.String())
	}
}

func TestManifestsEndpointReturnsManifests(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "apps.clusternet.io", Version: "v1alpha1", Resource: "manifests"}: "ManifestList",
	}, &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.clusternet.io/v1alpha1",
		"kind":       "Manifest",
		"metadata":  map[string]any{"name": "demo-manifest", "namespace": "default"},
		"template":  map[string]any{"kind": "Deployment"},
	}})
	explorer := kube.NewExplorer(nil, nil, dynamicClient)
	handler := New(config.Config{BasePath: "/clusternet"}, WithExplorer(explorer))

	req := httptest.NewRequest(http.MethodGet, "/clusternet/api/manifests", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), `"templateKind":"Deployment"`) {
		t.Fatalf("expected manifest response, got %s", res.Body.String())
	}
}
