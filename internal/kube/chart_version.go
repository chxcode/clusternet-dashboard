package kube

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
)

// Helm also accepts v-prefixed exact versions. A numeric prerelease identifier
// may be "0" but cannot start with zero followed by more digits.
const semverNumber = `(0|[1-9][0-9]*)`
const semverPrerelease = `(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)`

var exactChartVersion = regexp.MustCompile(`^v?` + semverNumber + `\.` + semverNumber + `\.` + semverNumber +
	`(-` + semverPrerelease + `(\.` + semverPrerelease + `)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

type ChartVersionSubscription struct {
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	BoundTargets int    `json:"boundTargets"`
}

type ChartVersionImpact struct {
	UID            string                     `json:"uid"`
	Namespace      string                     `json:"namespace"`
	Name           string                     `json:"name"`
	CurrentVersion string                     `json:"currentVersion"`
	Fingerprint    string                     `json:"fingerprint"`
	Subscriptions  []ChartVersionSubscription `json:"subscriptions"`
	BoundTargets   int                        `json:"boundTargets"`
	Targets        []string                   `json:"targets"`

	resourceVersion string
}

type chartReferenceSnapshot struct {
	UID, Namespace, Name  string
	FeedVersions, Targets []string
	Scheduling            map[string]any
}

// HelmChartVersionImpact captures direct subscriptions and their currently
// reported bindings. Future scheduling or effective per-cluster overrides can
// change the eventual distribution.
func (e *Explorer) HelmChartVersionImpact(ctx context.Context, namespace, name string) (ChartVersionImpact, error) {
	var result ChartVersionImpact
	if e.dynamic == nil {
		return result, apierrors.NewServiceUnavailable("Kubernetes client unavailable")
	}
	chart, err := e.dynamic.Resource(helmChartGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return result, err
	}
	if chart.GetDeletionTimestamp() != nil {
		return result, apierrors.NewConflict(helmChartGVR.GroupResource(), name, errors.New("HelmChart is being deleted"))
	}
	if chart.GetUID() == "" || chart.GetResourceVersion() == "" {
		return result, errors.New("HelmChart lacks identity or resourceVersion for a conditional update")
	}
	current, _, err := unstructured.NestedString(chart.Object, "spec", "version")
	if err != nil {
		return result, err
	}
	subs, err := e.listDynamic(ctx, subscriptionGVR, metav1.NamespaceAll)
	if err != nil {
		return result, err
	}
	result = ChartVersionImpact{
		UID: string(chart.GetUID()), Namespace: namespace, Name: name, CurrentVersion: current,
		resourceVersion: chart.GetResourceVersion(), Subscriptions: []ChartVersionSubscription{},
	}
	allTargets := sets.New[string]()
	references := []chartReferenceSnapshot{}
	for i := range subs.Items {
		sub := &subs.Items[i]
		feedVersions := sets.New[string]()
		for _, feed := range subscriptionFeeds(sub) {
			// Clusternet v0.14's deployer fetches this namespaced chart by
			// kind/name/namespace even if the declared apiVersion differs.
			if feed.Kind == "HelmChart" && feed.Namespace == namespace && feed.Name == name {
				feedVersions.Insert(feed.APIVersion)
			}
		}
		if len(feedVersions) == 0 {
			continue
		}
		bindings, _, err := unstructured.NestedStringSlice(sub.Object, "status", "bindingClusters")
		if err != nil {
			return ChartVersionImpact{}, err
		}
		targets := sets.New(bindings...)
		targets.Delete("")
		allTargets.Insert(sets.List(targets)...)
		// Include scheduling inputs even while bindingClusters still reflects
		// a previous schedule. Other feeds and heartbeat/status data may vary.
		scheduling, _, err := unstructured.NestedMap(sub.Object, "spec")
		if err != nil {
			return ChartVersionImpact{}, err
		}
		delete(scheduling, "feeds")
		references = append(references, chartReferenceSnapshot{
			UID: string(sub.GetUID()), Namespace: sub.GetNamespace(), Name: sub.GetName(),
			FeedVersions: sets.List(feedVersions), Targets: sets.List(targets), Scheduling: scheduling,
		})
	}
	sort.Slice(references, func(i, j int) bool {
		if references[i].Namespace != references[j].Namespace {
			return references[i].Namespace < references[j].Namespace
		}
		return references[i].Name < references[j].Name
	})
	for _, sub := range references {
		result.Subscriptions = append(result.Subscriptions, ChartVersionSubscription{
			Namespace: sub.Namespace, Name: sub.Name, BoundTargets: len(sub.Targets),
		})
	}
	result.Targets = sets.List(allTargets)
	result.BoundTargets = len(result.Targets)
	snapshot, err := json.Marshal(struct {
		Kind, APIVersion, UID, Namespace, Name, Version string
		ChartGeneration                                 int64
		References                                      []chartReferenceSnapshot
	}{"HelmChart", helmChartGVR.GroupVersion().String(), result.UID, namespace, name, current, chart.GetGeneration(), references})
	if err != nil {
		return ChartVersionImpact{}, err
	}
	sum := sha256.Sum256(snapshot)
	result.Fingerprint = hex.EncodeToString(sum[:])
	return result, nil
}

func (e *Explorer) ChangeHelmChartVersion(ctx context.Context, namespace, name, uid, currentVersion, newVersion, fingerprint string) (string, error) {
	if uid == "" || len(fingerprint) != sha256.Size*2 {
		return "", apierrors.NewBadRequest("HelmChart UID and impact preview fingerprint are required")
	}
	if !exactChartVersion.MatchString(newVersion) ||
		strings.TrimPrefix(newVersion, "v") == strings.TrimPrefix(currentVersion, "v") {
		return "", apierrors.NewBadRequest("newVersion must be a different exact semantic version")
	}
	e.chartWriteMu.Lock()
	defer e.chartWriteMu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	stale := errors.New("HelmChart or subscription scope changed; review the impact again")
	var savedVersion string
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		impact, err := e.HelmChartVersionImpact(ctx, namespace, name)
		if err != nil {
			return err
		}
		if impact.UID != uid || impact.CurrentVersion != currentVersion || impact.Fingerprint != fingerprint {
			return stale
		}
		body, err := json.Marshal(map[string]any{
			"metadata": map[string]string{"resourceVersion": impact.resourceVersion},
			"spec":     map[string]string{"version": newVersion},
		})
		if err != nil {
			return err
		}
		obj, err := e.dynamic.Resource(helmChartGVR).Namespace(namespace).Patch(ctx, name, types.MergePatchType, body, metav1.PatchOptions{})
		if err != nil {
			return err
		}
		savedVersion, _, err = unstructured.NestedString(obj.Object, "spec", "version")
		if err == nil && savedVersion != newVersion {
			return fmt.Errorf("API returned HelmChart version %q after requesting %q; refresh the chart", savedVersion, newVersion)
		}
		return err
	})
	if errors.Is(err, stale) {
		return "", apierrors.NewConflict(helmChartGVR.GroupResource(), name, stale)
	}
	return savedVersion, err
}
