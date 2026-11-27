# Clusternet Dashboard

A lightweight, optional, read-only dashboard for [Clusternet](https://github.com/clusternet/clusternet).

The first version focuses on operational visibility for parent / hub clusters:

- Managed cluster status and labels
- Subscription progress and target-cluster propagation status
- HelmRelease status and failure descriptions
- Clusternet API resources, component versions, and read-only RBAC diagnostics

## Security model

Clusternet Dashboard is intentionally read-only.

- The browser never receives Kubernetes credentials and never talks directly to the Kubernetes apiserver.
- The backend talks to Kubernetes using the configured kubeconfig in local development or an in-cluster ServiceAccount in Kubernetes.
- The Helm chart creates read-only RBAC by default.
- The dashboard does not include authentication in the first version. Deploy it behind a trusted network boundary such as VPN, private ingress, ingress authentication, or an existing identity-aware gateway.

Do not expose the dashboard publicly without authentication.

## Helm install

The published Helm chart pins the matching dashboard image. To install version `0.1.1` in a Clusternet parent cluster, run the following with an OCI-capable Helm client.

```bash
helm upgrade --install clusternet-dashboard \
  oci://registry-1.docker.io/changxuan/clusternet-dashboard \
  --version 0.1.1 \
  --namespace clusternet-system --create-namespace
```

The chart defaults to a ClusterIP service. To open the dashboard locally, forward the service port and visit [the dashboard](http://localhost:8080/clusternet/).

```bash
kubectl -n clusternet-system port-forward svc/clusternet-dashboard 8080:80
```

If your cluster has a private Ingress controller, set `ingressClassName` to its class (`nginx` in this example). Save these values as `values.yaml` and add `-f values.yaml` to the install command.

```yaml
ingress:
  enabled: true
  ingressClassName: nginx
  path: /clusternet
```

To install from a source checkout, use `./charts/clusternet-dashboard` as the chart argument, omit `--version`, and pass `--set-string image.tag=v0.1.1` to select the same image. See the [chart defaults](charts/clusternet-dashboard/values.yaml).

## How to contribute

When [reporting a bug](https://github.com/chxcode/clusternet-dashboard/issues), include the dashboard version, steps to reproduce it, and the expected and observed behavior. For a code or documentation change, start with the [contributing guide](CONTRIBUTING.md).

- [Run the dashboard locally](CONTRIBUTING.md#run-the-dashboard-locally) with or without a Clusternet cluster.
- [Check your changes](CONTRIBUTING.md#verify-your-change) and [open a pull request](CONTRIBUTING.md#open-a-pull-request) against `main`.
- Maintainers can [publish and verify a release](docs/releasing.md). Pushing a version tag publishes a multi-architecture image and an OCI Helm chart to Docker Hub.

Report suspected vulnerabilities privately. See the [security policy](SECURITY.md).

## API endpoints

These read-only endpoints are relative to `BASE_PATH`, which defaults to `/clusternet`.

```text
GET /api/health
GET /api/namespaces
GET /api/events
GET /api/clusternet/resources
GET /api/clusters
GET /api/subscriptions
GET /api/subscriptions/{namespace}/{name}
GET /api/manifests
GET /api/globalizations
GET /api/localizations
GET /api/feedinventories
GET /api/helmreleases
GET /api/diagnostics
```

## Operational notes

- HelmRelease listing can be slower than Subscription listing on large clusters. The frontend loads HelmReleases separately so the main Subscriptions page is not blocked.
- Older Clusternet versions may not expose `status.completedReleases`. In that case the UI labels fallback progress as observed status only.
- The dashboard is an optional component and can be upgraded independently from Clusternet core.

## Acknowledgements

This project was designed and implemented with the assistance of [Hermes Agent](https://github.com/NousResearch/hermes-agent), using the `gpt-5.5` model. The initial codebase was generated through an iterative discussion about Clusternet dashboard requirements, product scope, operational workflows, and implementation tradeoffs.

## License

This project is licensed under the Apache License 2.0. See [LICENSE](LICENSE).
