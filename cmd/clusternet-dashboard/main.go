package main

import (
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"

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

	explorer, connection, err := newExplorer()
	if err != nil {
		slog.Warn("kubernetes client unavailable; API endpoints will return empty MVP responses", "error", err)
	}

	editLabels := os.Getenv("ENABLE_LABEL_EDITING") == "true"
	triage := os.Getenv("ENABLE_RELEASE_TRIAGE") == "true"
	editCharts := os.Getenv("ENABLE_CHART_VERSION_EDITING") == "true"
	deleteReleases := os.Getenv("ENABLE_FAILED_RELEASE_DELETION") == "true"
	editSource := os.Getenv("ENABLE_CHART_SOURCE_EDITING") == "true"
	nativePublishing := os.Getenv("ENABLE_NATIVE_PUBLISHING") == "true"
	registrationDelete := os.Getenv("ENABLE_FAILED_REGISTRATION_DELETION") == "true"
	deployCharts := os.Getenv("ENABLE_CHART_DEPLOYMENT") == "true"
	store := kube.TriageStore{Namespace: os.Getenv("TRIAGE_NAMESPACE"), Name: os.Getenv("TRIAGE_CONFIGMAP"), CreateOnFirstWrite: !connection.InCluster}
	if store.Namespace == "" {
		store.Namespace = "clusternet-system"
	}
	if store.Name == "" {
		store.Name = "clusternet-dashboard-triage"
	}
	host := os.Getenv("LISTEN_HOST")
	if (editLabels || triage || editCharts || deleteReleases || deployCharts || editSource || nativePublishing || registrationDelete) && !connection.InCluster && host == "" {
		host = "127.0.0.1"
	}
	addr := net.JoinHostPort(host, cfg.Port)
	slog.Info("starting clusternet dashboard", "addr", addr, "basePath", cfg.BasePath)

	handler := server.New(cfg, server.WithExplorer(explorer),
		server.WithParent(connection, os.Getenv("PARENT_CLUSTER_NAME")),
		server.WithReleaseDeletion(deleteReleases),
		server.WithNativePublishing(nativePublishing), server.WithFailedRegistrationDeletion(registrationDelete),
		server.WithChartDeployment(deployCharts), server.WithChartSourceEditing(editSource),
		server.WithLabelEditing(editLabels, strings.Split(os.Getenv("LABEL_EDITING_ALLOWED_HOSTS"), ",")...),
		server.WithReleaseOperations(triage, editCharts, store, strings.Split(os.Getenv("WORKSPACE_WRITE_ALLOWED_HOSTS"), ",")...))
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func newExplorer() (*kube.Explorer, kube.ParentConnection, error) {
	cfg, connection, err := kube.NewRestConfig()
	if err != nil {
		return nil, connection, err
	}
	explorer, err := kube.NewExplorerFromConfig(cfg)
	return explorer, connection, err
}
