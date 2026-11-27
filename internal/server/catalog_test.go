package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/chxcode/clusternet-dashboard/internal/config"
	"github.com/chxcode/clusternet-dashboard/internal/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestCatalogEndpointsUnderNestedBase(t *testing.T) {
	for _, tc := range []struct {
		gvr             schema.GroupVersionResource
		kind, path, key string
	}{
		{schema.GroupVersionResource{Group: "clusters.clusternet.io", Version: "v1beta1", Resource: "clusterregistrationrequests"}, "ClusterRegistrationRequest", "registrations", "registrations"},
		{schema.GroupVersionResource{Group: "apps.clusternet.io", Version: "v1alpha1", Resource: "helmcharts"}, "HelmChart", "helmcharts", "helmCharts"},
		{schema.GroupVersionResource{Group: "apps.clusternet.io", Version: "v1alpha1", Resource: "bases"}, "Base", "bases", "bases"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			item := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": tc.gvr.GroupVersion().String(), "kind": tc.kind, "metadata": map[string]any{"name": "sample"},
			}}
			if tc.kind != "ClusterRegistrationRequest" {
				item.SetNamespace("tenant")
			}
			client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{tc.gvr: tc.kind + "List"}, item)
			handler := New(config.Config{BasePath: "/tools/clusternet"}, WithExplorer(kube.NewExplorer(nil, nil, client)))
			rec := workspaceGET(handler, "/tools/clusternet/api/"+tc.path)
			var result map[string][]map[string]any
			if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &result) != nil || len(result[tc.key]) != 1 || result[tc.key][0]["name"] != "sample" {
				t.Fatalf("invalid catalog response: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}
