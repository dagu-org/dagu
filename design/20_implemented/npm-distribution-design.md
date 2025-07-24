# NPM Distribution Implementation Plan for Dagu

## Overview
This plan outlines the implementation of npm distribution for Dagu, enabling Node.js developers to install Dagu via `npm install @dagu-org/dagu`. The approach follows best practices from Sentry's implementation, using a two-pronged strategy with platform-specific packages and fallback mechanisms.

## Architecture

### Package Structure
```
@dagu-org/dagu (main package)
├── @dagu-org/dagu-linux-x64
├── @dagu-org/dagu-linux-arm64
├── @dagu-org/dagu-darwin-x64
├── @dagu-org/dagu-darwin-arm64
└── @dagu-org/dagu-win32-x64
```

## Implementation Steps

### 1. Main Package Structure (`@dagu-org/dagu`)
```
npm/dagu/
├── package.json
├── index.js
├── install.js
├── bin/
│   └── dagu (wrapper script)
└── lib/
    ├── platform.js
    ├── download.js
    └── constants.js
```

**package.json:**
```json
{
  "name": "@dagu-org/dagu",
  "version": "1.0.0",
  "description": "Modern workflow engine with zero dependencies",
  "bin": {
    "dagu": "./bin/dagu"
  },
  "scripts": {
    "postinstall": "node install.js"
  },
  "optionalDependencies": {
    "@dagu-org/dagu-linux-x64": "1.0.0",
    "@dagu-org/dagu-linux-arm64": "1.0.0",
    "@dagu-org/dagu-darwin-x64": "1.0.0",
    "@dagu-org/dagu-darwin-arm64": "1.0.0",
    "@dagu-org/dagu-win32-x64": "1.0.0"
  }
}
```

### 2. Platform-Specific Packages
Each platform package (e.g., `@dagu-org/dagu-linux-x64`) contains:
```
npm/dagu-linux-x64/
├── package.json
└── bin/
    └── dagu (actual binary)
```

**package.json:**
```json
{
  "name": "@dagu-org/dagu-linux-x64",
  "version": "1.0.0",
  "os": ["linux"],
  "cpu": ["x64"],
  "bin": {
    "dagu": "./bin/dagu"
  }
}
```

### 3. Installation Logic (`install.js`)
```javascript
const { platform, arch } = require('os');
const { download } = require('./lib/download');
const { getBinaryPath, setPlatformBinary } = require('./lib/platform');

async function install() {
  try {
    // Try to resolve platform-specific package
    const binaryPath = getBinaryPath();
    if (binaryPath) {
      console.log('Using pre-installed binary from optional dependency');
      return;
    }
  } catch (e) {
    // Fallback to download
    console.log('Downloading Dagu binary for your platform...');
    await downloadBinary();
  }
}

async function downloadBinary() {
  const platformKey = `${platform()}-${arch()}`;
  const downloadUrl = getDownloadUrl(platformKey);
  
  await download(downloadUrl, './bin/dagu');
  setPlatformBinary('./bin/dagu');
}
```

### 4. Binary Wrapper (`bin/dagu`)
```javascript
#!/usr/bin/env node
const { spawn } = require('child_process');
const { getBinaryPath } = require('../lib/platform');

const binaryPath = getBinaryPath();
if (!binaryPath) {
  console.error('Dagu binary not found. Please reinstall.');
  process.exit(1);
}

const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: 'inherit'
});

child.on('exit', (code) => process.exit(code));
```

### 5. Platform Module (`lib/platform.js`)
```javascript
const os = require('os');
const path = require('path');
const fs = require('fs');

const PLATFORM_MAPPING = {
  'darwin-x64': '@dagu-org/dagu-darwin-x64',
  'darwin-arm64': '@dagu-org/dagu-darwin-arm64',
  'linux-x64': '@dagu-org/dagu-linux-x64',
  'linux-arm64': '@dagu-org/dagu-linux-arm64',
  'win32-x64': '@dagu-org/dagu-win32-x64',
};

function getPlatformPackage() {
  const platform = os.platform();
  const arch = os.arch();
  const key = `${platform}-${arch}`;
  
  return PLATFORM_MAPPING[key];
}

function getBinaryPath() {
  // First, try platform-specific package
  const platformPackage = getPlatformPackage();
  if (platformPackage) {
    try {
      const binaryName = process.platform === 'win32' ? 'dagu.exe' : 'dagu';
      return require.resolve(`${platformPackage}/bin/${binaryName}`);
    } catch (e) {
      // Package not installed
    }
  }
  
  // Fallback to local binary
  const localBinary = path.join(__dirname, '..', 'bin', 
    process.platform === 'win32' ? 'dagu.exe' : 'dagu');
  
  if (fs.existsSync(localBinary)) {
    return localBinary;
  }
  
  return null;
}

module.exports = { getPlatformPackage, getBinaryPath };
```

### 6. Download Module (`lib/download.js`)
```javascript
const https = require('https');
const fs = require('fs');
const path = require('path');
const { pipeline } = require('stream/promises');
const tar = require('tar');

const GITHUB_RELEASES_URL = 'https://github.com/dagu-org/dagu/releases/download';

function getDownloadUrl(version, platform, arch) {
  const assetName = getAssetName(platform, arch);
  return `${GITHUB_RELEASES_URL}/v${version}/${assetName}`;
}

function getAssetName(platform, arch) {
  const platformMap = {
    'darwin': 'Darwin',
    'linux': 'Linux',
    'win32': 'Windows'
  };
  
  const archMap = {
    'x64': 'x86_64',
    'arm64': 'arm64'
  };
  
  const ext = platform === 'win32' ? '.zip' : '.tar.gz';
  return `dagu_${version}_${platformMap[platform]}_${archMap[arch]}${ext}`;
}

async function download(url, destination) {
  const tempFile = `${destination}.download`;
  
  await new Promise((resolve, reject) => {
    https.get(url, (response) => {
      if (response.statusCode === 302) {
        // Follow redirect
        return download(response.headers.location, destination)
          .then(resolve)
          .catch(reject);
      }
      
      if (response.statusCode !== 200) {
        reject(new Error(`Failed to download: ${response.statusCode}`));
        return;
      }
      
      const file = fs.createWriteStream(tempFile);
      response.pipe(file);
      
      file.on('finish', () => {
        file.close(resolve);
      });
    }).on('error', reject);
  });
  
  // Extract binary
  if (url.endsWith('.tar.gz')) {
    await tar.extract({
      file: tempFile,
      cwd: path.dirname(destination),
      filter: (path) => path.endsWith('dagu')
    });
  } else {
    // Handle .zip for Windows
    // Implementation depends on chosen zip library
  }
  
  // Clean up
  fs.unlinkSync(tempFile);
  
  // Make executable
  if (process.platform !== 'win32') {
    fs.chmodSync(destination, 0o755);
  }
}

module.exports = { download, getDownloadUrl };
```

### 7. GitHub Actions Workflow
```yaml
name: Publish to NPM
on:
  release:
    types: [created]

jobs:
  publish-platform-packages:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - platform: linux
            arch: x64
            package: dagu-linux-x64
            asset: dagu_${{ github.event.release.tag_name }}_Linux_x86_64.tar.gz
          - platform: linux
            arch: arm64
            package: dagu-linux-arm64
            asset: dagu_${{ github.event.release.tag_name }}_Linux_arm64.tar.gz
          - platform: darwin
            arch: x64
            package: dagu-darwin-x64
            asset: dagu_${{ github.event.release.tag_name }}_Darwin_x86_64.tar.gz
          - platform: darwin
            arch: arm64
            package: dagu-darwin-arm64
            asset: dagu_${{ github.event.release.tag_name }}_Darwin_arm64.tar.gz
          - platform: win32
            arch: x64
            package: dagu-win32-x64
            asset: dagu_${{ github.event.release.tag_name }}_Windows_x86_64.zip
    
    steps:
      - uses: actions/checkout@v3
      
      - uses: actions/setup-node@v3
        with:
          node-version: '18'
          registry-url: 'https://registry.npmjs.org'
      
      - name: Download release asset
        uses: dsaltares/fetch-gh-release-asset@master
        with:
          repo: 'dagu-org/dagu'
          version: ${{ github.event.release.tag_name }}
          file: ${{ matrix.asset }}
          token: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Extract binary
        run: |
          mkdir -p npm/${{ matrix.package }}/bin
          if [[ "${{ matrix.asset }}" == *.tar.gz ]]; then
            tar -xzf ${{ matrix.asset }} -C npm/${{ matrix.package }}/bin dagu
          else
            unzip ${{ matrix.asset }} -d npm/${{ matrix.package }}/bin dagu.exe
          fi
          
      - name: Update package version
        run: |
          cd npm/${{ matrix.package }}
          npm version ${{ github.event.release.tag_name }} --no-git-tag-version
          
      - name: Publish to NPM
        run: |
          cd npm/${{ matrix.package }}
          npm publish --access public
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
          
  publish-main-package:
    needs: publish-platform-packages
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - uses: actions/setup-node@v3
        with:
          node-version: '18'
          registry-url: 'https://registry.npmjs.org'
          
      - name: Update main package version
        run: |
          cd npm/dagu
          npm version ${{ github.event.release.tag_name }} --no-git-tag-version
          
          # Update optionalDependencies versions
          node -e "
            const pkg = require('./package.json');
            Object.keys(pkg.optionalDependencies).forEach(dep => {
              pkg.optionalDependencies[dep] = '${{ github.event.release.tag_name }}';
            });
            require('fs').writeFileSync('./package.json', JSON.stringify(pkg, null, 2));
          "
          
      - name: Publish main package
        run: |
          cd npm/dagu
          npm publish --access public
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
```

## Key Implementation Details

### Platform Detection
- Use `process.platform` and `process.arch` to detect current environment
- Map to standardized platform keys (e.g., `darwin-arm64`)
- Support for: linux-x64, linux-arm64, darwin-x64, darwin-arm64, win32-x64

### Binary Management
- Store binaries in `bin/` directory of platform-specific packages
- Set executable permissions (chmod +x) during installation
- Handle Windows .exe extension appropriately

### Fallback Strategy
1. First try: Install via `optionalDependencies`
2. Fallback: Download from GitHub releases in `postinstall`
3. Cache downloaded binaries to avoid re-downloading

### Version Synchronization
- Automate version updates across all packages
- Tie npm package versions to Dagu binary versions
- Use GitHub Actions to publish on new releases

## Benefits
- **Familiar Installation**: Node.js developers can use standard npm commands
- **Version Management**: Pin specific versions in package.json
- **CI/CD Integration**: Easy to include in Node.js pipelines
- **Cross-Platform**: Automatic platform detection and binary selection
- **Offline Support**: Works with npm cache and private registries

## Testing Strategy
1. Test installation on all supported platforms
2. Verify binary execution and PATH setup
3. Test version pinning and updates
4. Validate fallback download mechanism
5. Test in CI environments (GitHub Actions, CircleCI, etc.)

## Migration Path
- Maintain existing installation methods (curl, homebrew)
- Add npm as an additional option
- Document in README and installation guide
- Announce in release notes

## Directory Structure
```
dagu/
├── npm/
│   ├── dagu/
│   │   ├── package.json
│   │   ├── index.js
│   │   ├── install.js
│   │   ├── bin/
│   │   │   └── dagu
│   │   └── lib/
│   │       ├── platform.js
│   │       ├── download.js
│   │       └── constants.js
│   ├── dagu-linux-x64/
│   │   ├── package.json
│   │   └── bin/
│   ├── dagu-linux-arm64/
│   │   ├── package.json
│   │   └── bin/
│   ├── dagu-darwin-x64/
│   │   ├── package.json
│   │   └── bin/
│   ├── dagu-darwin-arm64/
│   │   ├── package.json
│   │   └── bin/
│   └── dagu-win32-x64/
│       ├── package.json
│       └── bin/
└── .github/
    └── workflows/
        └── npm-publish.yml
```

## Next Steps
1. Create the npm-packages directory structure
2. Implement the JavaScript modules (platform.js, download.js, install.js)
3. Create package.json files for all packages
4. Set up NPM organization (@dagu-org)
5. Configure NPM_TOKEN secret in GitHub repository
6. Test locally with npm link
7. Create initial release to test the workflow

This implementation provides a robust, user-friendly npm distribution for Dagu while maintaining the simplicity of a single binary distribution model.