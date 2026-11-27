package kube

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	authv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	restfake "k8s.io/client-go/rest/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestWorkspaceCurrentContextMatchesClientConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	const config = `apiVersion: v1
kind: Config
clusters:
- name: hub
  cluster:
    server: https://parent.example.test
contexts:
- name: local-parent
  context:
    cluster: hub
current-context: local-parent
`
	if err := os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", path)
	cfg, alias, err := NewRestConfig()
	if err != nil || cfg == nil {
		t.Fatalf("load kubeconfig: %v", err)
	}
	if alias.InCluster || alias.Name != "local-parent" || cfg.Host != "https://parent.example.test" {
		t.Fatalf("alias %#v and host %q do not match the selected context", alias, cfg.Host)
	}
}

func TestWorkspaceRBACCheckPreservesConclusiveDecisions(t *testing.T) {
	const evaluationError = "one authorizer could not evaluate the request"
	statuses := map[string]authv1.SubjectAccessReviewStatus{
		managedClusterGVR.Resource: {EvaluationError: evaluationError},
		subscriptionGVR.Resource:   {Allowed: true, EvaluationError: evaluationError},
		manifestGVR.Resource:       {Denied: true, EvaluationError: evaluationError},
	}
	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "selfsubjectaccessreviews", func(action clienttesting.Action) (bool, runtime.Object, error) {
		review := action.(clienttesting.CreateAction).GetObject().(*authv1.SelfSubjectAccessReview).DeepCopy()
		review.Status = statuses[review.Spec.ResourceAttributes.Resource]
		return true, review, nil
	})
	checks := NewExplorer(client, nil, nil).readOnlyRBACChecks(t.Context())
	expectedChecks := map[string]bool{
		"list clusters.clusternet.io/clusterregistrationrequests": true, "list apps.clusternet.io/helmcharts": true, "list apps.clusternet.io/bases": true,
		"list clusters.clusternet.io/managedclusters": true,
		"list apps.clusternet.io/subscriptions":       true, "get apps.clusternet.io/subscriptions": true,
		"list apps.clusternet.io/helmreleases": true, "get apps.clusternet.io/descriptions": true,
		"list apps.clusternet.io/manifests": true, "list apps.clusternet.io/globalizations": true,
		"list apps.clusternet.io/localizations": true, "list apps.clusternet.io/feedinventories": true,
		"list apps/deployments": true, "list /events": true,
	}
	for _, action := range client.Actions() {
		review := action.(clienttesting.CreateAction).GetObject().(*authv1.SelfSubjectAccessReview)
		attr := review.Spec.ResourceAttributes
		key := fmt.Sprintf("%s %s/%s", attr.Verb, attr.Group, attr.Resource)
		if !expectedChecks[key] {
			t.Errorf("unexpected or duplicate access check: %s", key)
		}
		delete(expectedChecks, key)
	}
	if len(expectedChecks) > 0 {
		t.Errorf("missing access checks: %#v", expectedChecks)
	}
	for _, check := range checks {
		status := statuses[check.Resource]
		wantError := ""
		if !status.Allowed && !status.Denied {
			wantError = status.EvaluationError
		}
		if check.Allowed != status.Allowed || check.Error != wantError {
			t.Errorf("%s: allowed=%t, error=%q; want %t, %q", check.Resource, check.Allowed, check.Error, status.Allowed, wantError)
		}
	}
	if checks := NewExplorer(nil, nil, nil).readOnlyRBACChecks(t.Context()); len(checks) != 0 {
		t.Errorf("unconfigured client reported unexecuted checks: %#v", checks)
	}
}

func TestWorkspaceHelmReleasePagination(t *testing.T) {
	for _, secondPageStatus := range []int{http.StatusOK, http.StatusGone} {
		t.Run(http.StatusText(secondPageStatus), func(t *testing.T) {
			calls := 0
			transport := restfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				calls++
				query := req.URL.Query()
				if req.Method != http.MethodGet || req.URL.Path != "/apis/apps.clusternet.io/v1alpha1/helmreleases" || query.Get("limit") != "500" {
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL)
				}
				var body string
				code := http.StatusOK
				switch calls {
				case 1:
					if query.Get("continue") != "" {
						return nil, fmt.Errorf("unexpected initial continuation")
					}
					body = `{"apiVersion":"apps.clusternet.io/v1alpha1","kind":"HelmReleaseList","metadata":{"continue":"next-page"},"items":[{"metadata":{"name":"first","uid":"uid-first"}}]}`
				case 2:
					if query.Get("continue") != "next-page" {
						return nil, fmt.Errorf("missing previous continuation")
					}
					code = secondPageStatus
					body = `{"apiVersion":"apps.clusternet.io/v1alpha1","kind":"HelmReleaseList","items":[{"metadata":{"name":"second","uid":"uid-second"}}]}`
					if code != http.StatusOK {
						body = `{"apiVersion":"v1","kind":"Status","status":"Failure","code":410,"reason":"Expired","message":"expired continuation"}`
					}
				default:
					return nil, fmt.Errorf("unexpected request after final page")
				}
				return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			client, err := dynamic.NewForConfigAndClient(&rest.Config{Host: "https://parent.example.test"}, transport)
			if err != nil {
				t.Fatal(err)
			}
			items, err := NewExplorer(nil, nil, client).HelmReleases(t.Context())
			if calls != 2 {
				t.Fatalf("got %d requests, want 2", calls)
			}
			if secondPageStatus == http.StatusGone {
				if err == nil || len(items) != 0 {
					t.Fatalf("partial list reported after expired continuation: %#v, %v", items, err)
				}
				return
			}
			if err != nil || len(items) != 2 || items[0].UID != "uid-first" || items[1].UID != "uid-second" {
				t.Fatalf("combined release list: %#v, %v", items, err)
			}
		})
	}
}
