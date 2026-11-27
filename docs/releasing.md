# Releasing Clusternet Dashboard

Pushing a version tag starts the GitHub Actions workflow that publishes the container image and Helm chart to Docker Hub. A push to `main` alone does not start a release. [Release workflow](../.github/workflows/release.yaml)

## Choose the commit and version

These commands assume that `origin` points to `chxcode/clusternet-dashboard`. If you cloned a fork, substitute the project remote (usually `upstream`) in the fetch and push commands and the remote-tracking branch below. From the repository root, fetch the tags and inspect the published commit you plan to release.

```sh
git fetch --tags origin
COMMIT=$(git rev-parse origin/main)
git log -1 --oneline "$COMMIT"
git tag --list 'v*' --sort=-version:refname
```

Confirm that the displayed commit is the one you intend to release. Choose a version unused in both Git and Docker Hub, including any artifacts published manually. The workflow accepts tags such as `v1.2.3` and `v1.2.3-rc.1`. **A prerelease also updates the image's `latest` tag.** [Docker Hub tags](https://hub.docker.com/r/changxuan/clusternet-dashboard/tags)

```sh
printf 'Enter the new Git tag: '
read -r TAG
VERSION=${TAG#v}
IMAGE=docker.io/changxuan/clusternet-dashboard
CHART=oci://registry-1.docker.io/changxuan/clusternet-dashboard
```

## Publish the release

The workflow uses the GitHub repository secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` to authenticate both Docker and Helm. The account and token need push access to `changxuan/clusternet-dashboard`.

Create the tag on the inspected commit and push that tag.

```sh
git tag "$TAG" "$COMMIT" && git push origin "refs/tags/$TAG"
```

Follow the run in [GitHub Actions](https://github.com/chxcode/clusternet-dashboard/actions/workflows/release.yaml). For a manual run, select the ref containing the exact source revision you want to build and enter the release tag. The selected ref controls checkout. The tag input sets the artifact versions and does not create a Git tag.

| Stage | What the workflow does |
| --- | --- |
| Verify | Installs frontend dependencies with `npm ci --prefix web`, runs `npm audit` at the high threshold, tests and builds the frontend, then runs `go test ./...` and `helm lint`. |
| Publish image | Builds the frontend and Go server into one image with Buildx for `linux/amd64` and `linux/arm64`. Embeds the release tag in the server for the sidebar version. Pushes `$IMAGE:$TAG` and `$IMAGE:latest`. |
| Publish chart | Sets the packaged chart's image tag and app version to `$TAG` and the chart version to `$VERSION` (without the leading `v`). Packages `dist/clusternet-dashboard-${VERSION}.tgz`, pushes it to Docker Hub as an OCI chart, then reads back its metadata with `helm show chart`. |

The version overrides apply in the runner's checkout. If chart publication fails, the image may already be public. Cluster upgrades are a separate operation.

## Verify the published artifacts

Use the variables above to inspect the registry. Check both image platforms, the chart's `version` and `appVersion`, and the image repository and tag in the published values.

```sh
docker buildx imagetools inspect "$IMAGE:$TAG"
helm show chart "$CHART" --version "$VERSION"
helm show values "$CHART" --version "$VERSION"
```

## Optional local image smoke test

Build the current checkout and start the container in a terminal.

```sh
docker build -t clusternet-dashboard:smoke .
docker run --rm -p 127.0.0.1:18080:8080 clusternet-dashboard:smoke
```

In a second terminal, run `curl -fsS http://127.0.0.1:18080/clusternet/api/health`. Stop the container with Ctrl-C in the first terminal when finished.

## Manual publication

Use this path if you need to publish from a local checkout. Start from a clean checkout of the intended commit. Choose an unused version as above, run the checks in the [contributing guide](../CONTRIBUTING.md#verify-your-change), including the npm audit, and smoke-test the image. Pushing the Git tag also starts Actions, so use one publisher per version.

Prepare a copy of the chart for packaging.

```sh
STAGE=$(mktemp -d)
cp -R charts/clusternet-dashboard "$STAGE/"
```

In `$STAGE/clusternet-dashboard/values.yaml`, set `image.tag` to the full release tag (including `v`). Then package the chart and check that its default image matches the image you intend to push.

```sh
helm package "$STAGE/clusternet-dashboard" --destination "$STAGE" \
  --version "$VERSION" --app-version "$TAG"
PACKAGE="$STAGE/clusternet-dashboard-${VERSION}.tgz"
helm show values "$PACKAGE"
```

Sign in with a Docker Hub account that can push to the repository. Use a Buildx builder that supports both target platforms. The following commands publish the same image tags and chart as the automated workflow.

```sh
docker login
helm registry login registry-1.docker.io
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg DASHBOARD_VERSION="$TAG" \
  -t "$IMAGE:$TAG" -t "$IMAGE:latest" --push . &&
  helm push "$PACKAGE" oci://registry-1.docker.io/changxuan
```

After both pushes, [verify the published artifacts](#verify-the-published-artifacts).
