package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/chxcode/clusternet-dashboard/internal/kube"
)

func WithChartDeployment(enabled bool) Option {
	return func(o *options) { o.chartDeployment = enabled }
}

func (o options) deploymentHandler(create bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !o.allowWorkspaceWrite(w, r, o.chartDeployment, o.releaseHosts) {
			return
		}
		var request kube.DeploymentRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil || !errors.Is(decoder.Decode(&struct{}{}), io.EOF) {
			http.Error(w, "invalid deployment request", http.StatusBadRequest)
			return
		}
		namespace, chart := r.PathValue("namespace"), r.PathValue("name")
		if !create {
			preview, err := o.explorer.PreviewDeployment(r.Context(), namespace, chart, request)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(preview)(w, r)
			return
		}
		result, err := o.explorer.CreateDeployment(r.Context(), namespace, chart, request)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(result)
	}
}
