package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/chxcode/clusternet-dashboard/internal/config"
	"github.com/chxcode/clusternet-dashboard/internal/kube"
	"github.com/chxcode/clusternet-dashboard/internal/server"
)

func main() {
	cfg := config.LoadFromEnv(map[string]string{
		"BASE_PATH":  os.Getenv("BASE_PATH"),
		"PORT":       os.Getenv("PORT"),
		"STATIC_DIR": os.Getenv("STATIC_DIR"),
	})

	explorer, err := newExplorer()
	if err != nil {
		slog.Warn("kubernetes client unavailable; API endpoints will return empty MVP responses", "error", err)
	}

	addr := ":" + cfg.Port
	slog.Info("starting clusternet dashboard", "addr", addr, "basePath", cfg.BasePath)

	if err := http.ListenAndServe(addr, server.New(cfg, server.WithExplorer(explorer))); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func newExplorer() (*kube.Explorer, error) {
	cfg, err := kube.NewRestConfig()
	if err != nil {
		return nil, err
	}
	return kube.NewExplorerFromConfig(cfg)
}
