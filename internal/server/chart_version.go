package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"k8s.io/apimachinery/pkg/util/validation"
)

func (o options) chartVersionImpact(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if o.explorer == nil {
		http.Error(w, "Kubernetes client unavailable", http.StatusServiceUnavailable)
		return
	}
	namespace, name := r.PathValue("namespace"), r.PathValue("name")
	if len(validation.IsDNS1123Label(namespace)) != 0 || len(validation.IsDNS1123Subdomain(name)) != 0 {
		http.Error(w, "invalid HelmChart name or namespace", http.StatusBadRequest)
		return
	}
	impact, err := o.explorer.HelmChartVersionImpact(r.Context(), namespace, name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(impact)(w, r)
}

func (o options) changeChartVersion(w http.ResponseWriter, r *http.Request) {
	if !o.allowWorkspaceWrite(w, r, o.chartVersionEditing, o.releaseHosts) {
		return
	}
	var payload struct {
		UID            string  `json:"uid"`
		CurrentVersion *string `json:"currentVersion"`
		NewVersion     string  `json:"newVersion"`
		Fingerprint    string  `json:"fingerprint"`
	}
	reader := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	reader.DisallowUnknownFields()
	if err := reader.Decode(&payload); err != nil || payload.CurrentVersion == nil || !errors.Is(reader.Decode(&struct{}{}), io.EOF) {
		http.Error(w, "invalid HelmChart version change request", http.StatusBadRequest)
		return
	}
	version, err := o.explorer.ChangeHelmChartVersion(r.Context(), r.PathValue("namespace"), r.PathValue("name"),
		payload.UID, *payload.CurrentVersion, payload.NewVersion, payload.Fingerprint)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(map[string]string{"version": version})(w, r)
}
