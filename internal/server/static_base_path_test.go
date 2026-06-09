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

func TestServedIndexInjectsConfiguredBasePath(t *testing.T) {
	staticDir := t.TempDir()
	index := `<script>window.__CLUSTERNET_DASHBOARD_BASE_PATH__ = "%BASE_PATH%";</script>`
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := New(config.Config{BasePath: "/clusternet", StaticDir: staticDir})
	req := httptest.NewRequest(http.MethodGet, "/clusternet/", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	body := res.Body.String()
	if strings.Contains(body, "%BASE_PATH%") {
		t.Fatalf("expected placeholder to be replaced, got %s", body)
	}
	if !strings.Contains(body, `window.__CLUSTERNET_DASHBOARD_BASE_PATH__ = "/clusternet"`) {
		t.Fatalf("expected configured base path in index, got %s", body)
	}
}
