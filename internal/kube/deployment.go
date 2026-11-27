package kube

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/yaml"
)

const deploymentIDLabel = "clusters.clusternet.io/cluster-id"

var clusterUUID = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
var deploymentNamespaces = schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}

type DeploymentTarget struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	UID       string `json:"uid"`
}

type DeploymentRequest struct {
	Name        string             `json:"name"`
	TargetMode  string             `json:"targetMode,omitempty"`
	Selector    string             `json:"selector,omitempty"`
	Targets     []DeploymentTarget `json:"targets,omitempty"`
	Fingerprint string             `json:"fingerprint,omitempty"`
}

type CreatedSubscription DeploymentTarget

type DeploymentPreviewTarget struct {
	DeploymentTarget
	ClusterID string `json:"clusterId"`
}

type DeploymentPreview struct {
	Namespace        string                    `json:"namespace"`
	Name             string                    `json:"name"`
	ChartName        string                    `json:"chartName"`
	ChartVersion     string                    `json:"chartVersion"`
	ChartPhase       string                    `json:"chartPhase"`
	TargetNamespace  string                    `json:"targetNamespace"`
	TargetMode       string                    `json:"targetMode"`
	Selector         string                    `json:"selector"`
	SubscriptionYAML string                    `json:"subscriptionYAML"`
	ScopeWarnings    []string                  `json:"scopeWarnings"`
	Targets          []DeploymentPreviewTarget `json:"targets"`
	Fingerprint      string                    `json:"fingerprint"`
	Conflicts        []string                  `json:"conflicts"`
	subscription     *unstructured.Unstructured
}

func (e *Explorer) PreviewDeployment(ctx context.Context, namespace, chartName string, req DeploymentRequest) (DeploymentPreview, error) {
	var out DeploymentPreview
	if len(validation.IsDNS1123Label(namespace)) != 0 || len(validation.IsDNS1123Subdomain(chartName)) != 0 {
		return out, apierrors.NewBadRequest("invalid source HelmChart name or namespace")
	}
	// On v0.18 a Description is <subscription>-<five random chars>-helm.
	// Its name is used as a label value on each generated HelmRelease.
	if len(validation.IsDNS1123Subdomain(req.Name)) != 0 || len(req.Name) > 52 {
		return out, apierrors.NewBadRequest("subscription name must be a DNS subdomain of at most 52 characters")
	}
	scope, err := parseDeploymentScope(req)
	if err != nil {
		return out, err
	}
	if e.dynamic == nil {
		return out, apierrors.NewServiceUnavailable("Kubernetes client unavailable")
	}
	chart, err := e.dynamic.Resource(helmChartGVR).Namespace(namespace).Get(ctx, chartName, metav1.GetOptions{})
	if err != nil {
		return out, err
	}
	if chart.GetDeletionTimestamp() != nil || chart.GetUID() == "" || chart.GetResourceVersion() == "" {
		return out, apierrors.NewConflict(helmChartGVR.GroupResource(), chartName, errors.New("source HelmChart identity is unavailable or it is being deleted"))
	}
	targets, futureNamespaces, err := e.verifyDeploymentTargets(ctx, scope)
	if err != nil {
		return out, err
	}
	out = DeploymentPreview{Namespace: namespace, Name: req.Name, ChartName: chartName,
		ChartVersion: getNestedString(chart, "spec", "version"), ChartPhase: getNestedString(chart, "status", "phase"),
		TargetNamespace: getNestedString(chart, "spec", "targetNamespace"), TargetMode: scope.mode, ScopeWarnings: []string{}, Targets: make([]DeploymentPreviewTarget, 0, len(targets)),
	}
	for _, cluster := range targets {
		out.Targets = append(out.Targets, DeploymentPreviewTarget{
			DeploymentTarget: DeploymentTarget{Namespace: cluster.Namespace, Name: cluster.Name, UID: cluster.UID},
			ClusterID:        cluster.ClusterID,
		})
	}
	if scope.mode == "fixed" {
		ids := make([]string, 0, len(targets))
		for _, c := range targets {
			ids = append(ids, c.ClusterID)
		}
		sort.Strings(ids)
		scope.affinity.MatchExpressions = []metav1.LabelSelectorRequirement{{Key: deploymentIDLabel, Operator: metav1.LabelSelectorOpIn, Values: ids}}
		scope.query, err = metav1.LabelSelectorAsSelector(&scope.affinity)
		if err != nil {
			return DeploymentPreview{}, err
		}
	} else {
		out.ScopeWarnings = append(out.ScopeWarnings, "新注册的集群或标签变化的集群符合规则时，调度器会重新评估此订阅。实际绑定取决于调度器和污点等条件，安装结果请查看发布记录。")
		if len(targets) == 0 {
			out.ScopeWarnings = append(out.ScopeWarnings, "当前没有标签匹配的集群；创建后将保留订阅，等待符合条件的集群。")
		}
	}
	out.Selector = normalizedDeploymentSelector(scope.query)
	affinity, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&scope.affinity)
	if err != nil {
		return DeploymentPreview{}, err
	}
	out.subscription = &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": subscriptionGVR.GroupVersion().String(), "kind": "Subscription",
		"metadata": map[string]any{"namespace": namespace, "name": req.Name},
		"spec": map[string]any{
			"schedulingStrategy": "Replication",
			"feeds":              []any{map[string]any{"apiVersion": helmChartGVR.GroupVersion().String(), "kind": "HelmChart", "namespace": namespace, "name": chartName}},
			"subscribers":        []any{map[string]any{"clusterAffinity": affinity}},
		},
	}}
	file, err := yaml.Marshal(out.subscription.Object)
	if err != nil {
		return DeploymentPreview{}, err
	}
	out.SubscriptionYAML = string(file)
	conflicts, err := e.deploymentConflicts(ctx, &out, targets, futureNamespaces, scope.query)
	if err != nil {
		return DeploymentPreview{}, err
	}
	out.Conflicts = sets.List(conflicts)
	// Bind the desired source and its identity, Subscription, current matches
	// and conflicts. Routine heartbeat and status timestamp writes do not
	// invalidate confirmation when the actual deployment scope is unchanged.
	snapshot, err := json.Marshal(struct {
		Preview                       DeploymentPreview
		ChartUID                      string
		ChartSpec                     any
		ChartLabels, ChartAnnotations map[string]string
	}{out, string(chart.GetUID()), chart.Object["spec"], chart.GetLabels(), chart.GetAnnotations()})
	if err != nil {
		return DeploymentPreview{}, err
	}
	out.Fingerprint = digest(string(snapshot))
	return out, nil
}

func (e *Explorer) verifyDeploymentTargets(ctx context.Context, scope deploymentScope) ([]ManagedClusterSummary, sets.Set[string], error) {
	clusters, err := e.listDynamic(ctx, managedClusterGVR, metav1.NamespaceAll)
	if err != nil {
		return nil, nil, err
	}
	labelCount, specCount, namespaceCount := map[string]int{}, map[string]int{}, map[string]int{}
	selected := make([]ManagedClusterSummary, 0, len(scope.requested))
	unmatched := map[string]string{}
	for i := range clusters.Items {
		c := &clusters.Items[i]
		id := c.GetLabels()[deploymentIDLabel]
		specID := getNestedString(c, "spec", "clusterId")
		labelCount[strings.ToLower(id)]++
		specCount[strings.ToLower(specID)]++
		namespaceCount[c.GetNamespace()]++
		if scope.mode == "fixed" {
			t, ok := scope.requested[ClusterReference{c.GetNamespace(), c.GetName()}]
			if !ok {
				continue
			}
			if c.GetDeletionTimestamp() != nil || string(c.GetUID()) != t.UID {
				return nil, nil, apierrors.NewConflict(managedClusterGVR.GroupResource(), c.GetName(), errors.New("a selected cluster was replaced or is terminating"))
			}
		} else {
			if c.GetDeletionTimestamp() != nil {
				continue
			}
			if !scope.query.Matches(labels.Set(c.GetLabels())) {
				unmatched[c.GetNamespace()] = ""
				if c.GetUID() != "" && clusterUUID.MatchString(id) && id == specID {
					unmatched[c.GetNamespace()] = id
				}
				continue
			}
		}
		if c.GetUID() == "" || !clusterUUID.MatchString(id) || id != specID {
			return nil, nil, apierrors.NewConflict(managedClusterGVR.GroupResource(), c.GetName(), errors.New("cluster lacks a stable identity or matching cluster-id UUID"))
		}
		selected = append(selected, ManagedClusterSummary{Namespace: c.GetNamespace(), Name: c.GetName(), UID: string(c.GetUID()), ClusterID: id, Labels: c.GetLabels()})
	}
	if scope.mode == "fixed" && len(selected) != len(scope.requested) {
		return nil, nil, apierrors.NewConflict(managedClusterGVR.GroupResource(), "", errors.New("a selected managed cluster no longer exists"))
	}
	namespaces, err := e.listDynamic(ctx, deploymentNamespaces, metav1.NamespaceAll)
	if err != nil {
		return nil, nil, err
	}
	namespaceIDs := map[string]string{}
	for i := range namespaces.Items {
		ns := &namespaces.Items[i]
		if ns.GetDeletionTimestamp() == nil && getNestedString(ns, "status", "phase") != "Terminating" {
			namespaceIDs[ns.GetName()] = ns.GetLabels()[deploymentIDLabel]
		}
	}
	for _, c := range selected {
		id := strings.ToLower(c.ClusterID)
		if labelCount[id] != 1 || specCount[id] != 1 || namespaceCount[c.Namespace] != 1 || namespaceIDs[c.Namespace] != c.ClusterID {
			return nil, nil, apierrors.NewConflict(managedClusterGVR.GroupResource(), c.Name, errors.New("cluster-id is ambiguous or does not match its dedicated namespace"))
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].Namespace != selected[j].Namespace {
			return selected[i].Namespace < selected[j].Namespace
		}
		return selected[i].Name < selected[j].Name
	})
	// Other labels may be changed later. Only exclude a currently unmatched
	// cluster from future occupancy checks if its verified ID contradicts
	// the immutable identity constraints of the new rule.
	future := sets.New[string]()
	if len(unmatched) > 0 {
		identityFilter := deploymentIdentityFilter(scope.query)
		for ns, id := range unmatched {
			lower := strings.ToLower(id)
			unique := id != "" && labelCount[lower] == 1 && specCount[lower] == 1 && namespaceCount[ns] == 1 && namespaceIDs[ns] == id
			if !unique || identityFilter.Matches(labels.Set{deploymentIDLabel: id}) {
				future.Insert(ns)
			}
		}
	}
	return selected, future, nil
}

func (e *Explorer) deploymentConflicts(ctx context.Context, plan *DeploymentPreview, targets []ManagedClusterSummary, futureNamespaces sets.Set[string], query labels.Selector) (sets.Set[string], error) {
	conflicts := sets.New[string]()
	if plan.ChartPhase == "NotFound" {
		conflicts.Insert("源 HelmChart 上报 NotFound，请先修正仓库或版本。")
	}
	if len(plan.ChartName) > 53 {
		conflicts.Insert("源 HelmChart 名称超过 Helm 发布名的 53 字符限制。")
	}
	if len(validation.IsDNS1123Label(plan.TargetNamespace)) != 0 {
		conflicts.Insert("源 HelmChart 未声明有效的目标命名空间。")
	}
	namespaces := sets.New[string]()
	for _, c := range targets {
		namespaces.Insert(c.Namespace)
	}
	bases, err := e.listDynamicWithOptions(ctx, baseGVR, metav1.NamespaceAll, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", plan.Name).String(),
	})
	if err != nil {
		return nil, err
	}
	busyBases, futureBases := sets.New[string](), sets.New[string]()
	for i := range bases.Items {
		b := &bases.Items[i]
		if b.GetName() == plan.Name {
			ns := b.GetNamespace()
			if namespaces.Has(ns) {
				busyBases.Insert(ns)
			} else if futureNamespaces.Has(ns) {
				futureBases.Insert(ns)
			}
		}
	}
	if len(busyBases) > 0 {
		conflicts.Insert(fmt.Sprintf("%d 个目标集群的专属命名空间已有同名 Base，可能与旧版控制器生成的 Base 冲突。", len(busyBases)))
	}
	if len(futureBases) > 0 {
		plan.ScopeWarnings = append(plan.ScopeWarnings, "当前未匹配的集群已有同名 Base。如果标签变化使其加入，此订阅可能与旧版控制器的 Base 冲突。涉及专属命名空间："+strings.Join(sets.List(futureBases), "、")+"。")
	}
	// Inspect parent-side HelmReleases, including orphans and terminating
	// records. Discard notes, values and other payloads after each page.
	claims, err := listCatalog(ctx, e, helmReleaseGVR, func(release *unstructured.Unstructured) string {
		if (!namespaces.Has(release.GetNamespace()) && !futureNamespaces.Has(release.GetNamespace())) || getNestedString(release, "spec", "targetNamespace") != plan.TargetNamespace {
			return ""
		}
		effective := release.GetName()
		if raw, _, _ := unstructured.NestedFieldNoCopy(release.Object, "spec", "releaseName"); raw != nil {
			// The upstream falls back to metadata.name only for a nil pointer.
			effective = stringValue(raw)
		}
		if effective == plan.ChartName {
			return release.GetNamespace()
		}
		return ""
	})
	if err != nil {
		return nil, err
	}
	claimNamespaces := sets.New(claims...)
	occupied := claimNamespaces.Intersection(namespaces)
	futureReleases := claimNamespaces.Intersection(futureNamespaces)
	if len(futureReleases) > 0 {
		plan.ScopeWarnings = append(plan.ScopeWarnings, "当前未匹配的集群已有 HelmRelease 占用相同发布名和目标命名空间。如果标签变化使其加入，请先检查已有发布。涉及专属命名空间："+strings.Join(sets.List(futureReleases), "、")+"。")
	}
	if len(occupied) > 0 {
		conflicts.Insert(fmt.Sprintf("%d 个目标集群已有 HelmRelease 使用相同的 Helm 发布名和目标命名空间。", len(occupied)))
	}
	subs, err := e.listDynamic(ctx, subscriptionGVR, metav1.NamespaceAll)
	if err != nil {
		return nil, err
	}
	chartNamespaces := map[string]string{plan.Namespace: plan.TargetNamespace}
	for i := range subs.Items {
		sub := &subs.Items[i]
		key := sub.GetNamespace() + "/" + sub.GetName()
		if sub.GetName() == plan.Name && sub.GetNamespace() == plan.Namespace {
			conflicts.Insert("已存在同名订阅 " + key + "。")
			continue
		}
		feedNamespaces := sets.New[string]()
		for _, f := range subscriptionFeeds(sub) {
			if f.Kind == "HelmChart" && f.Name == plan.ChartName {
				feedNamespaces.Insert(f.Namespace)
			}
		}
		if len(feedNamespaces) == 0 && sub.GetName() != plan.Name {
			continue
		}
		reason := "已绑定或可能选中相同目标"
		if !deploymentMayOverlap(sub, targets) {
			if !deploymentRulesMayIntersect(query, subscriptionDistribution(sub)) {
				continue
			}
			reason = "目标规则可能在未来的集群注册或标签变化后相交"
		}
		if sub.GetName() == plan.Name {
			conflicts.Insert("订阅 " + key + " " + reason + "，旧版控制器会生成同名 Base。")
		}
		for ns := range feedNamespaces {
			childNamespace, known := chartNamespaces[ns]
			if !known && ns != "" {
				other, err := e.dynamic.Resource(helmChartGVR).Namespace(ns).Get(ctx, plan.ChartName, metav1.GetOptions{})
				if err != nil && !apierrors.IsNotFound(err) {
					return nil, err
				}
				if err == nil {
					childNamespace = getNestedString(other, "spec", "targetNamespace")
				}
				chartNamespaces[ns] = childNamespace
			}
			if childNamespace == "" || childNamespace == plan.TargetNamespace {
				conflicts.Insert("订阅 " + key + " " + reason + "，引用的同名 HelmChart 存在发布冲突。")
			}
		}
	}
	return conflicts, nil
}

func deploymentMayOverlap(sub *unstructured.Unstructured, targets []ManagedClusterSummary) bool {
	if len(targets) == 0 {
		return false
	}
	bindings, _, err := unstructured.NestedStringSlice(sub.Object, "status", "bindingClusters")
	if err != nil {
		return true
	}
	// Unqualified legacy bindings are ambiguous if a matching target name
	// exists. Fail closed for unreadable selectors and malformed bindings.
	if bindings != nil {
		bound := registeredBindingKeys(targets, bindings)
		if bound == nil {
			return true
		}
		for _, target := range targets {
			if bound[ClusterReference{target.Namespace, target.Name}] {
				return true
			}
		}
	}
	matched := subscriptionCoverage(subscriptionDistribution(sub), targets, nil).LabelMatchedClusters
	return matched == nil || *matched > 0
}

func (e *Explorer) CreateDeployment(ctx context.Context, namespace, chartName string, req DeploymentRequest) (CreatedSubscription, error) {
	if _, err := hex.DecodeString(req.Fingerprint); len(req.Fingerprint) != 64 || err != nil {
		return CreatedSubscription{}, apierrors.NewBadRequest("deployment preview fingerprint required")
	}
	e.chartWriteMu.Lock()
	defer e.chartWriteMu.Unlock()
	if err := ctx.Err(); err != nil {
		return CreatedSubscription{}, err
	}
	plan, err := e.PreviewDeployment(ctx, namespace, chartName, req)
	if err != nil {
		return CreatedSubscription{}, err
	}
	if req.Fingerprint != plan.Fingerprint || len(plan.Conflicts) != 0 {
		return CreatedSubscription{}, apierrors.NewConflict(subscriptionGVR.GroupResource(), req.Name, errors.New("deployment scope changed or conflicts exist; review the preview"))
	}
	saved, err := e.dynamic.Resource(subscriptionGVR).Namespace(namespace).Create(ctx, plan.subscription, metav1.CreateOptions{})
	if err != nil {
		return CreatedSubscription{}, err
	}
	if saved.GetNamespace() != namespace || saved.GetName() != req.Name || saved.GetUID() == "" {
		return CreatedSubscription{}, errors.New("create response lacks the expected subscription identity; reload the subscription list")
	}
	return CreatedSubscription{Namespace: saved.GetNamespace(), Name: saved.GetName(), UID: string(saved.GetUID())}, nil
}
