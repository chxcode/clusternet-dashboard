package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chxcode/clusternet-dashboard/internal/config"
	"github.com/chxcode/clusternet-dashboard/internal/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kt "k8s.io/client-go/testing"
)

func TestNativePublisherRoutes(t *testing.T) {
	apps := schema.GroupVersion{Group: "apps.clusternet.io", Version: "v1alpha1"}
	c := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		apps.WithResource("subscriptions"): "SubscriptionList", apps.WithResource("bases"): "BaseList",
		{Group: "clusters.clusternet.io", Version: "v1beta1", Resource: "managedclusters"}: "ManagedClusterList",
		{Version: "v1", Resource: "namespaces"}:                                            "NamespaceList",
	})
	// Shadow reports missing ConfigMaps using the core group of the template.
	c.PrependReactor("get", "configmaps", func(a kt.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, a.(kt.GetAction).GetName())
	})
	post := func(enabled bool, path, origin, body string) *httptest.ResponseRecorder {
		h := New(config.Config{BasePath: "/dash"}, WithExplorer(kube.NewExplorer(nil, nil, c)), WithChartSourceEditing(true), WithNativePublishing(enabled))
		r := httptest.NewRequest("POST", "http://localhost/dash/api/"+path, strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("cacheable source response: %s", path)
		}
		return w
	}
	const origin = "http://localhost"
	const bundle = "native-workloads/demo/catalog/"
	for _, path := range []string{"native-sources/demo/config/ConfigMap", bundle + "source", bundle + "preview", bundle + "apply"} {
		if w := post(false, path, origin, "{}"); w.Code != 403 {
			t.Fatalf("%s enabled by an unrelated option: %d", path, w.Code)
		}
		if w := post(true, path, "https://unrelated.invalid", "{}"); w.Code != 403 {
			t.Fatalf("%s accepted cross-origin request: %d", path, w.Code)
		}
		if w := post(true, path, origin, `{"extra":true}`); w.Code != 400 {
			t.Fatalf("%s accepted unknown fields: %d", path, w.Code)
		}
	}
	if w := post(true, "native-sources/demo/config/Secret", origin, "{}"); w.Code != 400 {
		t.Fatalf("unsupported source: %d", w.Code)
	}
	payload := kube.NativeWorkloadRequest{YAML: "apiVersion: v1\nkind: ConfigMap\nmetadata: {name: config}\ndata: {mode: test}\n", TargetMode: "selector", Selector: "rollout=pilot"}
	body, _ := json.Marshal(payload)
	if w := post(true, bundle+"apply", origin, string(body)); w.Code != 400 {
		t.Fatalf("applied without preview: %d", w.Code)
	}
	if len(c.Actions()) != 0 {
		t.Fatal("request guards reached Kubernetes")
	}
	w := post(true, bundle+"preview", origin, string(body))
	var plan kube.NativeWorkloadPreview
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &plan) != nil || len(plan.Resources) != 1 || plan.Resources[0].Action != "create" || plan.Fingerprint == "" || len(plan.Targets) != 0 || len(plan.Conflicts) != 0 {
		t.Fatalf("unexpected preview: %d %s", w.Code, w.Body.String())
	}
	payload.Fingerprint = plan.Fingerprint
	payload.Selector = "rollout=other"
	body, _ = json.Marshal(payload)
	if w := post(true, bundle+"apply", origin, string(body)); w.Code != 409 {
		t.Fatalf("stale scope confirmation: %d %s", w.Code, w.Body.String())
	}
	for _, a := range c.Actions() {
		if a.GetVerb() != "get" && a.GetVerb() != "list" {
			t.Fatalf("preview or rejected apply issued %s on %v", a.GetVerb(), a.GetResource())
		}
	}
}
