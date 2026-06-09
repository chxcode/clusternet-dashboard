package kube

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type NamespaceSummary struct {
	Name              string `json:"name"`
	Status            string `json:"status"`
	CreationTimestamp string `json:"creationTimestamp,omitempty"`
}

type EventSummary struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Type      string `json:"type,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
	Involved  string `json:"involved,omitempty"`
	LastSeen  string `json:"lastSeen,omitempty"`
}

type APIResourceSummary struct {
	Name       string   `json:"name"`
	Group      string   `json:"group,omitempty"`
	Version    string   `json:"version"`
	Kind       string   `json:"kind"`
	Namespaced bool     `json:"namespaced"`
	Verbs      []string `json:"verbs"`
}

type ComponentSummary struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	Kind            string `json:"kind"`
	Image           string `json:"image,omitempty"`
	Version         string `json:"version,omitempty"`
	Role            string `json:"role,omitempty"`
	ReadyReplicas   int32  `json:"readyReplicas"`
	DesiredReplicas int32  `json:"desiredReplicas"`
}

type RBACCheckSummary struct {
	Resource string `json:"resource"`
	Group    string `json:"group,omitempty"`
	Allowed  bool   `json:"allowed"`
	Error    string `json:"error,omitempty"`
}

type DiagnosticsSummary struct {
	Resources                 []APIResourceSummary `json:"resources"`
	Components                []ComponentSummary   `json:"components"`
	CompletedReleasesSupport  string               `json:"completedReleasesSupport"`
	ReadOnlyRBACChecks         []RBACCheckSummary   `json:"readOnlyRBACChecks"`
}

type ManagedClusterSummary struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace,omitempty"`
	Status            string            `json:"status"`
	Online            bool              `json:"online"`
	Labels            map[string]string `json:"labels"`
	KubernetesVersion string            `json:"kubernetesVersion,omitempty"`
	APIServerURL      string            `json:"apiserverURL,omitempty"`
	LastObservedTime  string            `json:"lastObservedTime,omitempty"`
	Platform          string            `json:"platform,omitempty"`
	AgentVersion      string            `json:"agentVersion,omitempty"`
}

type SubscriptionSummary struct {
	Name                string   `json:"name"`
	Namespace           string   `json:"namespace,omitempty"`
	Status              string   `json:"status"`
	DesiredReleases     int64    `json:"desiredReleases"`
	CompletedReleases   int64    `json:"completedReleases"`
	ObservedReleases    int64    `json:"observedReleases"`
	CompletionKnown     bool     `json:"completionKnown"`
	BindingClusterCount int      `json:"bindingClusterCount"`
	BindingClusters     []string `json:"bindingClusters"`
	SchedulerName       string   `json:"schedulerName,omitempty"`
	SchedulingStrategy  string   `json:"schedulingStrategy,omitempty"`
	SubscriberCount     int      `json:"subscriberCount"`
	FeedCount           int      `json:"feedCount"`
	FeedKinds           []string `json:"feedKinds"`
}

type ManifestSummary struct {
	Name               string `json:"name"`
	Namespace          string `json:"namespace,omitempty"`
	TemplateAPIVersion string `json:"templateAPIVersion,omitempty"`
	TemplateKind       string `json:"templateKind,omitempty"`
	TemplateName       string `json:"templateName,omitempty"`
	TemplateNamespace  string `json:"templateNamespace,omitempty"`
}

type OverrideSummary struct {
	Name          string `json:"name"`
	Namespace     string `json:"namespace,omitempty"`
	Kind          string `json:"kind"`
	OverrideCount int    `json:"overrideCount"`
}

type FeedInventorySummary struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	FeedCount int    `json:"feedCount"`
}

type HelmReleaseSummary struct {
	Name                  string `json:"name"`
	Namespace             string `json:"namespace,omitempty"`
	ClusterName           string `json:"clusterName,omitempty"`
	SubscriptionName      string `json:"subscriptionName,omitempty"`
	SubscriptionNamespace string `json:"subscriptionNamespace,omitempty"`
	ReleaseName           string `json:"releaseName,omitempty"`
	TargetNamespace       string `json:"targetNamespace,omitempty"`
	Chart                 string `json:"chart,omitempty"`
	ChartVersion          string `json:"chartVersion,omitempty"`
	Phase                 string `json:"phase,omitempty"`
	Description           string `json:"description,omitempty"`
	Revision              int64  `json:"revision,omitempty"`
	FirstDeployed         string `json:"firstDeployed,omitempty"`
	LastDeployed          string `json:"lastDeployed,omitempty"`
}

type FeedSummary struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Name       string `json:"name,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
}

type SubscriptionTargetCluster struct {
	Name          string         `json:"name"`
	Namespace     string         `json:"namespace,omitempty"`
	Online        bool           `json:"online"`
	Status        string         `json:"status"`
	Observed      bool           `json:"observed"`
	Available     bool           `json:"available"`
	ReplicaStatus map[string]any `json:"replicaStatus,omitempty"`
}

type SubscriptionDetail struct {
	Summary        SubscriptionSummary         `json:"summary"`
	Feeds          []FeedSummary               `json:"feeds"`
	TargetClusters []SubscriptionTargetCluster `json:"targetClusters"`
}

func NewHelmReleaseSummary(release *unstructured.Unstructured) HelmReleaseSummary {
	labels := release.GetLabels()
	return HelmReleaseSummary{
		Name:                  release.GetName(),
		Namespace:             release.GetNamespace(),
		ClusterName:           labels["clusters.clusternet.io/cluster-name"],
		SubscriptionName:      labels["apps.clusternet.io/subs.name"],
		SubscriptionNamespace: labels["apps.clusternet.io/subs.namespace"],
		ReleaseName:           getNestedString(release, "spec", "releaseName"),
		TargetNamespace:       getNestedString(release, "spec", "targetNamespace"),
		Chart:                 getNestedString(release, "spec", "chart"),
		ChartVersion:          getNestedString(release, "spec", "version"),
		Phase:                 getNestedString(release, "status", "phase"),
		Description:           getNestedString(release, "status", "description"),
		Revision:              getNestedInt64(release, "status", "version"),
		FirstDeployed:         getNestedString(release, "status", "firstDeployed"),
		LastDeployed:          getNestedString(release, "status", "lastDeployed"),
	}
}

func NewComponentSummary(name string, namespace string, kind string, image string, readyReplicas int32, desiredReplicas int32) ComponentSummary {
	return ComponentSummary{
		Name:            name,
		Namespace:       namespace,
		Kind:            kind,
		Image:           image,
		Version:         imageVersion(image),
		Role:            componentRole(name),
		ReadyReplicas:   readyReplicas,
		DesiredReplicas: desiredReplicas,
	}
}

func imageVersion(image string) string {
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > lastSlash {
		return image[lastColon+1:]
	}
	return ""
}

func componentRole(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "scheduler"):
		return "scheduler"
	case strings.Contains(lower, "controller-manager") || strings.Contains(lower, "controller"):
		return "controller-manager"
	case strings.Contains(lower, "agent"):
		return "agent"
	case strings.Contains(lower, "hub"):
		return "hub"
	default:
		return "component"
	}
}

func NewNamespaceSummary(ns corev1.Namespace) NamespaceSummary {
	return NamespaceSummary{
		Name:              ns.Name,
		Status:            string(ns.Status.Phase),
		CreationTimestamp: formatTime(ns.CreationTimestamp),
	}
}

func NewEventSummary(event corev1.Event) EventSummary {
	return EventSummary{
		Name:      event.Name,
		Namespace: event.Namespace,
		Type:      event.Type,
		Reason:    event.Reason,
		Message:   event.Message,
		Involved:  event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name,
		LastSeen:  formatTime(event.LastTimestamp),
	}
}

func NewSubscriptionSummary(subscription *unstructured.Unstructured) SubscriptionSummary {
	desired := getNestedInt64(subscription, "status", "desiredReleases")
	completed, completionKnown := getNestedInt64Found(subscription, "status", "completedReleases")
	bindingClusters := getNestedStringSlice(subscription, "status", "bindingClusters")
	observed := countObservedFeedStatusDetails(subscription)
	feeds := subscriptionFeeds(subscription)
	subscribers, _, _ := unstructured.NestedSlice(subscription.Object, "spec", "subscribers")

	status := "Pending"
	if completionKnown && desired > 0 && completed >= desired {
		status = "Completed"
	} else if desired > 0 || completed > 0 || observed > 0 || len(bindingClusters) > 0 {
		status = "Progressing"
	}

	return SubscriptionSummary{
		Name:                subscription.GetName(),
		Namespace:           subscription.GetNamespace(),
		Status:              status,
		DesiredReleases:     desired,
		CompletedReleases:   completed,
		ObservedReleases:    observed,
		CompletionKnown:     completionKnown,
		BindingClusterCount: len(bindingClusters),
		BindingClusters:     bindingClusters,
		SchedulerName:       getNestedString(subscription, "spec", "schedulerName"),
		SchedulingStrategy:  getNestedString(subscription, "spec", "schedulingStrategy"),
		SubscriberCount:     len(subscribers),
		FeedCount:           len(feeds),
		FeedKinds:           feedKinds(feeds),
	}
}

func NewSubscriptionDetail(subscription *unstructured.Unstructured, clusters []ManagedClusterSummary) SubscriptionDetail {
	clusterByNamespacedName := make(map[string]ManagedClusterSummary, len(clusters)*2)
	for _, cluster := range clusters {
		clusterByNamespacedName[cluster.Namespace+"/"+cluster.Name] = cluster
		clusterByNamespacedName[cluster.Name] = cluster
	}

	observed := observedFeedStatusDetails(subscription)
	bindingClusters := getNestedStringSlice(subscription, "status", "bindingClusters")
	targets := make([]SubscriptionTargetCluster, 0, len(bindingClusters))
	for _, bindingCluster := range bindingClusters {
		namespace, name := splitNamespacedName(bindingCluster)
		cluster, ok := clusterByNamespacedName[bindingCluster]
		if !ok {
			cluster = clusterByNamespacedName[name]
		}
		status := cluster.Status
		if status == "" {
			status = "Unknown"
		}
		detail, observedInStatus := observed[name]
		targets = append(targets, SubscriptionTargetCluster{
			Name:          name,
			Namespace:     namespace,
			Online:        cluster.Online,
			Status:        status,
			Observed:      observedInStatus,
			Available:     detail.Available,
			ReplicaStatus: detail.ReplicaStatus,
		})
	}

	return SubscriptionDetail{
		Summary:        NewSubscriptionSummary(subscription),
		Feeds:          subscriptionFeeds(subscription),
		TargetClusters: targets,
	}
}

func NewManifestSummary(manifest *unstructured.Unstructured) ManifestSummary {
	return ManifestSummary{
		Name:               manifest.GetName(),
		Namespace:          manifest.GetNamespace(),
		TemplateAPIVersion: getNestedString(manifest, "template", "apiVersion"),
		TemplateKind:       getNestedString(manifest, "template", "kind"),
		TemplateName:       getNestedString(manifest, "template", "metadata", "name"),
		TemplateNamespace:  getNestedString(manifest, "template", "metadata", "namespace"),
	}
}

func NewOverrideSummary(item *unstructured.Unstructured, kind string) OverrideSummary {
	overrides, _, _ := unstructured.NestedSlice(item.Object, "spec", "overrides")
	return OverrideSummary{
		Name:          item.GetName(),
		Namespace:     item.GetNamespace(),
		Kind:          kind,
		OverrideCount: len(overrides),
	}
}

func NewFeedInventorySummary(item *unstructured.Unstructured) FeedInventorySummary {
	feeds, _, _ := unstructured.NestedSlice(item.Object, "spec", "feeds")
	return FeedInventorySummary{
		Name:      item.GetName(),
		Namespace: item.GetNamespace(),
		FeedCount: len(feeds),
	}
}

func NewManagedClusterSummary(cluster *unstructured.Unstructured) ManagedClusterSummary {
	online := managedClusterOnline(cluster)
	status := "Offline"
	if online {
		status = "Online"
	}

	return ManagedClusterSummary{
		Name:              cluster.GetName(),
		Namespace:         cluster.GetNamespace(),
		Status:            status,
		Online:            online,
		Labels:            cluster.GetLabels(),
		KubernetesVersion: getNestedString(cluster, "status", "k8sVersion"),
		APIServerURL:      getNestedString(cluster, "status", "apiserverURL"),
		LastObservedTime:  getNestedString(cluster, "status", "lastObservedTime"),
		Platform:          getNestedString(cluster, "status", "platform"),
		AgentVersion:      getNestedString(cluster, "status", "agentVersion"),
	}
}

func managedClusterOnline(cluster *unstructured.Unstructured) bool {
	if readyz, found, _ := unstructured.NestedBool(cluster.Object, "status", "readyz"); found && readyz {
		return true
	}
	if livez, found, _ := unstructured.NestedBool(cluster.Object, "status", "livez"); found && livez {
		return true
	}
	healthz, found, _ := unstructured.NestedBool(cluster.Object, "status", "healthz")
	if found && healthz {
		return true
	}

	conditions, found, _ := unstructured.NestedSlice(cluster.Object, "status", "conditions")
	if !found {
		return false
	}
	for _, condition := range conditions {
		conditionMap, ok := condition.(map[string]any)
		if !ok {
			continue
		}
		if conditionMap["type"] == "Ready" && conditionMap["status"] == "True" {
			return true
		}
	}
	return false
}

func getNestedString(obj *unstructured.Unstructured, fields ...string) string {
	value, _, _ := unstructured.NestedString(obj.Object, fields...)
	return value
}

func getNestedInt64(obj *unstructured.Unstructured, fields ...string) int64 {
	value, _, _ := unstructured.NestedInt64(obj.Object, fields...)
	return value
}

func getNestedInt64Found(obj *unstructured.Unstructured, fields ...string) (int64, bool) {
	value, found, _ := unstructured.NestedInt64(obj.Object, fields...)
	return value, found
}

func countObservedFeedStatusDetails(obj *unstructured.Unstructured) int64 {
	return int64(len(observedFeedStatusDetails(obj)))
}

type observedFeedStatusDetail struct {
	Available     bool
	ReplicaStatus map[string]any
}

func observedFeedStatusDetails(obj *unstructured.Unstructured) map[string]observedFeedStatusDetail {
	aggregatedStatuses, found, _ := unstructured.NestedSlice(obj.Object, "status", "aggregatedStatuses")
	if !found {
		return map[string]observedFeedStatusDetail{}
	}
	result := map[string]observedFeedStatusDetail{}
	for _, aggregated := range aggregatedStatuses {
		aggregatedMap, ok := aggregated.(map[string]any)
		if !ok {
			continue
		}
		details, ok := aggregatedMap["feedStatusDetails"].([]any)
		if !ok {
			continue
		}
		for _, detail := range details {
			detailMap, ok := detail.(map[string]any)
			if !ok {
				continue
			}
			clusterName, ok := detailMap["clusterName"].(string)
			if !ok || clusterName == "" {
				continue
			}
			replicaStatus, _ := detailMap["replicaStatus"].(map[string]any)
			available, _, _ := unstructured.NestedBool(detailMap, "available")
			result[clusterName] = observedFeedStatusDetail{Available: available, ReplicaStatus: replicaStatus}
		}
	}
	return result
}

func subscriptionFeeds(obj *unstructured.Unstructured) []FeedSummary {
	feeds, found, _ := unstructured.NestedSlice(obj.Object, "spec", "feeds")
	if !found {
		return nil
	}
	result := make([]FeedSummary, 0, len(feeds))
	for _, feed := range feeds {
		feedMap, ok := feed.(map[string]any)
		if !ok {
			continue
		}
		result = append(result, FeedSummary{
			APIVersion: stringValue(feedMap["apiVersion"]),
			Kind:       stringValue(feedMap["kind"]),
			Name:       stringValue(feedMap["name"]),
			Namespace:  stringValue(feedMap["namespace"]),
		})
	}
	return result
}

func feedKinds(feeds []FeedSummary) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(feeds))
	for _, feed := range feeds {
		kind := feed.Kind
		if kind == "" || seen[kind] {
			continue
		}
		seen[kind] = true
		result = append(result, kind)
	}
	return result
}

func splitNamespacedName(value string) (string, string) {
	parts := strings.SplitN(value, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", value
}

func stringValue(value any) string {
	result, _ := value.(string)
	return result
}

func getNestedStringSlice(obj *unstructured.Unstructured, fields ...string) []string {
	values, found, _ := unstructured.NestedStringSlice(obj.Object, fields...)
	if found {
		return values
	}
	slice, found, _ := unstructured.NestedSlice(obj.Object, fields...)
	if !found {
		return nil
	}
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if value, ok := item.(string); ok {
			result = append(result, value)
		}
	}
	return result
}

func formatTime(t metav1.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Time.Format("2006-01-02T15:04:05Z07:00")
}
