package kube

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func labelSelectorText(raw map[string]any) (string, error) {
	var selector metav1.LabelSelector
	if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(raw, &selector, true); err != nil {
		return "", err
	}
	query, err := metav1.LabelSelectorAsSelector(&selector)
	if err != nil {
		return "", err
	}
	return query.String(), nil
}

// TargetScope describes the union of subscriber affinities, before taints,
// subgroup limits, resource constraints and other scheduling filters.
type SubscriptionDistribution struct {
	Strategy            string    `json:"strategy"`
	TargetScope         string    `json:"targetScope"`
	Selectors           []*string `json:"selectors"`
	DividingType        string    `json:"dividingType,omitempty"`
	DynamicStrategy     string    `json:"dynamicStrategy,omitempty"`
	SubgroupConfigured  *bool     `json:"subgroupConfigured,omitempty"`
	SubgroupMinClusters []*int64  `json:"subgroupMinClusters,omitempty"`
	SubscriberWeights   []*int64  `json:"subscriberWeights,omitempty"`
}

func subscriptionDistribution(subscription *unstructured.Unstructured) SubscriptionDistribution {
	strategy, found, err := unstructured.NestedString(subscription.Object, "spec", "schedulingStrategy")
	if !found && err == nil {
		strategy = "Replication" // Subscription CRD default.
	}
	d := SubscriptionDistribution{Strategy: strategy, TargetScope: "none", Selectors: []*string{},
		SubgroupConfigured: optionalSpecBool(subscription, "schedulingBySubGroup")}
	if strategy == "Dividing" {
		dividing, found, err := unstructured.NestedMap(subscription.Object, "spec", "dividingScheduling")
		if found && err == nil {
			d.DividingType = defaultedStrategy(dividing, "type", "Static")
			if d.DividingType == "Dynamic" {
				dynamic, found, err := unstructured.NestedMap(dividing, "dynamicDividing")
				if found && err == nil {
					d.DynamicStrategy = defaultedStrategy(dynamic, "strategy", "Spread")
				}
			}
		}
	}
	raw, _, err := unstructured.NestedFieldNoCopy(subscription.Object, "spec", "subscribers")
	subscribers, ok := raw.([]any)
	if err != nil || (raw != nil && !ok) {
		d.TargetScope = "unknown"
		return d
	}
	d.Selectors = make([]*string, len(subscribers))
	if d.DividingType == "Static" {
		d.SubscriberWeights = make([]*int64, len(subscribers))
	}
	if d.SubgroupConfigured != nil && *d.SubgroupConfigured {
		d.SubgroupMinClusters = make([]*int64, len(subscribers))
	}
	invalid := false
	for i, item := range subscribers {
		subscriber, ok := item.(map[string]any)
		if !ok {
			invalid = true
			continue
		}
		if d.SubscriberWeights != nil {
			if weight, found, err := unstructured.NestedInt64(subscriber, "weight"); found && err == nil && weight >= 0 {
				d.SubscriberWeights[i] = &weight
			}
		}
		if d.SubgroupMinClusters != nil {
			if subgroup, ok := subscriber["subGroupStrategy"].(map[string]any); ok {
				// A present SubGroupStrategy defaults its integer minClusters to zero.
				minimum, _, err := unstructured.NestedInt64(subgroup, "minClusters")
				if err == nil && minimum >= 0 {
					d.SubgroupMinClusters[i] = &minimum
				}
			}
		}
		// The scheduler's LabelSelectorAsSelector(nil) matches nothing.
		// A present empty LabelSelector matches all labels.
		if subscriber["clusterAffinity"] == nil {
			continue
		}
		affinity, ok := subscriber["clusterAffinity"].(map[string]any)
		if !ok {
			invalid = true
			continue
		}
		text, err := labelSelectorText(affinity)
		if err != nil {
			invalid = true
			continue
		}
		d.Selectors[i] = &text
		if text == "" {
			d.TargetScope = "all"
		} else if d.TargetScope == "none" {
			d.TargetScope = "selector"
		}
	}
	// The upstream scheduler aborts its candidate lookup if any affinity fails.
	if invalid {
		d.TargetScope = "unknown"
	}
	return d
}

func defaultedStrategy(settings map[string]any, field, fallback string) string {
	if _, present := settings[field]; !present {
		return fallback
	}
	return stringValue(settings[field])
}

func distinctSubscriptionBindings(subscription *unstructured.Unstructured) []string {
	keys, found, err := unstructured.NestedStringSlice(subscription.Object, "status", "bindingClusters")
	if !found || err != nil {
		return nil
	}
	seen := make(map[string]bool, len(keys))
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		if key != "" && !seen[key] {
			result = append(result, key)
			seen[key] = true
		}
	}
	return result
}
