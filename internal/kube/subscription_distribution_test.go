package kube

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSubscriptionDistributionAffinityUnion(t *testing.T) {
	all := map[string]any{"clusterAffinity": map[string]any{}}
	west := map[string]any{"clusterAffinity": map[string]any{"matchLabels": map[string]any{"region": "west"}}}
	gpu := map[string]any{"clusterAffinity": map[string]any{"matchExpressions": []any{
		map[string]any{"key": "gpu", "operator": "Exists"},
		map[string]any{"key": "team", "operator": "In", "values": []any{"sre", "qa"}},
	}}}
	invalid := map[string]any{"clusterAffinity": map[string]any{"matchExpressions": []any{map[string]any{"key": "team", "operator": "Bad"}}}}
	for _, tc := range []struct {
		name        string
		subscribers any
		scope       string
		selectors   string
	}{
		{"missing subscribers", nil, "none", "[]"},
		{"empty subscribers", []any{}, "none", "[]"},
		{"missing affinity", []any{map[string]any{}, map[string]any{"clusterAffinity": nil}}, "none", "[null,null]"},
		{"explicit empty", []any{all}, "all", `[""]`},
		{"OR different subscribers AND within each", []any{west, gpu}, "selector", `["region=west","gpu,team in (qa,sre)"]`},
		{"missing entry does not widen", []any{west, map[string]any{}}, "selector", `["region=west",null]`},
		{"any explicit all widens", []any{west, all, map[string]any{}}, "all", `["region=west","",null]`},
		{"invalid condition stops evaluation", []any{all, invalid}, "unknown", `["",null]`},
		{"unrecognized selector shape", []any{map[string]any{"clusterAffinity": map[string]any{"futureSelector": "x"}}}, "unknown", `[null]`},
		{"wrong affinity type", []any{map[string]any{"clusterAffinity": "all"}}, "unknown", `[null]`},
		{"malformed subscriber entry", []any{"all"}, "unknown", `[null]`},
		{"malformed subscriber list", "all", "unknown", `[]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := map[string]any{}
			if tc.subscribers != nil {
				spec["subscribers"] = tc.subscribers
			}
			obj := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
			summary := NewSubscriptionSummary(obj)
			policy := summary.Distribution
			raw, err := json.Marshal(policy.Selectors)
			if err != nil {
				t.Fatal(err)
			}
			if policy.Strategy != "Replication" || policy.TargetScope != tc.scope || string(raw) != tc.selectors {
				t.Fatalf("got strategy %q scope %q selectors %s", policy.Strategy, policy.TargetScope, raw)
			}
			detail := NewSubscriptionDetail(obj, nil)
			if !reflect.DeepEqual(detail.Summary.Distribution, policy) || !reflect.DeepEqual(detail.SubscriberSelectors, policy.Selectors) {
				t.Fatalf("list and detail disagree: %+v vs %+v", policy, detail.Summary.Distribution)
			}
		})
	}
}

func TestSubscriptionDividingModesAndSubgroupLimits(t *testing.T) {
	for _, tc := range []struct {
		name          string
		strategy      string
		dividing      map[string]any
		mode, dynamic string
	}{
		{"replication ignores dividing options", "Replication", map[string]any{"type": "Dynamic"}, "", ""},
		{"absent dividing settings", "Dividing", nil, "", ""},
		{"default static type", "Dividing", map[string]any{}, "Static", ""},
		{"static ignores dynamic options", "Dividing", map[string]any{"type": "Static", "dynamicDividing": map[string]any{"strategy": "Binpack"}}, "Static", ""},
		{"missing required dynamic config", "Dividing", map[string]any{"type": "Dynamic"}, "Dynamic", ""},
		{"default dynamic strategy", "Dividing", map[string]any{"type": "Dynamic", "dynamicDividing": map[string]any{}}, "Dynamic", "Spread"},
		{"binpack", "Dividing", map[string]any{"type": "Dynamic", "dynamicDividing": map[string]any{"strategy": "Binpack"}}, "Dynamic", "Binpack"},
		{"new strategy preserved", "FutureStrategy", map[string]any{"type": "Static"}, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := map[string]any{"schedulingStrategy": tc.strategy, "schedulerName": "example-scheduler"}
			if tc.dividing != nil {
				spec["dividingScheduling"] = tc.dividing
			}
			obj := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
			sum := NewSubscriptionSummary(obj)
			d := sum.Distribution
			if d.Strategy != tc.strategy || d.DividingType != tc.mode || d.DynamicStrategy != tc.dynamic || sum.SchedulerName != "example-scheduler" || d.SubgroupConfigured != nil {
				t.Fatalf("incorrect distribution: %+v", d)
			}
		})
	}
	obj := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"schedulingBySubGroup": true, "subscribers": []any{
			map[string]any{"clusterAffinity": map[string]any{}, "subGroupStrategy": map[string]any{"minClusters": int64(2)}},
			map[string]any{"clusterAffinity": map[string]any{}, "subGroupStrategy": map[string]any{}},
			map[string]any{"clusterAffinity": map[string]any{}},
		},
	}}}
	d := NewSubscriptionSummary(obj).Distribution
	raw, _ := json.Marshal(d.SubgroupMinClusters)
	if d.SubgroupConfigured == nil || !*d.SubgroupConfigured || string(raw) != "[2,0,null]" {
		t.Fatalf("incorrect subgroup configuration: %+v", d)
	}
	_ = unstructured.SetNestedField(obj.Object, false, "spec", "schedulingBySubGroup")
	d = NewSubscriptionSummary(obj).Distribution
	if d.SubgroupConfigured == nil || *d.SubgroupConfigured || d.SubgroupMinClusters != nil {
		t.Fatalf("disabled subgroup treated as active: %+v", d)
	}
}

func TestDuplicateBindingsRepresentOneTarget(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{"status": map[string]any{
		"bindingClusters":    []any{"ns/b", "", "ns/a", "ns/b", "other/a", "ns/a"},
		"aggregatedStatuses": []any{map[string]any{"name": "one", "feedStatusDetails": []any{map[string]any{"clusterName": "b", "available": true}}}},
	}}}
	list, detail := NewSubscriptionSummary(obj), NewSubscriptionDetail(obj, nil)
	for _, summary := range []SubscriptionSummary{list, detail.Summary} {
		if summary.BindingClusterCount != 3 || strings.Join(summary.BindingClusters, ",") != "ns/b,ns/a,other/a" || summary.ObservedReleases != 1 {
			t.Fatalf("invalid binding count or order: %+v", summary)
		}
	}
	if len(detail.TargetClusters) != 3 || !detail.TargetClusters[0].Observed {
		t.Fatalf("duplicate bindings made an unambiguous report indistinguishable: %+v", detail.TargetClusters)
	}
}

func TestDistributionStaticWeightsPreserveSubscriberOrder(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"schedulingStrategy": "Dividing", "dividingScheduling": map[string]any{"type": "Static"},
		"subscribers": []any{map[string]any{"weight": int64(3)}, map[string]any{}, map[string]any{"weight": int64(0)}},
	}}}
	distribution := subscriptionDistribution(obj)
	raw, err := json.Marshal(distribution.SubscriberWeights)
	if err != nil || string(raw) != "[3,null,0]" {
		t.Fatalf("invalid weights: %s, %v", raw, err)
	}
	_ = unstructured.SetNestedField(obj.Object, "Replication", "spec", "schedulingStrategy")
	if subscriptionDistribution(obj).SubscriberWeights != nil {
		t.Fatal("weights applied outside static dividing")
	}
}

func TestSubscriptionBindingsDistinguishUnavailableFromEmpty(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   map[string]any
		reported bool
	}{
		{"missing", map[string]any{}, false},
		{"null", map[string]any{"bindingClusters": nil}, false},
		{"invalid", map[string]any{"bindingClusters": []any{"z/a", int64(7)}}, false},
		{"empty", map[string]any{"bindingClusters": []any{}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{Object: map[string]any{"status": tc.status}}
			bindings := NewSubscriptionSummary(obj).BindingClusters
			if len(bindings) != 0 || (bindings != nil) != tc.reported {
				t.Fatalf("unexpected reported bindings: %#v", bindings)
			}
		})
	}
}
