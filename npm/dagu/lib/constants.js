/**
 * Constants for Dagu npm distribution
 */

const GITHUB_ORG = 'dagu-org';
const GITHUB_REPO = 'dagu';
const NPM_ORG = '@dagu-org';

// Tier classification for platform support
const PLATFORM_TIERS = {
  TIER_1: [
    'linux-x64',
    'linux-arm64',
    'darwin-x64',
    'darwin-arm64',
    'win32-x64'
  ],
  TIER_2: [
    'linux-ia32',
    'linux-armv7',
    'win32-ia32',
    'freebsd-x64'
  ],
  TIER_3: [
    'linux-armv6',
    'linux-ppc64',
    'linux-s390x',
    'win32-arm64',
    'freebsd-arm64',
    'freebsd-ia32',
    'freebsd-arm',
    'openbsd-x64',
    'openbsd-arm64'
  ]
};

// Error messages
const ERRORS = {
  UNSUPPORTED_PLATFORM: 'Unsupported platform',
  DOWNLOAD_FAILED: 'Failed to download binary',
  VALIDATION_FAILED: 'Binary validation failed',
  CHECKSUM_MISMATCH: 'Checksum verification failed',
  EXTRACTION_FAILED: 'Failed to extract archive'
};

// URLs
const URLS = {
  RELEASES: `https://github.com/${GITHUB_ORG}/${GITHUB_REPO}/releases`,
  ISSUES: `https://github.com/${GITHUB_ORG}/${GITHUB_REPO}/issues`,
  BUILD_DOCS: `https://github.com/${GITHUB_ORG}/${GITHUB_REPO}#building-from-source`
};

module.exports = {
  GITHUB_ORG,
  GITHUB_REPO,
  NPM_ORG,
  PLATFORM_TIERS,
  ERRORS,
  URLS
};