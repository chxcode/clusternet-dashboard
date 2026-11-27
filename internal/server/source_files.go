package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/chxcode/clusternet-dashboard/internal/kube"
)

func WithChartSourceEditing(enabled bool) Option {
	return func(o *options) { o.chartSourceEditing = enabled }
}

func (o options) chartSourceHandler(step string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !o.allowWorkspaceWrite(w, r, o.chartSourceEditing, o.releaseHosts) {
			return
		}
		var req kube.SourceFileRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		ns, name := r.PathValue("namespace"), r.PathValue("name")
		if err := decoder.Decode(&req); err != nil || !errors.Is(decoder.Decode(&struct{}{}), io.EOF) ||
			(req.Namespace != "" && req.Namespace != ns) || (req.Name != "" && req.Name != name) {
			http.Error(w, "invalid HelmChart configuration request", http.StatusBadRequest)
			return
		}
		req.Namespace, req.Name = ns, name
		var result any
		var err error
		switch step {
		case "read":
			result, err = o.explorer.HelmChartSource(r.Context(), ns, name)
		case "preview":
			result, err = o.explorer.PreviewHelmChartSource(r.Context(), req)
		case "save":
			result, err = o.explorer.SaveHelmChartSource(r.Context(), req)
		}
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if step == "save" && req.Mode == "create" {
			w.WriteHeader(http.StatusCreated)
		}
		_ = json.NewEncoder(w).Encode(result)
	}
}
