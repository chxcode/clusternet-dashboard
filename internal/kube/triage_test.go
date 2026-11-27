package kube

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientfake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func triageFixture() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.clusternet.io/v1alpha1", "kind": "HelmRelease",
		"metadata": map[string]any{"name": "app", "namespace": "child", "uid": "release-uid", "generation": int64(2)},
		"status":   map[string]any{"phase": "failed", "description": "example failure", "version": int64(3)},
	}}
}

func TestTriageFirstReadAndLocalCreation(t *testing.T) {
	ctx := t.Context()
	obj := triageFixture()
	c := clientfake.NewSimpleClientset()
	d := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), obj)
	e := NewExplorer(c, nil, d)
	store := TriageStore{Namespace: "console", Name: "follow-up", CreateOnFirstWrite: true}
	incident := NewHelmReleaseSummary(obj).TriageIncident
	states, err := e.TriageStates(ctx, store)
	if err != nil || len(states) != 0 || len(c.Actions()) != 1 || c.Actions()[0].GetVerb() != "get" {
		t.Fatalf("initial read = %v, %v; actions: %v", states, err, c.Actions())
	}
	store.CreateOnFirstWrite = false
	if _, err = e.TriageStates(ctx, store); !apierrors.IsNotFound(err) {
		t.Errorf("missing provisioned store read: %v", err)
	}
	if err = e.SetTriage(ctx, store, "child", "app", "release-uid", incident, "open", "investigating"); !apierrors.IsNotFound(err) {
		t.Fatalf("missing provisioned store write: %v", err)
	}
	store.CreateOnFirstWrite = true
	for i := 0; i < 2; i++ {
		if err = e.SetTriage(ctx, store, "child", "app", "release-uid", incident, "open", "investigating"); err != nil {
			t.Fatal(err)
		}
	}
	states, err = e.TriageStates(ctx, store)
	if err != nil || !reflect.DeepEqual(states, map[string]string{incident: "investigating"}) {
		t.Fatalf("unexpected persisted records: %v, %v", states, err)
	}
	writes := 0
	for _, action := range c.Actions() {
		if action.GetVerb() == "get" {
			continue
		}
		if !action.Matches("create", "configmaps") || action.GetNamespace() != store.Namespace {
			t.Fatalf("unexpected action %v", action)
		}
		cm := action.(clienttesting.CreateAction).GetObject().(*corev1.ConfigMap)
		if cm.Name != store.Name || len(cm.Data) != 1 {
			t.Fatalf("unexpected storage %v", cm)
		}
		writes++
	}
	if writes != 1 {
		t.Fatalf("expected one create, got %d writes", writes)
	}
	for _, action := range d.Actions() {
		if !action.Matches("get", "helmreleases") {
			t.Fatalf("unexpected release operation %v", action)
		}
	}
}

func TestTriageClearTouchesOnlySelectedRecordAndIsIdempotent(t *testing.T) {
	ctx := t.Context()
	obj := triageFixture()
	incident := releaseIncident(obj)
	store := TriageStore{Namespace: "console", Name: "follow-up"}
	key := triagePrefix + digest("release-uid")
	c := clientfake.NewSimpleClientset(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: store.Namespace, Name: store.Name, ResourceVersion: "16"},
		Data: map[string]string{key: incident + ":resolved", "other": "keep"}})
	e := NewExplorer(c, nil, dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), obj))
	for i := 0; i < 2; i++ {
		if err := e.SetTriage(ctx, store, "child", "app", "release-uid", incident, "resolved", "open"); err != nil {
			t.Fatal(err)
		}
	}
	cm, err := c.CoreV1().ConfigMaps(store.Namespace).Get(ctx, store.Name, metav1.GetOptions{})
	if err != nil || !reflect.DeepEqual(cm.Data, map[string]string{"other": "keep"}) {
		t.Fatalf("expected only the selected record to be cleared, got %v, %v", cm, err)
	}
	var patches int
	for _, action := range c.Actions() {
		if action.GetVerb() == "get" {
			continue
		}
		p, ok := action.(clienttesting.PatchAction)
		if !ok || p.GetName() != store.Name || p.GetNamespace() != store.Namespace {
			t.Fatalf("unexpected write: %v", action)
		}
		var body map[string]any
		if err := json.Unmarshal(p.GetPatch(), &body); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"metadata": map[string]any{"resourceVersion": "16"}, "data": map[string]any{key: nil}}
		if !reflect.DeepEqual(body, want) {
			t.Fatalf("patch: %v, want %v", body, want)
		}
		patches++
	}
	if patches != 1 {
		t.Fatalf("expected one patch across retries, got %d", patches)
	}
}

func TestTriageRejectsStaleIncidentOrDeletedRelease(t *testing.T) {
	for _, change := range []string{"uid", "generation", "revision", "failure", "recovered", "deleting"} {
		t.Run(change, func(t *testing.T) {
			obj := triageFixture()
			incident := releaseIncident(obj)
			uid := string(obj.GetUID())
			switch change {
			case "uid":
				obj.SetUID("replacement")
			case "generation":
				obj.SetGeneration(3)
			case "revision":
				obj.Object["status"].(map[string]any)["version"] = int64(4)
			case "failure":
				obj.Object["status"].(map[string]any)["description"] = "different failure"
			case "recovered":
				obj.Object["status"].(map[string]any)["phase"] = "deployed"
			case "deleting":
				now := metav1.Now()
				obj.SetDeletionTimestamp(&now)
			}
			c := clientfake.NewSimpleClientset()
			e := NewExplorer(c, nil, dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), obj))
			err := e.SetTriage(t.Context(), TriageStore{Namespace: "console", Name: "records", CreateOnFirstWrite: true}, "child", "app", uid, incident, "open", "investigating")
			if !apierrors.IsConflict(err) || len(c.Actions()) != 0 {
				t.Fatalf("stale release: error %v, storage actions %v", err, c.Actions())
			}
		})
	}
}

func TestTriageRetriesConcurrentStoreChanges(t *testing.T) {
	for _, sameRecord := range []bool{true, false} {
		name := "unrelated"
		if sameRecord {
			name = "same record"
		}
		t.Run(name, func(t *testing.T) {
			obj := triageFixture()
			incident := releaseIncident(obj)
			store := TriageStore{Namespace: "console", Name: "records"}
			cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: store.Namespace, Name: store.Name, ResourceVersion: "3"}}
			c := clientfake.NewSimpleClientset(cm)
			resource := corev1.SchemeGroupVersion.WithResource("configmaps")
			patches := 0
			c.PrependReactor("patch", "configmaps", func(a clienttesting.Action) (bool, runtime.Object, error) {
				patches++
				if patches > 1 {
					var body struct {
						Metadata struct{ ResourceVersion string }
					}
					if err := json.Unmarshal(a.(clienttesting.PatchAction).GetPatch(), &body); err != nil || body.Metadata.ResourceVersion != "4" {
						t.Fatalf("retry used stale version: %#v %v", body, err)
					}
					return false, nil, nil
				}
				updated := cm.DeepCopy()
				updated.ResourceVersion = "4"
				updated.Data = map[string]string{"another": "keep"}
				if sameRecord {
					updated.Data[triagePrefix+digest("release-uid")] = incident + ":resolved"
				}
				if err := c.Tracker().Update(resource, updated, store.Namespace); err != nil {
					t.Fatal(err)
				}
				return true, nil, apierrors.NewConflict(resource.GroupResource(), store.Name, errors.New("simulated concurrent update"))
			})
			e := NewExplorer(c, nil, dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), obj))
			err := e.SetTriage(t.Context(), store, "child", "app", "release-uid", incident, "open", "investigating")
			want := "investigating"
			if sameRecord {
				want = "resolved"
				if !apierrors.IsConflict(err) || patches != 1 {
					t.Fatalf("same-key race: %v; patches %d", err, patches)
				}
			} else if err != nil || patches != 2 {
				t.Fatalf("unrelated race: %v; patches %d", err, patches)
			}
			record, err := c.CoreV1().ConfigMaps(store.Namespace).Get(t.Context(), store.Name, metav1.GetOptions{})
			if err != nil || record.Data["another"] != "keep" || record.Data[triagePrefix+digest("release-uid")] != incident+":"+want {
				t.Fatalf("unexpected retained entries %v, %v", record, err)
			}
		})
	}
}
