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

func TestHelmReleasesEndpointReturnsReleaseStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{helmReleaseGVRForTest: "HelmReleaseList"}, &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.clusternet.io/v1alpha1",
		"kind":       "HelmRelease",
		"metadata": map[string]any{
			"name":      "demo",
			"namespace": "child-a",
		},
		"status": map[string]any{"phase": "failed", "description": "install failed"},
	}})
	handler := New(config.Config{BasePath: "/clusternet"}, WithExplorer(kube.NewExplorer(nil, nil, dynamicClient)))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/clusternet/api/helmreleases", nil)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "\"helmReleases\"") || !strings.Contains(body, "failed") || !strings.Contains(body, "install failed") {
		t.Fatalf("unexpected body: %s", body)
	}
}

var helmReleaseGVRForTest = schema.GroupVersionResource{Group: "apps.clusternet.io", Version: "v1alpha1", Resource: "helmreleases"}
