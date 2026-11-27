package server

import (
	"encoding/json"
	"errors"
	"html"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/chxcode/clusternet-dashboard/internal/config"
	"github.com/chxcode/clusternet-dashboard/internal/kube"
)

type options struct {
	nativePublishing           bool
	failedRegistrationDeletion bool
	chartSourceEditing         bool
	chartDeployment            bool
	releaseDeletion            bool
	releaseTriage              bool
	chartVersionEditing        bool
	triageStore                kube.TriageStore
	labelEditing               bool
	labelHosts                 map[string]bool
	releaseHosts               map[string]bool
	explorer                   *kube.Explorer
	connection                 kube.ParentConnection
	displayName                string
}

type Option func(*options)

func WithParent(connection kube.ParentConnection, displayName string) Option {
	return func(opts *options) {
		opts.connection = connection
		opts.displayName = strings.TrimSpace(displayName)
	}
}

func WithExplorer(explorer *kube.Explorer) Option {
	return func(opts *options) {
		opts.explorer = explorer
	}
}

// New builds the dashboard HTTP handler. All routes are mounted under cfg.BasePath.
func New(cfg config.Config, opts ...Option) http.Handler {
	options := options{}
	for _, opt := range opts {
		opt(&options)
	}

	mux := http.NewServeMux()
	apiPrefix := cfg.BasePath + "/api"

	mux.HandleFunc(apiPrefix+"/health", writeJSON(map[string]string{"status": "ok"}))
	mux.HandleFunc(apiPrefix+"/context", writeJSON(map[string]any{
		"name": options.connection.Name, "inCluster": options.connection.InCluster,
		"displayName": options.displayName, "configured": options.explorer != nil,
		"version": config.BuildVersion(), "labelEditing": options.labelEditing && options.explorer != nil,
		"releaseTriage":              options.releaseTriage && options.explorer != nil,
		"releaseDeletion":            options.releaseDeletion && options.explorer != nil,
		"nativePublishing":           options.nativePublishing && options.explorer != nil,
		"failedRegistrationDeletion": options.failedRegistrationDeletion && options.explorer != nil,
		"chartSourceEditing":         options.chartSourceEditing && options.explorer != nil,
		"chartDeployment":            options.chartDeployment && options.explorer != nil,
		"chartVersionEditing":        options.chartVersionEditing && options.explorer != nil,
	}))
	mux.HandleFunc(apiPrefix+"/helmrelease-events", helmReleaseEventsHandler(options.explorer))
	mux.HandleFunc("GET "+apiPrefix+"/description-status", descriptionStatusHandler(options.explorer))
	mux.HandleFunc("PATCH "+apiPrefix+"/clusters/{namespace}/{name}/labels", options.editClusterLabels)
	mux.HandleFunc("GET "+apiPrefix+"/release-triage", options.triageStates)
	mux.HandleFunc("POST "+apiPrefix+"/helmreleases/{namespace}/{name}/triage", options.editTriage)
	mux.HandleFunc("DELETE "+apiPrefix+"/helmreleases/{namespace}/{name}", options.deleteFailedRelease)
	mux.HandleFunc(apiPrefix+"/clusters", clustersHandler(options.explorer))
	mux.HandleFunc(apiPrefix+"/subscriptions/", http.NotFound)
	mux.HandleFunc("GET "+apiPrefix+"/subscriptions/{namespace}/{name}", subscriptionDetailHandler(options.explorer))
	mux.HandleFunc(apiPrefix+"/subscriptions", subscriptionsHandler(options.explorer))
	mux.HandleFunc(apiPrefix+"/manifests", manifestsHandler(options.explorer))
	mux.HandleFunc(apiPrefix+"/apps", writeJSON(map[string]any{"apps": []any{}}))
	mux.HandleFunc(apiPrefix+"/events", eventsHandler(options.explorer))
	mux.HandleFunc(apiPrefix+"/namespaces", namespacesHandler(options.explorer))
	mux.HandleFunc(apiPrefix+"/clusternet/resources", clusternetResourcesHandler(options.explorer))
	mux.HandleFunc(apiPrefix+"/globalizations", globalizationsHandler(options.explorer))
	mux.HandleFunc(apiPrefix+"/localizations", localizationsHandler(options.explorer))
	mux.HandleFunc(apiPrefix+"/feedinventories", feedInventoriesHandler(options.explorer))
	mux.HandleFunc(apiPrefix+"/helmreleases", helmReleasesHandler(options.explorer))
	mux.HandleFunc("DELETE "+apiPrefix+"/registrations/{name}", options.deleteFailedRegistration)
	mux.HandleFunc("POST "+apiPrefix+"/native-sources/{namespace}/{name}/{kind}", options.nativeSource)
	mux.HandleFunc("POST "+apiPrefix+"/native-workloads/{namespace}/{name}/source", options.nativeWorkload("source"))
	mux.HandleFunc("POST "+apiPrefix+"/native-workloads/{namespace}/{name}/preview", options.nativeWorkload("preview"))
	mux.HandleFunc("POST "+apiPrefix+"/native-workloads/{namespace}/{name}/apply", options.nativeWorkload("apply"))
	mux.HandleFunc("GET "+apiPrefix+"/registrations", catalogHandler(options.explorer, "registrations", (*kube.Explorer).Registrations))
	mux.HandleFunc("GET "+apiPrefix+"/helmcharts", catalogHandler(options.explorer, "helmCharts", (*kube.Explorer).HelmCharts))
	mux.HandleFunc("GET "+apiPrefix+"/helmcharts/{namespace}/{name}/version-impact", options.chartVersionImpact)
	mux.HandleFunc("POST "+apiPrefix+"/helmcharts/{namespace}/{name}/version", options.changeChartVersion)
	mux.HandleFunc("POST "+apiPrefix+"/helmcharts/{namespace}/{name}/deployment-preview", options.deploymentHandler(false))
	mux.HandleFunc("POST "+apiPrefix+"/helmcharts/{namespace}/{name}/deployments", options.deploymentHandler(true))
	mux.HandleFunc("POST "+apiPrefix+"/helmcharts/{namespace}/{name}/source", options.chartSourceHandler("read"))
	mux.HandleFunc("POST "+apiPrefix+"/helmcharts/{namespace}/{name}/source-preview", options.chartSourceHandler("preview"))
	mux.HandleFunc("POST "+apiPrefix+"/helmcharts/{namespace}/{name}/source-save", options.chartSourceHandler("save"))
	mux.HandleFunc("GET "+apiPrefix+"/bases", catalogHandler(options.explorer, "bases", (*kube.Explorer).Bases))
	mux.HandleFunc(apiPrefix+"/diagnostics", diagnosticsHandler(options.explorer))

	mountStatic(mux, cfg)

	return basePathGuard(cfg.BasePath, mux)
}

func basePathGuard(basePath string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hasUnsafePathSegments(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		if basePath != "" && !strings.HasPrefix(r.URL.Path, basePath+"/") && r.URL.Path != basePath {
			http.NotFound(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func hasUnsafePathSegments(path string) bool {
	for segment := range strings.SplitSeq(path, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func mountStatic(mux *http.ServeMux, cfg config.Config) {
	if cfg.StaticDir == "" {
		return
	}

	prefix := cfg.BasePath + "/"
	mux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		relPath := strings.TrimPrefix(r.URL.Path, prefix)
		if relPath == "" {
			relPath = "index.html"
		}

		cleanRelPath := filepath.Clean("/" + relPath)
		if strings.Contains(cleanRelPath, "..") {
			http.NotFound(w, r)
			return
		}
		candidate := filepath.Join(cfg.StaticDir, strings.TrimPrefix(cleanRelPath, "/"))
		if !isPathWithinBase(candidate, cfg.StaticDir) {
			http.NotFound(w, r)
			return
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			if filepath.Base(candidate) == "index.html" {
				serveIndex(w, candidate, cfg.BasePath)
				return
			}
			http.ServeFile(w, r, candidate)
			return
		}

		indexPath := filepath.Join(cfg.StaticDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			serveIndex(w, indexPath, cfg.BasePath)
			return
		}

		http.NotFound(w, r)
	})
}

func isPathWithinBase(candidate string, base string) bool {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absCandidate)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

func serveIndex(w http.ResponseWriter, indexPath string, basePath string) {
	content, err := os.ReadFile(indexPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	baseJSON, _ := json.Marshal(basePath)
	body := strings.ReplaceAll(string(content), `"%BASE_PATH%"`, string(baseJSON))
	assetBase := html.EscapeString(basePath) + "/assets/"
	body = strings.NewReplacer(`src="./assets/`, `src="`+assetBase, `href="./assets/`, `href="`+assetBase).Replace(body)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(body))
}

func clustersHandler(explorer *kube.Explorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if explorer == nil {
			writeJSON(map[string]any{"clusters": []any{}})(w, r)
			return
		}
		clusters, err := explorer.ManagedClusters(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(map[string]any{"clusters": clusters})(w, r)
	}
}

func subscriptionsHandler(explorer *kube.Explorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if explorer == nil {
			writeJSON(map[string]any{"subscriptions": []any{}})(w, r)
			return
		}
		subscriptions, err := explorer.Subscriptions(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(map[string]any{"subscriptions": subscriptions})(w, r)
	}
}

func subscriptionDetailHandler(explorer *kube.Explorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if explorer == nil {
			writeJSON(map[string]any{"summary": nil, "feeds": []any{}, "targetClusters": []any{}})(w, r)
			return
		}
		namespace, name := r.PathValue("namespace"), r.PathValue("name")
		detail, err := explorer.SubscriptionDetail(r.Context(), namespace, name)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(detail)(w, r)
	}
}

func manifestsHandler(explorer *kube.Explorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if explorer == nil {
			writeJSON(map[string]any{"manifests": []any{}})(w, r)
			return
		}
		manifests, err := explorer.Manifests(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(map[string]any{"manifests": manifests})(w, r)
	}
}

func namespacesHandler(explorer *kube.Explorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if explorer == nil {
			writeJSON(map[string]any{"namespaces": []any{}})(w, r)
			return
		}
		namespaces, err := explorer.Namespaces(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(map[string]any{"namespaces": namespaces})(w, r)
	}
}

func eventsHandler(explorer *kube.Explorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if explorer == nil {
			writeJSON(map[string]any{"events": []any{}})(w, r)
			return
		}
		events, err := explorer.Events(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(map[string]any{"events": events})(w, r)
	}
}

func clusternetResourcesHandler(explorer *kube.Explorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if explorer == nil {
			writeJSON(map[string]any{"resources": []any{}})(w, r)
			return
		}
		resources, err := explorer.ClusternetResources()
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(map[string]any{"resources": resources})(w, r)
	}
}

func globalizationsHandler(explorer *kube.Explorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if explorer == nil {
			writeJSON(map[string]any{"globalizations": []any{}})(w, r)
			return
		}
		items, err := explorer.Globalizations(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(map[string]any{"globalizations": items})(w, r)
	}
}

func localizationsHandler(explorer *kube.Explorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if explorer == nil {
			writeJSON(map[string]any{"localizations": []any{}})(w, r)
			return
		}
		items, err := explorer.Localizations(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(map[string]any{"localizations": items})(w, r)
	}
}

func feedInventoriesHandler(explorer *kube.Explorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if explorer == nil {
			writeJSON(map[string]any{"feedInventories": []any{}})(w, r)
			return
		}
		items, err := explorer.FeedInventories(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(map[string]any{"feedInventories": items})(w, r)
	}
}

func helmReleasesHandler(explorer *kube.Explorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if explorer == nil {
			writeJSON(map[string]any{"helmReleases": []any{}})(w, r)
			return
		}
		items, err := explorer.HelmReleases(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(map[string]any{"helmReleases": items})(w, r)
	}
}

func helmReleaseEventsHandler(explorer *kube.Explorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		namespace, uid := r.URL.Query().Get("namespace"), r.URL.Query().Get("uid")
		if len(validation.IsDNS1123Label(namespace)) > 0 || len(validation.IsDNS1123Subdomain(uid)) > 0 {
			http.Error(w, "valid namespace and uid required", http.StatusBadRequest)
			return
		}
		if explorer == nil {
			http.Error(w, "Kubernetes client unavailable", http.StatusServiceUnavailable)
			return
		}
		events, err := explorer.HelmReleaseEvents(r.Context(), namespace, uid)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(map[string]any{"events": events})(w, r)
	}
}

func descriptionStatusHandler(explorer *kube.Explorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		namespace, name, uid := query.Get("namespace"), query.Get("name"), query.Get("uid")
		if len(validation.IsDNS1123Label(namespace)) > 0 || len(validation.IsDNS1123Subdomain(name)) > 0 || len(validation.IsDNS1123Subdomain(uid)) > 0 {
			http.Error(w, "valid namespace, name and uid required", http.StatusBadRequest)
			return
		}
		if explorer == nil {
			http.Error(w, "Kubernetes client unavailable", http.StatusServiceUnavailable)
			return
		}
		status, err := explorer.DescriptionStatus(r.Context(), namespace, name, uid)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(status)(w, r)
	}
}

func diagnosticsHandler(explorer *kube.Explorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if explorer == nil {
			writeJSON(kube.DiagnosticsSummary{})(w, r)
			return
		}
		diagnostics, err := explorer.Diagnostics(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(diagnostics)(w, r)
	}
}

func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	code := http.StatusInternalServerError
	var status apierrors.APIStatus
	if errors.As(err, &status) {
		if upstream := int(status.Status().Code); upstream >= 400 && upstream < 600 {
			code = upstream
		}
	}
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func writeJSON(payload any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
