package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chxcode/clusternet-dashboard/internal/config"
)

func TestHealthEndpointServedUnderBasePath(t *testing.T) {
	handler := New(config.Config{BasePath: "/clusternet"})
	req := httptest.NewRequest(http.MethodGet, "/clusternet/api/health", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), `"status":"ok"`) {
		t.Fatalf("expected health response, got %s", res.Body.String())
	}
}

func TestEndpointOutsideBasePathReturnsNotFound(t *testing.T) {
	handler := New(config.Config{BasePath: "/clusternet"})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", res.Code)
	}
}

func TestClustersEndpointReturnsEmptyListForMVP(t *testing.T) {
	handler := New(config.Config{BasePath: "/clusternet"})
	req := httptest.NewRequest(http.MethodGet, "/clusternet/api/clusters", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), `"clusters":[]`) {
		t.Fatalf("expected empty clusters response, got %s", res.Body.String())
	}
}
