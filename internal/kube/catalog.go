package kube

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	registrationGVR = managedClusterGVR.GroupVersion().WithResource("clusterregistrationrequests")
	helmChartGVR    = subscriptionGVR.GroupVersion().WithResource("helmcharts")
	baseGVR         = subscriptionGVR.GroupVersion().WithResource("bases")
)

type RegistrationSummary struct {
	UID                string `json:"uid,omitempty"`
	ResourceVersion    string `json:"resourceVersion,omitempty"`
	Deleting           bool   `json:"deleting,omitempty"`
	DeletionMode       string `json:"deletionMode,omitempty"`
	FailureReason      string `json:"failureReason,omitempty"`
	NamespaceEvidence  string `json:"namespaceEvidence,omitempty"`
	Name               string `json:"name"`
	Created            string `json:"created,omitempty"`
	ClusterName        string `json:"clusterName,omitempty"`
	RequestedNamespace string `json:"requestedNamespace,omitempty"`
	ClusterID          string `json:"clusterID,omitempty"`
	ClusterType        string `json:"clusterType,omitempty"`
	SyncMode           string `json:"syncMode,omitempty"`
	Result             string `json:"result,omitempty"`
	ManagedCluster     string `json:"managedCluster,omitempty"`
	DedicatedNamespace string `json:"dedicatedNamespace,omitempty"`
}

func registrationSummary(obj *unstructured.Unstructured) RegistrationSummary {
	return RegistrationSummary{
		UID: string(obj.GetUID()), ResourceVersion: obj.GetResourceVersion(), Deleting: obj.GetDeletionTimestamp() != nil,
		DeletionMode: registrationDeletionMode(obj), FailureReason: registrationFailureReason(obj),
		Name: obj.GetName(), Created: formatTime(obj.GetCreationTimestamp()),
		ClusterName: getNestedString(obj, "spec", "clusterName"), RequestedNamespace: getNestedString(obj, "spec", "clusterNamespace"),
		ClusterID: getNestedString(obj, "spec", "clusterId"), ClusterType: getNestedString(obj, "spec", "clusterType"),
		SyncMode: getNestedString(obj, "spec", "syncMode"), Result: getNestedString(obj, "status", "result"),
		ManagedCluster: getNestedString(obj, "status", "managedClusterName"), DedicatedNamespace: getNestedString(obj, "status", "dedicatedNamespace"),
	}
}

type HelmChartSummary struct {
	Name            string          `json:"name"`
	Namespace       string          `json:"namespace"`
	Chart           string          `json:"chart,omitempty"`
	Version         string          `json:"version,omitempty"`
	TargetNamespace string          `json:"targetNamespace,omitempty"`
	Phase           string          `json:"phase,omitempty"`
	Options         map[string]bool `json:"options,omitempty"`
	TimeoutSeconds  *int64          `json:"timeoutSeconds,omitempty"`
}

func helmChartSummary(obj *unstructured.Unstructured) HelmChartSummary {
	chart := HelmChartSummary{
		Name: obj.GetName(), Namespace: obj.GetNamespace(), Chart: getNestedString(obj, "spec", "chart"),
		Version: getNestedString(obj, "spec", "version"), TargetNamespace: getNestedString(obj, "spec", "targetNamespace"),
		Phase: getNestedString(obj, "status", "phase"), Options: map[string]bool{},
	}
	// A chart location can be a repository URL. Only expose a plain chart name.
	if strings.ContainsAny(chart.Chart, "/:@?#") {
		chart.Chart = ""
	}
	for _, field := range []string{"atomic", "upgradeAtomic", "wait", "waitForJob", "force", "createNamespace", "replace", "replaceCRDs", "skipCRDs", "disableHooks"} {
		if value, ok, _ := unstructured.NestedBool(obj.Object, "spec", field); ok {
			chart.Options[field] = value
		}
	}
	if seconds, ok := getNestedInt64Found(obj, "spec", "timeoutSeconds"); ok {
		chart.TimeoutSeconds = &seconds
	}
	return chart
}

type BaseSummary struct {
	Name                  string        `json:"name"`
	Namespace             string        `json:"namespace"`
	ClusterName           string        `json:"clusterName,omitempty"`
	SubscriptionName      string        `json:"subscriptionName,omitempty"`
	SubscriptionNamespace string        `json:"subscriptionNamespace,omitempty"`
	Feeds                 []FeedSummary `json:"feeds"`
}

func baseSummary(obj *unstructured.Unstructured) BaseSummary {
	labels := obj.GetLabels()
	return BaseSummary{
		Name: obj.GetName(), Namespace: obj.GetNamespace(), ClusterName: labels["clusters.clusternet.io/cluster-name"],
		SubscriptionName: labels["apps.clusternet.io/subs.name"], SubscriptionNamespace: labels["apps.clusternet.io/subs.namespace"],
		Feeds: subscriptionFeeds(obj),
	}
}

type OverrideRuleSummary struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

func overrideSummary(obj *unstructured.Unstructured, kind string) OverrideSummary {
	out := OverrideSummary{Name: obj.GetName(), Namespace: obj.GetNamespace(), Kind: kind, Policy: getNestedString(obj, "spec", "overridePolicy")}
	if priority, ok := getNestedInt64Found(obj, "spec", "priority"); ok {
		out.Priority = &priority
	}
	feed, _, _ := unstructured.NestedMap(obj.Object, "spec", "feed")
	out.Feed = feedSummary(feed)
	rawRules, _, _ := unstructured.NestedSlice(obj.Object, "spec", "overrides")
	out.OverrideCount = len(rawRules)
	for _, raw := range rawRules {
		if rule, ok := raw.(map[string]any); ok {
			out.Rules = append(out.Rules, OverrideRuleSummary{Name: stringValue(rule["name"]), Type: stringValue(rule["type"])})
		}
	}
	if kind == "Globalization" {
		raw, found, err := unstructured.NestedMap(obj.Object, "spec", "clusterAffinity")
		if err != nil {
			out.InvalidSelector = true
		} else if found {
			selector, err := labelSelectorText(raw)
			out.InvalidSelector = err != nil
			out.ClusterSelector = selector
		}
	}
	return out
}

// Keep only projected status and identities when moving to the next Kubernetes page.
func listCatalog[T any](ctx context.Context, e *Explorer, resource schema.GroupVersionResource, summarize func(*unstructured.Unstructured) T) ([]T, error) {
	results := []T{}
	if e.dynamic == nil {
		return results, nil
	}
	options := metav1.ListOptions{Limit: 100}
	client := e.dynamic.Resource(resource)
	for {
		page, err := client.List(ctx, options)
		if err != nil {
			return nil, err
		}
		for i := range page.Items {
			results = append(results, summarize(&page.Items[i]))
		}
		options.Continue = page.GetContinue()
		if options.Continue == "" {
			return results, nil
		}
	}
}

func (e *Explorer) Registrations(ctx context.Context) ([]RegistrationSummary, error) {
	rows, err := listCatalog(ctx, e, registrationGVR, registrationSummary)
	if err == nil {
		e.addRegistrationNamespaceEvidence(ctx, rows)
	}
	return rows, registrationAPIError(err)
}
func (e *Explorer) HelmCharts(ctx context.Context) ([]HelmChartSummary, error) {
	return listCatalog(ctx, e, helmChartGVR, helmChartSummary)
}
func (e *Explorer) Bases(ctx context.Context) ([]BaseSummary, error) {
	return listCatalog(ctx, e, baseGVR, baseSummary)
}
