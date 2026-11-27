package kube

import (
	"errors"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
)

type deploymentScope struct {
	mode      string
	requested map[ClusterReference]DeploymentTarget
	affinity  metav1.LabelSelector
	query     labels.Selector
}

func parseDeploymentScope(req DeploymentRequest) (deploymentScope, error) {
	var scope deploymentScope
	switch req.TargetMode {
	case "", "fixed":
		scope.mode = "fixed"
		if req.Selector != "" || len(req.Targets) == 0 {
			return scope, apierrors.NewBadRequest("fixed mode requires at least one managed cluster and no selector")
		}
		scope.requested = make(map[ClusterReference]DeploymentTarget, len(req.Targets))
		uids := sets.New[string]()
		for _, t := range req.Targets {
			key := ClusterReference{Namespace: t.Namespace, Name: t.Name}
			if len(validation.IsDNS1123Label(t.Namespace)) != 0 || len(validation.IsDNS1123Subdomain(t.Name)) != 0 ||
				strings.TrimSpace(t.UID) == "" || t.UID != strings.TrimSpace(t.UID) || scope.requested[key].UID != "" || uids.Has(t.UID) {
				return scope, apierrors.NewBadRequest("invalid or duplicate managed cluster identity")
			}
			scope.requested[key] = t
			uids.Insert(t.UID)
		}
		return scope, nil
	case "all":
		if req.Targets != nil || req.Selector != "" {
			return scope, apierrors.NewBadRequest("all mode does not accept targets or a selector")
		}
	case "selector":
		if req.Targets != nil || strings.TrimSpace(req.Selector) == "" {
			return scope, apierrors.NewBadRequest("selector mode requires a nonempty label selector and no fixed targets")
		}
		affinity, err := parseDeploymentAffinity(req.Selector)
		if err != nil {
			return scope, apierrors.NewBadRequest(err.Error())
		}
		scope.affinity = affinity
	default:
		return scope, apierrors.NewBadRequest("targetMode must be fixed, selector or all")
	}
	scope.mode = req.TargetMode
	var err error
	scope.query, err = metav1.LabelSelectorAsSelector(&scope.affinity)
	return scope, err
}

// Preserve Kubernetes absent-key semantics for != and notin. Never turn a
// conflicting pair of equalities into a single matchLabels entry.
func parseDeploymentAffinity(text string) (metav1.LabelSelector, error) {
	var affinity metav1.LabelSelector
	reqs, err := labels.ParseToRequirements(text)
	if err != nil {
		return affinity, err
	}
	if len(reqs) == 0 {
		return affinity, errors.New("empty affinity is only allowed in all mode")
	}
	if !requirementsCanCoexist(reqs) {
		return affinity, errors.New("label selector contains mutually exclusive requirements")
	}
	for _, r := range reqs {
		values := r.Values().List()
		var op metav1.LabelSelectorOperator
		switch r.Operator() {
		case selection.Equals, selection.DoubleEquals:
			if affinity.MatchLabels == nil {
				affinity.MatchLabels = map[string]string{}
			}
			affinity.MatchLabels[r.Key()] = values[0]
			continue
		case selection.In:
			op = metav1.LabelSelectorOpIn
		case selection.NotEquals, selection.NotIn:
			op = metav1.LabelSelectorOpNotIn
		case selection.Exists:
			op = metav1.LabelSelectorOpExists
		case selection.DoesNotExist:
			op = metav1.LabelSelectorOpDoesNotExist
		default:
			return affinity, errors.New("clusterAffinity supports equality, in, notin, existence and absence")
		}
		affinity.MatchExpressions = append(affinity.MatchExpressions, metav1.LabelSelectorRequirement{Key: r.Key(), Operator: op, Values: values})
	}
	sort.Slice(affinity.MatchExpressions, func(i, j int) bool {
		a, b := affinity.MatchExpressions[i], affinity.MatchExpressions[j]
		if a.Key != b.Key {
			return a.Key < b.Key
		}
		if a.Operator != b.Operator {
			return a.Operator < b.Operator
		}
		return strings.Join(a.Values, ",") < strings.Join(b.Values, ",")
	})
	return affinity, nil
}

// Label selectors are conjunctions of independent per-key requirements. An
// intersection exists if each key may either be absent, or take some value
// satisfying all of its positive and negative requirements.
func requirementsCanCoexist(requirements labels.Requirements) bool {
	type domain struct {
		required, forbidden bool
		allowed, excluded   sets.Set[string]
	}
	keys := map[string]*domain{}
	for _, r := range requirements {
		d := keys[r.Key()]
		if d == nil {
			d = &domain{excluded: sets.New[string]()}
			keys[r.Key()] = d
		}
		switch r.Operator() {
		case selection.In, selection.Equals, selection.DoubleEquals:
			values := sets.New(r.Values().List()...)
			d.required = true
			if d.allowed == nil {
				d.allowed = values
			} else {
				d.allowed = d.allowed.Intersection(values)
			}
		case selection.NotIn, selection.NotEquals:
			d.excluded.Insert(r.Values().List()...)
		case selection.Exists:
			d.required = true
		case selection.DoesNotExist:
			d.forbidden = true
		}
	}
	for _, d := range keys {
		if d.required && (d.forbidden || (d.allowed != nil && d.allowed.Difference(d.excluded).Len() == 0)) {
			return false
		}
	}
	return true
}

// Kubernetes sorts selector terms by key only. Include the operator and values
// in the ordering so multiple requirements on one key also fingerprint stably.
func normalizedDeploymentSelector(query labels.Selector) string {
	requirements, _ := query.Requirements()
	terms := make([]string, 0, len(requirements))
	for _, r := range requirements {
		terms = append(terms, r.String())
	}
	sort.Strings(terms)
	return strings.Join(terms, ",")
}

func deploymentRulesMayIntersect(query labels.Selector, other SubscriptionDistribution) bool {
	if other.TargetScope == "unknown" || other.TargetScope == "all" {
		return true
	}
	requirements, _ := query.Requirements()
	for _, text := range other.Selectors {
		// Upstream's nil clusterAffinity matches nothing.
		if text == nil {
			continue
		}
		r, err := labels.ParseToRequirements(*text)
		if err != nil || requirementsCanCoexist(append(r, requirements...)) {
			return true
		}
	}
	return false
}

// Non-identity labels can change after preview. Requirements are ANDed, so
// retaining only cluster-id requirements gives every possible future match.
func deploymentIdentityFilter(selector labels.Selector) labels.Selector {
	requirements, _ := selector.Requirements()
	var identity labels.Requirements
	for _, r := range requirements {
		if r.Key() == deploymentIDLabel {
			identity = append(identity, r)
		}
	}
	return labels.NewSelector().Add(identity...)
}
