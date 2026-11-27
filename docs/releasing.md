# Releasing Clusternet Dashboard

A version tag starts the GitHub Actions workflow that publishes the dashboard image and OCI Helm chart to Docker Hub. The workflow needs read access to Git contents and the repository secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` with registry push access. [Release workflow](../.github/workflows/release.yaml)

## Prepare the version

Choose a tag unused in Git and Docker Hub, including versions published manually. For example, Git tag `v1.2.3` corresponds to image tag `v1.2.3` and OCI chart version `1.2.3`. Prereleases such as `v1.2.3-rc.1` are supported. **A prerelease also updates the image's `latest` tag.** [Docker Hub tags](https://hub.docker.com/r/changxuan/clusternet-dashboard/tags)

Run these commands in the release branch from the repository root. Enter the chosen tag when prompted.

```sh
printf 'Release tag: '
read -r TAG
VERSION=${TAG#v}
IMAGE=docker.io/changxuan/clusternet-dashboard
CHART=oci://registry-1.docker.io/changxuan/clusternet-dashboard
python3 scripts/release-version.py set "$TAG"
python3 scripts/release-version.py check "$TAG"
```

`set` updates the source chart's `version`, `appVersion` and default image tag, and the two README installation examples. `check` validates those pins without editing files. Without a tag argument it uses `Chart.yaml`'s version. Include the changes in the release PR and run the [verification steps](../CONTRIBUTING.md#verify-your-change).

## Publish

After merging the version change, fetch the release commit. These examples use `origin` for the project remote. If you cloned a fork, use the project's remote name.

```sh
git fetch --tags origin
COMMIT=$(git rev-parse origin/main)
git log -1 --oneline "$COMMIT"
git tag --list "$TAG"
```

Confirm the commit is the intended revision and the tag is unused. Then create and push the tag.

```sh
git tag "$TAG" "$COMMIT" && git push origin "refs/tags/$TAG"
```

The workflow checks the tag against the files in that checkout before authenticating to Docker Hub. It runs the frontend dependency audit, tests and build, Go tests and Helm lint. Buildx publishes `$IMAGE:$TAG` and `$IMAGE:latest` for Linux AMD64 and ARM64, embedding the tag for the dashboard's version display. Helm packages the checked-in chart and publishes `$CHART` at `$VERSION`. If chart publication fails, the image may already be public. [Follow the Release workflow](https://github.com/chxcode/clusternet-dashboard/actions/workflows/release.yaml)

For a manual workflow run, choose the ref containing the release pins and enter the matching tag. The selected ref provides the source revision. Dispatch itself does not create a Git tag.

## Verify the artifacts

Check the image platforms, the chart's `version` and `appVersion`, and the default image repository and tag. Installation and upgrade commands are in the [README](../README.md#install-or-upgrade-with-helm).

```sh
docker buildx imagetools inspect "$IMAGE:$TAG"
helm show chart "$CHART" --version "$VERSION"
helm show values "$CHART" --version "$VERSION"
```

## Optional local image smoke test

Build the current checkout and start the container in one terminal.

```sh
docker build -t clusternet-dashboard:smoke .
docker run --rm -p 127.0.0.1:18080:8080 clusternet-dashboard:smoke
```

In a second terminal, run `curl -fsS http://127.0.0.1:18080/clusternet/api/health`. Stop the container with Ctrl-C after the check.

## Manual publication

To publish without Actions, use a clean checkout of the intended commit, initialize `TAG`, `VERSION`, `IMAGE` and `CHART` as above, and run the verification steps. Use one publisher per version because pushing a matching Git tag also starts Actions.

```sh
python3 scripts/release-version.py check "$TAG"
STAGE=$(mktemp -d)
helm package charts/clusternet-dashboard --destination "$STAGE"
PACKAGE="$STAGE/clusternet-dashboard-${VERSION}.tgz"
helm show values "$PACKAGE"
```

Sign in with an account that can push to the repository. Use a Buildx builder supporting both target platforms.

```sh
docker login
helm registry login registry-1.docker.io
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg DASHBOARD_VERSION="$TAG" \
  -t "$IMAGE:$TAG" -t "$IMAGE:latest" --push . &&
  helm push "$PACKAGE" oci://registry-1.docker.io/changxuan
```

After both pushes, [verify the published artifacts](#verify-the-artifacts).
