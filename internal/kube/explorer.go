package kube

import (
	"context"
	"strings"

	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	client    kubernetes.Interface
	discovery discovery.DiscoveryInterface
	dynamic   dynamic.Interface
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
	if e.dynamic == nil {
		return &unstructured.UnstructuredList{}, nil
	}
	timeoutSeconds := int64(30)
	return e.dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{Limit: 500, TimeoutSeconds: &timeoutSeconds})
}

func (e *Explorer) ClusternetResources() ([]APIResourceSummary, error) {
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
	return result, nil
}

func (e *Explorer) Diagnostics(ctx context.Context) (DiagnosticsSummary, error) {
	resources, err := e.ClusternetResources()
	if err != nil {
		return DiagnosticsSummary{}, err
	}
	components, err := e.clusternetComponents(ctx)
	if err != nil {
		return DiagnosticsSummary{}, err
	}
	subscriptions, err := e.Subscriptions(ctx)
	if err != nil {
		return DiagnosticsSummary{}, err
	}
	return DiagnosticsSummary{
		Resources:                resources,
		Components:               components,
		CompletedReleasesSupport: completedReleasesSupport(subscriptions),
		ReadOnlyRBACChecks:       e.readOnlyRBACChecks(ctx),
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
		result = append(result, NewComponentSummary(item.Name, item.Namespace, "Deployment", image, item.Status.ReadyReplicas, desiredReplicas))
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
	checks := []RBACCheckSummary{
		{Group: managedClusterGVR.Group, Resource: managedClusterGVR.Resource},
		{Group: subscriptionGVR.Group, Resource: subscriptionGVR.Resource},
		{Group: manifestGVR.Group, Resource: manifestGVR.Resource},
	}
	if e.client == nil {
		return checks
	}
	for i := range checks {
		review := &authv1.SelfSubjectAccessReview{Spec: authv1.SelfSubjectAccessReviewSpec{ResourceAttributes: &authv1.ResourceAttributes{Group: checks[i].Group, Resource: checks[i].Resource, Verb: "list"}}}
		result, err := e.client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
		if err != nil {
			checks[i].Error = err.Error()
			continue
		}
		checks[i].Allowed = result.Status.Allowed
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
