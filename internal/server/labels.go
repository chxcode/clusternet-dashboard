package server

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/chxcode/clusternet-dashboard/internal/kube"
	"k8s.io/apimachinery/pkg/util/validation"
)

func WithLabelEditing(enabled bool, allowedHosts ...string) Option {
	return func(o *options) {
		o.labelEditing = enabled
		o.labelHosts = writeHostSet(allowedHosts)
	}
}

func writeHostSet(hosts []string) map[string]bool {
	result := make(map[string]bool, len(hosts))
	for _, host := range hosts {
		if host = strings.ToLower(strings.TrimSpace(host)); host != "" {
			result[host] = true
		}
	}
	return result
}

type labelRequest struct {
	UID     string `json:"uid"`
	Changes []struct {
		Key    string          `json:"key"`
		Before json.RawMessage `json:"before"`
		After  json.RawMessage `json:"after"`
	} `json:"changes"`
}

func (o options) allowWorkspaceWrite(w http.ResponseWriter, r *http.Request, enabled bool, hosts map[string]bool) bool {
	if !o.allowWorkspaceMutation(w, r, enabled, hosts) {
		return false
	}
	if len(validation.IsDNS1123Label(r.PathValue("namespace"))) > 0 {
		http.Error(w, "invalid object namespace", http.StatusBadRequest)
		return false
	}
	return true
}

// Cluster-scoped and namespaced mutations share the same host, origin,
// media type, object name and feature checks.
func (o options) allowWorkspaceMutation(w http.ResponseWriter, r *http.Request, enabled bool, hosts map[string]bool) bool {
	if !enabled || o.explorer == nil {
		http.Error(w, "workspace operation disabled", http.StatusForbidden)
		return false
	}
	// The explicit Host allowlist protects local kubeconfig access from browser
	// DNS rebinding. Ingress must preserve the public Host header.
	host := strings.ToLower((&url.URL{Host: r.Host}).Hostname())
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") ||
		!strings.EqualFold(origin.Host, r.Host) || origin.User != nil || origin.Path != "" ||
		(r.Header.Get("Sec-Fetch-Site") != "" && r.Header.Get("Sec-Fetch-Site") != "same-origin") ||
		(host != "localhost" && !net.ParseIP(host).IsLoopback() && !hosts[host]) {
		http.Error(w, "untrusted request origin or host", http.StatusForbidden)
		return false
	}
	if len(validation.IsDNS1123Subdomain(r.PathValue("name"))) > 0 {
		http.Error(w, "invalid object name or namespace", http.StatusBadRequest)
		return false
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		http.Error(w, "expected application/json", http.StatusUnsupportedMediaType)
		return false
	}
	return true
}

func (o options) editClusterLabels(w http.ResponseWriter, r *http.Request) {
	if !o.allowWorkspaceWrite(w, r, o.labelEditing, o.labelHosts) {
		return
	}
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	reader := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10))
	reader.DisallowUnknownFields()
	var payload labelRequest
	if err := reader.Decode(&payload); err != nil || !errors.Is(reader.Decode(&struct{}{}), io.EOF) {
		http.Error(w, "invalid label change request", http.StatusBadRequest)
		return
	}
	changes := make([]kube.LabelChange, 0, len(payload.Changes))
	for _, field := range payload.Changes {
		change := kube.LabelChange{Key: field.Key}
		if json.Unmarshal(field.Before, &change.Before) != nil || json.Unmarshal(field.After, &change.After) != nil {
			http.Error(w, "before and after must be strings or null", http.StatusBadRequest)
			return
		}
		changes = append(changes, change)
	}
	labels, err := o.explorer.ChangeClusterLabels(r.Context(), ns, name, payload.UID, changes)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(map[string]any{"labels": labels})(w, r)
}
