package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func WithReleaseDeletion(enabled bool) Option {
	return func(o *options) { o.releaseDeletion = enabled }
}

func (o options) deleteFailedRelease(w http.ResponseWriter, r *http.Request) {
	if !o.allowWorkspaceWrite(w, r, o.releaseDeletion, o.releaseHosts) {
		return
	}
	var request struct {
		UID             string `json:"uid"`
		ResourceVersion string `json:"resourceVersion"`
	}
	reader := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	reader.DisallowUnknownFields()
	if err := reader.Decode(&request); err != nil || !errors.Is(reader.Decode(&struct{}{}), io.EOF) {
		http.Error(w, "invalid release deletion request", http.StatusBadRequest)
		return
	}
	if err := o.explorer.DeleteFailedRelease(r.Context(), r.PathValue("namespace"), r.PathValue("name"), request.UID, request.ResourceVersion); err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deletionRequested"})
}
