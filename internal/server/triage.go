package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/chxcode/clusternet-dashboard/internal/kube"
)

func WithReleaseOperations(triage, chartVersions bool, store kube.TriageStore, allowedHosts ...string) Option {
	return func(o *options) {
		o.releaseTriage, o.chartVersionEditing, o.triageStore = triage, chartVersions, store
		o.releaseHosts = writeHostSet(allowedHosts)
	}
}

func (o options) triageStates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !o.releaseTriage || o.explorer == nil {
		http.Error(w, "release follow-up disabled", http.StatusForbidden)
		return
	}
	states, err := o.explorer.TriageStates(r.Context(), o.triageStore)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(states)(w, r)
}

func (o options) editTriage(w http.ResponseWriter, r *http.Request) {
	if !o.allowWorkspaceWrite(w, r, o.releaseTriage, o.releaseHosts) {
		return
	}
	var request struct{ UID, Incident, Before, State string }
	reader := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	reader.DisallowUnknownFields()
	if err := reader.Decode(&request); err != nil || !errors.Is(reader.Decode(&struct{}{}), io.EOF) {
		http.Error(w, "invalid follow-up request", http.StatusBadRequest)
		return
	}
	err := o.explorer.SetTriage(r.Context(), o.triageStore, r.PathValue("namespace"), r.PathValue("name"),
		request.UID, request.Incident, request.Before, request.State)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(map[string]string{"state": request.State})(w, r)
}
