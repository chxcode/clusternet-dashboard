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

After the image and chart are published to Docker Hub, users can install the dashboard directly from the OCI chart:

```bash
helm upgrade --install clusternet-dashboard \
  oci://registry-1.docker.io/changxuan/clusternet-dashboard \
  --version 0.1.0 \
  -n clusternet-system \
  --create-namespace \
  --set ingress.enabled=true \
  --set ingress.ingressClassName=nginx \
  --set ingress.path=/clusternet
```

If you install from the local chart source instead:

```bash
helm upgrade --install clusternet-dashboard ./charts/clusternet-dashboard \
  -n clusternet-system \
  --create-namespace \
  --set image.repository=changxuan/clusternet-dashboard \
  --set image.tag=v0.1.0 \
  --set ingress.enabled=true \
  --set ingress.ingressClassName=nginx \
  --set ingress.path=/clusternet
```

Minimal values example:

```yaml
image:
  repository: changxuan/clusternet-dashboard
  tag: v0.1.0
  pullPolicy: IfNotPresent

ingress:
  enabled: true
  ingressClassName: nginx
  path: /clusternet
```

## Publish release artifacts

Build and verify the image locally:

```bash
docker build \
  -t changxuan/clusternet-dashboard:v0.1.0 \
  -t changxuan/clusternet-dashboard:latest \
  .

docker run --rm -p 18080:8080 changxuan/clusternet-dashboard:v0.1.0
```

In another terminal, verify:

```bash
curl http://127.0.0.1:18080/clusternet/api/health
```

Push the image to Docker Hub:

```bash
docker login -u changxuan
docker push changxuan/clusternet-dashboard:v0.1.0
docker push changxuan/clusternet-dashboard:latest
```

Package and push the Helm chart as a Docker Hub OCI artifact:

```bash
helm lint charts/clusternet-dashboard
mkdir -p dist
helm package charts/clusternet-dashboard --destination dist

helm registry login registry-1.docker.io -u changxuan
helm push dist/clusternet-dashboard-0.1.0.tgz \
  oci://registry-1.docker.io/changxuan
```

The chart is then installable from:

```text
oci://registry-1.docker.io/changxuan/clusternet-dashboard --version 0.1.0
```

## Automated releases

This repository includes a GitHub Actions release workflow. When a `v*.*.*` tag is pushed, it will:

- run frontend, backend, audit, and Helm verification
- build and push a multi-architecture Docker image for `linux/amd64` and `linux/arm64`
- package the Helm chart
- push the Helm chart to Docker Hub as an OCI artifact

Required GitHub repository secrets:

```text
DOCKERHUB_USERNAME
DOCKERHUB_TOKEN
```

Create a Docker Hub access token, then add both secrets in GitHub repository settings.

To publish a new version:

```bash
git tag v0.1.1
git push origin v0.1.1
```

The workflow can also be run manually from GitHub Actions with a tag input such as `v0.1.1`.

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
