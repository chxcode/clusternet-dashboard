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

## Local development

Prerequisites:

- Go 1.26+
- Node.js 22+
- npm
- kubectl access to a Clusternet parent / hub cluster, if you want real cluster data

Run backend tests:

```bash
go test ./...
```

Run frontend tests:

```bash
npm test --prefix web
```

Build frontend:

```bash
npm run build --prefix web
```

Build backend:

```bash
go build ./cmd/clusternet-dashboard
```

Run locally:

```bash
BASE_PATH=/clusternet STATIC_DIR=web/dist PORT=8080 ./clusternet-dashboard
```

Then open:

```text
http://localhost:8080/clusternet/
```

Health endpoint:

```text
http://localhost:8080/clusternet/api/health
```

The server loads Kubernetes config in this order:

1. `KUBECONFIG`, when set
2. in-cluster config
3. `~/.kube/config`, for local development

If Kubernetes config is unavailable, the server still starts and API endpoints return empty MVP responses.

## API endpoints

All endpoints are mounted under `BASE_PATH`.

```text
GET /api/health
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

## Helm install

Build and push an image first, then install the chart with your image reference:

```bash
helm upgrade --install clusternet-dashboard ./charts/clusternet-dashboard \
  -n clusternet-system \
  --create-namespace \
  --set image.repository=ghcr.io/example/clusternet-dashboard \
  --set image.tag=v0.1.0 \
  --set ingress.enabled=true \
  --set ingress.ingressClassName=nginx \
  --set ingress.path=/clusternet
```

Minimal values example:

```yaml
image:
  repository: ghcr.io/example/clusternet-dashboard
  tag: v0.1.0
  pullPolicy: IfNotPresent

ingress:
  enabled: true
  ingressClassName: nginx
  path: /clusternet
```

## Operational notes

- HelmRelease listing can be slower than Subscription listing on large clusters. The frontend loads HelmReleases separately so the main Subscriptions page is not blocked.
- Older Clusternet versions may not expose `status.completedReleases`. In that case the UI labels fallback progress as observed status only.
- The dashboard is an optional component and can be upgraded independently from Clusternet core.

## Verification before release

```bash
go test ./...
npm test --prefix web
npm run build --prefix web
helm lint charts/clusternet-dashboard
go build ./cmd/clusternet-dashboard
```

## Acknowledgements

This project was designed and implemented with the assistance of [Hermes Agent](https://github.com/NousResearch/hermes-agent), using the `gpt-5.5` model. The initial codebase was generated through an iterative discussion about Clusternet dashboard requirements, product scope, operational workflows, and implementation tradeoffs.

## License

This project is licensed under the Apache License 2.0. See [LICENSE](LICENSE).
