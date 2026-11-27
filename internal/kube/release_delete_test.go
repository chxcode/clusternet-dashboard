package kube

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	restfake "k8s.io/client-go/rest/fake"
	clienttesting "k8s.io/client-go/testing"
)

func releaseDeletionObjects() (*unstructured.Unstructured, *unstructured.Unstructured) {
	gv := helmReleaseGVR.GroupVersion().String()
	ref := map[string]any{"apiVersion": gv, "kind": "Description", "name": "install", "uid": "owner-1", "controller": true, "blockOwnerDeletion": true}
	release := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gv, "kind": "HelmRelease",
		"metadata": map[string]any{"namespace": "edge", "name": "install-charts-web", "uid": "release-1", "resourceVersion": "5", "finalizers": []any{"apps.clusternet.io/finalizer"}, "ownerReferences": []any{ref}},
		"status":   map[string]any{"phase": "failed"},
	}}
	owner := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gv, "kind": "Description", "metadata": map[string]any{"namespace": "edge", "name": "install", "uid": "owner-1"},
		"spec": map[string]any{"deployer": "Helm", "charts": []any{map[string]any{"namespace": "charts", "name": "web"}}},
	}}
	return release, owner
}

func TestDeleteFailedReleaseRequiresCurrentDesiredOwner(t *testing.T) {
	for _, scenario := range []string{"empty UID", "empty version", "different UID", "different version", "release terminating", "release recovered",
		"no owner", "non-controller", "wrong group", "wrong version", "wrong kind", "owner without UID", "owner missing",
		"owner replaced", "owner terminating", "wrong deployer", "chart removed", "different chart namespace", "missing chart namespace", "read forbidden"} {
		t.Run(scenario, func(t *testing.T) {
			release, owner := releaseDeletionObjects()
			ref := release.Object["metadata"].(map[string]any)["ownerReferences"].([]any)[0].(map[string]any)
			now := metav1.Now()
			uid, rv := "release-1", "5"
			switch scenario {
			case "empty UID":
				uid = ""
			case "empty version":
				rv = ""
			case "different UID":
				uid = "other"
			case "different version":
				rv = "earlier"
			case "release terminating":
				release.SetDeletionTimestamp(&now)
			case "release recovered":
				_ = unstructured.SetNestedField(release.Object, "deployed", "status", "phase")
			case "no owner":
				release.SetOwnerReferences(nil)
			case "non-controller":
				ref["controller"] = false
			case "wrong group":
				ref["apiVersion"] = "another.example/v1alpha1"
			case "wrong version":
				ref["apiVersion"] = "apps.clusternet.io/v2"
			case "wrong kind":
				ref["kind"] = "HelmChart"
			case "owner without UID":
				delete(ref, "uid")
			case "owner missing":
				owner.SetNamespace("other")
			case "owner replaced":
				owner.SetUID("owner-2")
			case "owner terminating":
				owner.SetDeletionTimestamp(&now)
			case "wrong deployer":
				_ = unstructured.SetNestedField(owner.Object, "Generic", "spec", "deployer")
			case "chart removed":
				unstructured.RemoveNestedField(owner.Object, "spec", "charts")
			case "different chart namespace":
				_ = unstructured.SetNestedSlice(owner.Object, []any{map[string]any{"namespace": "other", "name": "web"}}, "spec", "charts")
			case "missing chart namespace":
				_ = unstructured.SetNestedSlice(owner.Object, []any{map[string]any{"name": "web"}}, "spec", "charts")
			}
			client := fake.NewSimpleDynamicClient(runtime.NewScheme(), release, owner)
			if scenario == "read forbidden" {
				client.PrependReactor("get", "descriptions", func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewForbidden(descriptionGVR.GroupResource(), "install", errors.New("forbidden"))
				})
			}
			err := NewExplorer(nil, nil, client).DeleteFailedRelease(t.Context(), "edge", "install-charts-web", uid, rv)
			switch scenario {
			case "empty UID", "empty version":
				if !apierrors.IsBadRequest(err) || len(client.Actions()) != 0 {
					t.Fatalf("wanted input error, got %v", err)
				}
			case "read forbidden":
				if !apierrors.IsForbidden(err) {
					t.Fatalf("wanted forbidden, got %v", err)
				}
			default:
				if !apierrors.IsConflict(err) {
					t.Fatalf("wanted conflict, got %v", err)
				}
			}
			for _, action := range client.Actions() {
				if action.GetVerb() != "get" {
					t.Fatalf("unexpected mutation: %s", action.GetVerb())
				}
			}
		})
	}
}

// The client-go dynamic fake discards DeleteOptions. Use the real serializer
// with an in-memory transport to verify the actual API request and stale race.
func TestDeleteFailedReleaseUsesBothPreconditionsOnWire(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusConflict, http.StatusForbidden, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			release, owner := releaseDeletionObjects()
			var calls []string
			transport := restfake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
				const resourcePrefix = "/apis/apps.clusternet.io/v1alpha1/namespaces/edge/"
				request := r.Method + " " + strings.TrimPrefix(r.URL.Path, resourcePrefix)
				calls = append(calls, request)
				var payload any
				code := http.StatusOK
				switch request {
				case "GET helmreleases/install-charts-web":
					payload = release.Object
				case "GET descriptions/install":
					payload = owner.Object
				case "DELETE helmreleases/install-charts-web":
					var opts metav1.DeleteOptions
					if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
						return nil, err
					}
					if opts.Preconditions == nil || opts.Preconditions.UID == nil || *opts.Preconditions.UID != "release-1" ||
						opts.Preconditions.ResourceVersion == nil || *opts.Preconditions.ResourceVersion != "5" || opts.GracePeriodSeconds != nil ||
						opts.PropagationPolicy != nil || opts.OrphanDependents != nil || len(opts.DryRun) > 0 {
						return nil, fmt.Errorf("unexpected DeleteOptions: %+v", opts)
					}
					code = status
					if code == http.StatusOK {
						payload = map[string]any{"apiVersion": "v1", "kind": "Status", "status": "Success"}
					} else {
						payload = map[string]any{"apiVersion": "v1", "kind": "Status", "status": "Failure", "reason": http.StatusText(code), "code": code}
					}
				default:
					return nil, fmt.Errorf("unexpected request: %s", request)
				}
				data, err := json.Marshal(payload)
				if err != nil {
					return nil, err
				}
				headers := http.Header{"Content-Type": {"application/json"}}
				if code == http.StatusTooManyRequests || code == http.StatusServiceUnavailable {
					headers.Set("Retry-After", "0")
				}
				return &http.Response{StatusCode: code, Header: headers, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
			})
			explorer, err := NewExplorerFromConfig(&rest.Config{Host: "https://parent.example.test", Transport: transport.Transport})
			if err != nil {
				t.Fatal(err)
			}
			err = explorer.DeleteFailedRelease(t.Context(), "edge", "install-charts-web", "release-1", "5")
			var upstream apierrors.APIStatus
			if status == http.StatusOK {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.As(err, &upstream) || int(upstream.Status().Code) != status {
				t.Fatalf("wanted API status %d, got %v", status, err)
			}
			want := []string{"GET helmreleases/install-charts-web", "GET descriptions/install", "DELETE helmreleases/install-charts-web"}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("unexpected calls or retry: %v", calls)
			}
		})
	}
}
