package kube

import "testing"

func TestNativeSubscriptionMatchesLaterRegistrations(t *testing.T) {
	client, explorer := nativeFixture(t)
	request := NativeWorkloadRequest{TargetMode: "selector", Selector: "zone=north", YAML: nativeDraft}
	result, err := explorer.ApplyNativeWorkload(t.Context(), "apps", "rollout", confirmNative(t, explorer, request))
	if err != nil || !result.SubscriptionSaved {
		t.Fatalf("publish with no current matches: %+v, %v", result, err)
	}
	checkCoverage := func(registered, matched int) {
		t.Helper()
		detail, err := explorer.SubscriptionDetail(t.Context(), "apps", "rollout")
		coverage := detail.Coverage
		if err != nil || coverage.LabelMatchedClusters == nil || *coverage.LabelMatchedClusters != matched || coverage.RegisteredClusters != registered {
			t.Fatalf("got coverage %+v (%v), want %d registered and %d matching", coverage, err, registered, matched)
		}
	}
	checkCoverage(2, 0)
	const newID = "aaaa0000-1234-abcd-0000-000000000003"
	cluster := deployCluster("north", newID)
	if err := client.Tracker().Create(managedClusterGVR, cluster, "north"); err != nil {
		t.Fatal(err)
	}
	checkCoverage(3, 1)
	cluster.SetLabels(map[string]string{deploymentIDLabel: newID, "zone": "west"})
	if err := client.Tracker().Update(managedClusterGVR, cluster, "north"); err != nil {
		t.Fatal(err)
	}
	checkCoverage(3, 0)
}

func TestSubscriptionLabelCoverage(t *testing.T) {
	clusters := []ManagedClusterSummary{
		{Labels: map[string]string{"app": "web", "region": "west"}},
		{Labels: map[string]string{"app": "web", "region": "east"}},
		{Labels: map[string]string{"app": "queue", "region": "east"}}, {},
	}
	west := "app=web,region in (west)"
	web := "app=web"
	invalid := "app in ("
	for _, tc := range []struct {
		scope string
		rules []*string
		want  int
	}{
		{"all", nil, 4}, {"none", nil, 0}, {"selector", []*string{&west}, 1},
		{"selector", []*string{&west, nil, &web}, 2},
		{"selector", []*string{&invalid}, -1}, {"unknown", nil, -1},
	} {
		got := subscriptionCoverage(SubscriptionDistribution{TargetScope: tc.scope, Selectors: tc.rules}, clusters, nil)
		if got.RegisteredClusters != 4 || (tc.want < 0) != (got.LabelMatchedClusters == nil) ||
			(got.LabelMatchedClusters != nil && *got.LabelMatchedClusters != tc.want) {
			t.Fatalf("coverage for scope %q: %#v, want %d matches", tc.scope, got, tc.want)
		}
	}
}

func TestSubscriptionCoverageUsesActualBindingIdentities(t *testing.T) {
	distribution := SubscriptionDistribution{TargetScope: "all"}
	clusters := []ManagedClusterSummary{
		{Namespace: "east", Name: "same"}, {Namespace: "west", Name: "same"}, {Namespace: "west", Name: "unique"},
	}
	got := subscriptionCoverage(distribution, clusters, []string{"east/same", "unique", "old/gone"})
	if len(got.UnboundMatches) != 1 || got.UnboundMatches[0] != (ClusterReference{"west", "same"}) {
		t.Fatalf("expected unmatched registered identity: %+v", got.UnboundMatches)
	}
	for _, unclear := range [][]string{nil, {"same"}, {"invalid/path/name"}} {
		if subscriptionCoverage(distribution, clusters, unclear).UnboundMatches != nil {
			t.Fatal("incomplete binding data cannot identify missing clusters")
		}
	}
	if len(subscriptionCoverage(distribution, clusters, []string{}).UnboundMatches) != 3 {
		t.Fatal("an explicit empty binding report should show all label matches")
	}
}
