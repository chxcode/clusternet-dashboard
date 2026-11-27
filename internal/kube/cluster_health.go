package kube

import (
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ResourceCondition struct {
	Type               string `json:"type"`
	Status             string `json:"status,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
}

type ClusterTaint struct {
	Key    string `json:"key"`
	Effect string `json:"effect,omitempty"`
}

type ClusterEvidence struct {
	CreatedAt                 string              `json:"createdAt,omitempty"`
	ClusterCIDRs              []string            `json:"clusterCIDRs,omitempty"`
	ServiceCIDRs              []string            `json:"serviceCIDRs,omitempty"`
	AppPusher                 *bool               `json:"appPusher,omitempty"`
	UseSocket                 *bool               `json:"useSocket,omitempty"`
	PredictorEnabled          *bool               `json:"predictorEnabled,omitempty"`
	PredictorDirectAccess     *bool               `json:"predictorDirectAccess,omitempty"`
	PredictorEndpoint         string              `json:"predictorEndpoint,omitempty"`
	Conditions                []ResourceCondition `json:"conditions,omitempty"`
	Readyz                    *bool               `json:"readyz,omitempty"`
	Livez                     *bool               `json:"livez,omitempty"`
	Healthz                   *bool               `json:"healthz,omitempty"`
	HeartbeatFrequencySeconds *int64              `json:"heartbeatFrequencySeconds,omitempty"`
	ClusterType               string              `json:"clusterType,omitempty"`
	SyncMode                  string              `json:"syncMode,omitempty"`
	Taints                    []ClusterTaint      `json:"taints"`
	NodeStatistics            map[string]int64    `json:"nodeStatistics,omitempty"`
	Capacity                  map[string]string   `json:"capacity,omitempty"`
	Allocatable               map[string]string   `json:"allocatable,omitempty"`
}

func newClusterEvidence(cluster *unstructured.Unstructured) ClusterEvidence {
	evidence := ClusterEvidence{
		CreatedAt:             formatTime(cluster.GetCreationTimestamp()),
		AppPusher:             reportedProbe(cluster, "appPusher"),
		UseSocket:             reportedProbe(cluster, "useSocket"),
		PredictorEnabled:      reportedProbe(cluster, "predictorEnabled"),
		PredictorDirectAccess: reportedProbe(cluster, "predictorDirectAccess"),
		PredictorEndpoint:     reportedEndpoint(getNestedString(cluster, "status", "predictorAddress")),
		ClusterCIDRs:          reportedCIDRs(getNestedString(cluster, "status", "clusterCIDR")),
		ServiceCIDRs:          reportedCIDRs(getNestedString(cluster, "status", "serviceCIDR")),
		Readyz:                reportedProbe(cluster, "readyz"),
		Livez:                 reportedProbe(cluster, "livez"),
		Healthz:               reportedProbe(cluster, "healthz"),
		ClusterType:           getNestedString(cluster, "spec", "clusterType"),
		SyncMode:              getNestedString(cluster, "spec", "syncMode"),
		Taints:                make([]ClusterTaint, 0),
		NodeStatistics:        make(map[string]int64),
		Capacity:              reportedResources(cluster, "capacity"),
		Allocatable:           reportedResources(cluster, "allocatable"),
	}
	if seconds, found := getNestedInt64Found(cluster, "status", "heartbeatFrequencySeconds"); found {
		evidence.HeartbeatFrequencySeconds = &seconds
	}
	for _, key := range []string{"readyNodes", "notReadyNodes", "unknownNodes", "lostNodes"} {
		if count, found := getNestedInt64Found(cluster, "status", "nodeStatistics", key); found {
			evidence.NodeStatistics[key] = count
		}
	}
	taints, _, _ := unstructured.NestedSlice(cluster.Object, "spec", "taints")
	for _, item := range taints {
		fields, ok := item.(map[string]any)
		if !ok {
			continue
		}
		key, _ := fields["key"].(string)
		if key != "" {
			effect, _ := fields["effect"].(string)
			evidence.Taints = append(evidence.Taints, ClusterTaint{Key: key, Effect: effect})
		}
	}
	conditions, _, _ := unstructured.NestedSlice(cluster.Object, "status", "conditions")
	for _, item := range conditions {
		fields, ok := item.(map[string]any)
		if !ok {
			continue
		}
		condition := &unstructured.Unstructured{Object: fields}
		kind := getNestedString(condition, "type")
		if kind == "" {
			continue
		}
		evidence.Conditions = append(evidence.Conditions, ResourceCondition{
			Type:               kind,
			Status:             getNestedString(condition, "status"),
			Reason:             getNestedString(condition, "reason"),
			Message:            getNestedString(condition, "message"),
			LastTransitionTime: getNestedString(condition, "lastTransitionTime"),
		})
	}
	return evidence
}

func reportedCIDRs(value string) []string {
	var cidrs []string
	for _, item := range strings.Split(value, ",") {
		if prefix, err := netip.ParsePrefix(strings.TrimSpace(item)); err == nil {
			cidrs = append(cidrs, prefix.String())
		}
	}
	return cidrs
}

// Show the network endpoint without URL credentials, paths, query strings or fragments.
func reportedEndpoint(value string) string {
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "//" + value
	}
	ep, err := url.Parse(value)
	if err != nil || ep.Hostname() == "" || (ep.Scheme != "" && ep.Scheme != "http" && ep.Scheme != "https") {
		return ""
	}
	if ep.Scheme == "" {
		return ep.Host
	}
	return ep.Scheme + "://" + ep.Host
}

func reportedProbe(cluster *unstructured.Unstructured, key string) *bool {
	value, found, _ := unstructured.NestedBool(cluster.Object, "status", key)
	if !found {
		return nil
	}
	return &value
}

func reportedResources(cluster *unstructured.Unstructured, key string) map[string]string {
	quantities := make(map[string]string)
	for _, resource := range []string{"cpu", "memory", "pods"} {
		value, _, _ := unstructured.NestedFieldNoCopy(cluster.Object, "status", key, resource)
		switch quantity := value.(type) {
		case string:
			if quantity != "" {
				quantities[resource] = quantity
			}
		case int64:
			quantities[resource] = strconv.FormatInt(quantity, 10)
		}
	}
	return quantities
}
