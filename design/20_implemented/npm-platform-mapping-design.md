# NPM Platform Mapping for Dagu

## Comprehensive Platform Support Matrix

### Platform Mapping Table

| NPM Package Name | Node.js Platform | Node.js Arch | Go OS | Go Arch | Go ARM | Archive Name | Priority |
|-----------------|------------------|--------------|-------|---------|---------|--------------|----------|
| @dagu-org/dagu-linux-x64 | linux | x64 | linux | amd64 | - | dagu_{version}_Linux_x86_64.tar.gz | Tier 1 |
| @dagu-org/dagu-linux-arm64 | linux | arm64 | linux | arm64 | - | dagu_{version}_Linux_arm64.tar.gz | Tier 1 |
| @dagu-org/dagu-darwin-x64 | darwin | x64 | darwin | amd64 | - | dagu_{version}_Darwin_x86_64.tar.gz | Tier 1 |
| @dagu-org/dagu-darwin-arm64 | darwin | arm64 | darwin | arm64 | - | dagu_{version}_Darwin_arm64.tar.gz | Tier 1 |
| @dagu-org/dagu-win32-x64 | win32 | x64 | windows | amd64 | - | dagu_{version}_Windows_x86_64.zip | Tier 1 |
| @dagu-org/dagu-linux-ia32 | linux | ia32 | linux | 386 | - | dagu_{version}_Linux_i386.tar.gz | Tier 2 |
| @dagu-org/dagu-linux-armv7 | linux | arm | linux | arm | 7 | dagu_{version}_Linux_armv7.tar.gz | Tier 2 |
| @dagu-org/dagu-linux-armv6 | linux | arm | linux | arm | 6 | dagu_{version}_Linux_armv6.tar.gz | Tier 3 |
| @dagu-org/dagu-win32-ia32 | win32 | ia32 | windows | 386 | - | dagu_{version}_Windows_i386.zip | Tier 2 |
| @dagu-org/dagu-win32-arm64 | win32 | arm64 | windows | arm64 | - | dagu_{version}_Windows_arm64.zip | Tier 3 |
| @dagu-org/dagu-linux-ppc64 | linux | ppc64 | linux | ppc64le | - | dagu_{version}_Linux_ppc64le.tar.gz | Tier 3 |
| @dagu-org/dagu-linux-s390x | linux | s390x | linux | s390x | - | dagu_{version}_Linux_s390x.tar.gz | Tier 3 |
| @dagu-org/dagu-freebsd-x64 | freebsd | x64 | freebsd | amd64 | - | dagu_{version}_FreeBSD_x86_64.tar.gz | Tier 2 |
| @dagu-org/dagu-freebsd-arm64 | freebsd | arm64 | freebsd | arm64 | - | dagu_{version}_FreeBSD_arm64.tar.gz | Tier 3 |
| @dagu-org/dagu-freebsd-ia32 | freebsd | ia32 | freebsd | 386 | - | dagu_{version}_FreeBSD_i386.tar.gz | Tier 3 |
| @dagu-org/dagu-freebsd-arm | freebsd | arm | freebsd | arm | - | dagu_{version}_FreeBSD_armv7.tar.gz | Tier 3 |
| @dagu-org/dagu-openbsd-x64 | openbsd | x64 | openbsd | amd64 | - | dagu_{version}_OpenBSD_x86_64.tar.gz | Tier 3 |
| @dagu-org/dagu-openbsd-arm64 | openbsd | arm64 | openbsd | arm64 | - | dagu_{version}_OpenBSD_arm64.tar.gz | Tier 3 |

## Platform Detection Logic

```javascript
// Platform detection with all edge cases handled
const PLATFORM_MAP = {
  // Primary mappings
  'linux-x64': '@dagu-org/dagu-linux-x64',
  'linux-arm64': '@dagu-org/dagu-linux-arm64',
  'darwin-x64': '@dagu-org/dagu-darwin-x64',
  'darwin-arm64': '@dagu-org/dagu-darwin-arm64',
  'win32-x64': '@dagu-org/dagu-win32-x64',
  
  // Secondary mappings
  'linux-ia32': '@dagu-org/dagu-linux-ia32',
  'win32-ia32': '@dagu-org/dagu-win32-ia32',
  'win32-arm64': '@dagu-org/dagu-win32-arm64',
  'linux-ppc64': '@dagu-org/dagu-linux-ppc64',
  'linux-s390x': '@dagu-org/dagu-linux-s390x',
  
  // BSD mappings
  'freebsd-x64': '@dagu-org/dagu-freebsd-x64',
  'freebsd-arm64': '@dagu-org/dagu-freebsd-arm64',
  'freebsd-ia32': '@dagu-org/dagu-freebsd-ia32',
  'freebsd-arm': '@dagu-org/dagu-freebsd-arm',
  'openbsd-x64': '@dagu-org/dagu-openbsd-x64',
  'openbsd-arm64': '@dagu-org/dagu-openbsd-arm64',
};

// ARM variant detection for Linux
function getArmVariant() {
  // Check process.config.variables.arm_version
  if (process.config && process.config.variables && process.config.variables.arm_version) {
    return process.config.variables.arm_version;
  }
  
  // Fallback: check /proc/cpuinfo on Linux
  if (process.platform === 'linux') {
    try {
      const cpuinfo = fs.readFileSync('/proc/cpuinfo', 'utf8');
      if (cpuinfo.includes('ARMv6')) return '6';
      if (cpuinfo.includes('ARMv7')) return '7';
    } catch (e) {
      // Ignore errors, default to v7
    }
  }
  
  return '7'; // Default to ARMv7
}

function getPlatformPackage() {
  let platform = process.platform;
  let arch = process.arch;
  
  // Special handling for ARM on Linux
  if (platform === 'linux' && arch === 'arm') {
    const variant = getArmVariant();
    return `@dagu-org/dagu-linux-armv${variant}`;
  }
  
  const key = `${platform}-${arch}`;
  return PLATFORM_MAP[key] || null;
}
```

## Archive Naming Convention

GoReleaser uses the following pattern for archive names:
- Format: `{project}_{version}_{Os}_{Arch}{Arm}.{ext}`
- OS name capitalization: First letter uppercase
- Architecture mapping:
  - amd64 → x86_64
  - 386 → i386
  - arm → armv6/armv7 (depending on variant)
- Extensions:
  - .tar.gz for Unix-like systems
  - .zip for Windows

## Download URL Construction

```javascript
function getDownloadInfo(platform, arch, version) {
  const osMap = {
    'linux': 'Linux',
    'darwin': 'Darwin',
    'win32': 'Windows',
    'windows': 'Windows',
    'freebsd': 'FreeBSD',
    'openbsd': 'OpenBSD',
    'netbsd': 'NetBSD'
  };
  
  const archMap = {
    'x64': 'x86_64',
    'amd64': 'x86_64',
    'ia32': 'i386',
    '386': 'i386',
    'arm64': 'arm64',
    'ppc64': 'ppc64le',
    's390x': 's390x'
  };
  
  let archName = archMap[arch] || arch;
  
  // Special handling for ARM variants
  if (arch === 'arm' && platform === 'linux') {
    const variant = getArmVariant();
    archName = `armv${variant}`;
  }
  
  const ext = (platform === 'win32' || platform === 'windows') ? '.zip' : '.tar.gz';
  const osName = osMap[platform] || platform;
  
  return {
    filename: `dagu_${version}_${osName}_${archName}${ext}`,
    url: `https://github.com/dagu-org/dagu/releases/download/v${version}/dagu_${version}_${osName}_${archName}${ext}`
  };
}
```

## Implementation Priority

### Tier 1 - Essential (Must publish)
1. **Linux x64** - Most common server platform
2. **Linux ARM64** - Growing server/cloud platform (AWS Graviton, etc.)
3. **Darwin x64** - Intel Macs
4. **Darwin ARM64** - Apple Silicon Macs
5. **Windows x64** - Standard Windows platform

### Tier 2 - Important (Should publish)
1. **Linux ia32** - Legacy 32-bit Linux systems
2. **Linux ARMv7** - Raspberry Pi and embedded systems
3. **Windows ia32** - 32-bit Windows systems
4. **FreeBSD x64** - Common BSD server platform

### Tier 3 - Nice to have (Could publish)
1. **Linux ARMv6** - Older Raspberry Pi models
2. **Linux ppc64le** - IBM POWER systems
3. **Linux s390x** - IBM Z mainframes
4. **Windows ARM64** - Windows on ARM
5. **FreeBSD variants** - Other architectures
6. **OpenBSD variants** - Security-focused deployments

## Platform Support Notes

### NetBSD
- Built by goreleaser but not directly supported by Node.js
- Users would need to manually download and install

### ARM Detection Challenges
- Node.js doesn't distinguish between ARMv6 and ARMv7
- Need runtime detection using /proc/cpuinfo or process.config
- Default to ARMv7 if detection fails

### BSD Support
- FreeBSD and OpenBSD are supported by Node.js
- Less common but important for certain deployments
- May have limited npm usage

## Package.json Configuration

### Main Package
```json
{
  "name": "@dagu-org/dagu",
  "version": "1.16.4",
  "description": "Modern workflow engine with zero dependencies",
  "keywords": ["workflow", "automation", "orchestration", "dag", "pipeline"],
  "bin": {
    "dagu": "./bin/dagu"
  },
  "scripts": {
    "postinstall": "node install.js"
  },
  "optionalDependencies": {
    "@dagu-org/dagu-linux-x64": "1.16.4",
    "@dagu-org/dagu-linux-arm64": "1.16.4",
    "@dagu-org/dagu-darwin-x64": "1.16.4",
    "@dagu-org/dagu-darwin-arm64": "1.16.4",
    "@dagu-org/dagu-win32-x64": "1.16.4",
    "@dagu-org/dagu-linux-ia32": "1.16.4",
    "@dagu-org/dagu-linux-armv7": "1.16.4",
    "@dagu-org/dagu-win32-ia32": "1.16.4",
    "@dagu-org/dagu-freebsd-x64": "1.16.4"
  }
}
```

### Platform-Specific Package Example
```json
{
  "name": "@dagu-org/dagu-linux-arm64",
  "version": "1.16.4",
  "description": "Dagu binary for Linux ARM64",
  "os": ["linux"],
  "cpu": ["arm64"],
  "bin": {
    "dagu": "./bin/dagu"
  },
  "files": ["bin/dagu"],
  "repository": {
    "type": "git",
    "url": "https://github.com/dagu-org/dagu.git"
  }
}
```

## Summary

This comprehensive platform mapping ensures that Dagu can be installed via npm on all platforms supported by the .goreleaser.yaml configuration. The tiered approach allows for gradual rollout, starting with the most commonly used platforms and expanding based on user demand.