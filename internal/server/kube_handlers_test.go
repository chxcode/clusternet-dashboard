package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/chxcode/clusternet-dashboard/internal/config"
	"github.com/chxcode/clusternet-dashboard/internal/kube"
)

func TestNamespacesEndpointReturnsKubernetesNamespaces(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	explorer := kube.NewExplorer(client, nil, nil)
	handler := New(config.Config{BasePath: "/clusternet"}, WithExplorer(explorer))

	req := httptest.NewRequest(http.MethodGet, "/clusternet/api/namespaces", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), `"name":"default"`) {
		t.Fatalf("expected namespace response, got %s", res.Body.String())
	}
}

func TestEventsEndpointReturnsKubernetesEvents(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-started", Namespace: "default"},
		Type:      corev1.EventTypeNormal,
		Reason:    "Started",
	})
	explorer := kube.NewExplorer(client, nil, nil)
	handler := New(config.Config{BasePath: "/clusternet"}, WithExplorer(explorer))

	req := httptest.NewRequest(http.MethodGet, "/clusternet/api/events", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), `"reason":"Started"`) {
		t.Fatalf("expected event response, got %s", res.Body.String())
	}
}
