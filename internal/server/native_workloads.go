package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/chxcode/clusternet-dashboard/internal/kube"
)

func WithNativePublishing(enabled bool) Option {
	return func(o *options) { o.nativePublishing = enabled }
}

// A read may include configuration, so it has the same origin and opt-in
// boundary as saving. All endpoints use JSON and disable response caching.
func (o options) decodeNativeRequest(w http.ResponseWriter, r *http.Request, dest any) bool {
	w.Header().Set("Cache-Control", "no-store")
	if !o.allowWorkspaceWrite(w, r, o.nativePublishing, o.releaseHosts) {
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 3<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil || !errors.Is(decoder.Decode(&struct{}{}), io.EOF) {
		http.Error(w, "invalid native workload request", http.StatusBadRequest)
		return false
	}
	return true
}

func (o options) nativeSource(w http.ResponseWriter, r *http.Request) {
	if !o.decodeNativeRequest(w, r, &struct{}{}) {
		return
	}
	result, err := o.explorer.NativeSource(r.Context(), r.PathValue("namespace"), r.PathValue("kind"), r.PathValue("name"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(result)(w, r)
}

func (o options) nativeWorkload(step string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ns, name := r.PathValue("namespace"), r.PathValue("name")
		if step == "source" {
			if !o.decodeNativeRequest(w, r, &struct{}{}) {
				return
			}
			doc, err := o.explorer.NativeWorkload(r.Context(), ns, name)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(doc)(w, r)
			return
		}
		var req kube.NativeWorkloadRequest
		if !o.decodeNativeRequest(w, r, &req) {
			return
		}
		if step == "preview" {
			plan, err := o.explorer.PreviewNativeWorkload(r.Context(), ns, name, req)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(plan)(w, r)
			return
		}
		result, err := o.explorer.ApplyNativeWorkload(r.Context(), ns, name, req)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if result.Status != "saved" {
			w.WriteHeader(http.StatusMultiStatus)
		}
		_ = json.NewEncoder(w).Encode(result)
	}
}
