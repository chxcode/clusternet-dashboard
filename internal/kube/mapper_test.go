package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNamespaceSummaryMapsKubernetesNamespace(t *testing.T) {
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}

	summary := NewNamespaceSummary(ns)

	if summary.Name != "default" {
		t.Fatalf("expected namespace name default, got %q", summary.Name)
	}
	if summary.Status != "Active" {
		t.Fatalf("expected namespace status Active, got %q", summary.Status)
	}
}

func TestEventSummaryMapsKubernetesEvent(t *testing.T) {
	event := corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-started", Namespace: "default"},
		Type:      corev1.EventTypeNormal,
		Reason:    "Started",
		Message:   "Started container",
	}

	summary := NewEventSummary(event)

	if summary.Name != "pod-started" || summary.Namespace != "default" {
		t.Fatalf("unexpected event identity: %#v", summary)
	}
	if summary.Type != "Normal" || summary.Reason != "Started" {
		t.Fatalf("unexpected event details: %#v", summary)
	}
}
