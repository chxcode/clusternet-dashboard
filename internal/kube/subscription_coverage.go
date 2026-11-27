package kube

import (
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
)

type ClusterReference struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type SubscriptionCoverage struct {
	RegisteredClusters   int                `json:"registeredClusters"`
	LabelMatchedClusters *int               `json:"labelMatchedClusters,omitempty"`
	UnboundMatches       []ClusterReference `json:"unboundMatches,omitempty"`
}

// Count each registered object once, even when it matches multiple subscribers.
// Scheduler filters and status.bindingClusters are separate from this label snapshot.
func subscriptionCoverage(distribution SubscriptionDistribution, clusters []ManagedClusterSummary, bindings []string) SubscriptionCoverage {
	result := SubscriptionCoverage{RegisteredClusters: len(clusters)}
	var queries []labels.Selector
	switch distribution.TargetScope {
	case "all":
		queries = append(queries, labels.Everything())
	case "none":
	case "selector":
		for _, raw := range distribution.Selectors {
			if raw == nil {
				continue
			}
			selector, err := labels.Parse(*raw)
			if err != nil {
				return result
			}
			queries = append(queries, selector)
		}
	default:
		return result
	}
	bound := registeredBindingKeys(clusters, bindings)
	matched := 0
	for _, cluster := range clusters {
		for _, query := range queries {
			if !query.Matches(labels.Set(cluster.Labels)) {
				continue
			}
			matched++
			key := ClusterReference{Namespace: cluster.Namespace, Name: cluster.Name}
			if bound != nil && !bound[key] {
				result.UnboundMatches = append(result.UnboundMatches, key)
			}
			break
		}
	}
	sort.Slice(result.UnboundMatches, func(i, j int) bool {
		a, b := result.UnboundMatches[i], result.UnboundMatches[j]
		if a.Namespace == b.Namespace {
			return a.Name < b.Name
		}
		return a.Namespace < b.Namespace
	})
	result.LabelMatchedClusters = &matched
	return result
}

// Older bindings can omit the namespace. An ambiguous name cannot establish
// which of the registered clusters is missing from the report.
func registeredBindingKeys(clusters []ManagedClusterSummary, bindings []string) map[ClusterReference]bool {
	if bindings == nil {
		return nil
	}
	byName := make(map[string][]ClusterReference, len(clusters))
	for _, c := range clusters {
		byName[c.Name] = append(byName[c.Name], ClusterReference{c.Namespace, c.Name})
	}
	bound := make(map[ClusterReference]bool, len(bindings))
	for _, binding := range bindings {
		namespace, name, qualified := strings.Cut(binding, "/")
		if qualified {
			if namespace == "" || name == "" || strings.Contains(name, "/") {
				return nil
			}
			bound[ClusterReference{namespace, name}] = true
			continue
		}
		if binding == "" || len(byName[binding]) > 1 {
			return nil
		}
		if matches := byName[binding]; len(matches) == 1 {
			bound[matches[0]] = true
		}
	}
	return bound
}
