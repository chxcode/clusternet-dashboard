package kube

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
)

type SourceFileIdentity struct {
	Namespace       string `json:"namespace"`
	Name            string `json:"name"`
	UID             string `json:"uid"`
	ResourceVersion string `json:"resourceVersion"`
}

type SourceFileDocument struct {
	SourceFileIdentity
	YAML string `json:"yaml"`
}

type SourceFileRequest struct {
	Mode            string `json:"mode"`
	Namespace       string `json:"namespace"`
	Name            string `json:"name"`
	YAML            string `json:"yaml"`
	UID             string `json:"uid,omitempty"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
	Fingerprint     string `json:"fingerprint,omitempty"`
}

type SourceFieldChange struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

type SourceFilePreview struct {
	Namespace     string                     `json:"namespace"`
	Name          string                     `json:"name"`
	Mode          string                     `json:"mode"`
	ChangedFields []string                   `json:"changedFields"`
	Changes       []SourceFieldChange        `json:"changes"`
	Subscriptions []ChartVersionSubscription `json:"subscriptions"`
	Targets       []string                   `json:"targets"`
	Fingerprint   string                     `json:"fingerprint"`
}

const maxSourceYAML = 16 * 1024

// Only cross-version fields for which this editor can validate and show the
// entire value. Other spec fields stay on the object throughout an edit.
var chartSourceFields = []string{
	"atomic", "chart", "createNamespace", "disableHooks", "repo", "skipCRDs",
	"targetNamespace", "timeoutSeconds", "version", "wait", "waitForJob",
}

// Existing charts may track numeric ranges or the latest release. The editor
// can open these projections, while every saved source pins an exact version.
var legacySourceVersion = regexp.MustCompile(`^[0-9xXv*~^<>=| ,.\-]+$`)

var sourceRepoPath = regexp.MustCompile(`^(/[a-zA-Z0-9._~-]+)*/?$`)

func validSourceName(ns, name string) bool {
	return len(validation.IsDNS1123Label(ns)) == 0 && len(validation.IsDNS1123Subdomain(name)) == 0
}

func validSourceRepo(value string) bool {
	u, err := url.Parse(value)
	if err != nil || len(value) > 1024 || (u.Scheme != "https" && u.Scheme != "oci" && u.Scheme != "http") ||
		u.Hostname() == "" || u.User != nil || strings.ContainsAny(value, "?#%") || !sourceRepoPath.MatchString(u.Path) {
		return false
	}
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func sourceFieldOK(field string, value any) bool {
	s, _ := value.(string)
	switch field {
	case "repo":
		return validSourceRepo(s)
	case "chart":
		return len(validation.IsDNS1123Subdomain(s)) == 0
	case "targetNamespace":
		return len(validation.IsDNS1123Label(s)) == 0
	case "version":
		// An existing unpinned chart can be opened and pinned in the editor.
		_, ok := value.(string)
		return ok && len(s) <= 128 && (s == "" || exactChartVersion.MatchString(s) || legacySourceVersion.MatchString(s))
	case "timeoutSeconds":
		n, ok := value.(int64)
		return ok && n >= 0 && n <= 2147483647
	default:
		_, ok := value.(bool)
		return ok
	}
}

func sourceObject(namespace, name string, spec map[string]any) map[string]any {
	return map[string]any{
		"apiVersion": helmChartGVR.GroupVersion().String(), "kind": "HelmChart",
		"metadata": map[string]any{"namespace": namespace, "name": name},
		"spec":     spec,
	}
}

func sourceIdentity(chart *unstructured.Unstructured) SourceFileIdentity {
	return SourceFileIdentity{Namespace: chart.GetNamespace(), Name: chart.GetName(),
		UID: string(chart.GetUID()), ResourceVersion: chart.GetResourceVersion()}
}

func sourceConflict(name, message string) error {
	return apierrors.NewConflict(helmChartGVR.GroupResource(), name, errors.New(message))
}

func (e *Explorer) liveChartSource(ctx context.Context, namespace, name string) (*unstructured.Unstructured, map[string]any, error) {
	obj, err := e.dynamic.Resource(helmChartGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	if obj.GetDeletionTimestamp() != nil || obj.GetResourceVersion() == "" || obj.GetUID() == "" {
		return nil, nil, sourceConflict(name, "HelmChart identity is unavailable or it is being deleted")
	}
	projected := map[string]any{}
	for _, key := range chartSourceFields {
		value, found, err := unstructured.NestedFieldNoCopy(obj.Object, "spec", key)
		if err != nil || (found && !sourceFieldOK(key, value)) {
			// Never include the stored value (notably repository credentials)
			// or raw spec in the error sent to the browser.
			return nil, nil, sourceConflict(name, "existing spec."+key+" cannot be safely opened in this editor")
		}
		if found {
			projected[key] = value
		}
	}
	return obj, projected, nil
}

func (e *Explorer) HelmChartSource(ctx context.Context, namespace, name string) (SourceFileDocument, error) {
	var out SourceFileDocument
	if !validSourceName(namespace, name) {
		return out, apierrors.NewBadRequest("invalid source HelmChart name or namespace")
	}
	if e.dynamic == nil {
		return out, apierrors.NewServiceUnavailable("Kubernetes client unavailable")
	}
	obj, spec, err := e.liveChartSource(ctx, namespace, name)
	if err != nil {
		return out, err
	}
	data, err := yaml.Marshal(sourceObject(namespace, name, spec))
	if err != nil {
		return out, errors.New("unable to encode HelmChart source projection")
	}
	return SourceFileDocument{sourceIdentity(obj), string(data)}, nil
}

// Decode only scalar values in the three mappings of the source file. Avoid
// alias expansion, implicit merges, untrusted extra documents and generic YAML
// passthrough. Decoder diagnostics may quote sensitive input, so stay generic.
func sourceMapping(n *yaml.Node, depth int) (map[string]any, bool) {
	if depth > 1 || n.Kind != yaml.MappingNode || n.Tag != "!!map" || n.Anchor != "" || n.Style&yaml.TaggedStyle != 0 {
		return nil, false
	}
	m := map[string]any{}
	for i := 0; i < len(n.Content); i += 2 {
		key, val := n.Content[i], n.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Anchor != "" || key.Style&yaml.TaggedStyle != 0 ||
			val.Anchor != "" || val.Style&yaml.TaggedStyle != 0 {
			return nil, false
		}
		if _, exists := m[key.Value]; exists {
			return nil, false
		}
		if val.Kind == yaml.MappingNode {
			nested, ok := sourceMapping(val, depth+1)
			if !ok {
				return nil, false
			}
			m[key.Value] = nested
			continue
		}
		if val.Kind != yaml.ScalarNode {
			return nil, false
		}
		switch val.Tag {
		case "!!str":
			m[key.Value] = val.Value
		case "!!bool":
			var b bool
			if val.Decode(&b) != nil {
				return nil, false
			}
			m[key.Value] = b
		case "!!int":
			var v int64
			if val.Decode(&v) != nil {
				return nil, false
			}
			m[key.Value] = v
		default:
			return nil, false
		}
	}
	return m, true
}

func parseChartSource(req SourceFileRequest) (map[string]any, error) {
	if len(req.YAML) == 0 || len(req.YAML) > maxSourceYAML {
		return nil, apierrors.NewBadRequest("provide one HelmChart YAML document of at most 16 KiB")
	}
	decoder := yaml.NewDecoder(strings.NewReader(req.YAML))
	var doc, tail yaml.Node
	if decoder.Decode(&doc) != nil || len(doc.Content) != 1 || decoder.Decode(&tail) != io.EOF {
		return nil, apierrors.NewBadRequest("expected exactly one valid YAML document")
	}
	obj, ok := sourceMapping(doc.Content[0], 0)
	if !ok || len(obj) != 4 || obj["apiVersion"] != helmChartGVR.GroupVersion().String() || obj["kind"] != "HelmChart" {
		return nil, apierrors.NewBadRequest("expected one apps.clusternet.io/v1alpha1 HelmChart with only apiVersion, kind, metadata and spec")
	}
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok || len(metadata) != 2 || metadata["namespace"] != req.Namespace || metadata["name"] != req.Name {
		return nil, apierrors.NewBadRequest("metadata must contain only namespace and name matching the selected source")
	}
	spec, ok := obj["spec"].(map[string]any)
	if !ok {
		return nil, apierrors.NewBadRequest("HelmChart spec is required")
	}
	for key, value := range spec {
		if !slices.Contains(chartSourceFields, key) {
			return nil, apierrors.NewBadRequest("unsupported field in HelmChart spec")
		}
		if !sourceFieldOK(key, value) {
			return nil, apierrors.NewBadRequest("invalid value for spec." + key)
		}
	}
	for _, key := range []string{"repo", "chart", "targetNamespace", "version"} {
		if spec[key] == nil || spec[key] == "" {
			return nil, apierrors.NewBadRequest("spec." + key + " is required (pin an exact Chart version)")
		}
	}
	if !exactChartVersion.MatchString(spec["version"].(string)) {
		return nil, apierrors.NewBadRequest("spec.version must pin an exact Chart version")
	}
	return spec, nil
}

// A newly created source can satisfy already-declared subscriptions. Capture
// these references for create as well as edit, including not-yet-bound refs.
func (e *Explorer) sourceReferences(ctx context.Context, namespace, name string, plan *SourceFilePreview) ([]string, error) {
	subs, err := e.listDynamic(ctx, subscriptionGVR, metav1.NamespaceAll)
	if err != nil {
		return nil, err
	}
	snapshots := []string{}
	targets := sets.New[string]()
	for i := range subs.Items {
		sub := &subs.Items[i]
		referencesChart := false
		for _, feed := range subscriptionFeeds(sub) {
			if feed.Kind == "HelmChart" && feed.Namespace == namespace && feed.Name == name {
				referencesChart = true
				break
			}
		}
		if !referencesChart {
			continue
		}
		bound, _, err := unstructured.NestedStringSlice(sub.Object, "status", "bindingClusters")
		if err != nil {
			return nil, errors.New("invalid subscription binding status")
		}
		perSub := sets.New(bound...)
		perSub.Delete("")
		targets.Insert(sets.List(perSub)...)
		plan.Subscriptions = append(plan.Subscriptions, ChartVersionSubscription{Namespace: sub.GetNamespace(), Name: sub.GetName(), BoundTargets: len(perSub)})
		snapshot, err := json.Marshal([]any{sub.GetNamespace(), sub.GetName(), sub.GetUID(), sub.Object["spec"], sets.List(perSub)})
		if err != nil {
			return nil, errors.New("unable to verify subscription scope")
		}
		snapshots = append(snapshots, digest(string(snapshot)))
	}
	sort.Slice(plan.Subscriptions, func(i, j int) bool {
		a, b := plan.Subscriptions[i], plan.Subscriptions[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
	sort.Strings(snapshots)
	plan.Targets = sets.List(targets)
	return snapshots, nil
}

func (e *Explorer) planChartSource(ctx context.Context, req SourceFileRequest) (SourceFilePreview, map[string]any, error) {
	var plan SourceFilePreview
	if !validSourceName(req.Namespace, req.Name) || (req.Mode != "create" && req.Mode != "edit") {
		return plan, nil, apierrors.NewBadRequest("provide a valid HelmChart namespace, name and create or edit mode")
	}
	if req.Mode == "create" && len(req.Name) > 53 {
		return plan, nil, apierrors.NewBadRequest("HelmChart name must fit the 53-character Helm release name limit")
	}
	if req.Mode == "edit" && (req.UID == "" || req.ResourceVersion == "") ||
		req.Mode == "create" && (req.UID != "" || req.ResourceVersion != "") {
		return plan, nil, apierrors.NewBadRequest("edit requires the opened UID and resourceVersion; create requires a new identity")
	}
	after, err := parseChartSource(req)
	if err != nil {
		return plan, nil, err
	}
	if e.dynamic == nil {
		return plan, nil, apierrors.NewServiceUnavailable("Kubernetes client unavailable")
	}

	before := map[string]any{}
	if req.Mode == "create" {
		_, err := e.dynamic.Resource(helmChartGVR).Namespace(req.Namespace).Get(ctx, req.Name, metav1.GetOptions{})
		if err == nil {
			return plan, nil, apierrors.NewAlreadyExists(helmChartGVR.GroupResource(), req.Name)
		}
		if !apierrors.IsNotFound(err) {
			return plan, nil, err
		}
	} else {
		live, current, err := e.liveChartSource(ctx, req.Namespace, req.Name)
		if apierrors.IsNotFound(err) {
			return plan, nil, sourceConflict(req.Name, "the opened HelmChart no longer exists")
		}
		if err != nil {
			return plan, nil, err
		}
		if string(live.GetUID()) != req.UID || live.GetResourceVersion() != req.ResourceVersion {
			return plan, nil, sourceConflict(req.Name, "HelmChart changed; reopen the source file")
		}
		before = current
		ref, found, err := unstructured.NestedMap(live.Object, "spec", "chartPullSecret")
		if before["repo"] != after["repo"] && (err != nil || found && len(ref) != 0) {
			return plan, nil, sourceConflict(req.Name, "cannot redirect a HelmChart repository while a pull credential is configured")
		}
	}

	plan = SourceFilePreview{Namespace: req.Namespace, Name: req.Name, Mode: req.Mode, Subscriptions: []ChartVersionSubscription{}}
	for _, field := range chartSourceFields {
		if before[field] == after[field] {
			continue
		}
		path := "spec." + field
		plan.ChangedFields = append(plan.ChangedFields, path)
		plan.Changes = append(plan.Changes, SourceFieldChange{path, before[field], after[field]})
	}
	if len(plan.Changes) == 0 {
		return plan, nil, sourceConflict(req.Name, "the source already has the requested configuration")
	}
	references, err := e.sourceReferences(ctx, req.Namespace, req.Name, &plan)
	if err != nil {
		return plan, nil, err
	}
	if req.Mode == "edit" && len(references) > 0 && before["targetNamespace"] != after["targetNamespace"] {
		return plan, nil, sourceConflict(req.Name, "cannot move the installation namespace while subscriptions reference this HelmChart")
	}
	snapshot, err := json.Marshal([]any{plan, req.UID, req.ResourceVersion, after, references})
	if err != nil {
		return plan, nil, errors.New("unable to fingerprint the HelmChart source preview")
	}
	plan.Fingerprint = digest(string(snapshot))
	return plan, after, nil
}

func (e *Explorer) PreviewHelmChartSource(ctx context.Context, req SourceFileRequest) (SourceFilePreview, error) {
	plan, _, err := e.planChartSource(ctx, req)
	return plan, err
}

func (e *Explorer) SaveHelmChartSource(ctx context.Context, req SourceFileRequest) (SourceFileIdentity, error) {
	var out SourceFileIdentity
	if _, err := hex.DecodeString(req.Fingerprint); len(req.Fingerprint) != 64 || err != nil {
		return out, apierrors.NewBadRequest("a source preview fingerprint is required")
	}
	e.chartWriteMu.Lock()
	defer e.chartWriteMu.Unlock()
	if err := ctx.Err(); err != nil {
		return out, err
	}
	plan, spec, err := e.planChartSource(ctx, req)
	if err != nil {
		return out, err
	}
	if req.Fingerprint != plan.Fingerprint {
		return out, sourceConflict(req.Name, "the source or subscription scope changed; preview again")
	}
	client := e.dynamic.Resource(helmChartGVR).Namespace(req.Namespace)
	var saved *unstructured.Unstructured
	if req.Mode == "create" {
		obj := &unstructured.Unstructured{Object: sourceObject(req.Namespace, req.Name, spec)}
		saved, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		fields := map[string]any{}
		for _, change := range plan.Changes {
			fields[strings.TrimPrefix(change.Field, "spec.")] = change.After
		}
		data, encodeErr := json.Marshal(map[string]any{"spec": fields, "metadata": map[string]string{"uid": req.UID, "resourceVersion": req.ResourceVersion}})
		if encodeErr != nil {
			return out, errors.New("unable to encode conditional source patch")
		}
		saved, err = client.Patch(ctx, req.Name, types.MergePatchType, data, metav1.PatchOptions{})
	}
	if err != nil {
		// An admission webhook may echo the complete stored object, including
		// hidden fields. Keep the Kubernetes result code without that payload.
		var status apierrors.APIStatus
		if errors.As(err, &status) {
			s := status.Status()
			return out, &apierrors.StatusError{ErrStatus: metav1.Status{
				Status: metav1.StatusFailure, Code: s.Code, Reason: s.Reason,
				Message: "Kubernetes rejected the HelmChart write; refresh the source and check admission and RBAC",
			}}
		}
		return out, errors.New("HelmChart write outcome unavailable; reload the source before trying again")
	}
	if saved == nil || saved.GetNamespace() != req.Namespace || saved.GetName() != req.Name || saved.GetResourceVersion() == "" ||
		saved.GetUID() == "" || req.Mode == "edit" && string(saved.GetUID()) != req.UID {
		return out, errors.New("HelmChart write returned an unexpected identity; reload the source")
	}
	return sourceIdentity(saved), nil
}
