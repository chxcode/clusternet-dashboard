package kube

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	kt "k8s.io/client-go/testing"
)

const nativeDraft = `apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
data:
  team: support
---
apiVersion: v1
kind: Service
metadata:
  name: web
spec:
  ports:
    - port: 80
  selector:
    app: web
`

// Model the two upstream shadow layers. GET decorates the raw template with
// Manifest UID/RV. PATCH runs JSON Patch on the undecorated original instead.
func nativeFixture(t *testing.T) (*fake.FakeDynamicClient, *Explorer) {
	t.Helper()
	kinds := map[schema.GroupVersionResource]string{manifestGVR: "ManifestList", subscriptionGVR: "SubscriptionList", baseGVR: "BaseList", managedClusterGVR: "ManagedClusterList", deploymentNamespaces: "NamespaceList"}
	for k := range nativeKinds {
		kinds[shadowGVR(k)] = k + "List"
	}
	c := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), kinds, deployCluster("east", demoID1), deployNamespace("east", demoID1), deployCluster("west", demoID2), deployNamespace("west", demoID2))
	c.PrependReactor("create", "subscriptions", func(a kt.Action) (bool, runtime.Object, error) {
		s := a.(kt.CreateAction).GetObject().(*unstructured.Unstructured)
		s.SetUID("sub-uid")
		s.SetResourceVersion("1")
		return false, nil, nil
	})
	n := 0
	c.PrependReactor("*", "*", func(a kt.Action) (bool, runtime.Object, error) {
		if a.GetResource().Group != "shadow" {
			return false, nil, nil
		}
		resource, ns := a.GetResource().Resource, a.GetNamespace()
		var raw *unstructured.Unstructured
		var name string
		switch a.GetVerb() {
		case "get":
			name = a.(kt.GetAction).GetName()
		case "patch":
			name = a.(kt.PatchAction).GetName()
		case "create":
			raw = a.(kt.CreateAction).GetObject().(*unstructured.Unstructured).DeepCopy()
			name = raw.GetName()
		default:
			t.Fatalf("unexpected shadow verb %s", a.GetVerb())
		}
		storageName := resource + "." + ns + "." + name
		m, err := c.Tracker().Get(manifestGVR, "reserved", storageName)
		var manifest *unstructured.Unstructured
		if err == nil {
			manifest = m.(*unstructured.Unstructured).DeepCopy()
			if a.GetVerb() == "create" {
				return true, nil, apierrors.NewAlreadyExists(a.GetResource().GroupResource(), name)
			}
			v, _, _ := unstructured.NestedMap(manifest.Object, "template")
			raw = &unstructured.Unstructured{Object: v}
		} else if a.GetVerb() != "create" {
			group := ""
			if resource == "deployments" {
				group = "apps"
			}
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: group, Resource: resource}, name)
		}
		if a.GetVerb() == "create" {
			n++
			raw.SetUID(types.UID(fmt.Sprintf("raw-%d", n)))
			manifest = &unstructured.Unstructured{Object: map[string]any{"apiVersion": manifestGVR.GroupVersion().String(), "kind": "Manifest", "metadata": map[string]any{"name": storageName, "namespace": "reserved"}}}
			manifest.SetUID(types.UID(fmt.Sprintf("manifest-%d", n)))
			manifest.SetLabels(map[string]string{"apps.clusternet.io/config.kind": raw.GetKind(), "apps.clusternet.io/config.namespace": ns, "apps.clusternet.io/config.name": name})
			if raw.GetKind() == "Service" {
				_ = unstructured.SetNestedField(raw.Object, "ClusterIP", "spec", "type")
			}
		} else if a.GetVerb() == "patch" {
			p, err := jsonpatch.DecodePatch(a.(kt.PatchAction).GetPatch())
			if err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(raw.Object)
			data, err := p.Apply(b)
			if err != nil {
				return true, nil, apierrors.NewConflict(a.GetResource().GroupResource(), name, err)
			}
			if err = raw.UnmarshalJSON(data); err != nil {
				t.Fatal(err)
			}
		}
		if a.GetVerb() != "get" {
			n++
			manifest.SetResourceVersion(fmt.Sprint(n))
			_ = unstructured.SetNestedMap(manifest.Object, raw.Object, "template")
			if a.GetVerb() == "create" {
				err = c.Tracker().Create(manifestGVR, manifest, "reserved")
			} else {
				err = c.Tracker().Update(manifestGVR, manifest, "reserved")
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		raw.SetUID(manifest.GetUID())
		raw.SetResourceVersion(manifest.GetResourceVersion())
		return true, raw, nil
	})
	return c, NewExplorer(nil, nil, c)
}

func confirmNative(t *testing.T, e *Explorer, req NativeWorkloadRequest) NativeWorkloadRequest {
	t.Helper()
	p, err := e.PreviewNativeWorkload(t.Context(), "apps", "rollout", req)
	if err != nil || len(p.Conflicts) > 0 {
		t.Fatalf("preview: %+v %v", p.Conflicts, err)
	}
	req.Fingerprint = p.Fingerprint
	return req
}
func TestNativeShadowBundleCreateReadEdit(t *testing.T) {
	client, e := nativeFixture(t)
	req := confirmNative(t, e, NativeWorkloadRequest{TargetMode: "all", YAML: nativeDraft})
	for _, a := range client.Actions() {
		if a.GetVerb() != "get" && a.GetVerb() != "list" {
			t.Fatalf("preview wrote %v", a)
		}
	}
	result, err := e.ApplyNativeWorkload(t.Context(), "apps", "rollout", req)
	if err != nil || result.Status != "saved" || len(result.Resources) != 2 || !result.SubscriptionSaved {
		t.Fatalf("create: %+v %v", result, err)
	}
	file, err := e.NativeWorkload(t.Context(), "apps", "rollout")
	if err != nil || len(file.SourceBaselines) != 2 || strings.Contains(file.YAML, "sha256") || strings.Contains(file.YAML, "raw-") {
		t.Fatalf("load: %+v %v", file, err)
	}
	source, err := e.NativeSource(t.Context(), "apps", "ConfigMap", "settings")
	if err != nil || len(source.Subscriptions) != 1 || source.Subscriptions[0] != "apps/rollout" {
		t.Fatalf("source: %+v %v", source, err)
	}
	s, err := client.Resource(subscriptionGVR).Namespace("apps").Get(t.Context(), "rollout", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(subscriptionFeeds(s)) != 2 {
		t.Fatalf("wrong feeds %+v", s)
	}
	_ = unstructured.SetNestedStringSlice(s.Object, []string{"east/edge", "west/edge"}, "status", "bindingClusters")
	if err = client.Tracker().Update(subscriptionGVR, s, "apps"); err != nil {
		t.Fatal(err)
	}
	edit := file.NativeWorkloadRequest
	edit.TargetMode = "selector"
	edit.Selector = "zone=east"
	edit.YAML = strings.Replace(edit.YAML, "team: support", "team: engineering", 1)
	p, err := e.PreviewNativeWorkload(t.Context(), "apps", "rollout", edit)
	if err != nil || len(p.Conflicts) > 0 || !strings.Contains(strings.Join(p.ScopeWarnings, " "), "子端资源可能撤回（1）：west/edge") || p.SubscriptionAction != "update" {
		t.Fatalf("edit plan: %+v %v", p, err)
	}
	edit.Fingerprint = p.Fingerprint
	result, err = e.ApplyNativeWorkload(t.Context(), "apps", "rollout", edit)
	if err != nil || result.Status != "saved" {
		t.Fatalf("edit: %+v %v", result, err)
	}
	file2, err := e.NativeWorkload(t.Context(), "apps", "rollout")
	if err != nil || !strings.Contains(file2.YAML, "team: engineering") || file2.Selector != "zone=east" {
		t.Fatalf("saved file %+v %v", file2, err)
	}
	if _, err = e.ApplyNativeWorkload(t.Context(), "apps", "rollout", edit); !apierrors.IsConflict(err) {
		t.Fatalf("stale submit %v", err)
	}
	for _, a := range client.Actions() {
		if a.GetVerb() == "list" || a.GetVerb() == "get" {
			continue
		}
		g := a.GetResource()
		if g.Group != "shadow" && g != subscriptionGVR {
			t.Fatalf("unexpected target %v", a)
		}
	}
}

func TestNativePartialSaveAndResume(t *testing.T) {
	client, e := nativeFixture(t)
	fail := true
	client.PrependReactor("create", "services", func(a kt.Action) (bool, runtime.Object, error) {
		if fail {
			fail = false
			return true, nil, apierrors.NewForbidden(a.GetResource().GroupResource(), "web", errors.New("denied"))
		}
		return false, nil, nil
	})
	req := confirmNative(t, e, NativeWorkloadRequest{TargetMode: "all", YAML: nativeDraft})
	r, err := e.ApplyNativeWorkload(t.Context(), "apps", "rollout", req)
	if err != nil || r.Status != "partial" || len(r.Resources) != 1 || r.SubscriptionSaved {
		t.Fatalf("%+v %v", r, err)
	}
	req = confirmNative(t, e, req)
	r, err = e.ApplyNativeWorkload(t.Context(), "apps", "rollout", req)
	if err != nil || r.Status != "saved" || r.Resources[0].Action != "unchanged" {
		t.Fatalf("resume %+v %v", r, err)
	}
	n := 0
	for _, a := range client.Actions() {
		if a.GetVerb() == "create" && a.GetResource() == shadowGVR("ConfigMap") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("repeated source create %d", n)
	}
}

func TestNativeRejectsMixedResourcesAndUnsafeMetadata(t *testing.T) {
	for _, input := range []string{strings.Replace(nativeDraft, "kind: ConfigMap", "kind: Secret", 1), strings.Replace(nativeDraft, "name: settings", "name: settings\n  namespace: foreign", 1), strings.Replace(nativeDraft, "name: settings", "name: settings\n  uid: replace", 1), strings.Replace(nativeDraft, "  team: support", "  team: support\n  team: override", 1), strings.Replace(nativeDraft, "  team: support", "  team: &x support\n  other: *x", 1), strings.Replace(nativeDraft, "name: settings", "name: settings\n  annotations:\n    apps.clusternet.io/skip-validating: 'true'", 1), nativeDraft + "---\n" + nativeDraft, strings.Replace(nativeDraft, "    - port: 80", "    - port: 99999", 1)} {
		if _, err := parseNativeYAML("apps", input); err == nil {
			t.Fatalf("accepted unexpected YAML")
		}
	}
}
func TestNativeShadowAbsentAndSharedSources(t *testing.T) {
	t.Run("unserved", func(t *testing.T) {
		c, e := nativeFixture(t)
		c.PrependReactor("get", "configmaps", func(a kt.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "shadow", Resource: "configmaps"}, "")
		})
		if _, err := e.PreviewNativeWorkload(t.Context(), "apps", "rollout", NativeWorkloadRequest{TargetMode: "all", YAML: nativeDraft}); !apierrors.IsServiceUnavailable(err) {
			t.Fatal(err)
		}
		for _, a := range c.Actions() {
			if a.GetVerb() != "get" && a.GetVerb() != "list" {
				t.Fatal("unexpected write")
			}
		}
	})
	t.Run("shared", func(t *testing.T) {
		_, e := nativeFixture(t)
		req := confirmNative(t, e, NativeWorkloadRequest{TargetMode: "all", YAML: nativeDraft})
		if r, err := e.ApplyNativeWorkload(t.Context(), "apps", "rollout", req); err != nil || r.Status != "saved" {
			t.Fatalf("%+v %v", r, err)
		}
		s, err := e.NativeSource(t.Context(), "apps", "ConfigMap", "settings")
		if err != nil {
			t.Fatal(err)
		}
		p, err := e.PreviewNativeWorkload(t.Context(), "apps", "another", NativeWorkloadRequest{TargetMode: "all", YAML: s.YAML, SourceBaselines: map[string]string{"ConfigMap/settings": s.Baseline}})
		if err != nil || !strings.Contains(strings.Join(p.Conflicts, " "), "apps/rollout") {
			t.Fatalf("%v %v", p.Conflicts, err)
		}
	})
}

func TestNativeExternalSourceRequiresOpenedSnapshot(t *testing.T) {
	client, e := nativeFixture(t)
	obj, _ := parseNativeYAML("apps", strings.Split(nativeDraft, "---")[0])
	_, err := client.Resource(shadowGVR("ConfigMap")).Namespace("apps").Create(t.Context(), obj[0], metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	req := NativeWorkloadRequest{TargetMode: "selector", Selector: "zone=north", YAML: strings.Replace(strings.Split(nativeDraft, "---")[0], "team: support", "team: operations", 1)}
	p, err := e.PreviewNativeWorkload(t.Context(), "apps", "rollout", req)
	if err != nil || len(p.Conflicts) != 1 {
		t.Fatalf("unopened existing source %v %v", p.Conflicts, err)
	}
	file, err := e.NativeSource(t.Context(), "apps", "ConfigMap", "settings")
	if err != nil || len(file.Subscriptions) > 0 {
		t.Fatal(err)
	}
	req.SourceBaselines = map[string]string{"ConfigMap/settings": file.Baseline}
	req = confirmNative(t, e, req)
	// Simulate foreign editing of the raw template between the final preview
	// and the shadow patch callback. Keep the outer UID unchanged.
	client.PrependReactor("patch", "configmaps", func(a kt.Action) (bool, runtime.Object, error) {
		x, err := client.Tracker().Get(manifestGVR, "reserved", "configmaps.apps.settings")
		if err != nil {
			t.Fatal(err)
		}
		m := x.(*unstructured.Unstructured).DeepCopy()
		_ = unstructured.SetNestedField(m.Object, "concurrent", "template", "data", "team")
		if err = client.Tracker().Update(manifestGVR, m, "reserved"); err != nil {
			t.Fatal(err)
		}
		return false, nil, nil
	})
	result, err := e.ApplyNativeWorkload(t.Context(), "apps", "rollout", req)
	if !apierrors.IsConflict(err) || len(result.Resources) > 0 || result.SubscriptionSaved {
		t.Fatalf("lost update %+v %v", result, err)
	}
	file2, err := e.NativeSource(t.Context(), "apps", "ConfigMap", "settings")
	if err != nil || !strings.Contains(file2.YAML, "team: concurrent") {
		t.Fatalf("unexpected source %+v %v", file2, err)
	}
	p, err = e.PreviewNativeWorkload(t.Context(), "apps", "rollout", req)
	if err != nil || len(p.Conflicts) != 1 {
		t.Fatalf("stale opened file approved %+v %v", p.Conflicts, err)
	}
}

func TestNativeUnknownResponseStopsFollowingWrites(t *testing.T) {
	client, e := nativeFixture(t)
	req := confirmNative(t, e, NativeWorkloadRequest{TargetMode: "all", YAML: nativeDraft})
	client.PrependReactor("create", "configmaps", func(a kt.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewTimeoutError("timeout", 1)
	})
	client.ClearActions()
	result, err := e.ApplyNativeWorkload(t.Context(), "apps", "rollout", req)
	if err != nil || result.Status != "uncertain" || len(result.Resources) != 0 || result.SubscriptionSaved {
		t.Fatalf("unknown outcome %+v %v", result, err)
	}
	attempts := 0
	for _, a := range client.Actions() {
		if a.GetVerb() == "create" || a.GetVerb() == "patch" {
			attempts++
			if a.GetResource() != shadowGVR("ConfigMap") {
				t.Fatalf("advanced to next object %v", a)
			}
		}
	}
	if attempts != 1 {
		t.Fatalf("unexpected writes %d", attempts)
	}
}

func TestNativeValidDeploymentAndSelectors(t *testing.T) {
	file := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  replicas: 0
  selector:
    matchLabels: {app: api}
  template:
    metadata:
      labels: {app: api}
    spec:
      containers:
      - name: api
        image: example.test/app:1
`
	v, err := parseNativeYAML("workloads", file)
	if err != nil || len(v) != 1 || v[0].GetNamespace() != "workloads" {
		t.Fatal(err)
	}
	for _, bad := range []string{strings.Replace(file, "matchLabels: {app: api}", "matchLabels: {app: other}", 1), strings.Replace(file, "replicas: 0", "replicas: -1", 1), strings.Replace(file, "image: example.test/app:1", "unrecognizedKey: x", 1)} {
		if _, err = parseNativeYAML("workloads", bad); err == nil {
			t.Fatal("invalid Deployment accepted")
		}
	}
}

func TestNativeUpstreamErrorsDoNotDisclosePayloads(t *testing.T) {
	const marker = "private-template-fragment"
	for _, resource := range []string{"configmaps", "subscriptions", "bases", "managedclusters"} {
		t.Run(resource, func(t *testing.T) {
			c, e := nativeFixture(t)
			c.PrependReactor("*", resource, func(a kt.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewForbidden(a.GetResource().GroupResource(), "", errors.New(marker))
			})
			p, err := e.PreviewNativeWorkload(t.Context(), "apps", "rollout", NativeWorkloadRequest{TargetMode: "all", YAML: nativeDraft})
			if !apierrors.IsForbidden(err) || strings.Contains(err.Error(), marker) || p.Fingerprint != "" {
				t.Fatalf("unsafe error or preview: %v", err)
			}
		})
	}
	t.Run("gateway fabricated a named 404", func(t *testing.T) {
		c, e := nativeFixture(t)
		c.PrependReactor("get", "configmaps", func(a kt.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewGenericServerResponse(404, "get", a.GetResource().GroupResource(), "settings", marker, 0, true)
		})
		if _, err := e.PreviewNativeWorkload(t.Context(), "apps", "rollout", NativeWorkloadRequest{TargetMode: "all", YAML: nativeDraft}); !apierrors.IsServiceUnavailable(err) || strings.Contains(err.Error(), marker) {
			t.Fatalf("ambiguous 404 should block: %v", err)
		}
	})
}

func TestNativeServiceShadowAllocations(t *testing.T) {
	for _, tc := range []struct{ spec, field string }{
		{"clusterIP: 10.23.0.2\n  ports: [{port: 80}]", "clusterIP"},
		{"clusterIPs: [10.23.0.2]\n  ports: [{port: 80}]", "clusterIPs"},
		{"type: NodePort\n  ports: [{port: 80, nodePort: 30001}]", "nodePort"},
		{"type: LoadBalancer\n  healthCheckNodePort: 32001\n  ports: [{port: 80}]", "healthCheckNodePort"},
		{"type: NodePort\n  ports: [{port: 80}]", ""},
		{"type: LoadBalancer\n  externalTrafficPolicy: Local\n  ports: [{port: 80}]", ""},
		{"clusterIP: None\n  clusterIPs: [None]", ""},
	} {
		doc := "apiVersion: v1\nkind: Service\nmetadata: {name: endpoint}\nspec:\n  " + tc.spec + "\n"
		c, e := nativeFixture(t)
		p, err := e.PreviewNativeWorkload(t.Context(), "apps", "rollout", NativeWorkloadRequest{TargetMode: "all", YAML: doc})
		if tc.field == "" {
			if err != nil || len(p.Conflicts) > 0 {
				t.Fatalf("valid Service %v %v", p.Conflicts, err)
			}
		} else if !apierrors.IsBadRequest(err) || !strings.Contains(err.Error(), tc.field) || len(c.Actions()) > 0 {
			t.Fatalf("unsupported field %q got %v actions %d", tc.field, err, len(c.Actions()))
		}
	}
	// An upstream v0.14 dry-run allocates this port on creation. Retain it
	// internally when editing other fields, but do not ask users to submit it.
	obj, err := parseNativeYAML("apps", "apiVersion: v1\nkind: Service\nmetadata: {name: edge}\nspec:\n  type: LoadBalancer\n  externalTrafficPolicy: Local\n  ports: [{port: 80}]\n")
	if err != nil {
		t.Fatal(err)
	}
	raw := obj[0].DeepCopy()
	_ = unstructured.SetNestedField(raw.Object, int64(30001), "spec", "healthCheckNodePort")
	file, err := nativeFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := unstructured.NestedInt64(file.Object, "spec", "healthCheckNodePort"); ok {
		t.Fatal("hidden upstream allocation leaked")
	}
	next := nativeWritable(file, &nativeLive{raw: raw}, "apps/editor", "intent")
	if port, _, _ := unstructured.NestedInt64(next.Object, "spec", "healthCheckNodePort"); port != 30001 {
		t.Fatal("existing v0.14 allocation lost")
	}
}

func TestNativeQuantitiesAndDeliberateFieldRemoval(t *testing.T) {
	const workload = `apiVersion: apps/v1
kind: Deployment
metadata: {name: api}
spec:
  selector: {matchLabels: {app: api}}
  template:
    metadata: {labels: {app: api}}
    spec:
      containers:
      - name: api
        image: example.test/app:1
        env: [{name: FACTOR, value: "0.5"}]
        resources: {requests: {cpu: "0.5"}}
`
	client, e := nativeFixture(t)
	req := confirmNative(t, e, NativeWorkloadRequest{TargetMode: "all", YAML: workload})
	if r, err := e.ApplyNativeWorkload(t.Context(), "apps", "rollout", req); err != nil || r.Status != "saved" {
		t.Fatalf("CPU quantity failed to converge: %+v %v", r, err)
	}
	doc, err := e.NativeWorkload(t.Context(), "apps", "rollout")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := parseNativeYAML("apps", doc.YAML)
	if err != nil {
		t.Fatal(err)
	}
	cs, _, _ := unstructured.NestedSlice(obj[0].Object, "spec", "template", "spec", "containers")
	container := cs[0].(map[string]any)
	cpu, _, _ := unstructured.NestedString(container, "resources", "requests", "cpu")
	env, _, _ := unstructured.NestedSlice(container, "env")
	if cpu != "500m" || env[0].(map[string]any)["value"] != "0.5" {
		t.Fatalf("wrong resource normalization: %v", container)
	}
	// An externally added field is in the document the author opened. A fresh
	// explicit edit that removes it must issue a patch, even when the remaining
	// fields and older dashboard intent marker match a previous create.
	x, err := client.Tracker().Get(manifestGVR, "reserved", "deployments.apps.api")
	if err != nil {
		t.Fatal(err)
	}
	m := x.(*unstructured.Unstructured).DeepCopy()
	_ = unstructured.SetNestedField(m.Object, int64(3), "template", "spec", "replicas")
	if err = client.Tracker().Update(manifestGVR, m, "reserved"); err != nil {
		t.Fatal(err)
	}
	doc, err = e.NativeWorkload(t.Context(), "apps", "rollout")
	if err != nil {
		t.Fatal(err)
	}
	edit := doc.NativeWorkloadRequest
	edit.YAML = workload
	p, err := e.PreviewNativeWorkload(t.Context(), "apps", "rollout", edit)
	if err != nil || p.Resources[0].Action != "update" {
		t.Fatalf("explicit removal hidden: %+v %v", p.Resources, err)
	}
	edit.Fingerprint = p.Fingerprint
	if r, err := e.ApplyNativeWorkload(t.Context(), "apps", "rollout", edit); err != nil || r.Status != "saved" {
		t.Fatalf("removal: %+v %v", r, err)
	}
	doc, err = e.NativeWorkload(t.Context(), "apps", "rollout")
	if err != nil || strings.Contains(doc.YAML, "replicas:") {
		t.Fatalf("replicas retained: %v", err)
	}
}

func TestNativeSourceNameFitsManifestLabel(t *testing.T) {
	name := strings.Repeat("n", 64)
	client, e := nativeFixture(t)
	_, err := e.PreviewNativeWorkload(t.Context(), "apps", "rollout", NativeWorkloadRequest{TargetMode: "all", YAML: strings.Replace(nativeDraft, "name: settings", "name: "+name, 1)})
	if !apierrors.IsBadRequest(err) || !strings.Contains(err.Error(), "63") || len(client.Actions()) != 0 {
		t.Fatalf("long source name %v", err)
	}
}
