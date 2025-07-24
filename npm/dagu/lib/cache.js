const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const os = require('os');

/**
 * Get cache directory for binaries
 * @returns {string} Cache directory path
 */
function getCacheDir() {
  // Use standard cache locations based on platform
  const homeDir = os.homedir();
  let cacheDir;
  
  if (process.platform === 'win32') {
    // Windows: %LOCALAPPDATA%\dagu-cache
    cacheDir = path.join(process.env.LOCALAPPDATA || path.join(homeDir, 'AppData', 'Local'), 'dagu-cache');
  } else if (process.platform === 'darwin') {
    // macOS: ~/Library/Caches/dagu
    cacheDir = path.join(homeDir, 'Library', 'Caches', 'dagu');
  } else {
    // Linux/BSD: ~/.cache/dagu
    cacheDir = path.join(process.env.XDG_CACHE_HOME || path.join(homeDir, '.cache'), 'dagu');
  }
  
  // Allow override via environment variable
  if (process.env.DAGU_CACHE_DIR) {
    cacheDir = process.env.DAGU_CACHE_DIR;
  }
  
  return cacheDir;
}

/**
 * Get cached binary path
 * @param {string} version Version of the binary
 * @param {string} platform Platform identifier
 * @returns {string} Path to cached binary
 */
function getCachedBinaryPath(version, platform) {
  const cacheDir = getCacheDir();
  const binaryName = process.platform === 'win32' ? 'dagu.exe' : 'dagu';
  return path.join(cacheDir, `${version}-${platform}`, binaryName);
}

/**
 * Check if binary exists in cache
 * @param {string} version Version of the binary
 * @param {string} platform Platform identifier
 * @returns {boolean} True if cached, false otherwise
 */
function isCached(version, platform) {
  const cachedPath = getCachedBinaryPath(version, platform);
  return fs.existsSync(cachedPath);
}

/**
 * Save binary to cache
 * @param {string} sourcePath Path to the binary to cache
 * @param {string} version Version of the binary
 * @param {string} platform Platform identifier
 * @returns {string} Path to cached binary
 */
function cacheBinary(sourcePath, version, platform) {
  const cachedPath = getCachedBinaryPath(version, platform);
  const cacheDir = path.dirname(cachedPath);
  
  // Create cache directory if it doesn't exist
  if (!fs.existsSync(cacheDir)) {
    fs.mkdirSync(cacheDir, { recursive: true });
  }
  
  // Copy binary to cache
  fs.copyFileSync(sourcePath, cachedPath);
  
  // Preserve executable permissions
  if (process.platform !== 'win32') {
    fs.chmodSync(cachedPath, 0o755);
  }
  
  // Create a metadata file with cache info
  const metadataPath = path.join(cacheDir, 'metadata.json');
  const metadata = {
    version,
    platform,
    cachedAt: new Date().toISOString(),
    checksum: calculateChecksum(cachedPath)
  };
  fs.writeFileSync(metadataPath, JSON.stringify(metadata, null, 2));
  
  return cachedPath;
}

/**
 * Get binary from cache
 * @param {string} version Version of the binary
 * @param {string} platform Platform identifier
 * @returns {string|null} Path to cached binary or null if not found
 */
function getCachedBinary(version, platform) {
  if (!isCached(version, platform)) {
    return null;
  }
  
  const cachedPath = getCachedBinaryPath(version, platform);
  
  // Verify the cached binary still works
  try {
    fs.accessSync(cachedPath, fs.constants.X_OK);
    return cachedPath;
  } catch (e) {
    // Cached binary is corrupted or not executable
    // Remove it from cache
    cleanCacheEntry(version, platform);
    return null;
  }
}

/**
 * Calculate checksum of a file
 * @param {string} filePath Path to the file
 * @returns {string} SHA256 checksum
 */
function calculateChecksum(filePath) {
  const hash = crypto.createHash('sha256');
  const data = fs.readFileSync(filePath);
  hash.update(data);
  return hash.digest('hex');
}

/**
 * Clean specific cache entry
 * @param {string} version Version of the binary
 * @param {string} platform Platform identifier
 */
function cleanCacheEntry(version, platform) {
  const cacheDir = path.dirname(getCachedBinaryPath(version, platform));
  
  if (fs.existsSync(cacheDir)) {
    fs.rmSync(cacheDir, { recursive: true, force: true });
  }
}

/**
 * Clean old cache entries (older than specified days)
 * @param {number} maxAgeDays Maximum age in days (default 30)
 */
function cleanOldCache(maxAgeDays = 30) {
  const cacheDir = getCacheDir();
  
  if (!fs.existsSync(cacheDir)) {
    return;
  }
  
  const maxAgeMs = maxAgeDays * 24 * 60 * 60 * 1000;
  const now = Date.now();
  
  try {
    const entries = fs.readdirSync(cacheDir);
    
    for (const entry of entries) {
      const entryPath = path.join(cacheDir, entry);
      const metadataPath = path.join(entryPath, 'metadata.json');
      
      if (fs.existsSync(metadataPath)) {
        try {
          const metadata = JSON.parse(fs.readFileSync(metadataPath, 'utf8'));
          const cachedAt = new Date(metadata.cachedAt).getTime();
          
          if (now - cachedAt > maxAgeMs) {
            fs.rmSync(entryPath, { recursive: true, force: true });
          }
        } catch (e) {
          // Invalid metadata, remove entry
          fs.rmSync(entryPath, { recursive: true, force: true });
        }
      }
    }
  } catch (e) {
    // Ignore errors during cleanup
  }
}

/**
 * Get cache size
 * @returns {number} Total size in bytes
 */
function getCacheSize() {
  const cacheDir = getCacheDir();
  
  if (!fs.existsSync(cacheDir)) {
    return 0;
  }
  
  let totalSize = 0;
  
  function calculateDirSize(dirPath) {
    const entries = fs.readdirSync(dirPath, { withFileTypes: true });
    
    for (const entry of entries) {
      const fullPath = path.join(dirPath, entry.name);
      
      if (entry.isDirectory()) {
        calculateDirSize(fullPath);
      } else {
        const stats = fs.statSync(fullPath);
        totalSize += stats.size;
      }
    }
  }
  
  try {
    calculateDirSize(cacheDir);
  } catch (e) {
    // Ignore errors
  }
  
  return totalSize;
}

module.exports = {
  getCacheDir,
  getCachedBinaryPath,
  isCached,
  cacheBinary,
  getCachedBinary,
  cleanCacheEntry,
  cleanOldCache,
  getCacheSize
};