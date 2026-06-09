package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/chxcode/clusternet-dashboard/internal/config"
	"github.com/chxcode/clusternet-dashboard/internal/kube"
)

func TestDiagnosticsEndpointReturnsCompatibilityAndComponents(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "apps.clusternet.io", Version: "v1alpha1", Resource: "subscriptions"}: "SubscriptionList",
	}, &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.clusternet.io/v1alpha1",
		"kind":       "Subscription",
		"metadata":  map[string]any{"name": "demo", "namespace": "default"},
		"status":    map[string]any{"desiredReleases": int64(1), "completedReleases": int64(1)},
	}})
	client := k8sfake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "clusternet-hub", Namespace: "clusternet-system", Labels: map[string]string{"app": "clusternet-hub"}},
		Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "hub", Image: "ghcr.io/clusternet/clusternet-hub:v0.18.1"}}}}},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1, Replicas: 1},
	})
	discoveryClient := &discoveryfake.FakeDiscovery{Fake: &client.Fake}
	explorer := kube.NewExplorer(client, discoveryClient, dynamicClient)
	handler := New(config.Config{BasePath: "/clusternet"}, WithExplorer(explorer))

	req := httptest.NewRequest(http.MethodGet, "/clusternet/api/diagnostics", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if !strings.Contains(body, `"completedReleasesSupport":"supported"`) || !strings.Contains(body, `"image":"ghcr.io/clusternet/clusternet-hub:v0.18.1"`) {
		t.Fatalf("expected diagnostics payload, got %s", body)
	}
}
