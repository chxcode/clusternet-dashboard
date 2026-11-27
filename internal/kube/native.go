package kube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"
)

type NativeWorkloadRequest struct {
	YAML            string            `json:"yaml"`
	TargetMode      string            `json:"targetMode"`
	Selector        string            `json:"selector,omitempty"`
	Baseline        string            `json:"baseline,omitempty"`
	SourceBaselines map[string]string `json:"sourceBaselines,omitempty"`
	Fingerprint     string            `json:"fingerprint,omitempty"`
}
type NativeResource struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	Action     string `json:"action"`
}
type NativeSourceDocument struct {
	APIVersion    string   `json:"apiVersion"`
	Kind          string   `json:"kind"`
	Namespace     string   `json:"namespace"`
	Name          string   `json:"name"`
	UID           string   `json:"uid"`
	Baseline      string   `json:"baseline"`
	YAML          string   `json:"yaml"`
	Subscriptions []string `json:"subscriptions"`
}
type NativeWorkloadDocument struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	NativeWorkloadRequest
}
type NativeWorkloadPreview struct {
	Namespace          string                    `json:"namespace"`
	Name               string                    `json:"name"`
	TargetMode         string                    `json:"targetMode"`
	Selector           string                    `json:"selector"`
	Targets            []DeploymentPreviewTarget `json:"targets"`
	Resources          []NativeResource          `json:"resources"`
	SubscriptionAction string                    `json:"subscriptionAction"`
	SubscriptionYAML   string                    `json:"subscriptionYAML"`
	ScopeWarnings      []string                  `json:"scopeWarnings"`
	Conflicts          []string                  `json:"conflicts"`
	Fingerprint        string                    `json:"fingerprint"`
}
type NativeWorkloadResult struct {
	Status            string           `json:"status"`
	Resources         []NativeResource `json:"resources"`
	SubscriptionSaved bool             `json:"subscriptionSaved"`
	Message           string           `json:"message,omitempty"`
}
type nativeLive struct {
	raw, file     *unstructured.Unstructured
	uid, baseline string
}
type nativeStep struct {
	info    NativeResource
	desired *unstructured.Unstructured
	live    *nativeLive
}
type nativePlan struct {
	NativeWorkloadPreview
	intent       string
	steps        []nativeStep
	sub, desired *unstructured.Unstructured
}

func nativeHash(v any) string { data, _ := json.Marshal(v); return digest(string(data)) }

// Upstream admission, aggregator and proxy errors can include portions of a
// template. Preserve only the HTTP outcome at this API boundary.
func nativeReadError(err error) error {
	var status apierrors.APIStatus
	code := 503
	if errors.As(err, &status) && status.Status().Code >= 400 && status.Status().Code <= 599 {
		code = int(status.Status().Code)
	}
	return apierrors.NewGenericServerResponse(code, "get", schema.GroupResource{}, "", "", 0, false)
}

func nativeConflict(name, msg string) error {
	return apierrors.NewConflict(subscriptionGVR.GroupResource(), name, errors.New(msg))
}
func nativeKey(o *unstructured.Unstructured) string { return o.GetKind() + "/" + o.GetName() }
func nativeSubBaseline(s *unstructured.Unstructured) string {
	return nativeHash([]any{s.GetUID(), s.Object["spec"]})
}
func nativeFile(raw *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	o := raw.DeepCopy()
	meta := map[string]any{"name": raw.GetName(), "namespace": raw.GetNamespace()}
	o.Object["metadata"] = meta
	l := raw.GetLabels()
	for k := range l {
		if nativeManagedKey(k) {
			delete(l, k)
		}
	}
	if len(l) > 0 {
		o.SetLabels(l)
	}
	a := raw.GetAnnotations()
	for k := range a {
		if nativeManagedKey(k) {
			delete(a, k)
		}
	}
	if len(a) > 0 {
		o.SetAnnotations(a)
	}
	// v0.14 keeps the dry-run allocation for a Local LoadBalancer, while
	// v0.18 strips it. Both expose an editable file without that allocation.
	if o.GetKind() == "Service" && getNestedString(o, "spec", "type") == "LoadBalancer" && getNestedString(o, "spec", "externalTrafficPolicy") == "Local" {
		unstructured.RemoveNestedField(o.Object, "spec", "healthCheckNodePort")
	}
	if err := validateNativeDocument(o); err != nil {
		return nil, apierrors.NewBadRequest("此源包含编辑器不支持的配置。")
	}
	return o, nil
}

func (e *Explorer) readNativeSource(ctx context.Context, ns, kind, name string) (*nativeLive, error) {
	k, ok := nativeKinds[kind]
	if !ok || !validSourceName(ns, name) {
		return nil, apierrors.NewBadRequest("无效的原生资源身份。")
	}
	if e.dynamic == nil {
		return nil, apierrors.NewServiceUnavailable("Kubernetes client unavailable")
	}
	shadow, err := e.dynamic.Resource(shadowGVR(kind)).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			var status apierrors.APIStatus
			group, _, withGroup := strings.Cut(k.version, "/")
			if !withGroup {
				group = ""
			}
			if !errors.As(err, &status) || status.Status().Details == nil || status.Status().Details.Name != name || status.Status().Details.Group != group || status.Status().Details.Kind != k.resource {
				return nil, apierrors.NewServiceUnavailable("Shadow API 未提供此资源类型，请检查 clusternet-hub 和 APIService。")
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{Group: group, Resource: k.resource}, name)
		}
		return nil, nativeReadError(err)
	}
	if shadow.GetUID() == "" || shadow.GetResourceVersion() == "" || shadow.GetDeletionTimestamp() != nil || shadow.GetAPIVersion() != k.version || shadow.GetKind() != kind || shadow.GetNamespace() != ns || shadow.GetName() != name {
		return nil, nativeConflict(name, "Shadow 源身份变化或正在删除。")
	}
	q := labels.Set{"apps.clusternet.io/config.kind": kind, "apps.clusternet.io/config.name": name, "apps.clusternet.io/config.namespace": ns}.AsSelector()
	manifests, err := e.listDynamicWithOptions(ctx, manifestGVR, metav1.NamespaceAll, metav1.ListOptions{LabelSelector: q.String()})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewServiceUnavailable("无法核对 Manifest 存储。")
		}
		return nil, nativeReadError(err)
	}
	for i := range manifests.Items {
		m := &manifests.Items[i]
		if m.GetUID() != shadow.GetUID() {
			continue
		}
		if m.GetResourceVersion() != shadow.GetResourceVersion() || m.GetDeletionTimestamp() != nil {
			return nil, nativeConflict(name, "Shadow 缓存与源身份尚未同步，请重新读取。")
		}
		obj, found, err := unstructured.NestedMap(m.Object, "template")
		if err != nil || !found {
			return nil, nativeConflict(name, "缺少原始资源模板。")
		}
		raw := &unstructured.Unstructured{Object: obj}
		if raw.GetUID() == "" || raw.GetKind() != kind || raw.GetName() != name || raw.GetNamespace() != ns || raw.GetAPIVersion() != k.version {
			return nil, nativeConflict(name, "缺少可核验的原始模板 UID 或资源身份。")
		}
		file, err := nativeFile(raw)
		if err != nil {
			return nil, err
		}
		return &nativeLive{raw, file, string(m.GetUID()), digest(string(m.GetUID())) + "." + nativeHash(raw.Object)}, nil
	}
	return nil, nativeConflict(name, "未找到与 Shadow UID 对应的 Manifest，请重新读取。")
}
func nativeFeedKey(f FeedSummary, ns string) string {
	if f.Namespace != ns || nativeKinds[f.Kind].version == "" || nativeKinds[f.Kind].version != f.APIVersion {
		return ""
	}
	return f.Kind + "/" + f.Name
}
func nativeReferences(subs []unstructured.Unstructured, ns string) map[string][]string {
	r := map[string][]string{}
	for i := range subs {
		s := &subs[i]
		for _, f := range subscriptionFeeds(s) {
			if f.Namespace == ns && nativeKinds[f.Kind].resource != "" {
				key := f.Kind + "/" + f.Name
				r[key] = append(r[key], s.GetNamespace()+"/"+s.GetName())
			}
		}
	}
	for k := range r {
		r[k] = sets.List(sets.New(r[k]...))
	}
	return r
}
func (e *Explorer) NativeSource(ctx context.Context, ns, kind, name string) (NativeSourceDocument, error) {
	var result NativeSourceDocument
	src, err := e.readNativeSource(ctx, ns, kind, name)
	if err != nil {
		return result, err
	}
	subs, err := e.listDynamic(ctx, subscriptionGVR, metav1.NamespaceAll)
	if err != nil {
		return result, nativeReadError(err)
	}
	data, err := yaml.Marshal(src.file.Object)
	if err != nil {
		return result, errors.New("无法编码源文件。")
	}
	refs := nativeReferences(subs.Items, ns)[kind+"/"+name]
	if refs == nil {
		refs = []string{}
	}
	if len(data) > 256<<10 {
		return result, apierrors.NewBadRequest("源文件超过编辑器大小限制。")
	}
	return NativeSourceDocument{src.raw.GetAPIVersion(), kind, ns, name, src.uid, src.baseline, string(data), refs}, nil
}

func nativeSubScope(sub *unstructured.Unstructured) (string, string, error) {
	d := subscriptionDistribution(sub)
	items, _, _ := unstructured.NestedSlice(sub.Object, "spec", "subscribers")
	if sub.GetDeletionTimestamp() != nil || sub.GetUID() == "" || sub.GetResourceVersion() == "" || d.Strategy != "Replication" || len(items) != 1 || len(d.Selectors) != 1 || d.Selectors[0] == nil || d.TargetScope == "unknown" {
		return "", "", nativeConflict(sub.GetName(), "仅支持一个集群选择条件的 Replication 订阅。")
	}
	item, _ := items[0].(map[string]any)
	if len(item) != 1 {
		return "", "", nativeConflict(sub.GetName(), "此订阅还配置了权重、容忍或子分组，请用原工具管理。")
	}
	if *d.Selectors[0] == "" {
		return "all", "", nil
	}
	return "selector", *d.Selectors[0], nil
}

func (e *Explorer) NativeWorkload(ctx context.Context, ns, name string) (NativeWorkloadDocument, error) {
	var d NativeWorkloadDocument
	if !validSourceName(ns, name) {
		return d, apierrors.NewBadRequest("无效的订阅身份。")
	}
	if e.dynamic == nil {
		return d, apierrors.NewServiceUnavailable("Kubernetes client unavailable")
	}
	s, err := e.dynamic.Resource(subscriptionGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return d, nativeReadError(err)
	}
	mode, q, err := nativeSubScope(s)
	if err != nil {
		return d, err
	}
	d = NativeWorkloadDocument{ns, name, NativeWorkloadRequest{TargetMode: mode, Selector: q, Baseline: nativeSubBaseline(s), SourceBaselines: map[string]string{}}}
	var files []string
	feeds := subscriptionFeeds(s)
	if len(feeds) == 0 || len(feeds) > 16 {
		return NativeWorkloadDocument{}, apierrors.NewBadRequest("不支持此订阅的源数量。")
	}
	for _, f := range feeds {
		key := nativeFeedKey(f, ns)
		if key == "" || d.SourceBaselines[key] != "" {
			return NativeWorkloadDocument{}, apierrors.NewBadRequest("此订阅包含跨命名空间、不支持或重复的源。")
		}
		src, err := e.readNativeSource(ctx, ns, f.Kind, f.Name)
		if err != nil {
			return NativeWorkloadDocument{}, err
		}
		data, err := yaml.Marshal(src.file.Object)
		if err != nil {
			return NativeWorkloadDocument{}, errors.New("无法编码源文件。")
		}
		files = append(files, string(data))
		d.SourceBaselines[key] = src.baseline
	}
	d.YAML = strings.Join(files, "---\n")
	if len(d.YAML) > 256<<10 {
		return NativeWorkloadDocument{}, apierrors.NewBadRequest("此订阅的 YAML 超过大小限制。")
	}
	return d, nil
}

func nativeWritable(o *unstructured.Unstructured, live *nativeLive, owner, intent string) *unstructured.Unstructured {
	result := o.DeepCopy()
	a := result.GetAnnotations()
	if a == nil {
		a = map[string]string{}
	}
	if live != nil {
		// Preserve server and Clusternet metadata including the independent raw
		// template UID, which the shadow Update implementation actually tests.
		if o.GetKind() == "Service" && getNestedString(o, "spec", "type") == "LoadBalancer" && getNestedString(o, "spec", "externalTrafficPolicy") == "Local" {
			if port, ok, _ := unstructured.NestedInt64(live.raw.Object, "spec", "healthCheckNodePort"); ok {
				_ = unstructured.SetNestedField(result.Object, port, "spec", "healthCheckNodePort")
			}
		}
		old, _, _ := unstructured.NestedMap(live.raw.Object, "metadata")
		delete(old, "labels")
		delete(old, "annotations")
		result.Object["metadata"] = old
		l := o.GetLabels()
		if l == nil {
			l = map[string]string{}
		}
		for k, v := range live.raw.GetLabels() {
			if nativeManagedKey(k) {
				l[k] = v
			}
		}
		if len(l) > 0 {
			result.SetLabels(l)
		}
		for k, v := range live.raw.GetAnnotations() {
			if nativeManagedKey(k) {
				a[k] = v
			}
		}
	}
	a[nativeOwnerAnnotation] = owner
	a[nativeIntentAnnotation] = intent
	a[nativeInputAnnotation] = nativeHash(o.Object)
	result.SetAnnotations(a)
	return result
}

// For replay of a partly completed create only: admission may have added
// defaults. Never write on a stale baseline just because these fields match.
func nativeContains(stored, desired any) bool {
	switch x := desired.(type) {
	case map[string]any:
		s, ok := stored.(map[string]any)
		if !ok {
			return false
		}
		for k, v := range x {
			if !nativeContains(s[k], v) {
				return false
			}
		}
		return true
	case []any:
		s, ok := stored.([]any)
		if !ok || len(s) != len(x) {
			return false
		}
		for i, v := range x {
			if !nativeContains(s[i], v) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(stored, desired)
	}
}

func (e *Explorer) planNative(ctx context.Context, ns, name string, req NativeWorkloadRequest) (nativePlan, error) {
	var p nativePlan
	if !validSourceName(ns, name) || len(name) > 52 {
		return p, apierrors.NewBadRequest("订阅名称需为不超过 52 个字符的 DNS 名称。")
	}
	if e.dynamic == nil {
		return p, apierrors.NewServiceUnavailable("Kubernetes client unavailable")
	}
	if req.TargetMode != "all" && req.TargetMode != "selector" {
		return p, apierrors.NewBadRequest("选择 all 或 selector 目标模式。")
	}
	scope, err := parseDeploymentScope(DeploymentRequest{TargetMode: req.TargetMode, Selector: req.Selector})
	if err != nil {
		return p, err
	}
	objects, err := parseNativeYAML(ns, req.YAML)
	if err != nil {
		return p, apierrors.NewBadRequest(err.Error())
	}
	p.NativeWorkloadPreview = NativeWorkloadPreview{Namespace: ns, Name: name, TargetMode: req.TargetMode, Selector: normalizedDeploymentSelector(scope.query), Targets: []DeploymentPreviewTarget{}, Resources: []NativeResource{}, ScopeWarnings: []string{}, Conflicts: []string{}, SubscriptionAction: "create"}
	targets, future, err := e.verifyDeploymentTargets(ctx, scope)
	if err != nil {
		return p, nativeReadError(err)
	}
	currentNS := sets.New[string]()
	for _, c := range targets {
		p.Targets = append(p.Targets, DeploymentPreviewTarget{DeploymentTarget{c.Namespace, c.Name, c.UID}, c.ClusterID})
		currentNS.Insert(c.Namespace)
	}
	p.ScopeWarnings = append(p.ScopeWarnings, "订阅持续适用于未来匹配的集群。已创建的源会保留，多个文件和订阅按顺序保存。")
	subs, err := e.listDynamic(ctx, subscriptionGVR, metav1.NamespaceAll)
	if err != nil {
		return p, nativeReadError(err)
	}
	owner := ns + "/" + name
	p.intent = nativeHash([]any{owner, req.TargetMode, p.Selector, objects})
	for i := range subs.Items {
		sub := &subs.Items[i]
		if sub.GetName() != name {
			continue
		}
		if sub.GetNamespace() == ns {
			p.sub = sub
			continue
		}
		if deploymentMayOverlap(sub, targets) || deploymentRulesMayIntersect(scope.query, subscriptionDistribution(sub)) {
			p.Conflicts = append(p.Conflicts, "订阅 "+sub.GetNamespace()+"/"+name+" 的目标可能相交；旧版控制器会生成同名 Base。")
		}
	}
	bases, err := e.listDynamicWithOptions(ctx, baseGVR, metav1.NamespaceAll, metav1.ListOptions{FieldSelector: "metadata.name=" + name})
	if err != nil {
		return p, nativeReadError(err)
	}
	for i := range bases.Items {
		b := &bases.Items[i]
		if b.GetName() != name {
			continue
		}
		l := b.GetLabels()
		if p.sub != nil && l["apps.clusternet.io/subs.uid"] == string(p.sub.GetUID()) && l["apps.clusternet.io/subs.namespace"] == ns && l["apps.clusternet.io/subs.name"] == name {
			continue
		}
		if currentNS.Has(b.GetNamespace()) {
			p.Conflicts = append(p.Conflicts, "专属命名空间 "+b.GetNamespace()+" 已存在同名 Base。")
		} else if future.Has(b.GetNamespace()) {
			p.ScopeWarnings = append(p.ScopeWarnings, "未来标签匹配时，请检查专属命名空间 "+b.GetNamespace()+" 的同名 Base。")
		}
	}
	existingFeeds := sets.New[string]()
	spec := map[string]any{"schedulingStrategy": "Replication"}
	subBaseline := ""
	if p.sub != nil {
		if _, _, err = nativeSubScope(p.sub); err != nil {
			return p, err
		}
		subBaseline = nativeSubBaseline(p.sub)
		if req.Baseline != subBaseline {
			p.Conflicts = append(p.Conflicts, "同名订阅已存在或已变化，请打开原订阅再编辑。")
		}
		spec, _, _ = unstructured.NestedMap(p.sub.Object, "spec")
		p.SubscriptionAction = "update"
		feeds := subscriptionFeeds(p.sub)
		for _, f := range feeds {
			k := nativeFeedKey(f, ns)
			if k == "" || existingFeeds.Has(k) {
				return p, nativeConflict(name, "现有订阅含不受支持或重复的源。")
			}
			existingFeeds.Insert(k)
		}
		oldMode, oldRule, _ := nativeSubScope(p.sub)
		if oldMode != p.TargetMode || oldRule != p.Selector {
			if oldMode == "all" {
				oldRule = "全部集群"
			}
			newRule := p.Selector
			if p.TargetMode == "all" {
				newRule = "全部集群"
			}
			p.ScopeWarnings = append(p.ScopeWarnings, "目标规则由 "+oldRule+" 调整为 "+newRule+"；未来匹配范围也会随之变化。")
		}
		warnings, err := e.nativeScopeChange(ctx, p.sub, targets)
		if err != nil {
			return p, err
		}
		p.ScopeWarnings = append(p.ScopeWarnings, warnings...)
	} else if req.Baseline != "" {
		p.Conflicts = append(p.Conflicts, "原订阅已不存在，请重新核对目标。")
	}
	refs := nativeReferences(subs.Items, ns)
	affinity, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&scope.affinity)
	if err != nil {
		return p, err
	}
	feeds := make([]any, 0, len(objects))
	desiredKeys := sets.New[string]()
	sourceState := map[string]string{}
	for _, o := range objects {
		k := nativeKey(o)
		desiredKeys.Insert(k)
		feeds = append(feeds, map[string]any{"apiVersion": o.GetAPIVersion(), "kind": o.GetKind(), "namespace": ns, "name": o.GetName()})
		others := sets.New(refs[k]...)
		others.Delete(owner)
		if len(others) > 0 {
			p.Conflicts = append(p.Conflicts, k+" 还被订阅 "+strings.Join(sets.List(others), "、")+" 引用。")
		}
		live, err := e.readNativeSource(ctx, ns, o.GetKind(), o.GetName())
		action := "create"
		if err != nil && !apierrors.IsNotFound(err) {
			return p, err
		}
		if live != nil {
			sourceState[k] = live.baseline
			a := live.raw.GetAnnotations()
			replay := req.SourceBaselines[k] != live.baseline && a[nativeOwnerAnnotation] == owner && a[nativeIntentAnnotation] == p.intent && a[nativeInputAnnotation] == nativeHash(o.Object) && nativeContains(live.file.Object, o.Object)
			action = "update"
			if reflect.DeepEqual(live.file.Object, o.Object) || replay {
				action = "unchanged"
			}
			expected := req.SourceBaselines[k]
			sameIdentity := expected == "" || strings.HasPrefix(expected, digest(live.uid)+".")
			if expected != live.baseline && !(replay && sameIdentity) {
				p.Conflicts = append(p.Conflicts, k+" 已存在或已变化，请先打开源文件核对内容。")
			}
		} else if req.SourceBaselines[k] != "" || existingFeeds.Has(k) {
			p.Conflicts = append(p.Conflicts, k+" 的原始源已不存在，请重新核对。")
		}
		info := NativeResource{o.GetAPIVersion(), o.GetKind(), ns, o.GetName(), action}
		p.Resources = append(p.Resources, info)
		p.steps = append(p.steps, nativeStep{info, o, live})
	}
	for k := range req.SourceBaselines {
		if !desiredKeys.Has(k) {
			p.Conflicts = append(p.Conflicts, "原稿中的源 "+k+" 缺失，请重新打开文件。")
		}
	}
	for _, k := range sets.List(existingFeeds.Difference(desiredKeys)) {
		p.Conflicts = append(p.Conflicts, "编辑时必须保留现有源 "+k+"；移除可能清理子集群资源。")
	}
	spec["feeds"] = feeds
	spec["subscribers"] = []any{map[string]any{"clusterAffinity": affinity}}
	p.desired = &unstructured.Unstructured{Object: map[string]any{"apiVersion": subscriptionGVR.GroupVersion().String(), "kind": "Subscription", "metadata": map[string]any{"namespace": ns, "name": name}, "spec": spec}}
	if p.sub != nil && reflect.DeepEqual(p.sub.Object["spec"], spec) {
		p.SubscriptionAction = "unchanged"
	}
	b, err := yaml.Marshal(p.desired.Object)
	if err != nil {
		return p, errors.New("无法编码订阅文件。")
	}
	p.SubscriptionYAML = string(b)
	sort.Strings(p.Conflicts)
	sort.Strings(p.ScopeWarnings)
	p.Fingerprint = nativeHash([]any{p.NativeWorkloadPreview, objects, subBaseline, sourceState})
	return p, nil
}

func (e *Explorer) PreviewNativeWorkload(ctx context.Context, ns, name string, req NativeWorkloadRequest) (NativeWorkloadPreview, error) {
	p, err := e.planNative(ctx, ns, name, req)
	if err != nil {
		return NativeWorkloadPreview{}, err
	}
	return p.NativeWorkloadPreview, nil
}

func nativeFailure(result NativeWorkloadResult, label string, err error) (NativeWorkloadResult, error) {
	certain := apierrors.IsConflict(err) || apierrors.IsInvalid(err) || apierrors.IsAlreadyExists(err) || apierrors.IsForbidden(err) || apierrors.IsBadRequest(err) || apierrors.IsNotFound(err)
	if !certain {
		result.Status = "uncertain"
		result.Message = label + " 的结果未确认。请保留原稿，重新读取并预览。"
		return result, nil
	}
	if len(result.Resources) > 0 {
		result.Status = "partial"
		result.Message = label + " 未保存。请保留原稿并重新预览。"
		return result, nil
	}
	if apierrors.IsForbidden(err) {
		return result, apierrors.NewForbidden(subscriptionGVR.GroupResource(), label, errors.New("缺少写入权限。"))
	}
	if apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) {
		return result, apierrors.NewBadRequest(label + " 未通过 API 校验或确认条件已经变化。")
	}
	return result, nativeConflict(label, "资源已变化，请重新读取并预览。")
}
func nativePatch(ops ...map[string]any) []byte { b, _ := json.Marshal(ops); return b }
func nativeTest(path string, v any) map[string]any {
	return map[string]any{"op": "test", "path": path, "value": v}
}
func nativeReplace(path string, v any) map[string]any {
	return map[string]any{"op": "replace", "path": path, "value": v}
}

func (e *Explorer) ApplyNativeWorkload(ctx context.Context, ns, name string, req NativeWorkloadRequest) (NativeWorkloadResult, error) {
	res := NativeWorkloadResult{Resources: []NativeResource{}}
	if len(req.Fingerprint) != 64 {
		return res, apierrors.NewBadRequest("请先预览工作负载。")
	}
	e.chartWriteMu.Lock()
	defer e.chartWriteMu.Unlock()
	p, err := e.planNative(ctx, ns, name, req)
	if err != nil {
		return res, err
	}
	if p.Fingerprint != req.Fingerprint || len(p.Conflicts) > 0 {
		return res, nativeConflict(name, "预览已过期或有冲突。")
	}
	owner := ns + "/" + name
	// Subscription references are separate K8s objects. Recheck their scope
	// before each write as another API client may edit them during the batch.
	guard := func() error {
		subs, err := e.listDynamic(ctx, subscriptionGVR, metav1.NamespaceAll)
		if err != nil {
			return err
		}
		refs := nativeReferences(subs.Items, ns)
		found := false
		for i := range subs.Items {
			s := &subs.Items[i]
			if s.GetNamespace() == ns && s.GetName() == name {
				found = true
				if p.sub == nil || s.GetDeletionTimestamp() != nil || nativeSubBaseline(s) != nativeSubBaseline(p.sub) {
					return nativeConflict(name, "订阅身份或配置已变化。")
				}
			}
		}
		if !found && p.sub != nil {
			return nativeConflict(name, "订阅已不存在。")
		}
		for _, step := range p.steps {
			r := sets.New(refs[nativeKey(step.desired)]...)
			r.Delete(owner)
			if len(r) > 0 {
				return nativeConflict(name, "源新增了其他订阅引用。")
			}
		}
		return nil
	}
	expectedUIDs := map[string]string{}
	for _, step := range p.steps {
		if step.info.Action == "unchanged" {
			expectedUIDs[nativeKey(step.desired)] = step.live.uid
			res.Resources = append(res.Resources, step.info)
			continue
		}
		label := nativeKey(step.desired)
		if err = guard(); err != nil {
			return nativeFailure(res, label, err)
		}
		o := nativeWritable(step.desired, step.live, owner, p.intent)
		r := e.dynamic.Resource(shadowGVR(step.info.Kind)).Namespace(ns)
		var saved *unstructured.Unstructured
		if step.live == nil {
			saved, err = r.Create(ctx, o, metav1.CreateOptions{})
		} else {
			patch := nativePatch(nativeTest("", step.live.raw.Object), nativeReplace("", o.Object))
			saved, err = r.Patch(ctx, step.info.Name, types.JSONPatchType, patch, metav1.PatchOptions{})
		}
		if err != nil {
			return nativeFailure(res, label, err)
		}
		if saved.GetUID() == "" || saved.GetResourceVersion() == "" || saved.GetNamespace() != ns || saved.GetName() != o.GetName() || saved.GetKind() != o.GetKind() || saved.GetAPIVersion() != o.GetAPIVersion() || step.live != nil && string(saved.GetUID()) != step.live.uid {
			return nativeFailure(res, label, errors.New("unknown write result"))
		}
		expectedUIDs[label] = string(saved.GetUID())
		res.Resources = append(res.Resources, step.info)
	}
	if err = guard(); err != nil {
		return nativeFailure(res, "Subscription/"+name, err)
	}
	// Shadow Get uses the hub informer. Give it a bounded chance to observe
	// writes. All iterations below only read, and never replay a write.
	for attempt := 0; ; attempt++ {
		fresh, checkErr := e.planNative(ctx, ns, name, req)
		ready := checkErr == nil && len(fresh.Conflicts) == 0 && reflect.DeepEqual(fresh.Targets, p.Targets) && reflect.DeepEqual(fresh.ScopeWarnings, p.ScopeWarnings)
		if ready {
			for _, s := range fresh.steps {
				if s.info.Action != "unchanged" || s.live == nil || s.live.uid != expectedUIDs[nativeKey(s.desired)] {
					ready = false
					break
				}
			}
		}
		if ready {
			p.sub = fresh.sub
			break
		}
		if attempt == 4 || ctx.Err() != nil {
			return nativeFailure(res, "Subscription/"+name, nativeConflict(name, "源尚未同步或目标发生变化，重新预览。"))
		}
		select {
		case <-ctx.Done():
			return nativeFailure(res, "Subscription/"+name, ctx.Err())
		case <-time.After(time.Duration(100*(attempt+1)) * time.Millisecond):
		}
	}
	r := e.dynamic.Resource(subscriptionGVR).Namespace(ns)
	var sub *unstructured.Unstructured
	switch p.SubscriptionAction {
	case "create":
		sub, err = r.Create(ctx, p.desired, metav1.CreateOptions{})
	case "update":
		patch := nativePatch(nativeTest("/metadata/uid", string(p.sub.GetUID())), nativeTest("/metadata/resourceVersion", p.sub.GetResourceVersion()), nativeReplace("/spec", p.desired.Object["spec"]))
		sub, err = r.Patch(ctx, name, types.JSONPatchType, patch, metav1.PatchOptions{})
	default:
		sub = p.sub
	}
	if err != nil {
		return nativeFailure(res, "Subscription/"+name, err)
	}
	if sub.GetUID() == "" || sub.GetNamespace() != ns || sub.GetName() != name || p.sub != nil && sub.GetUID() != p.sub.GetUID() {
		return nativeFailure(res, "Subscription/"+name, fmt.Errorf("unknown subscription identity"))
	}
	res.Status = "saved"
	res.SubscriptionSaved = true
	return res, nil
}

func (e *Explorer) nativeScopeChange(ctx context.Context, sub *unstructured.Unstructured, targets []ManagedClusterSummary) ([]string, error) {
	d := subscriptionDistribution(sub)
	query, err := labels.Parse(*d.Selectors[0])
	if err != nil {
		return nil, err
	}
	clusters, err := e.listDynamic(ctx, managedClusterGVR, metav1.NamespaceAll)
	if err != nil {
		return nil, nativeReadError(err)
	}
	registered := make([]ManagedClusterSummary, 0, len(clusters.Items))
	old, newTargets := sets.New[string](), sets.New[string]()
	for i := range clusters.Items {
		c := &clusters.Items[i]
		if c.GetDeletionTimestamp() != nil {
			continue
		}
		registered = append(registered, ManagedClusterSummary{Name: c.GetName(), Namespace: c.GetNamespace()})
		if query.Matches(labels.Set(c.GetLabels())) {
			old.Insert(c.GetNamespace() + "/" + c.GetName())
		}
	}
	for _, c := range targets {
		newTargets.Insert(c.Namespace + "/" + c.Name)
	}
	var warnings []string
	add := func(prefix string, keys sets.Set[string]) {
		if len(keys) > 0 {
			warnings = append(warnings, fmt.Sprintf("%s（%d）：%s。", prefix, len(keys), strings.Join(sets.List(keys), "、")))
		}
	}
	add("原规则中不再匹配的候选集群", old.Difference(newTargets))
	add("新增候选集群", newTargets.Difference(old))
	reported := distinctSubscriptionBindings(sub)
	if reported == nil {
		return append(warnings, "父集群尚未报告可核验的绑定清单。"), nil
	}
	bound := registeredBindingKeys(registered, reported)
	if bound == nil {
		return append(warnings, "上报的绑定身份有歧义，调整规则可能撤回已有资源。原始绑定："+strings.Join(sets.List(sets.New(reported...)), "、")+"。"), nil
	}
	names := sets.New[string]()
	for k := range bound {
		names.Insert(k.Namespace + "/" + k.Name)
	}
	add("源修改可在订阅更新前传播到当前绑定", names)
	add("原绑定中不在新候选范围的集群，子端资源可能撤回", names.Difference(newTargets))
	return warnings, nil
}
