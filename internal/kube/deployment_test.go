package kube

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	restfake "k8s.io/client-go/rest/fake"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"
)

const demoID1 = "aaaa0000-1234-abcd-0000-000000000001"
const demoID2 = "aaaa0000-1234-abcd-0000-000000000002"

func deployCluster(ns, id string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": managedClusterGVR.GroupVersion().String(), "kind": "ManagedCluster",
		"metadata": map[string]any{"namespace": ns, "name": "edge", "uid": ns + "-uid", "labels": map[string]any{deploymentIDLabel: id, "zone": ns}},
		"spec":     map[string]any{"clusterId": id},
	}}
}

func deployNamespace(ns, id string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{"name": ns, "labels": map[string]any{deploymentIDLabel: id}}}}
}

func deployChart() *unstructured.Unstructured {
	chart := versionChart()
	_ = unstructured.SetNestedField(chart.Object, "workloads", "spec", "targetNamespace")
	_ = unstructured.SetNestedField(chart.Object, "Found", "status", "phase")
	return chart
}

func deploymentFixture(extra ...runtime.Object) (*fake.FakeDynamicClient, *Explorer, DeploymentRequest) {
	objects := []runtime.Object{deployChart(), deployCluster("east", demoID1), deployNamespace("east", demoID1), deployCluster("west", demoID2), deployNamespace("west", demoID2)}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		managedClusterGVR: "ManagedClusterList", subscriptionGVR: "SubscriptionList", baseGVR: "BaseList", deploymentNamespaces: "NamespaceList", helmReleaseGVR: "HelmReleaseList",
	}, append(objects, extra...)...)
	client.PrependReactor("create", "subscriptions", func(action clienttesting.Action) (bool, runtime.Object, error) {
		action.(clienttesting.CreateAction).GetObject().(*unstructured.Unstructured).SetUID("created-sub")
		return false, nil, nil
	})
	return client, NewExplorer(nil, nil, client), DeploymentRequest{Name: "rollout", Targets: []DeploymentTarget{{"west", "edge", "west-uid"}, {"east", "edge", "east-uid"}}}
}

func TestDeploymentCreatesOnlyExactNamedReplicationSubscription(t *testing.T) {
	client, e, req := deploymentFixture()
	preview, err := e.PreviewDeployment(t.Context(), "charts", "web", req)
	if err != nil || len(preview.Conflicts) != 0 || preview.ChartName != "web" || preview.TargetNamespace != "workloads" || preview.ChartVersion != "1.2.3" || preview.Targets[0].Namespace != "east" || preview.Targets[0].ClusterID != demoID1 {
		t.Fatalf("preview: %+v, %v", preview, err)
	}
	for _, action := range client.Actions() {
		if action.GetVerb() != "get" && action.GetVerb() != "list" {
			t.Fatalf("unexpected preview action %s", action.GetVerb())
		}
	}
	req.Fingerprint = preview.Fingerprint
	req.Targets[0], req.Targets[1] = req.Targets[1], req.Targets[0]
	created, err := e.CreateDeployment(t.Context(), "charts", "web", req)
	if err != nil || created != (CreatedSubscription{Namespace: "charts", Name: "rollout", UID: "created-sub"}) {
		t.Fatalf("create: %+v %v", created, err)
	}
	sub, err := client.Resource(subscriptionGVR).Namespace("charts").Get(t.Context(), "rollout", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assertPreviewSubscriptionFile(t, preview.SubscriptionYAML, sub)
	want := map[string]any{
		"schedulingStrategy": "Replication",
		"feeds":              []any{map[string]any{"kind": "HelmChart", "apiVersion": "apps.clusternet.io/v1alpha1", "namespace": "charts", "name": "web"}},
		"subscribers":        []any{map[string]any{"clusterAffinity": map[string]any{"matchExpressions": []any{map[string]any{"key": deploymentIDLabel, "operator": "In", "values": []any{demoID1, demoID2}}}}}},
	}
	if !reflect.DeepEqual(sub.Object["spec"], want) || len(sub.Object) != 4 || len(sub.GetAnnotations()) != 0 {
		t.Fatalf("unexpected subscription: %v", sub.Object)
	}
	if _, err = e.CreateDeployment(t.Context(), "charts", "web", req); !apierrors.IsConflict(err) {
		t.Fatalf("repeat: %v", err)
	}
	writes := 0
	for _, a := range client.Actions() {
		if a.GetVerb() == "create" {
			writes++
		} else if a.GetVerb() != "get" && a.GetVerb() != "list" {
			t.Fatalf("unexpected write: %s", a.GetVerb())
		}
	}
	if writes != 1 {
		t.Fatalf("wanted one create, got %d", writes)
	}
}

func TestDeploymentChecksAllSourcesOfOverlap(t *testing.T) {
	for _, scenario := range []string{"binding only", "candidate only", "other version of GVK", "other chart namespace", "separate target namespace", "separate clusters", "unreadable scope", "same-name subscription", "legacy Base name", "orphan Base", "unavailable chart", "missing target namespace"} {
		t.Run(scenario, func(t *testing.T) {
			sub := versionSub("existing", "current", "charts")
			unstructured.SetNestedSlice(sub.Object, []any{map[string]any{"clusterAffinity": map[string]any{"matchLabels": map[string]any{"zone": "east"}}}}, "spec", "subscribers")
			extra := []runtime.Object{sub}
			switch scenario {
			case "binding only":
				unstructured.RemoveNestedField(sub.Object, "spec", "subscribers")
				_ = unstructured.SetNestedStringSlice(sub.Object, []string{"east/edge"}, "status", "bindingClusters")
			case "other version of GVK":
				sub.Object["spec"].(map[string]any)["feeds"].([]any)[0].(map[string]any)["apiVersion"] = "other.test/v1"
			case "other chart namespace", "separate target namespace":
				sub.Object["spec"].(map[string]any)["feeds"].([]any)[0].(map[string]any)["namespace"] = "other"
				other := deployChart()
				other.SetNamespace("other")
				other.SetUID("other-chart")
				if scenario == "separate target namespace" {
					_ = unstructured.SetNestedField(other.Object, "alternate", "spec", "targetNamespace")
				}
				extra = append(extra, other)
			case "separate clusters":
				_ = unstructured.SetNestedSlice(sub.Object, []any{map[string]any{"clusterAffinity": map[string]any{"matchLabels": map[string]any{deploymentIDLabel: "aaaa0000-1234-abcd-0000-000000000003"}}}}, "spec", "subscribers")
			case "unreadable scope":
				_ = unstructured.SetNestedField(sub.Object, "all", "spec", "subscribers")
			case "same-name subscription":
				sub.SetName("rollout")
				sub.SetNamespace("charts")
				unstructured.RemoveNestedField(sub.Object, "spec", "feeds")
			case "legacy Base name":
				sub.SetName("rollout")
				unstructured.RemoveNestedField(sub.Object, "spec", "feeds")
			case "orphan Base":
				extra = []runtime.Object{&unstructured.Unstructured{Object: map[string]any{"apiVersion": baseGVR.GroupVersion().String(), "kind": "Base", "metadata": map[string]any{"namespace": "east", "name": "rollout"}}}}
			case "unavailable chart", "missing target namespace":
				extra = nil
			}
			client, e, req := deploymentFixture(extra...)
			if scenario == "unavailable chart" || scenario == "missing target namespace" {
				chart := deployChart()
				if scenario == "unavailable chart" {
					_ = unstructured.SetNestedField(chart.Object, "NotFound", "status", "phase")
				} else {
					unstructured.RemoveNestedField(chart.Object, "spec", "targetNamespace")
				}
				if err := client.Tracker().Update(helmChartGVR, chart, "charts"); err != nil {
					t.Fatal(err)
				}
			}
			got, err := e.PreviewDeployment(t.Context(), "charts", "web", req)
			if err != nil {
				t.Fatal(err)
			}
			expectConflict := scenario != "separate target namespace" && scenario != "separate clusters"
			if expectConflict != (len(got.Conflicts) > 0) {
				t.Fatalf("unexpected conflicts %v", got.Conflicts)
			}
			if expectConflict {
				req.Fingerprint = got.Fingerprint
				if _, err = e.CreateDeployment(t.Context(), "charts", "web", req); !apierrors.IsConflict(err) {
					t.Fatalf("submit did not enforce conflicts: %v", err)
				}
			}
		})
	}
}

func TestDeploymentRechecksIdentitySourceAndConflict(t *testing.T) {
	for _, scenario := range []string{"empty targets", "duplicate target", "long name", "replaced target", "missing target", "target terminating", "invalid UUID", "label disagrees with spec", "duplicate cluster ID", "duplicate spec ID", "namespace disagrees", "shared namespace", "source changed", "source terminating", "different selection", "different name", "new conflict"} {
		t.Run(scenario, func(t *testing.T) {
			client, e, req := deploymentFixture()
			p, err := e.PreviewDeployment(t.Context(), "charts", "web", req)
			if err != nil {
				t.Fatal(err)
			}
			req.Fingerprint = p.Fingerprint
			c, chart, ns := deployCluster("east", demoID1), deployChart(), deployNamespace("east", demoID1)
			now := metav1.Now()
			switch scenario {
			case "empty targets":
				req.Targets = nil
			case "duplicate target":
				req.Targets = append(req.Targets, req.Targets[0])
			case "long name":
				req.Name = strings.Repeat("a", 53)
			case "replaced target":
				c.SetUID("replacement")
			case "missing target":
				req.Targets[0].Name = "missing"
			case "target terminating":
				c.SetDeletionTimestamp(&now)
			case "invalid UUID":
				c.SetLabels(map[string]string{deploymentIDLabel: "x"})
				_ = unstructured.SetNestedField(c.Object, "x", "spec", "clusterId")
			case "label disagrees with spec":
				c.SetLabels(map[string]string{deploymentIDLabel: demoID2})
			case "duplicate cluster ID", "duplicate spec ID":
				other := deployCluster("third", demoID1)
				if scenario == "duplicate spec ID" {
					other.SetLabels(map[string]string{deploymentIDLabel: "aaaa0000-1234-abcd-0000-000000000003"})
				}
				err = client.Tracker().Add(other)
			case "namespace disagrees":
				ns.SetLabels(map[string]string{deploymentIDLabel: demoID2})
			case "shared namespace":
				other := deployCluster("east", "aaaa0000-1234-abcd-0000-000000000003")
				other.SetName("another")
				err = client.Tracker().Add(other)
			case "source changed":
				_ = unstructured.SetNestedField(chart.Object, "https://changed.example.test", "spec", "repo")
			case "source terminating":
				chart.SetDeletionTimestamp(&now)
			case "different selection":
				req.Targets = req.Targets[:1]
			case "different name":
				req.Name = "different"
			case "new conflict":
				err = client.Tracker().Add(versionSub("other", "existing", "charts", "east/edge"))
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = client.Tracker().Update(managedClusterGVR, c, "east"); err != nil {
				t.Fatal(err)
			}
			if err = client.Tracker().Update(helmChartGVR, chart, "charts"); err != nil {
				t.Fatal(err)
			}
			if err = client.Tracker().Update(deploymentNamespaces, ns, ""); err != nil {
				t.Fatal(err)
			}
			_, err = e.CreateDeployment(t.Context(), "charts", "web", req)
			if !apierrors.IsConflict(err) && !apierrors.IsBadRequest(err) {
				t.Fatalf("expected rejection, got: %v", err)
			}
			for _, a := range client.Actions() {
				if a.GetVerb() != "get" && a.GetVerb() != "list" {
					t.Fatalf("unexpected write: %s", a.GetVerb())
				}
			}
		})
	}
}

func TestDeploymentSerializesOverlappingCreates(t *testing.T) {
	_, e, first := deploymentFixture()
	second := first
	second.Name = "another"
	results := make(chan error, 2)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for _, req := range []DeploymentRequest{first, second} {
		p, err := e.PreviewDeployment(t.Context(), "charts", "web", req)
		if err != nil {
			t.Fatal(err)
		}
		req.Fingerprint = p.Fingerprint
		workers.Add(1)
		go func(req DeploymentRequest) {
			defer workers.Done()
			<-start
			_, err := e.CreateDeployment(t.Context(), "charts", "web", req)
			results <- err
		}(req)
	}
	close(start)
	workers.Wait()
	close(results)
	var successes, conflicts int
	for err := range results {
		if err == nil {
			successes++
		} else if apierrors.IsConflict(err) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("overlap race: successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestDeploymentCreateRetryAfterDoesNotResubmit(t *testing.T) {
	var posts int
	var lastBasePages int
	transport := restfake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
		var data any
		code := 200
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/namespaces/charts/subscriptions"):
			posts++
			var sub unstructured.Unstructured
			if err := json.NewDecoder(r.Body).Decode(&sub); err != nil || sub.GetName() != "rollout" || sub.GetNamespace() != "charts" {
				return nil, fmt.Errorf("unexpected create payload")
			}
			code = 503
			data = map[string]any{"apiVersion": "v1", "kind": "Status", "code": 503, "reason": "ServiceUnavailable", "status": "Failure"}
		case r.Method != "GET":
			return nil, fmt.Errorf("unexpected method %s", r.Method)
		case strings.HasSuffix(r.URL.Path, "/helmcharts/web"):
			data = deployChart().Object
		case strings.HasSuffix(r.URL.Path, "/managedclusters"):
			data = map[string]any{"apiVersion": managedClusterGVR.GroupVersion().String(), "kind": "ManagedClusterList", "items": []any{deployCluster("east", demoID1)}}
		case r.URL.Path == "/api/v1/namespaces":
			data = map[string]any{"apiVersion": "v1", "kind": "NamespaceList", "items": []any{deployNamespace("east", demoID1)}}
		case strings.HasSuffix(r.URL.Path, "/bases"):
			if r.URL.Query().Get("fieldSelector") != "metadata.name=rollout" {
				return nil, fmt.Errorf("missing scoped Base lookup")
			}
			page := map[string]any{"apiVersion": baseGVR.GroupVersion().String(), "kind": "BaseList", "items": []any{}}
			if r.URL.Query().Get("continue") == "" {
				page["metadata"] = map[string]any{"continue": "next-base"}
			} else {
				lastBasePages++
			}
			data = page
		case strings.HasSuffix(r.URL.Path, "/subscriptions"):
			data = map[string]any{"apiVersion": subscriptionGVR.GroupVersion().String(), "kind": "SubscriptionList", "items": []any{}}
		case strings.HasSuffix(r.URL.Path, "/helmreleases"):
			data = map[string]any{"apiVersion": helmReleaseGVR.GroupVersion().String(), "kind": "HelmReleaseList", "items": []any{}}
		default:
			return nil, fmt.Errorf("unexpected path %s", r.URL.Path)
		}
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		h := http.Header{"Content-Type": {"application/json"}}
		if code == 503 {
			h.Set("Retry-After", "0")
		}
		return &http.Response{StatusCode: code, Header: h, Body: io.NopCloser(strings.NewReader(string(raw)))}, nil
	})
	e, err := NewExplorerFromConfig(&rest.Config{Host: "https://example.test", Transport: transport.Transport})
	if err != nil {
		t.Fatal(err)
	}
	req := DeploymentRequest{Name: "rollout", Targets: []DeploymentTarget{{"east", "edge", "east-uid"}}}
	plan, err := e.PreviewDeployment(t.Context(), "charts", "web", req)
	if err != nil {
		t.Fatal(err)
	}
	req.Fingerprint = plan.Fingerprint
	if _, err = e.CreateDeployment(t.Context(), "charts", "web", req); !apierrors.IsServiceUnavailable(err) || posts != 1 || lastBasePages != 2 {
		t.Fatalf("posts=%d complete Base lookups=%d err=%v", posts, lastBasePages, err)
	}
}

func TestDeploymentDetectsExistingChildReleaseIdentity(t *testing.T) {
	type hr struct {
		namespace, name, target string
		explicit                any
	}
	for _, tc := range []struct {
		name              string
		releases          []hr
		conflictedTargets int
	}{
		{"explicit child release name", []hr{{"east", "generated", "workloads", "web"}}, 1},
		{"absent uses metadata name", []hr{{"east", "web", "workloads", nil}}, 1},
		{"empty explicit does not fall back", []hr{{"east", "web", "workloads", ""}}, 0},
		{"different child namespace", []hr{{"east", "generated", "other", "web"}}, 0},
		{"different cluster", []hr{{"north", "generated", "workloads", "web"}}, 0},
		{"different release name", []hr{{"east", "web", "workloads", "another"}}, 0},
		{"one message with unique targets", []hr{{"east", "one", "workloads", "web"}, {"west", "web", "workloads", nil}, {"east", "two", "workloads", "web"}}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var objects []runtime.Object
			for _, r := range tc.releases {
				spec := map[string]any{"targetNamespace": r.target}
				if r.explicit != nil {
					spec["releaseName"] = r.explicit
				}
				objects = append(objects, &unstructured.Unstructured{Object: map[string]any{"apiVersion": helmReleaseGVR.GroupVersion().String(), "kind": "HelmRelease", "metadata": map[string]any{"namespace": r.namespace, "name": r.name}, "spec": spec}})
			}
			_, e, req := deploymentFixture(objects...)
			p, err := e.PreviewDeployment(t.Context(), "charts", "web", req)
			if err != nil {
				t.Fatal(err)
			}
			if tc.conflictedTargets == 0 {
				if len(p.Conflicts) != 0 {
					t.Fatalf("unexpected conflicts %v", p.Conflicts)
				}
			} else if len(p.Conflicts) != 1 || !strings.HasPrefix(p.Conflicts[0], fmt.Sprint(tc.conflictedTargets)+" 个目标集群已有 HelmRelease") {
				t.Fatalf("wrong aggregated collision %v", p.Conflicts)
			}
		})
	}
}

func TestDeploymentDynamicCreatesRealAffinity(t *testing.T) {
	tests := []struct {
		mode, selector, normalized        string
		targets                           int
		matchEast, matchWest, matchFuture bool
	}{
		{"all", "", "", 2, true, true, true},
		{"selector", "zone in ( east, north ),!retired", "!retired,zone in (east,north)", 1, true, false, true},
		{"selector", "stage!=prod,zone==north", "stage notin (prod),zone=north", 0, false, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.mode+" "+tc.selector, func(t *testing.T) {
			client, e, _ := deploymentFixture()
			req := DeploymentRequest{Name: "dynamic", TargetMode: tc.mode, Selector: tc.selector}
			p, err := e.PreviewDeployment(t.Context(), "charts", "web", req)
			if err != nil || p.Conflicts == nil || len(p.Conflicts) != 0 || len(p.ScopeWarnings) == 0 || p.Targets == nil || len(p.Targets) != tc.targets || p.Selector != tc.normalized || p.TargetMode != tc.mode {
				t.Fatalf("preview = %+v, %v", p, err)
			}
			if len(p.Targets) > 0 && (p.Targets[0].UID != "east-uid" || p.Targets[0].ClusterID != demoID1) {
				t.Fatalf("unexpected live target %+v", p.Targets[0])
			}
			for _, a := range client.Actions() {
				if a.GetVerb() != "list" && a.GetVerb() != "get" {
					t.Fatal("preview wrote to the cluster")
				}
			}
			req.Fingerprint = p.Fingerprint
			if _, err = e.CreateDeployment(t.Context(), "charts", "web", req); err != nil {
				t.Fatal(err)
			}
			sub, err := client.Resource(subscriptionGVR).Namespace("charts").Get(t.Context(), "dynamic", metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			assertPreviewSubscriptionFile(t, p.SubscriptionYAML, sub)
			d := subscriptionDistribution(sub)
			if d.Strategy != "Replication" || len(d.Selectors) != 1 || d.Selectors[0] == nil || *d.Selectors[0] != tc.normalized {
				t.Fatalf("saved subscription distribution = %+v", d)
			}
			// Re-evaluate the saved rule with a synthetic newly joined cluster.
			q, err := labels.Parse(*d.Selectors[0])
			if err != nil {
				t.Fatal(err)
			}
			if q.Matches(labels.Set{"zone": "east"}) != tc.matchEast || q.Matches(labels.Set{"zone": "west"}) != tc.matchWest || q.Matches(labels.Set{"zone": "north"}) != tc.matchFuture {
				t.Fatalf("saved affinity does not select the intended clusters: %s", q)
			}
		})
	}
}

func TestDeploymentValidatesModesBeforeClusterRead(t *testing.T) {
	_, _, fixed := deploymentFixture()
	for _, req := range []DeploymentRequest{
		{Name: "x", TargetMode: "selector"},
		{Name: "x", TargetMode: "selector", Selector: "  "},
		{Name: "x", TargetMode: "selector", Selector: "zone=east", Targets: []DeploymentTarget{}},
		{Name: "x", TargetMode: "selector", Selector: "zone=east", Targets: fixed.Targets},
		{Name: "x", TargetMode: "selector", Selector: "zone=("},
		{Name: "x", TargetMode: "selector", Selector: "zone>3"},
		{Name: "x", TargetMode: "selector", Selector: "zone=east,zone=west"},
		{Name: "x", TargetMode: "selector", Selector: "zone,!zone"},
		{Name: "x", TargetMode: "all", Targets: fixed.Targets},
		{Name: "x", TargetMode: "all", Selector: "zone=east"},
		{Name: "x", TargetMode: "fixed", Selector: "zone=east", Targets: fixed.Targets},
		{Name: "x", TargetMode: "oops", Targets: fixed.Targets},
		{Name: "x", Selector: "zone=east"},
	} {
		client, e, _ := deploymentFixture()
		if _, err := e.PreviewDeployment(t.Context(), "charts", "web", req); !apierrors.IsBadRequest(err) || len(client.Actions()) != 0 {
			t.Fatalf("mixed/invalid mode accepted %+v: %v actions %d", req, err, len(client.Actions()))
		}
	}
}

func TestDeploymentFutureOverlapAndORBranches(t *testing.T) {
	// No current fixture has the new candidate zone=north.
	tests := []struct {
		name, otherSelector                                                 string
		secondBranch, sameSubscriptionName, differentTargetNS, wantConflict bool
	}{
		{"independent requirements", "tier=gold", false, false, false, true},
		{"not in allows absent", "zone notin (east),!region", false, false, false, true},
		{"distinct values", "zone in (east,west)", false, false, false, false},
		{"excluded requirements", "zone notin (north)", false, false, false, false},
		{"required label vs absent", "!zone", false, false, false, false},
		{"OR overlap", "zone=west", true, false, false, true},
		{"legacy Base name", "tier=gold", false, true, true, true},
		{"separate Helm target namespaces", "tier=gold", false, false, true, false},
		{"disjoint Base names", "zone=west", false, true, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			other := versionSub("other", "existing", "other")
			if tc.sameSubscriptionName {
				other.SetName("rollout")
			}
			chart := deployChart()
			chart.SetNamespace("other")
			if tc.differentTargetNS {
				_ = unstructured.SetNestedField(chart.Object, "alternate", "spec", "targetNamespace")
			}
			affinity, err := parseDeploymentAffinity(tc.otherSelector)
			if err != nil {
				t.Fatal(err)
			}
			object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&affinity)
			if err != nil {
				t.Fatal(err)
			}
			subscribers := []any{map[string]any{"clusterAffinity": object}}
			if tc.secondBranch {
				subscribers = append(subscribers, map[string]any{"clusterAffinity": map[string]any{"matchLabels": map[string]any{"zone": "north"}}})
			}
			_ = unstructured.SetNestedSlice(other.Object, subscribers, "spec", "subscribers")
			client, e, _ := deploymentFixture(other, chart)
			req := DeploymentRequest{Name: "rollout", TargetMode: "selector", Selector: "zone=north"}
			p, err := e.PreviewDeployment(t.Context(), "charts", "web", req)
			if err != nil || len(p.Targets) != 0 || (len(p.Conflicts) > 0) != tc.wantConflict {
				t.Fatalf("targets=%v conflicts=%v err=%v", p.Targets, p.Conflicts, err)
			}
			if tc.wantConflict {
				if !strings.Contains(strings.Join(p.Conflicts, " "), "未来") {
					t.Fatalf("missing future scope reason %v", p.Conflicts)
				}
				req.Fingerprint = p.Fingerprint
				if _, err = e.CreateDeployment(t.Context(), "charts", "web", req); !apierrors.IsConflict(err) {
					t.Fatal(err)
				}
				for _, a := range client.Actions() {
					if a.GetVerb() != "list" && a.GetVerb() != "get" {
						t.Fatal("wrote despite collision")
					}
				}
			}
		})
	}
}

func TestDeploymentRevalidatesDynamicScope(t *testing.T) {
	for _, scenario := range []string{"newly matched cluster", "new labels", "different scope with same candidates", "fresh heartbeat", "new conflicting rule"} {
		t.Run(scenario, func(t *testing.T) {
			client, e, _ := deploymentFixture()
			req := DeploymentRequest{Name: "rollout", TargetMode: "selector", Selector: "zone=north"}
			p, err := e.PreviewDeployment(t.Context(), "charts", "web", req)
			if err != nil {
				t.Fatal(err)
			}
			req.Fingerprint = p.Fingerprint
			switch scenario {
			case "newly matched cluster":
				id := "aaaa0000-1234-abcd-0000-000000000003"
				if err = client.Tracker().Add(deployCluster("north", id)); err != nil {
					t.Fatal(err)
				}
				err = client.Tracker().Add(deployNamespace("north", id))
			case "new labels", "fresh heartbeat":
				c := deployCluster("east", demoID1)
				chart := deployChart()
				chart.SetResourceVersion("33")
				if err = client.Tracker().Update(helmChartGVR, chart, "charts"); err != nil {
					t.Fatal(err)
				}
				if scenario == "new labels" {
					c.SetLabels(map[string]string{deploymentIDLabel: demoID1, "zone": "north"})
				}
				c.SetResourceVersion("1234")
				_ = unstructured.SetNestedField(c.Object, "2026-01-01T10:00:00Z", "status", "lastObservedTime")
				err = client.Tracker().Update(managedClusterGVR, c, "east")
			case "different scope with same candidates":
				req.Selector = "zone=south"
			case "new conflicting rule":
				sub := versionSub("other", "existing", "charts")
				_ = unstructured.SetNestedSlice(sub.Object, []any{map[string]any{"clusterAffinity": map[string]any{}}}, "spec", "subscribers")
				err = client.Tracker().Add(sub)
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = e.CreateDeployment(t.Context(), "charts", "web", req)
			if scenario == "fresh heartbeat" {
				if err != nil {
					t.Fatalf("heartbeat invalidated preview: %v", err)
				}
			} else if !apierrors.IsConflict(err) {
				t.Fatalf("expected stale preview, got %v", err)
			}
		})
	}
}

func TestDeploymentIntersectionsFollowKubernetesLabelSemantics(t *testing.T) {
	// These selectors mention only the values a and b. A third value and an
	// absent key cover every remaining possibility for this finite test domain.
	selectors := []string{"", "k=a", "k==a", "k=", "k in (a,b)", "k!=a", "k notin (a,b)", "k", "!k", "k in (a,b),k!=a", "k!=a,!k", "k=a,k=b", "k,!k", "k=,k notin (a,b)"}
	for _, left := range selectors {
		a, err := labels.Parse(left)
		if err != nil {
			t.Fatal(err)
		}
		for _, right := range selectors {
			b, err := labels.Parse(right)
			if err != nil {
				t.Fatal(err)
			}
			l, _ := a.Requirements()
			r, _ := b.Requirements()
			got := requirementsCanCoexist(append(append(labels.Requirements{}, l...), r...))
			found := false
			for _, assignment := range []labels.Set{{}, {"k": ""}, {"k": "a"}, {"k": "b"}, {"k": "other"}} {
				if a.Matches(assignment) && b.Matches(assignment) {
					found = true
					break
				}
			}
			if got != found {
				t.Fatalf("%q intersect %q = %v, Kubernetes matching gives %v", left, right, got, found)
			}
		}
	}
}

func TestDeploymentFixedAndFutureRulesCheckedInBothOrders(t *testing.T) {
	for _, fixedFirst := range []bool{false, true} {
		client, e, fixed := deploymentFixture()
		dyn := DeploymentRequest{Name: "later", TargetMode: "selector", Selector: "stage=canary"}
		first, second := fixed, dyn
		if !fixedFirst {
			first, second = dyn, fixed
		}
		p, err := e.PreviewDeployment(t.Context(), "charts", "web", first)
		if err != nil || len(p.Conflicts) > 0 {
			t.Fatalf("first preview %+v %v", p, err)
		}
		first.Fingerprint = p.Fingerprint
		if _, err = e.CreateDeployment(t.Context(), "charts", "web", first); err != nil {
			t.Fatal(err)
		}
		secondPlan, err := e.PreviewDeployment(t.Context(), "charts", "web", second)
		if err != nil || !strings.Contains(strings.Join(secondPlan.Conflicts, " "), "未来") {
			t.Fatalf("fixed first %t: conflicts %v, %v", fixedFirst, secondPlan.Conflicts, err)
		}
		second.Fingerprint = secondPlan.Fingerprint
		if _, err = e.CreateDeployment(t.Context(), "charts", "web", second); !apierrors.IsConflict(err) {
			t.Fatal(err)
		}
		creates := 0
		for _, a := range client.Actions() {
			if a.GetVerb() == "create" {
				creates++
			}
		}
		if creates != 1 {
			t.Fatalf("expected only the first named create; got %d", creates)
		}
	}
}

func assertPreviewSubscriptionFile(t *testing.T, file string, saved *unstructured.Unstructured) {
	t.Helper()
	var preview map[string]any
	if err := yaml.UnmarshalStrict([]byte(file), &preview); err != nil {
		t.Fatal(err)
	}
	want := saved.DeepCopy()
	// The fake API server assigned UID after receiving the create.
	unstructured.RemoveNestedField(want.Object, "metadata", "uid")
	if !reflect.DeepEqual(preview, want.Object) {
		t.Fatalf("YAML differs from the submitted Subscription: %s", file)
	}
}

func TestDeploymentPreviewsFutureExistingResourceOccupancy(t *testing.T) {
	tests := []struct {
		name, selector, releaseNS, targetNS               string
		base, disjointSubscription, wantBase, wantRelease bool
	}{
		{"orphan Base and release may join", "zone=north", "east", "workloads", true, false, true, true},
		{"exclude verified cluster ID", "zone=north," + deploymentIDLabel + "!=" + demoID1, "east", "workloads", true, false, false, false},
		{"allow verified cluster ID", "zone=north," + deploymentIDLabel + " in (" + demoID1 + ")", "east", "workloads", true, false, true, true},
		{"different Helm namespace", "zone=north", "east", "elsewhere", false, false, false, false},
		{"different child cluster ID", "zone=north," + deploymentIDLabel + "=" + demoID2, "east", "workloads", false, false, false, false},
		{"unregistered namespace", "zone=north", "unregistered", "workloads", true, false, false, false},
		{"existing disjoint deployment stays possible", "zone=north", "east", "workloads", false, true, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			release := &unstructured.Unstructured{Object: map[string]any{"apiVersion": helmReleaseGVR.GroupVersion().String(), "kind": "HelmRelease", "metadata": map[string]any{"namespace": tc.releaseNS, "name": "web"}, "spec": map[string]any{"targetNamespace": tc.targetNS}}}
			objects := []runtime.Object{release}
			if tc.base {
				objects = append(objects, &unstructured.Unstructured{Object: map[string]any{"apiVersion": baseGVR.GroupVersion().String(), "kind": "Base", "metadata": map[string]any{"namespace": tc.releaseNS, "name": "rollout"}}})
			}
			if tc.disjointSubscription {
				sub := versionSub("other", "eastern", "charts", "east/edge")
				_ = unstructured.SetNestedSlice(sub.Object, []any{map[string]any{"clusterAffinity": map[string]any{"matchLabels": map[string]any{"zone": "east"}}}}, "spec", "subscribers")
				objects = append(objects, sub)
			}
			client, e, _ := deploymentFixture(objects...)
			req := DeploymentRequest{Name: "rollout", TargetMode: "selector", Selector: tc.selector}
			p, err := e.PreviewDeployment(t.Context(), "charts", "web", req)
			warnings := strings.Join(p.ScopeWarnings, " ")
			if err != nil || len(p.Conflicts) != 0 || strings.Contains(warnings, "已有同名 Base") != tc.wantBase || strings.Contains(warnings, "已有 HelmRelease") != tc.wantRelease {
				t.Fatalf("conflicts %v, warnings %v, error %v", p.Conflicts, p.ScopeWarnings, err)
			}
			if (tc.wantBase || tc.wantRelease) && !strings.Contains(warnings, tc.releaseNS) {
				t.Fatalf("missing affected namespace in %s", warnings)
			}
			// ResourceVersion-only updates preserve review of the same occupancy.
			release.SetResourceVersion("22")
			if err = client.Tracker().Update(helmReleaseGVR, release, tc.releaseNS); err != nil {
				t.Fatal(err)
			}
			req.Fingerprint = p.Fingerprint
			if _, err = e.CreateDeployment(t.Context(), "charts", "web", req); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDeploymentRechecksNewFutureOccupancyOnSubmit(t *testing.T) {
	client, e, _ := deploymentFixture()
	req := DeploymentRequest{Name: "rollout", TargetMode: "selector", Selector: "zone=north"}
	p, err := e.PreviewDeployment(t.Context(), "charts", "web", req)
	if err != nil {
		t.Fatal(err)
	}
	base := &unstructured.Unstructured{Object: map[string]any{"apiVersion": baseGVR.GroupVersion().String(), "kind": "Base", "metadata": map[string]any{"namespace": "east", "name": "rollout"}}}
	if err = client.Tracker().Add(base); err != nil {
		t.Fatal(err)
	}
	req.Fingerprint = p.Fingerprint
	if _, err = e.CreateDeployment(t.Context(), "charts", "web", req); !apierrors.IsConflict(err) {
		t.Fatalf("new future occupancy should require review: %v", err)
	}
}
