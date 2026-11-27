# Contributing

## Start a change

Fork [chxcode/clusternet-dashboard](https://github.com/chxcode/clusternet-dashboard) on GitHub. Replace `YOUR-USERNAME` with your GitHub username and choose a branch name for your change.

```sh
git clone https://github.com/YOUR-USERNAME/clusternet-dashboard.git
cd clusternet-dashboard
git remote add upstream https://github.com/chxcode/clusternet-dashboard.git
git fetch upstream main
git switch -c my-change upstream/main
```

| Area | Where to work |
| --- | --- |
| Go service | [cmd/clusternet-dashboard](cmd/clusternet-dashboard) starts the server. [internal/config](internal/config) reads runtime settings, [internal/server](internal/server) serves HTTP and static assets, and [internal/kube](internal/kube) reads cluster resources. |
| Web UI | [web/src](web/src) contains the React components, API client, and their tests. |
| Packaging | [Dockerfile](Dockerfile) builds the combined image. [charts/clusternet-dashboard](charts/clusternet-dashboard) contains Helm values, workload templates, and RBAC. |

## Run the dashboard locally

Install Go 1.26 or later and Node.js 22.x (22.13 or newer) with npm. Run these commands from the repository root.

```sh
npm ci --prefix web
npm run build --prefix web
go run ./cmd/clusternet-dashboard
```

Open [the local dashboard](http://localhost:8080/clusternet/). The Go server serves the built UI from `web/dist` (`STATIC_DIR`) on port `8080` (`PORT`). Rebuild the frontend and refresh the page after editing it. In another terminal, check that the server responds with `{"status":"ok"}`.

```sh
curl -fsS http://localhost:8080/clusternet/api/health
```

For live data, set `KUBECONFIG` to a kubeconfig file for a Clusternet parent cluster before starting the server. If unset, the backend tries in-cluster credentials and then `~/.kube/config`. If it cannot create a Kubernetes client, the UI and health endpoint still work, and resource lists are empty. With a configured but unreachable cluster, resource requests can return errors.

## Verify your change

Run the checks for each area you changed. Go and frontend tests run without a live cluster.

- **Backend.** Format edited Go files with `gofmt`. Run `go test ./...` and `go build ./...` from the repository root.
- **Frontend.** Run `npm test --prefix web` and `npm run build --prefix web`. For dependency changes, also run `npm audit --prefix web --audit-level=high`.
- **Helm chart.** With Helm installed, lint the chart and inspect the rendered resources. Enable Ingress in the render to exercise its template.

  ```sh
  helm lint charts/clusternet-dashboard
  helm template clusternet-dashboard charts/clusternet-dashboard \
    --set ingress.enabled=true --set ingress.ingressClassName=nginx
  ```

## Open a pull request

1. Add a test for changed behavior where practical. Preserve read-only cluster access and keep Kubernetes credentials on the backend. Update user-facing documentation and chart values when they change.
2. Review `git status --short` and `git diff` before staging the intended files. Exclude local binaries, frontend build output, and incidental `web/tsconfig.tsbuildinfo` changes. Keep examples free of credentials, real cluster names, and private registry references.
3. Commit the change and run `git push -u origin HEAD`. Open a pull request against `chxcode/clusternet-dashboard:main`. Describe the change, how you verified it, and any checks you could not run. For a visible UI change, a screenshot of the affected view helps reviewers.

Maintainers handle version tags and publication of release artifacts. See the [release guide](docs/releasing.md).

Report suspected vulnerabilities privately using the [security policy](SECURITY.md).
