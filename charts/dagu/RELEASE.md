Helm chart release for Dagu.

This GitHub release publishes the packaged `dagu` Helm chart for the Helm repository at `https://dagucloud.github.io/dagu`. It is not a Dagu application binary release.

Install the chart with:

```bash
helm repo add dagu https://dagucloud.github.io/dagu
helm repo update
helm install dagu dagu/dagu --set persistence.storageClass=<your-rwx-storage-class>
```

Application releases use separate `vX.Y.Z` tags.
