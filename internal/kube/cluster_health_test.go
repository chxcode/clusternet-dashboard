package kube

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestClusterEvidencePreservesReportedValues(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{"syncMode": "Pull", "clusterType": "Edge", "taints": []any{
			map[string]any{"key": "zone", "effect": "NoSchedule", "value": "unlisted"},
		}},
		"status": map[string]any{
			"platform":                  "linux/amd64",
			"readyz":                    false,
			"livez":                     true,
			"heartbeatFrequencySeconds": int64(20),
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "Unknown", "reason": "StatusUnknown",
					"message": "Status report delayed", "lastTransitionTime": "2026-04-10T08:50:00Z"},
				map[string]any{"reason": "Invalid condition"},
			},
			"nodeStatistics": map[string]any{"readyNodes": int64(3), "notReadyNodes": int64(0), "unlisted": int64(2)},
			"capacity":       map[string]any{"cpu": "4", "memory": "16Gi", "pods": int64(0), "unlisted": "unlisted"},
			"allocatable":    map[string]any{"cpu": "3500m"},
			"clusterCIDR":    "unlisted",
		},
	}}
	summary := NewManagedClusterSummary(cluster)
	evidence := summary.Evidence
	if summary.Platform != "linux/amd64" || evidence.SyncMode != "Pull" || evidence.ClusterType != "Edge" ||
		evidence.HeartbeatFrequencySeconds == nil || *evidence.HeartbeatFrequencySeconds != 20 {
		t.Fatalf("unexpected cluster reporting settings: %#v", summary)
	}
	if evidence.Readyz == nil || *evidence.Readyz || evidence.Livez == nil || !*evidence.Livez || evidence.Healthz != nil {
		t.Fatalf("unexpected probe states: %#v", evidence)
	}
	wantCondition := ResourceCondition{Type: "Ready", Status: "Unknown", Reason: "StatusUnknown",
		Message: "Status report delayed", LastTransitionTime: "2026-04-10T08:50:00Z"}
	if !reflect.DeepEqual(evidence.Conditions, []ResourceCondition{wantCondition}) {
		t.Fatalf("unexpected conditions: %#v", evidence.Conditions)
	}
	if !reflect.DeepEqual(evidence.Taints, []ClusterTaint{{Key: "zone", Effect: "NoSchedule"}}) {
		t.Fatalf("unexpected taints: %#v", evidence.Taints)
	}
	if !reflect.DeepEqual(evidence.NodeStatistics, map[string]int64{"readyNodes": 3, "notReadyNodes": 0}) ||
		!reflect.DeepEqual(evidence.Capacity, map[string]string{"cpu": "4", "memory": "16Gi", "pods": "0"}) ||
		!reflect.DeepEqual(evidence.Allocatable, map[string]string{"cpu": "3500m"}) {
		t.Fatalf("unexpected resource snapshot: %#v", evidence)
	}
	encoded, err := json.Marshal(evidence)
	if err != nil || !strings.Contains(string(encoded), `"readyz":false`) ||
		strings.Contains(string(encoded), "healthz") || strings.Contains(string(encoded), "unlisted") {
		t.Fatalf("unexpected public fields: %s, %v", encoded, err)
	}
}

func TestClusterEvidenceMissingStatus(t *testing.T) {
	cluster := &unstructured.Unstructured{}
	encoded, err := json.Marshal(NewManagedClusterSummary(cluster).Evidence)
	if err != nil || string(encoded) != `{"taints":[]}` {
		t.Fatalf("missing status should be omitted and absent spec.taints should be empty, got %s, %v", encoded, err)
	}
}

func TestClusterNetworkSummaryShowsReportedCapabilitiesAndProjectsEndpoints(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{"clusterId": "example-id"},
		"status": map[string]any{
			"apiserverURL":     "https://user:example@api.example.test:6443/path?private=example",
			"predictorAddress": "predictor.example.test:8080/path", "appPusher": false,
			"clusterCIDR": "192.0.2.0/24, 2001:db8::/48", "useSocket": true,
		},
	}}
	c := NewManagedClusterSummary(obj)
	if c.ClusterID != "example-id" || c.APIServerURL != "https://api.example.test:6443" || c.Evidence.PredictorEndpoint != "predictor.example.test:8080" {
		t.Fatal("unexpected endpoint or cluster identity")
	}
	if c.Evidence.UseSocket == nil || !*c.Evidence.UseSocket || c.Evidence.AppPusher == nil || *c.Evidence.AppPusher || c.Evidence.PredictorEnabled != nil ||
		!reflect.DeepEqual(c.Evidence.ClusterCIDRs, []string{"192.0.2.0/24", "2001:db8::/48"}) {
		t.Fatal("invalid network capability summary")
	}
}
