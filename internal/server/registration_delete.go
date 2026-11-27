package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func WithFailedRegistrationDeletion(enabled bool) Option {
	return func(o *options) { o.failedRegistrationDeletion = enabled }
}

func (o options) deleteFailedRegistration(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !o.allowWorkspaceMutation(w, r, o.failedRegistrationDeletion, o.releaseHosts) {
		return
	}
	var body struct {
		UID             string `json:"uid"`
		ResourceVersion string `json:"resourceVersion"`
		ConfirmName     string `json:"confirmName"`
	}
	reader := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	reader.DisallowUnknownFields()
	if err := reader.Decode(&body); err != nil || !errors.Is(reader.Decode(&struct{}{}), io.EOF) {
		http.Error(w, "invalid registration withdrawal request", http.StatusBadRequest)
		return
	}
	if err := o.explorer.DeleteFailedRegistration(r.Context(), r.PathValue("name"), body.UID, body.ResourceVersion, body.ConfirmName); err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deletionRequested"})
}
