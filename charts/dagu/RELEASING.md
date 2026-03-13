# Releasing The Dagu Helm Chart

This document covers publication of the Helm chart in `charts/dagu`.

## One-Time Repository Setup

1. Create a `gh-pages` branch in `dagu-org/dagu`.
2. In GitHub repository settings, enable GitHub Pages from the `gh-pages` branch root.
3. Ensure GitHub Actions can write repository contents with `GITHUB_TOKEN`.

The publish workflow in this repository assumes the `gh-pages` branch already exists. That matches the upstream `helm/chart-releaser-action` prerequisites.

## What Triggers A Chart Release

- `.github/workflows/chart-release.yaml` runs on pushes to `main` that touch `charts/**` or the workflow file itself.
- A new chart release is created only when `charts/dagu/Chart.yaml` contains a chart `version` that has not already been published.
- Published chart releases are created as GitHub Releases named `dagu-<chart-version>`.
- The workflow sets `mark_as_latest: false` so chart releases do not replace the repository's application release as GitHub's "latest release".

## Before Merging A Chart Release

1. Update `charts/dagu/Chart.yaml -> version`.
2. Update `values.yaml -> image.tag` only if you want the chart default image to change from `latest` to a different tag.
3. Ensure the chart CI workflow passes:
   - `helm lint ./charts/dagu`
   - `helm template dagu ./charts/dagu --set persistence.storageClass=nfs-client`
   - `helm package ./charts/dagu`

## After Merge

1. Confirm the `ReleaseHelmCharts` workflow succeeded on `main`.
2. Confirm a GitHub Release named `dagu-<chart-version>` exists and includes the `.tgz` package.
3. Confirm `gh-pages/index.yaml` contains the new chart version.
4. Confirm the published repository works:

```bash
helm repo add dagu https://dagu-org.github.io/dagu
helm repo update
helm search repo dagu
helm pull dagu/dagu --version <chart-version>
```

## Removing A Broken Chart Version

No automation is provided for yanking a published chart version.

If a published version must be removed:

1. Delete the GitHub Release `dagu-<chart-version>` and its chart asset.
2. Remove that version entry from `gh-pages/index.yaml`.
3. Commit the `index.yaml` change to `gh-pages`.
4. Publish a new chart version. Do not reuse the removed version number.
