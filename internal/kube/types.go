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
	Name                string              `json:"name"`
	Namespace           string              `json:"namespace"`
	Kind                string              `json:"kind"`
	Image               string              `json:"image,omitempty"`
	Version             string              `json:"version,omitempty"`
	Role                string              `json:"role,omitempty"`
	ReadyReplicas       int32               `json:"readyReplicas"`
	DesiredReplicas     int32               `json:"desiredReplicas"`
	AvailableReplicas   int32               `json:"availableReplicas"`
	UpdatedReplicas     int32               `json:"updatedReplicas"`
	UnavailableReplicas int32               `json:"unavailableReplicas"`
	Generation          int64               `json:"generation,omitempty"`
	ObservedGeneration  int64               `json:"observedGeneration,omitempty"`
	Conditions          []ResourceCondition `json:"conditions,omitempty"`
}

type RBACCheckSummary struct {
	Resource string `json:"resource"`
	Verb     string `json:"verb"`
	Group    string `json:"group,omitempty"`
	Allowed  bool   `json:"allowed"`
	Error    string `json:"error,omitempty"`
}

type DiagnosticsSummary struct {
	SectionErrors            map[string]string    `json:"sectionErrors,omitempty"`
	Resources                []APIResourceSummary `json:"resources"`
	Components               []ComponentSummary   `json:"components"`
	CompletedReleasesSupport string               `json:"completedReleasesSupport"`
	ReadOnlyRBACChecks       []RBACCheckSummary   `json:"readOnlyRBACChecks"`
}

type ManagedClusterSummary struct {
	UID               string            `json:"uid,omitempty"`
	ClusterID         string            `json:"clusterId,omitempty"`
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
	Evidence          ClusterEvidence   `json:"evidence"`
}

type SubscriptionSummary struct {
	Distribution        SubscriptionDistribution `json:"distribution"`
	Name                string                   `json:"name"`
	Namespace           string                   `json:"namespace,omitempty"`
	Status              string                   `json:"status"`
	DesiredReleases     int64                    `json:"desiredReleases"`
	CompletedReleases   int64                    `json:"completedReleases"`
	ObservedReleases    int64                    `json:"observedReleases"`
	CompletionKnown     bool                     `json:"completionKnown"`
	BindingClusterCount int                      `json:"bindingClusterCount"`
	BindingClusters     []string                 `json:"bindingClusters"`
	SchedulerName       string                   `json:"schedulerName,omitempty"`
	SchedulingStrategy  string                   `json:"schedulingStrategy,omitempty"`
	SubscriberCount     int                      `json:"subscriberCount"`
	FeedCount           int                      `json:"feedCount"`
	FeedKinds           []string                 `json:"feedKinds"`
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
	Feed            FeedSummary           `json:"feed"`
	Policy          string                `json:"policy,omitempty"`
	Priority        *int64                `json:"priority,omitempty"`
	ClusterSelector string                `json:"clusterSelector,omitempty"`
	InvalidSelector bool                  `json:"invalidSelector,omitempty"`
	Rules           []OverrideRuleSummary `json:"rules,omitempty"`
	Name            string                `json:"name"`
	Namespace       string                `json:"namespace,omitempty"`
	Kind            string                `json:"kind"`
	OverrideCount   int                   `json:"overrideCount"`
}

type FeedInventorySummary struct {
	Feeds     []FeedSummary `json:"feeds,omitempty"`
	Name      string        `json:"name"`
	Namespace string        `json:"namespace,omitempty"`
	FeedCount int           `json:"feedCount"`
}

type HelmReleaseSummary struct {
	ResourceVersion         string                `json:"resourceVersion,omitempty"`
	Deleting                bool                  `json:"deleting,omitempty"`
	ControlledByDescription bool                  `json:"controlledByDescription,omitempty"`
	Atomic                  *bool                 `json:"atomic,omitempty"`
	UpgradeAtomic           *bool                 `json:"upgradeAtomic,omitempty"`
	TriageIncident          string                `json:"triageIncident,omitempty"`
	DescriptionOwner        *DescriptionReference `json:"descriptionOwner,omitempty"`
	UID                     string                `json:"uid,omitempty"`
	Name                    string                `json:"name"`
	Namespace               string                `json:"namespace,omitempty"`
	ClusterName             string                `json:"clusterName,omitempty"`
	SubscriptionName        string                `json:"subscriptionName,omitempty"`
	SubscriptionNamespace   string                `json:"subscriptionNamespace,omitempty"`
	ReleaseName             string                `json:"releaseName,omitempty"`
	TargetNamespace         string                `json:"targetNamespace,omitempty"`
	Chart                   string                `json:"chart,omitempty"`
	ChartVersion            string                `json:"chartVersion,omitempty"`
	Phase                   string                `json:"phase,omitempty"`
	Description             string                `json:"description,omitempty"`
	Revision                int64                 `json:"revision,omitempty"`
	FirstDeployed           string                `json:"firstDeployed,omitempty"`
	LastDeployed            string                `json:"lastDeployed,omitempty"`
}

type FeedSummary struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Name       string `json:"name,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
}

type SubscriptionTargetCluster struct {
	UnattributedFeeds []FeedSummary      `json:"unattributedFeeds,omitempty"`
	Name              string             `json:"name"`
	Namespace         string             `json:"namespace,omitempty"`
	Online            bool               `json:"online"`
	Status            string             `json:"status"`
	Observed          bool               `json:"observed"`
	Available         *bool              `json:"available,omitempty"`
	ReplicaStatus     map[string]int64   `json:"replicaStatus,omitempty"`
	FeedReports       []TargetFeedReport `json:"feedReports,omitempty"`
}

type TargetFeedReport struct {
	FeedSummary
	clusterID     string
	Available     *bool            `json:"available,omitempty"`
	ReplicaStatus map[string]int64 `json:"replicaStatus,omitempty"`
}

type SubscriptionDetail struct {
	Coverage            SubscriptionCoverage        `json:"coverage"`
	SubscriberSelectors []*string                   `json:"subscriberSelectors,omitempty"`
	Summary             SubscriptionSummary         `json:"summary"`
	Feeds               []FeedSummary               `json:"feeds"`
	TargetClusters      []SubscriptionTargetCluster `json:"targetClusters"`
}

func NewHelmReleaseSummary(release *unstructured.Unstructured) HelmReleaseSummary {
	labels := release.GetLabels()
	owner, controlled := releaseDescriptionOwner(release)
	return HelmReleaseSummary{
		ResourceVersion:         release.GetResourceVersion(),
		Deleting:                release.GetDeletionTimestamp() != nil,
		ControlledByDescription: controlled,
		Atomic:                  optionalSpecBool(release, "atomic"),
		UpgradeAtomic:           optionalSpecBool(release, "upgradeAtomic"),
		TriageIncident:          releaseIncident(release),
		DescriptionOwner:        owner,
		UID:                     string(release.GetUID()),
		Name:                    release.GetName(),
		Namespace:               release.GetNamespace(),
		ClusterName:             labels["clusters.clusternet.io/cluster-name"],
		SubscriptionName:        labels["apps.clusternet.io/subs.name"],
		SubscriptionNamespace:   labels["apps.clusternet.io/subs.namespace"],
		ReleaseName:             getNestedString(release, "spec", "releaseName"),
		TargetNamespace:         getNestedString(release, "spec", "targetNamespace"),
		Chart:                   getNestedString(release, "spec", "chart"),
		ChartVersion:            getNestedString(release, "spec", "version"),
		Phase:                   getNestedString(release, "status", "phase"),
		Description:             getNestedString(release, "status", "description"),
		Revision:                getNestedInt64(release, "status", "version"),
		FirstDeployed:           getNestedString(release, "status", "firstDeployed"),
		LastDeployed:            getNestedString(release, "status", "lastDeployed"),
	}
}

func optionalSpecBool(obj *unstructured.Unstructured, field string) *bool {
	value, found, _ := unstructured.NestedBool(obj.Object, "spec", field)
	if !found {
		return nil
	}
	return &value
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
	lastSeen := event.LastTimestamp
	if event.Series != nil && !event.Series.LastObservedTime.IsZero() {
		lastSeen = metav1.NewTime(event.Series.LastObservedTime.Time)
	} else if lastSeen.IsZero() && !event.EventTime.IsZero() {
		lastSeen = metav1.NewTime(event.EventTime.Time)
	}
	return EventSummary{
		Name:      event.Name,
		Namespace: event.Namespace,
		Type:      event.Type,
		Reason:    event.Reason,
		Message:   event.Message,
		Involved:  event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name,
		LastSeen:  formatTime(lastSeen),
	}
}

func NewSubscriptionSummary(subscription *unstructured.Unstructured) SubscriptionSummary {
	reports := observedFeedStatusDetails(subscription)
	observed := 0
	for _, entries := range reports {
		ids := distinctReportingIDs(entries)
		observed += max(1, len(ids))
	}
	return newSubscriptionSummary(subscription, int64(observed), distinctSubscriptionBindings(subscription))
}

func newSubscriptionSummary(subscription *unstructured.Unstructured, observed int64, bindingClusters []string) SubscriptionSummary {
	desired := getNestedInt64(subscription, "status", "desiredReleases")
	completed, completionKnown := getNestedInt64Found(subscription, "status", "completedReleases")
	feeds := subscriptionFeeds(subscription)
	distribution := subscriptionDistribution(subscription)

	status := "Pending"
	if completionKnown && desired > 0 && completed >= desired {
		status = "Completed"
	} else if desired > 0 || completed > 0 || observed > 0 || len(bindingClusters) > 0 {
		status = "Progressing"
	}

	return SubscriptionSummary{
		Distribution:        distribution,
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
		SubscriberCount:     len(distribution.Selectors),
		FeedCount:           len(feeds),
		FeedKinds:           feedKinds(feeds),
	}
}

func NewSubscriptionDetail(subscription *unstructured.Unstructured, clusters []ManagedClusterSummary) SubscriptionDetail {
	clusterByNamespacedName := make(map[string]ManagedClusterSummary, len(clusters))
	clustersByName := make(map[string][]ManagedClusterSummary, len(clusters))
	for _, cluster := range clusters {
		clusterByNamespacedName[cluster.Namespace+"/"+cluster.Name] = cluster
		clustersByName[cluster.Name] = append(clustersByName[cluster.Name], cluster)
	}

	observed := observedFeedStatusDetails(subscription)
	feeds := subscriptionFeeds(subscription)
	bindingClusters := distinctSubscriptionBindings(subscription)
	boundByName := make(map[string]int, len(bindingClusters))
	for _, binding := range bindingClusters {
		_, name := splitNamespacedName(binding)
		boundByName[name]++
	}
	var observedTargets int64
	targets := make([]SubscriptionTargetCluster, 0, len(bindingClusters))
	for _, bindingCluster := range bindingClusters {
		namespace, name := splitNamespacedName(bindingCluster)
		candidates := clustersByName[name]
		cluster := clusterByNamespacedName[bindingCluster]
		if namespace == "" && len(candidates) == 1 {
			cluster = candidates[0]
		}
		status := cluster.Status
		if status == "" {
			status = "Unknown"
		}
		// Older reporters may omit the ID. Use the name only when it identifies one target.
		uniqueName := boundByName[name] == 1 && (len(candidates) == 0 || (len(candidates) == 1 && cluster.Name != ""))
		reports, uncertain := targetReports(observed[name], cluster.ClusterID, uniqueName)
		target := SubscriptionTargetCluster{
			Name:              name,
			Namespace:         namespace,
			Online:            cluster.Online,
			Status:            status,
			Observed:          len(reports) > 0,
			FeedReports:       reports,
			UnattributedFeeds: uncertain,
		}
		if target.Observed {
			observedTargets++
		}
		// A single Feed has unambiguous legacy target-level status.
		if len(reports) == 1 && len(uncertain) == 0 && (len(feeds) == 0 || (len(feeds) == 1 && feeds[0] == reports[0].FeedSummary)) {
			target.Available = reports[0].Available
			target.ReplicaStatus = reports[0].ReplicaStatus
		}
		targets = append(targets, target)
	}

	summary := newSubscriptionSummary(subscription, observedTargets, bindingClusters)
	return SubscriptionDetail{
		Coverage:            subscriptionCoverage(summary.Distribution, clusters, bindingClusters),
		SubscriberSelectors: summary.Distribution.Selectors,
		Summary:             summary,
		Feeds:               feeds,
		TargetClusters:      targets,
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
	return overrideSummary(item, kind)
}

func NewFeedInventorySummary(item *unstructured.Unstructured) FeedInventorySummary {
	feeds, _, _ := unstructured.NestedSlice(item.Object, "spec", "feeds")
	return FeedInventorySummary{
		Feeds:     subscriptionFeeds(item),
		Name:      item.GetName(),
		Namespace: item.GetNamespace(),
		FeedCount: len(feeds),
	}
}

func NewManagedClusterSummary(cluster *unstructured.Unstructured) ManagedClusterSummary {
	status := managedClusterStatus(cluster)

	return ManagedClusterSummary{
		UID:               string(cluster.GetUID()),
		ClusterID:         getNestedString(cluster, "spec", "clusterId"),
		Name:              cluster.GetName(),
		Namespace:         cluster.GetNamespace(),
		Status:            status,
		Online:            status == "Online",
		Labels:            cluster.GetLabels(),
		KubernetesVersion: getNestedString(cluster, "status", "k8sVersion"),
		APIServerURL:      reportedEndpoint(getNestedString(cluster, "status", "apiserverURL")),
		LastObservedTime:  getNestedString(cluster, "status", "lastObservedTime"),
		Platform:          getNestedString(cluster, "status", "platform"),
		AgentVersion:      getNestedString(cluster, "status", "agentVersion"),
		Evidence:          newClusterEvidence(cluster),
	}
}

func managedClusterStatus(cluster *unstructured.Unstructured) string {
	conditions, _, _ := unstructured.NestedSlice(cluster.Object, "status", "conditions")
	for _, condition := range conditions {
		conditionMap, ok := condition.(map[string]any)
		if !ok || conditionMap["type"] != "Ready" {
			continue
		}
		switch conditionMap["status"] {
		case "True":
			return "Online"
		case "False":
			return "NotReady"
		default:
			return "Unknown"
		}
	}
	// Older reporters may publish probes without a Ready condition.
	for _, probe := range []string{"readyz", "livez", "healthz"} {
		if healthy, found, _ := unstructured.NestedBool(cluster.Object, "status", probe); found {
			if healthy {
				return "Online"
			}
			return "NotReady"
		}
	}
	return "Unknown"
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

func distinctReportingIDs(reports []TargetFeedReport) map[string]bool {
	ids := make(map[string]bool)
	for _, report := range reports {
		if report.clusterID != "" {
			ids[report.clusterID] = true
		}
	}
	return ids
}

func targetReports(entries []TargetFeedReport, clusterID string, uniqueName bool) ([]TargetFeedReport, []FeedSummary) {
	ids := distinctReportingIDs(entries)
	nameMatches := uniqueName && (len(ids) == 0 || (len(ids) == 1 && (clusterID == "" || ids[clusterID])))
	var reports []TargetFeedReport
	var uncertain []FeedSummary
	identified := make(map[FeedSummary]bool)
	for _, report := range entries {
		if clusterID != "" && report.clusterID != "" {
			if clusterID != report.clusterID {
				continue
			}
		} else if !nameMatches {
			uncertain = append(uncertain, report.FeedSummary)
			continue
		}
		identified[report.FeedSummary] = true
		reports = append(reports, report)
	}
	result := uncertain[:0]
	for _, feed := range uncertain {
		if !identified[feed] {
			identified[feed] = true
			result = append(result, feed)
		}
	}
	return reports, result
}

func observedFeedStatusDetails(obj *unstructured.Unstructured) map[string][]TargetFeedReport {
	aggregatedStatuses, _, _ := unstructured.NestedSlice(obj.Object, "status", "aggregatedStatuses")
	result := map[string][]TargetFeedReport{}
	for _, aggregated := range aggregatedStatuses {
		aggregatedMap, ok := aggregated.(map[string]any)
		if !ok {
			continue
		}
		feed := feedSummary(aggregatedMap)
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
			report := TargetFeedReport{FeedSummary: feed, clusterID: stringValue(detailMap["clusterId"])}
			if available, ok := detailMap["available"].(bool); ok {
				report.Available = &available
			}
			raw, _ := detailMap["replicaStatus"].(map[string]any)
			for _, field := range []string{"replicas", "currentReplicas", "readyReplicas", "availableReplicas", "unavailableReplicas", "updatedReplicas", "active", "succeeded", "failed", "observedGeneration"} {
				if count, ok := raw[field].(int64); ok {
					if report.ReplicaStatus == nil {
						report.ReplicaStatus = map[string]int64{}
					}
					report.ReplicaStatus[field] = count
				}
			}
			result[clusterName] = append(result[clusterName], report)
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
		result = append(result, feedSummary(feedMap))
	}
	return result
}

func feedSummary(feed map[string]any) FeedSummary {
	return FeedSummary{
		APIVersion: stringValue(feed["apiVersion"]),
		Kind:       stringValue(feed["kind"]),
		Name:       stringValue(feed["name"]),
		Namespace:  stringValue(feed["namespace"]),
	}
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
