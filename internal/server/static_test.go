package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chxcode/clusternet-dashboard/internal/config"
)

func TestServesIndexUnderBasePath(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<div>dashboard</div>"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := New(config.Config{BasePath: "/clusternet", StaticDir: staticDir})
	req := httptest.NewRequest(http.MethodGet, "/clusternet/", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "dashboard") {
		t.Fatalf("expected index content, got %s", res.Body.String())
	}
}

func TestSPAFallbackUnderBasePath(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<div>dashboard</div>"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := New(config.Config{BasePath: "/clusternet", StaticDir: staticDir})
	req := httptest.NewRequest(http.MethodGet, "/clusternet/clusters", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "dashboard") {
		t.Fatalf("expected index content, got %s", res.Body.String())
	}
}
