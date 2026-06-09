package kube

import "testing"

func TestNewComponentSummaryExtractsRoleAndVersionFromImage(t *testing.T) {
	component := NewComponentSummary("clusternet-scheduler", "clusternet-system", "Deployment", "ghcr.io/clusternet/clusternet-scheduler:v0.18.1", 1, 1)

	if component.Role != "scheduler" {
		t.Fatalf("expected scheduler role, got %q", component.Role)
	}
	if component.Version != "v0.18.1" {
		t.Fatalf("expected image tag version, got %q", component.Version)
	}
}
