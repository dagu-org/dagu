const os = require('os');
const path = require('path');
const fs = require('fs');

// Platform mapping from Node.js to npm package names
const PLATFORM_MAP = {
  // Tier 1 - Most common platforms
  'linux-x64': '@dagu-org/dagu-linux-x64',
  'linux-arm64': '@dagu-org/dagu-linux-arm64',
  'darwin-x64': '@dagu-org/dagu-darwin-x64',
  'darwin-arm64': '@dagu-org/dagu-darwin-arm64',
  'win32-x64': '@dagu-org/dagu-win32-x64',
  
  // Tier 2 - Common but less frequent
  'linux-ia32': '@dagu-org/dagu-linux-ia32',
  'win32-ia32': '@dagu-org/dagu-win32-ia32',
  'freebsd-x64': '@dagu-org/dagu-freebsd-x64',
  
  // Tier 3 - Rare platforms
  'win32-arm64': '@dagu-org/dagu-win32-arm64',
  'linux-ppc64': '@dagu-org/dagu-linux-ppc64',
  'linux-s390x': '@dagu-org/dagu-linux-s390x',
  'freebsd-arm64': '@dagu-org/dagu-freebsd-arm64',
  'freebsd-ia32': '@dagu-org/dagu-freebsd-ia32',
  'freebsd-arm': '@dagu-org/dagu-freebsd-arm',
  'openbsd-x64': '@dagu-org/dagu-openbsd-x64',
  'openbsd-arm64': '@dagu-org/dagu-openbsd-arm64',
};

// Cache for binary path
let cachedBinaryPath = null;

/**
 * Detect ARM variant on Linux systems
 * @returns {string} ARM variant ('6' or '7')
 */
function getArmVariant() {
  // First try process.config
  if (process.config && process.config.variables && process.config.variables.arm_version) {
    return String(process.config.variables.arm_version);
  }
  
  // On Linux, check /proc/cpuinfo
  if (process.platform === 'linux') {
    try {
      const cpuinfo = fs.readFileSync('/proc/cpuinfo', 'utf8');
      
      // Check for specific ARM architecture indicators
      if (cpuinfo.includes('ARMv6') || cpuinfo.includes('ARM926') || cpuinfo.includes('ARM1176')) {
        return '6';
      }
      if (cpuinfo.includes('ARMv7') || cpuinfo.includes('Cortex-A')) {
        return '7';
      }
      
      // Check CPU architecture field
      const archMatch = cpuinfo.match(/^CPU architecture:\s*(\d+)/m);
      if (archMatch && archMatch[1]) {
        const arch = parseInt(archMatch[1], 10);
        if (arch >= 7) return '7';
        if (arch === 6) return '6';
      }
    } catch (e) {
      // Ignore errors, fall through to default
    }
  }
  
  // Default to ARMv7 (more common)
  return '7';
}

/**
 * Get the npm package name for the current platform
 * @returns {string|null} Package name or null if unsupported
 */
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

/**
 * Get supported platforms list for error messages
 * @returns {string} Formatted list of supported platforms
 */
function getSupportedPlatforms() {
  const platforms = [
    'Linux: x64, arm64, arm (v6/v7), ia32, ppc64le, s390x',
    'macOS: x64 (Intel), arm64 (Apple Silicon)',
    'Windows: x64, ia32, arm64',
    'FreeBSD: x64, arm64, ia32, arm',
    'OpenBSD: x64, arm64'
  ];
  return platforms.join('\n  - ');
}

/**
 * Get the path to the Dagu binary
 * @returns {string|null} Path to binary or null if not found
 */
function getBinaryPath() {
  // Return cached path if available
  if (cachedBinaryPath && fs.existsSync(cachedBinaryPath)) {
    return cachedBinaryPath;
  }
  
  const binaryName = process.platform === 'win32' ? 'dagu.exe' : 'dagu';
  
  // First, try platform-specific package
  const platformPackage = getPlatformPackage();
  if (platformPackage) {
    try {
      // Try to resolve the binary using require.resolve (Sentry approach)
      const binaryPath = require.resolve(`${platformPackage}/bin/${binaryName}`);
      if (fs.existsSync(binaryPath)) {
        cachedBinaryPath = binaryPath;
        return binaryPath;
      }
    } catch (e) {
      // Package not installed or binary not found
    }
  }
  
  // Fallback to local binary in main package
  const localBinary = path.join(__dirname, '..', 'bin', binaryName);
  if (fs.existsSync(localBinary)) {
    cachedBinaryPath = localBinary;
    return localBinary;
  }
  
  return null;
}

/**
 * Set the cached binary path
 * @param {string} binaryPath Path to the binary
 */
function setPlatformBinary(binaryPath) {
  cachedBinaryPath = binaryPath;
}

/**
 * Get platform details for debugging
 * @returns {object} Platform information
 */
function getPlatformInfo() {
  return {
    platform: process.platform,
    arch: process.arch,
    nodeVersion: process.version,
    v8Version: process.versions.v8,
    systemPlatform: os.platform(),
    systemArch: os.arch(),
    systemRelease: os.release(),
    armVariant: process.platform === 'linux' && process.arch === 'arm' ? getArmVariant() : null,
    detectedPackage: getPlatformPackage()
  };
}

/**
 * Check if platform-specific package is installed
 * @returns {boolean} True if installed, false otherwise
 */
function isPlatformSpecificPackageInstalled() {
  const platformPackage = getPlatformPackage();
  if (!platformPackage) {
    return false;
  }
  
  const binaryName = process.platform === 'win32' ? 'dagu.exe' : 'dagu';
  
  try {
    // Resolving will fail if the optionalDependency was not installed
    require.resolve(`${platformPackage}/bin/${binaryName}`);
    return true;
  } catch (e) {
    return false;
  }
}

/**
 * Check if we're in a cross-platform scenario (node_modules moved between architectures)
 * @returns {object|null} Warning info if cross-platform detected
 */
function checkCrossPlatformScenario() {
  const pkg = require('../package.json');
  const optionalDeps = Object.keys(pkg.optionalDependencies || {});
  const currentPlatformPackage = getPlatformPackage();
  
  // Check if any platform package is installed but it's not the right one
  for (const dep of optionalDeps) {
    try {
      require.resolve(`${dep}/package.json`);
      // Package is installed
      if (dep !== currentPlatformPackage) {
        // Wrong platform package is installed
        const installedPlatform = dep.replace('@dagu-org/dagu-', '');
        const currentPlatform = `${process.platform}-${process.arch}`;
        return {
          installed: installedPlatform,
          current: currentPlatform,
          message: `WARNING: Found binary for ${installedPlatform} but current platform is ${currentPlatform}.\nThis usually happens when node_modules are copied between different systems.\nPlease reinstall @dagu-org/dagu to get the correct binary.`
        };
      }
    } catch (e) {
      // Package not installed, continue checking
    }
  }
  
  return null;
}

module.exports = {
  getPlatformPackage,
  getBinaryPath,
  setPlatformBinary,
  getSupportedPlatforms,
  getPlatformInfo,
  getArmVariant,
  isPlatformSpecificPackageInstalled,
  checkCrossPlatformScenario
};