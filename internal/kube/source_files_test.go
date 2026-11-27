package kube

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clienttesting "k8s.io/client-go/testing"
)

const newChartYAML = `apiVersion: apps.clusternet.io/v1alpha1
kind: HelmChart
metadata:
  namespace: charts
  name: web
spec:
  repo: https://charts.example.test/catalog
  chart: web
  version: 1.2.3
  targetNamespace: frontend
  timeoutSeconds: 120
  wait: true
`

func createSourceRequest() SourceFileRequest {
	return SourceFileRequest{Namespace: "charts", Name: "web", Mode: "create", YAML: newChartYAML}
}

func editableChart() *unstructured.Unstructured {
	chart := versionChart()
	chart.Object["spec"].(map[string]any)["targetNamespace"] = "frontend"
	return chart
}

func TestSourceFileCreatePreviewReferencesAndRejectReplay(t *testing.T) {
	ctx := context.Background()
	first := versionSub("a", "existing", "charts", "dedicated/west", "dedicated/east", "dedicated/east")
	second := versionSub("b", "pending", "charts")
	client, explorer := versionExplorer(first, second, versionSub("b", "other", "elsewhere", "other"))
	req := createSourceRequest()
	plan, err := explorer.PreviewHelmChartSource(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	expectedSubs := []ChartVersionSubscription{{"a", "existing", 2}, {"b", "pending", 0}}
	if !reflect.DeepEqual(plan.Targets, []string{"dedicated/east", "dedicated/west"}) ||
		!reflect.DeepEqual(expectedSubs, plan.Subscriptions) || len(plan.Changes) != 6 ||
		!reflect.DeepEqual(plan.Changes[0], SourceFieldChange{"spec.chart", nil, "web"}) {
		t.Fatalf("incorrect preview: %+v", plan)
	}
	for _, action := range client.Actions() {
		if action.GetVerb() != "get" && action.GetVerb() != "list" {
			t.Fatalf("preview performed %s", action.GetVerb())
		}
	}
	req.Fingerprint = plan.Fingerprint
	creates := 0
	client.PrependReactor("create", "helmcharts", func(action clienttesting.Action) (bool, runtime.Object, error) {
		creates++
		obj := action.(clienttesting.CreateAction).GetObject().(*unstructured.Unstructured)
		want := sourceObject("charts", "web", map[string]any{"repo": "https://charts.example.test/catalog", "chart": "web", "version": "1.2.3", "targetNamespace": "frontend", "timeoutSeconds": int64(120), "wait": true})
		if !reflect.DeepEqual(obj.Object, want) {
			t.Fatalf("unexpected Kubernetes write: %+v", obj.Object)
		}
		obj.SetUID("created-uid")
		obj.SetResourceVersion("4")
		return false, nil, nil
	})
	// A real resourceVersion and UID are allocated by the apiserver.
	got, err := explorer.SaveHelmChartSource(ctx, req)
	if err != nil || got != (SourceFileIdentity{"charts", "web", "created-uid", "4"}) {
		t.Fatalf("unexpected result %+v, %v", got, err)
	}
	if _, err = explorer.SaveHelmChartSource(ctx, req); !apierrors.IsAlreadyExists(err) || creates != 1 {
		t.Fatalf("create replay: writes=%d err=%v", creates, err)
	}
}

func TestSourceFileEditProjectionPreservesManagedAndSensitiveFields(t *testing.T) {
	ctx := context.Background()
	chart := editableChart()
	spec := chart.Object["spec"].(map[string]any)
	spec["chartPullSecret"] = map[string]any{"name": "private-pull-ref", "namespace": "secret-ns"}
	spec["replaceCRDs"] = true
	spec["wait"] = true
	chart.SetAnnotations(map[string]string{"example.test/private": "private-annotation"})
	chart.Object["status"] = map[string]any{"reason": "private-controller-message"}
	client, e := versionExplorer(chart)
	doc, err := e.HelmChartSource(ctx, "charts", "web")
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{"private", "annotations", "chartPullSecret", "replaceCRDs", "status", "resourceVersion"} {
		if strings.Contains(doc.YAML, hidden) {
			t.Fatalf("projection exposed %q", hidden)
		}
	}
	req := SourceFileRequest{Mode: "edit", Namespace: "charts", Name: "web", UID: doc.UID, ResourceVersion: doc.ResourceVersion,
		YAML: strings.ReplaceAll(strings.Replace(doc.YAML, "version: 1.2.3", "version: 2.0.0", 1), "    wait: true\n", "")}
	plan, err := e.PreviewHelmChartSource(ctx, req)
	wantDiff := []SourceFieldChange{{"spec.version", "1.2.3", "2.0.0"}, {"spec.wait", true, nil}}
	if err != nil || !reflect.DeepEqual(plan.Changes, wantDiff) {
		t.Fatalf("preview=%+v err=%v; YAML %q", plan, err, req.YAML)
	}
	req.Fingerprint = plan.Fingerprint
	var patchCount int
	client.PrependReactor("patch", "helmcharts", func(action clienttesting.Action) (bool, runtime.Object, error) {
		patchCount++
		p := action.(clienttesting.PatchAction)
		want := map[string]any{"metadata": map[string]any{"uid": "chart-uid", "resourceVersion": "17"}, "spec": map[string]any{"version": "2.0.0", "wait": nil}}
		var actual map[string]any
		if json.Unmarshal(p.GetPatch(), &actual) != nil || !reflect.DeepEqual(actual, want) || p.GetPatchType() != types.MergePatchType {
			t.Fatalf("incorrect conditional patch: %s", p.GetPatch())
		}
		return false, nil, nil
	})
	if _, err = e.SaveHelmChartSource(ctx, req); err != nil {
		t.Fatal(err)
	}
	updated, err := client.Resource(helmChartGVR).Namespace("charts").Get(ctx, "web", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	updatedSpec := updated.Object["spec"].(map[string]any)
	if !reflect.DeepEqual(updatedSpec["chartPullSecret"], spec["chartPullSecret"]) || updatedSpec["replaceCRDs"] != true ||
		updated.GetAnnotations()["example.test/private"] != "private-annotation" || updatedSpec["wait"] != nil {
		t.Fatal("patch altered fields outside its projection or retained the removed optional value")
	}
	if _, err = e.SaveHelmChartSource(ctx, req); !apierrors.IsConflict(err) || patchCount != 1 {
		t.Fatalf("edit replay patchCount=%d, err=%v", patchCount, err)
	}
}

func TestSourceFileRejectsUntrustedYAML(t *testing.T) {
	for name, contents := range map[string]string{
		"other resource":       strings.Replace(newChartYAML, "kind: HelmChart", "kind: Secret", 1),
		"wrong version":        strings.Replace(newChartYAML, "apps.clusternet.io/v1alpha1", "v1", 1),
		"multiple docs":        newChartYAML + "---\nkind: Secret\n",
		"second empty doc":     newChartYAML + "---\n",
		"duplicate key":        newChartYAML + "  repo: https://other.example.test\n",
		"document sequence":    "- apiVersion: v1\n",
		"secret ref":           newChartYAML + "  chartPullSecret: {name: private-input-marker}\n",
		"values":               newChartYAML + "  values: private-input-marker\n",
		"status":               newChartYAML + "status: {}\n",
		"managed metadata":     strings.Replace(newChartYAML, "  name: web", "  name: web\n  resourceVersion: '7'", 1),
		"path mismatch":        strings.Replace(newChartYAML, "namespace: charts", "namespace: other", 1),
		"invalid name":         strings.Replace(newChartYAML, "name: web", "name: a/b", 1),
		"plaintext credential": strings.Replace(newChartYAML, "https://charts.", "https://user:private-input-marker@charts.", 1),
		"signed link":          strings.Replace(newChartYAML, "/catalog", "/catalog?sig=private-input-marker", 1),
		"fragment":             strings.Replace(newChartYAML, "/catalog", "/catalog#private-input-marker", 1),
		"escape":               strings.Replace(newChartYAML, "/catalog", "/%74oken", 1),
		"file URL":             strings.Replace(newChartYAML, "https:", "file:", 1),
		"relative url path":    strings.Replace(newChartYAML, "/catalog", "/../catalog", 1),
		"alias":                strings.Replace(newChartYAML, "chart: web", "chart: &alias web", 1),
		"merge key":            newChartYAML + "  <<: {}\n",
		"explicit YAML tag":    strings.Replace(newChartYAML, "chart: web", "chart: !!str web", 1),
		"unknown option":       newChartYAML + "  force: true\n",
		"non-boolean option":   strings.Replace(newChartYAML, "wait: true", "wait: 'true'", 1),
		"wrong timeout type":   strings.Replace(newChartYAML, "120", "[120]", 1),
		"oversized timeout":    strings.Replace(newChartYAML, "120", "99999999999", 1),
		"missing version":      strings.Replace(newChartYAML, "  version: 1.2.3\n", "", 1),
		"unpinned version":     strings.Replace(newChartYAML, "1.2.3", "'*'", 1),
		"too large":            newChartYAML + strings.Repeat("#", maxSourceYAML),
	} {
		t.Run(name, func(t *testing.T) {
			client, e := versionExplorer()
			req := createSourceRequest()
			req.YAML = contents
			_, err := e.PreviewHelmChartSource(context.Background(), req)
			if !apierrors.IsBadRequest(err) || strings.Contains(err.Error(), "private-input-marker") || len(client.Actions()) != 0 {
				t.Fatalf("unsafe input was accepted, leaked, or reached Kubernetes: %v", err)
			}
		})
	}
}

func TestSourceFileRejectsUnsafeStoredConfigurationAndMoves(t *testing.T) {
	ctx := context.Background()
	t.Run("existing sensitive URL", func(t *testing.T) {
		chart := editableChart()
		chart.Object["spec"].(map[string]any)["repo"] = "https://private.test/charts?secret=private-stored-marker"
		_, e := versionExplorer(chart)
		got, err := e.HelmChartSource(ctx, "charts", "web")
		if !apierrors.IsConflict(err) || got.YAML != "" || strings.Contains(err.Error(), "private-stored-marker") {
			t.Fatalf("unsafe stored URL was returned: %+v %v", got, err)
		}
	})
	for _, test := range []struct{ name, from, to string }{
		{"pull secret repo redirection", "https://example.test", "https://elsewhere.test"},
		{"install namespace with pending subscription", "targetNamespace: frontend", "targetNamespace: elsewhere"},
	} {
		t.Run(test.name, func(t *testing.T) {
			chart := editableChart()
			chart.Object["spec"].(map[string]any)["chartPullSecret"] = map[string]any{"name": "credential-ref"}
			client, e := versionExplorer(chart, versionSub("apps", "pending", "charts"))
			doc, err := e.HelmChartSource(ctx, "charts", "web")
			if err != nil {
				t.Fatal(err)
			}
			req := SourceFileRequest{Mode: "edit", Namespace: "charts", Name: "web", UID: doc.UID, ResourceVersion: doc.ResourceVersion, YAML: strings.Replace(doc.YAML, test.from, test.to, 1)}
			if _, err := e.PreviewHelmChartSource(ctx, req); !apierrors.IsConflict(err) {
				t.Fatalf("unsafe move was accepted: %v", err)
			}
			for _, a := range client.Actions() {
				if a.GetVerb() != "get" && a.GetVerb() != "list" {
					t.Fatalf("preflight wrote %s", a.GetVerb())
				}
			}
		})
	}
}

func TestSourceFileStalePreviewAndObjectRace(t *testing.T) {
	ctx := context.Background()
	for _, scenario := range []string{"draft modified", "feed appeared", "new UID", "changed resourceVersion", "atomic race"} {
		t.Run(scenario, func(t *testing.T) {
			chart := editableChart()
			client, e := versionExplorer(chart)
			doc, err := e.HelmChartSource(ctx, "charts", "web")
			if err != nil {
				t.Fatal(err)
			}
			req := SourceFileRequest{Mode: "edit", Namespace: "charts", Name: "web", UID: doc.UID, ResourceVersion: doc.ResourceVersion, YAML: strings.Replace(doc.YAML, "1.2.3", "2.0.0", 1)}
			plan, err := e.PreviewHelmChartSource(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			req.Fingerprint = plan.Fingerprint
			switch scenario {
			case "draft modified":
				req.YAML = strings.Replace(req.YAML, "2.0.0", "2.0.1", 1)
			case "feed appeared":
				err = client.Tracker().Add(versionSub("charts", "new", "charts"))
			case "new UID":
				chart.SetUID("replacement")
				err = client.Tracker().Update(helmChartGVR, chart, "charts")
			case "changed resourceVersion":
				chart.SetResourceVersion("21")
				err = client.Tracker().Update(helmChartGVR, chart, "charts")
			case "atomic race":
				client.PrependReactor("patch", "helmcharts", func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewConflict(helmChartGVR.GroupResource(), "web", errors.New("private-admission-details"))
				})
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = e.SaveHelmChartSource(ctx, req)
			if !apierrors.IsConflict(err) || strings.Contains(err.Error(), "private-admission-details") {
				t.Fatalf("stale request accepted or admission response disclosed: %v", err)
			}
			writes := 0
			for _, a := range client.Actions() {
				if a.GetVerb() == "patch" {
					writes++
				}
			}
			if scenario == "atomic race" && writes != 1 || scenario != "atomic race" && writes != 0 {
				t.Fatalf("unexpected %d write(s) during %s", writes, scenario)
			}
		})
	}
}

func TestSourceFileNameFitsDefaultHelmRelease(t *testing.T) {
	_, e := versionExplorer()
	for _, length := range []int{53, 54} {
		req := createSourceRequest()
		req.Name = strings.Repeat("a", length)
		req.YAML = strings.Replace(req.YAML, "name: web", "name: "+req.Name, 1)
		_, err := e.PreviewHelmChartSource(t.Context(), req)
		if length == 53 && err != nil || length == 54 && !apierrors.IsBadRequest(err) {
			t.Fatalf("create with %d characters returned %v", length, err)
		}
	}
	old := editableChart()
	old.SetName(strings.Repeat("a", 54))
	_, e = versionExplorer(old)
	if _, err := e.HelmChartSource(t.Context(), old.GetNamespace(), old.GetName()); err != nil {
		t.Fatalf("existing long identity cannot be opened: %v", err)
	}
}

func TestSourceFileRejectsInvalidSaveIdentity(t *testing.T) {
	for _, defect := range []string{"namespace", "name", "uid", "resourceVersion", "replaced UID"} {
		t.Run(defect, func(t *testing.T) {
			ctx := t.Context()
			chart := editableChart()
			client, e := versionExplorer(chart)
			doc, err := e.HelmChartSource(ctx, chart.GetNamespace(), chart.GetName())
			if err != nil {
				t.Fatal(err)
			}
			req := SourceFileRequest{Mode: "edit", Namespace: doc.Namespace, Name: doc.Name, UID: doc.UID, ResourceVersion: doc.ResourceVersion,
				YAML: strings.Replace(doc.YAML, "1.2.3", "2.0.0", 1)}
			plan, err := e.PreviewHelmChartSource(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			req.Fingerprint = plan.Fingerprint
			saved := chart.DeepCopy()
			switch defect {
			case "namespace":
				saved.SetNamespace("wrong")
			case "name":
				saved.SetName("wrong")
			case "uid":
				saved.SetUID("")
			case "resourceVersion":
				saved.SetResourceVersion("")
			case "replaced UID":
				saved.SetUID("new-object")
			}
			client.PrependReactor("patch", "helmcharts", func(clienttesting.Action) (bool, runtime.Object, error) {
				return true, saved, nil
			})
			if id, err := e.SaveHelmChartSource(ctx, req); err == nil || id != (SourceFileIdentity{}) {
				t.Fatalf("untrusted success receipt: %+v %v", id, err)
			}
		})
	}
}

func TestChartWritesSerializeDeploymentWithSourceAndVersionEdits(t *testing.T) {
	for _, pair := range []struct{ first, second string }{
		{"source", "deployment"}, {"version", "deployment"}, {"deployment", "source"}, {"deployment", "version"},
	} {
		t.Run(pair.first+" before "+pair.second, func(t *testing.T) {
			ctx := t.Context()
			client, e, req := deploymentFixture()
			chart, err := e.HelmChartSource(ctx, "charts", "web")
			if err != nil {
				t.Fatal(err)
			}
			source := SourceFileRequest{Namespace: chart.Namespace, Name: chart.Name, Mode: "edit", UID: chart.UID,
				ResourceVersion: chart.ResourceVersion, YAML: strings.Replace(chart.YAML, "targetNamespace: workloads", "targetNamespace: moved", 1)}
			sp, err := e.PreviewHelmChartSource(ctx, source)
			if err != nil {
				t.Fatal(err)
			}
			source.Fingerprint = sp.Fingerprint
			impact, err := e.HelmChartVersionImpact(ctx, "charts", "web")
			if err != nil {
				t.Fatal(err)
			}
			dp, err := e.PreviewDeployment(ctx, "charts", "web", req)
			if err != nil {
				t.Fatal(err)
			}
			req.Fingerprint = dp.Fingerprint
			run := func(op string) error {
				switch op {
				case "deployment":
					_, err := e.CreateDeployment(ctx, "charts", "web", req)
					return err
				case "source":
					_, err := e.SaveHelmChartSource(ctx, source)
					return err
				default:
					_, err := e.ChangeHelmChartVersion(ctx, "charts", "web", chart.UID, impact.CurrentVersion, "3.0.0", impact.Fingerprint)
					return err
				}
			}
			atWrite := make(chan struct{})
			startSecond := make(chan struct{})
			verb, resource := "patch", "helmcharts"
			if pair.first == "deployment" {
				verb, resource = "create", "subscriptions"
			}
			client.PrependReactor(verb, resource, func(clienttesting.Action) (bool, runtime.Object, error) {
				// Kubernetes may block while committing. A fake client's own lock
				// would also serialize API calls here, so independently verify the
				// dashboard's cross-resource lock covers this exact boundary.
				if e.chartWriteMu.TryLock() {
					e.chartWriteMu.Unlock()
					t.Error("unprotected commit after impact recheck")
				}
				close(atWrite)
				<-startSecond
				return false, nil, nil
			})
			first, second := make(chan error, 1), make(chan error, 1)
			go func() { first <- run(pair.first) }()
			select {
			case err := <-first:
				t.Fatalf("first operation ended before committing: %v", err)
			case <-atWrite:
			}
			go func() { close(startSecond); second <- run(pair.second) }()
			if err := <-first; err != nil {
				t.Fatalf("first write rejected: %v", err)
			}
			if err := <-second; !apierrors.IsConflict(err) {
				t.Fatalf("second operator's stale preview should conflict, got %v", err)
			}
			writes := 0
			for _, a := range client.Actions() {
				if a.GetVerb() == "create" || a.GetVerb() == "patch" {
					writes++
				}
			}
			if writes != 1 {
				t.Fatalf("got %d Kubernetes writes, want one", writes)
			}
		})
	}
}
