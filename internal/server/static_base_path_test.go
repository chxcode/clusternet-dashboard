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

func TestDeepLinkFindsBuiltAssetsAtRuntimeMount(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<link href="./assets/app.css"><script src="./assets/app.js"></script>`), 0644); err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{"", "/tools/fleet"} {
		res := workspaceGET(New(config.Config{BasePath: base, StaticDir: dir}), base+"/clusters?q=edge")
		if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `href="`+base+`/assets/app.css"`) ||
			!strings.Contains(res.Body.String(), `src="`+base+`/assets/app.js"`) {
			t.Errorf("mount %q: %s", base, res.Body.String())
		}
	}
}
