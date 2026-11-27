# Security Policy

## Supported versions

Clusternet Dashboard is currently in an initial MVP stage. Security fixes are applied to the latest released version.

## Reporting a vulnerability

Please report suspected vulnerabilities privately to the repository maintainers instead of opening a public issue.

When reporting, include:

- Affected version or commit
- Reproduction steps
- Impact assessment
- Any relevant logs or screenshots

## Security assumptions

- The default configuration is read-only. All visitors share the backend's Kubernetes identity. Restrict write routes at an authenticated gateway to the operators authorized to perform the corresponding action. Preserve the public `Host` header and configure the gateway's exact hostname under `labelEditing.allowedHosts` or `releaseOperations.allowedHosts`. Origin and host checks protect browser writes. The gateway authenticates the person making the request.
- The browser never receives Kubernetes credentials.
- The first version does not include built-in authentication. Deploy it behind a trusted network boundary or an identity-aware proxy.

### Optional write permissions

| Helm setting | Additional RBAC created by the chart | Data changed |
| --- | --- | --- |
| `labelEditing.enabled` | Cluster-wide `patch` on ManagedClusters | The label API changes the specified keys after checking the resource UID and their previous values. |
| `releaseOperations.triage` | Namespaced `get` and `patch` on the named follow-up ConfigMap | The dashboard records progress on a specific failed HelmRelease incident. The chart provisions the ConfigMap in the release namespace and marks it for retention on Helm uninstall. |
| `releaseOperations.chartVersionEditing` | Cluster-wide `patch` on HelmCharts | The version API changes only `spec.version` on a source HelmChart. Directly referencing subscriptions can roll this change out to multiple targets, including healthy clusters. |
| `releaseOperations.deleteFailedRelease` | Cluster-wide `delete` on individual HelmReleases | The API accepts a single failed HelmRelease with matching UID, resource version and active controlling Description that still declares the chart. Kubernetes processes its finalizers. The granted verb is `delete` only, with no `deletecollection` permission. |
| `releaseOperations.deployCharts` | Cluster-wide `create` on Subscriptions | An authorized operator can deploy an existing HelmChart to the selected fixed IDs, an ongoing label selector or all clusters. The handler creates one Replication Subscription in the source chart's namespace with a single Feed. |
| `releaseOperations.editChartSources` | Cluster-wide `create` and `patch` on HelmCharts | Authorized operators can create HelmChart sources and change the repository, chart, version and installation options used by existing subscriptions. |
| `releaseOperations.publishNativeWorkloads` | `get/create/patch` for Deployments, Services and ConfigMaps in API group `shadow`, plus `create/patch` for Subscriptions | An authorized operator can publish native workloads, read their configuration, change workloads or their target scope, and run containers on matching child clusters. |
| `registrationOperations.deleteFailedRequests` | `delete` for individual cluster-scoped ClusterRegistrationRequests | An operator can withdraw an eligible unapproved request after a fresh identity and allocation check. |

Kubernetes grants patch access at the resource level. The dashboard enforces the field restrictions above in its HTTP handlers. The impact preview is read-only and available with editing disabled. With custom RBAC, grant `get` on the source HelmChart and `list` on subscriptions across namespaces for this preview. Follow-up needs `get` on the failed HelmRelease. With a local kubeconfig, `KUBE_CONTEXT` selects the identity and cluster for this dashboard process. For local follow-up, pre-create the follow-up ConfigMap or grant the local identity permission to create it on first write. The defaults are namespace `clusternet-system` and name `clusternet-dashboard-triage`, configurable with `TRIAGE_NAMESPACE` and `TRIAGE_CONFIGMAP`.

Version changes recheck the resource identity, source version and previewed subscription bindings before patching. The eventual rollout follows Clusternet scheduling and per-cluster overrides. From upstream v0.14.0 through v0.18.1, ordinary HelmRelease annotation updates skip reconciliation. Deleting a HelmRelease can uninstall the workload, and reconciling an undeployed Helm revision with `atomic` enabled can uninstall and reinstall it. [v0.14.0 update handler](https://github.com/clusternet/clusternet/blob/v0.14.0/pkg/controllers/apps/helmrelease/helmrelease.go#L122-L138) · [v0.18.1 update filter](https://github.com/clusternet/clusternet/blob/v0.18.1/pkg/controllers/apps/helmrelease/helmrelease.go#L75-L94) · [v0.18.1 Helm reconciliation](https://github.com/clusternet/clusternet/blob/v0.18.1/pkg/utils/deployer.go#L145-L233)

The failed-release deletion switch, `ENABLE_FAILED_RELEASE_DELETION`, uses the release operation host allowlist (`WORKSPACE_WRITE_ALLOWED_HOSTS`). Custom RBAC also needs `get` on the HelmRelease and its owning Description. The application checks the failure phase and owner before sending Kubernetes a DELETE with UID and resourceVersion preconditions. Restrict access to operators authorized to uninstall workloads in the target clusters. An HTTP 202 acknowledges deletion. Clusternet may recreate the object with a new UID if its owner continues to declare it. Check the replacement's reported phase and the target workload before marking the incident resolved. [Owner notification](https://github.com/clusternet/clusternet/blob/v0.18.1/pkg/controllers/apps/description/description.go#L106-L133) · [replacement creation](https://github.com/clusternet/clusternet/blob/v0.18.1/pkg/controllermanager/deployer/helm/helm.go#L369-L379)

Chart deployment uses the independent `ENABLE_CHART_DEPLOYMENT` switch and `WORKSPACE_WRITE_ALLOWED_HOSTS`. Authorize operators who may deploy the selected chart to its child-cluster target namespace. With custom RBAC, grant `get` on the HelmCharts being compared, cluster-wide `list` on Namespaces, ManagedClusters, Bases, HelmReleases and Subscriptions, and `create` on Subscriptions in the source namespaces where deployments are allowed.

On confirmation the server rechecks the source chart, target identities and known conflicts before creating the named Subscription. The preview reads parent-cluster state. Other dashboard replicas, external controllers and operators can change that state concurrently. Existing Helm releases managed directly in child clusters can also affect the rollout. [Clusternet's Helm release name and target namespace](https://github.com/clusternet/clusternet/blob/v0.18.1/pkg/controllermanager/deployer/helm/helm.go#L281-L298)

A dynamic subscription applies its rule to future clusters as well. Authorize users of this feature to distribute to that future scope. Chart source editing uses the separate `ENABLE_CHART_SOURCE_EDITING` switch and the same trusted host configuration. The editor endpoint returns a projection of editable fields. Changes to a chart may roll out through existing subscriptions. Local-only editing processes drafts in the browser. When native publishing is enabled, the authenticated workbench can read and submit workload configuration. Deployment and ConfigMap bodies may contain sensitive application data. Restrict access to operators allowed to read that data and deploy to all selected child namespaces, including future matches. The publisher uses named Shadow endpoints and never needs write access to the parent’s native workload APIs or the Manifest backing store. The editor operations and registration withdrawal use the existing workspace write-host allowlist.
