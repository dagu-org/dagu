# NPM Packages for Dagu

This directory contains the npm package structure for distributing Dagu binaries via npm.

## Important Note on Versions

**All versions in package.json files are placeholders!** They use `0.0.0-development` as the version number.

These versions are automatically updated during the release process:

1. When a GitHub release is created (e.g., v1.16.4)
2. The GitHub Actions workflow (`npm-publish.yaml`) runs
3. It updates all package versions to match the release version
4. Then publishes all packages with the correct version

## Package Structure

```
npm/
├── dagu/                # Main package (users install this)
├── dagu-linux-x64/      # Platform-specific packages
├── dagu-linux-arm64/    # (automatically installed as dependencies)
├── dagu-darwin-x64/
├── dagu-darwin-arm64/
└── ...
```

## Development Versions

The `0.0.0-development` version indicates:
- These packages are not meant to be published manually
- Versions are controlled by the CI/CD pipeline
- Local testing uses these placeholder versions

## How Versioning Works

1. **Source Control**: All package.json files have `0.0.0-development`
2. **Release Trigger**: When tagging a release (e.g., `git tag v1.16.4`)
3. **CI Updates**: The workflow updates all versions to `1.16.4`
4. **NPM Publish**: Packages are published with the release version
5. **User Install**: Users get the properly versioned packages

This ensures version consistency across all packages and prevents accidental publishing with wrong versions.
