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

func TestStaticFilesCannotEscapeStaticDir(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "static")
	if err := os.Mkdir(staticDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := New(config.Config{BasePath: "/clusternet", StaticDir: staticDir})
	req := httptest.NewRequest(http.MethodGet, "/clusternet/../secret.txt", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if strings.Contains(res.Body.String(), "secret") {
		t.Fatalf("static handler leaked file outside static dir: %s", res.Body.String())
	}
}
