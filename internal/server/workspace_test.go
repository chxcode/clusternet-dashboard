package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/chxcode/clusternet-dashboard/internal/config"
	"github.com/chxcode/clusternet-dashboard/internal/kube"
)

func workspaceGET(handler http.Handler, target string) *httptest.ResponseRecorder {
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, target, nil))
	return res
}

func TestWorkspaceContextIdentifiesParentAndRunningBuild(t *testing.T) {
	for _, tc := range []struct {
		name     string
		explorer *kube.Explorer
	}{
		{"configured", kube.NewExplorer(fake.NewSimpleClientset(), nil, nil)},
		{"unconfigured", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := New(config.Config{BasePath: "/console"}, WithExplorer(tc.explorer),
				WithParent(kube.ParentConnection{Name: "in-cluster", InCluster: true}, "  Example hub  "))
			r := workspaceGET(handler, "/console/api/context")
			var got map[string]any
			if r.Code != http.StatusOK || json.Unmarshal(r.Body.Bytes(), &got) != nil {
				t.Fatalf("context response %d: %s", r.Code, r.Body.String())
			}
			want := map[string]any{"configured": tc.explorer != nil, "name": "in-cluster", "inCluster": true,
				"displayName": "Example hub", "version": config.BuildVersion(), "labelEditing": false, "releaseTriage": false, "chartVersionEditing": false, "releaseDeletion": false, "chartDeployment": false, "chartSourceEditing": false, "nativePublishing": false, "failedRegistrationDeletion": false}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("context = %#v, want %#v", got, want)
			}
		})
	}
}

func TestWorkspaceSubscriptionDetailAtRootAndNestedMount(t *testing.T) {
	sub := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.clusternet.io/v1alpha1", "kind": "Subscription",
		"metadata": map[string]any{"name": "web", "namespace": "demo"},
	}}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		{Group: "clusters.clusternet.io", Version: "v1beta1", Resource: "managedclusters"}: "ManagedClusterList",
	}, sub)
	for _, base := range []string{"", "/platform/operations"} {
		handler := New(config.Config{BasePath: base}, WithExplorer(kube.NewExplorer(nil, nil, client)))
		prefix := base + "/api/subscriptions/"
		r := workspaceGET(handler, prefix+"demo/web")
		var got kube.SubscriptionDetail
		if r.Code != http.StatusOK || json.Unmarshal(r.Body.Bytes(), &got) != nil || got.Summary.Name != "web" || got.Summary.Namespace != "demo" {
			t.Fatalf("%s detail response %d: %s", base, r.Code, r.Body.String())
		}
		for _, tail := range []string{"demo", "extra/demo/web"} {
			if res := workspaceGET(handler, prefix+tail); res.Code != http.StatusNotFound {
				t.Errorf("%s%s: expected 404, got %d", prefix, tail, res.Code)
			}
		}
	}
}

func TestWorkspaceEventLookupUsesExactReleaseIdentity(t *testing.T) {
	const uid = "a0000000-0000-4000-a000-000000000001"
	client := fake.NewSimpleClientset(&corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "helm-failed", Namespace: "demo"},
		Reason:     "Failed", InvolvedObject: corev1.ObjectReference{Name: "web", Kind: "HelmRelease", UID: uid},
	})
	handler := New(config.Config{}, WithExplorer(kube.NewExplorer(client, nil, nil)))
	r := workspaceGET(handler, "/api/helmrelease-events?namespace=demo&uid="+uid)
	var response struct{ Events []kube.EventSummary }
	if r.Code != http.StatusOK || json.Unmarshal(r.Body.Bytes(), &response) != nil || len(response.Events) != 1 || response.Events[0].Involved != "HelmRelease/web" {
		t.Fatalf("events response %d: %s", r.Code, r.Body.String())
	}
	actions := client.Actions()
	if len(actions) != 1 || !actions[0].Matches("list", "events") || actions[0].GetNamespace() != "demo" {
		t.Fatalf("expected one list request scoped to demo, got %#v", actions)
	}
	selector := actions[0].(clienttesting.ListAction).GetListRestrictions().Fields
	for key, want := range map[string]string{"involvedObject.uid": uid, "involvedObject.kind": "HelmRelease"} {
		if got, exact := selector.RequiresExactMatch(key); !exact || got != want {
			t.Errorf("missing exact field selector %s=%s in %q", key, want, selector)
		}
	}
}

func TestWorkspaceInvalidReleaseIdentityNeverCallsKubernetes(t *testing.T) {
	client := fake.NewSimpleClientset()
	handler := New(config.Config{}, WithExplorer(kube.NewExplorer(client, nil, nil)))
	for _, query := range []string{"", "uid=release-id", "namespace=demo", "namespace=WrongCase&uid=id", "namespace=demo&uid=invalid+uid"} {
		if res := workspaceGET(handler, "/api/helmrelease-events?"+query); res.Code != http.StatusBadRequest {
			t.Errorf("query %q: got %d, want 400", query, res.Code)
		}
	}
	if len(client.Actions()) != 0 {
		t.Fatalf("invalid identity triggered %d Kubernetes requests", len(client.Actions()))
	}
}
