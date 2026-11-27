package kube

import (
	"context"
	"strings"
	"sync"

	authv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var managedClusterGVR = schema.GroupVersionResource{
	Group:    "clusters.clusternet.io",
	Version:  "v1beta1",
	Resource: "managedclusters",
}

var subscriptionGVR = schema.GroupVersionResource{
	Group:    "apps.clusternet.io",
	Version:  "v1alpha1",
	Resource: "subscriptions",
}

var manifestGVR = schema.GroupVersionResource{
	Group:    "apps.clusternet.io",
	Version:  "v1alpha1",
	Resource: "manifests",
}

var globalizationGVR = schema.GroupVersionResource{
	Group:    "apps.clusternet.io",
	Version:  "v1alpha1",
	Resource: "globalizations",
}

var localizationGVR = schema.GroupVersionResource{
	Group:    "apps.clusternet.io",
	Version:  "v1alpha1",
	Resource: "localizations",
}

var feedInventoryGVR = schema.GroupVersionResource{
	Group:    "apps.clusternet.io",
	Version:  "v1alpha1",
	Resource: "feedinventories",
}

var helmReleaseGVR = schema.GroupVersionResource{
	Group:    "apps.clusternet.io",
	Version:  "v1alpha1",
	Resource: "helmreleases",
}

type Explorer struct {
	// Source and version changes can invalidate a pending deployment preview.
	// Serialize their final scope checks and writes on this dashboard instance.
	chartWriteMu sync.Mutex
	client       kubernetes.Interface
	discovery    discovery.DiscoveryInterface
	dynamic      dynamic.Interface
}

func NewExplorer(client kubernetes.Interface, discoveryClient discovery.DiscoveryInterface, dynamicClient dynamic.Interface) *Explorer {
	return &Explorer{client: client, discovery: discoveryClient, dynamic: dynamicClient}
}

func (e *Explorer) Namespaces(ctx context.Context) ([]NamespaceSummary, error) {
	items, err := e.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]NamespaceSummary, 0, len(items.Items))
	for _, item := range items.Items {
		result = append(result, NewNamespaceSummary(item))
	}
	return result, nil
}

func (e *Explorer) Events(ctx context.Context) ([]EventSummary, error) {
	items, err := e.client.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]EventSummary, 0, len(items.Items))
	for _, item := range items.Items {
		result = append(result, NewEventSummary(item))
	}
	return result, nil
}

// HelmReleaseEvents restricts the lookup to one object incarnation in the parent cluster.
func (e *Explorer) HelmReleaseEvents(ctx context.Context, namespace, uid string) ([]EventSummary, error) {
	selector := fields.Set{"involvedObject.kind": "HelmRelease", "involvedObject.uid": uid}.AsSelector()
	items, err := e.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{FieldSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	events := make([]EventSummary, 0, len(items.Items))
	for _, item := range items.Items {
		events = append(events, NewEventSummary(item))
	}
	return events, nil
}

func (e *Explorer) ManagedClusters(ctx context.Context) ([]ManagedClusterSummary, error) {
	if e.dynamic == nil {
		return []ManagedClusterSummary{}, nil
	}
	items, err := e.dynamic.Resource(managedClusterGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]ManagedClusterSummary, 0, len(items.Items))
	for i := range items.Items {
		result = append(result, NewManagedClusterSummary(&items.Items[i]))
	}
	return result, nil
}

func (e *Explorer) Subscriptions(ctx context.Context) ([]SubscriptionSummary, error) {
	if e.dynamic == nil {
		return []SubscriptionSummary{}, nil
	}
	items, err := e.dynamic.Resource(subscriptionGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]SubscriptionSummary, 0, len(items.Items))
	for i := range items.Items {
		result = append(result, NewSubscriptionSummary(&items.Items[i]))
	}
	return result, nil
}

func (e *Explorer) SubscriptionDetail(ctx context.Context, namespace string, name string) (SubscriptionDetail, error) {
	if e.dynamic == nil {
		return SubscriptionDetail{}, nil
	}
	subscription, err := e.dynamic.Resource(subscriptionGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return SubscriptionDetail{}, err
	}
	clusters, err := e.ManagedClusters(ctx)
	if err != nil {
		return SubscriptionDetail{}, err
	}
	return NewSubscriptionDetail(subscription, clusters), nil
}

func (e *Explorer) Manifests(ctx context.Context) ([]ManifestSummary, error) {
	if e.dynamic == nil {
		return []ManifestSummary{}, nil
	}
	items, err := e.dynamic.Resource(manifestGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]ManifestSummary, 0, len(items.Items))
	for i := range items.Items {
		result = append(result, NewManifestSummary(&items.Items[i]))
	}
	return result, nil
}

func (e *Explorer) Globalizations(ctx context.Context) ([]OverrideSummary, error) {
	items, err := e.listDynamic(ctx, globalizationGVR, metav1.NamespaceAll)
	if err != nil {
		return nil, err
	}
	result := make([]OverrideSummary, 0, len(items.Items))
	for i := range items.Items {
		result = append(result, NewOverrideSummary(&items.Items[i], "Globalization"))
	}
	return result, nil
}

func (e *Explorer) Localizations(ctx context.Context) ([]OverrideSummary, error) {
	items, err := e.listDynamic(ctx, localizationGVR, metav1.NamespaceAll)
	if err != nil {
		return nil, err
	}
	result := make([]OverrideSummary, 0, len(items.Items))
	for i := range items.Items {
		result = append(result, NewOverrideSummary(&items.Items[i], "Localization"))
	}
	return result, nil
}

func (e *Explorer) FeedInventories(ctx context.Context) ([]FeedInventorySummary, error) {
	items, err := e.listDynamic(ctx, feedInventoryGVR, metav1.NamespaceAll)
	if err != nil {
		return nil, err
	}
	result := make([]FeedInventorySummary, 0, len(items.Items))
	for i := range items.Items {
		result = append(result, NewFeedInventorySummary(&items.Items[i]))
	}
	return result, nil
}

func (e *Explorer) HelmReleases(ctx context.Context) ([]HelmReleaseSummary, error) {
	items, err := e.listDynamic(ctx, helmReleaseGVR, metav1.NamespaceAll)
	if err != nil {
		return nil, err
	}
	result := make([]HelmReleaseSummary, 0, len(items.Items))
	for i := range items.Items {
		result = append(result, NewHelmReleaseSummary(&items.Items[i]))
	}
	return result, nil
}

func (e *Explorer) listDynamic(ctx context.Context, gvr schema.GroupVersionResource, namespace string) (*unstructured.UnstructuredList, error) {
	return e.listDynamicWithOptions(ctx, gvr, namespace, metav1.ListOptions{})
}

func (e *Explorer) listDynamicWithOptions(ctx context.Context, gvr schema.GroupVersionResource, namespace string, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	if e.dynamic == nil {
		return &unstructured.UnstructuredList{}, nil
	}
	timeoutSeconds := int64(30)
	resource := e.dynamic.Resource(gvr).Namespace(namespace)
	opts.Limit, opts.TimeoutSeconds = 500, &timeoutSeconds
	result := &unstructured.UnstructuredList{}
	for {
		page, err := resource.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		result.Items = append(result.Items, page.Items...)
		opts.Continue = page.GetContinue()
		if opts.Continue == "" {
			return result, nil
		}
	}
}

func (e *Explorer) ClusternetResources() ([]APIResourceSummary, error) {
	resources, err := e.discoverClusternetResources()
	if resources != nil {
		return resources, nil
	}
	return nil, err
}

func (e *Explorer) discoverClusternetResources() ([]APIResourceSummary, error) {
	if e.discovery == nil {
		return []APIResourceSummary{}, nil
	}

	lists, err := e.discovery.ServerPreferredResources()
	if err != nil && len(lists) == 0 {
		return nil, err
	}

	result := []APIResourceSummary{}
	for _, list := range lists {
		if !strings.Contains(list.GroupVersion, "clusternet") {
			continue
		}
		group, version := parseGroupVersion(list.GroupVersion)
		for _, resource := range list.APIResources {
			result = append(result, APIResourceSummary{
				Name:       resource.Name,
				Group:      group,
				Version:    version,
				Kind:       resource.Kind,
				Namespaced: resource.Namespaced,
				Verbs:      resource.Verbs,
			})
		}
	}
	if groups, ok := discovery.GroupDiscoveryFailedErrorGroups(err); ok {
		for group := range groups {
			if strings.HasSuffix(group.Group, ".clusternet.io") {
				return result, err
			}
		}
		return result, nil
	}
	return result, err
}

func (e *Explorer) Diagnostics(ctx context.Context) (DiagnosticsSummary, error) {
	sectionErrors := map[string]string{}
	resources, err := e.discoverClusternetResources()
	if err != nil {
		sectionErrors["resources"] = string(apierrors.ReasonForError(err))
	}
	components, err := e.clusternetComponents(ctx)
	if err != nil {
		sectionErrors["components"] = string(apierrors.ReasonForError(err))
	}
	subscriptions, err := e.Subscriptions(ctx)
	if err != nil {
		sectionErrors["subscriptions"] = string(apierrors.ReasonForError(err))
	}
	if err := ctx.Err(); err != nil {
		return DiagnosticsSummary{}, err
	}
	return DiagnosticsSummary{
		Resources:                resources,
		Components:               components,
		CompletedReleasesSupport: completedReleasesSupport(subscriptions),
		ReadOnlyRBACChecks:       e.readOnlyRBACChecks(ctx),
		SectionErrors:            sectionErrors,
	}, nil
}

func (e *Explorer) clusternetComponents(ctx context.Context) ([]ComponentSummary, error) {
	if e.client == nil {
		return []ComponentSummary{}, nil
	}
	items, err := e.client.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]ComponentSummary, 0, len(items.Items))
	for _, item := range items.Items {
		if !isClusternetComponent(item.Name, item.Namespace, item.Labels) {
			continue
		}
		image := ""
		if len(item.Spec.Template.Spec.Containers) > 0 {
			image = item.Spec.Template.Spec.Containers[0].Image
		}
		desiredReplicas := int32(1)
		if item.Spec.Replicas != nil {
			desiredReplicas = *item.Spec.Replicas
		}
		component := NewComponentSummary(item.Name, item.Namespace, "Deployment", image, item.Status.ReadyReplicas, desiredReplicas)
		component.AvailableReplicas = item.Status.AvailableReplicas
		component.UnavailableReplicas = item.Status.UnavailableReplicas
		component.UpdatedReplicas = item.Status.UpdatedReplicas
		component.Generation = item.Generation
		component.ObservedGeneration = item.Status.ObservedGeneration
		for _, condition := range item.Status.Conditions {
			component.Conditions = append(component.Conditions, ResourceCondition{
				Type: string(condition.Type), Status: string(condition.Status),
				Reason: condition.Reason, Message: condition.Message,
				LastTransitionTime: formatTime(condition.LastTransitionTime),
			})
		}
		result = append(result, component)
	}
	return result, nil
}

func isClusternetComponent(name string, namespace string, labels map[string]string) bool {
	if strings.Contains(name, "clusternet") || strings.Contains(namespace, "clusternet") {
		return true
	}
	for _, value := range labels {
		if strings.Contains(value, "clusternet") {
			return true
		}
	}
	return false
}

func (e *Explorer) readOnlyRBACChecks(ctx context.Context) []RBACCheckSummary {
	if e.client == nil {
		return []RBACCheckSummary{}
	}
	checks := []RBACCheckSummary{
		{Group: registrationGVR.Group, Resource: registrationGVR.Resource, Verb: "list"},
		{Group: helmChartGVR.Group, Resource: helmChartGVR.Resource, Verb: "list"},
		{Group: baseGVR.Group, Resource: baseGVR.Resource, Verb: "list"},
		{Group: managedClusterGVR.Group, Resource: managedClusterGVR.Resource, Verb: "list"},
		{Group: subscriptionGVR.Group, Resource: subscriptionGVR.Resource, Verb: "list"},
		{Group: subscriptionGVR.Group, Resource: subscriptionGVR.Resource, Verb: "get"},
		{Group: helmReleaseGVR.Group, Resource: helmReleaseGVR.Resource, Verb: "list"},
		{Group: descriptionGVR.Group, Resource: descriptionGVR.Resource, Verb: "get"},
		{Group: manifestGVR.Group, Resource: manifestGVR.Resource, Verb: "list"},
		{Group: globalizationGVR.Group, Resource: globalizationGVR.Resource, Verb: "list"},
		{Group: localizationGVR.Group, Resource: localizationGVR.Resource, Verb: "list"},
		{Group: feedInventoryGVR.Group, Resource: feedInventoryGVR.Resource, Verb: "list"},
		{Group: "apps", Resource: "deployments", Verb: "list"},
		{Resource: "events", Verb: "list"},
	}
	for i := range checks {
		review := &authv1.SelfSubjectAccessReview{Spec: authv1.SelfSubjectAccessReviewSpec{ResourceAttributes: &authv1.ResourceAttributes{Group: checks[i].Group, Resource: checks[i].Resource, Verb: checks[i].Verb}}}
		result, err := e.client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
		if err != nil {
			checks[i].Error = err.Error()
			continue
		}
		checks[i].Allowed = result.Status.Allowed
		if !result.Status.Allowed && !result.Status.Denied {
			checks[i].Error = result.Status.EvaluationError
		}
	}
	return checks
}

func completedReleasesSupport(subscriptions []SubscriptionSummary) string {
	if len(subscriptions) == 0 {
		return "unknown"
	}
	for _, subscription := range subscriptions {
		if subscription.CompletionKnown {
			return "supported"
		}
	}
	return "not-observed"
}

func parseGroupVersion(groupVersion string) (string, string) {
	parts := strings.Split(groupVersion, "/")
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}
