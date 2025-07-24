const https = require('https');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const zlib = require('zlib');
const tar = require('tar');
const { getCachedBinary, cacheBinary, cleanOldCache } = require('./cache');

// Get package version
const PACKAGE_VERSION = require('../package.json').version;
const GITHUB_RELEASES_URL = 'https://github.com/dagu-org/dagu/releases/download';

/**
 * Map Node.js platform/arch to goreleaser asset names
 */
function getAssetName(version) {
  const platform = process.platform;
  const arch = process.arch;
  
  // Platform name mapping (matches goreleaser output - lowercase)
  const osMap = {
    'darwin': 'darwin',
    'linux': 'linux',
    'win32': 'windows',
    'freebsd': 'freebsd',
    'openbsd': 'openbsd'
  };
  
  // Architecture name mapping (matches goreleaser output)
  const archMap = {
    'x64': 'amd64',
    'ia32': '386',
    'arm64': 'arm64',
    'ppc64': 'ppc64le',
    's390x': 's390x'
  };
  
  let osName = osMap[platform] || platform;
  let archName = archMap[arch] || arch;
  
  // Special handling for ARM
  if (arch === 'arm' && platform === 'linux') {
    const { getArmVariant } = require('./platform');
    const variant = getArmVariant();
    archName = `armv${variant}`;
  }
  
  // All assets are .tar.gz now (goreleaser changed this)
  const ext = '.tar.gz';
  return `dagu_${version}_${osName}_${archName}${ext}`;
}

/**
 * Make HTTP request and return buffer (Sentry-style)
 */
function makeRequest(url) {
  return new Promise((resolve, reject) => {
    https
      .get(url, (response) => {
        if (response.statusCode >= 200 && response.statusCode < 300) {
          const chunks = [];
          response.on('data', (chunk) => chunks.push(chunk));
          response.on('end', () => {
            resolve(Buffer.concat(chunks));
          });
        } else if (
          response.statusCode >= 300 &&
          response.statusCode < 400 &&
          response.headers.location
        ) {
          // Follow redirects
          makeRequest(response.headers.location).then(resolve, reject);
        } else {
          reject(
            new Error(
              `Server responded with status code ${response.statusCode} when downloading the package!`
            )
          );
        }
      })
      .on('error', (error) => {
        reject(error);
      });
  });
}

/**
 * Download file with progress reporting
 */
async function downloadFile(url, destination, options = {}) {
  const { onProgress, maxRetries = 3 } = options;
  let lastError;
  
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      return await downloadFileAttempt(url, destination, { onProgress, attempt });
    } catch (error) {
      lastError = error;
      if (attempt < maxRetries) {
        console.log(`Download failed (attempt ${attempt}/${maxRetries}), retrying...`);
        await new Promise(resolve => setTimeout(resolve, 1000 * attempt)); // Exponential backoff
      }
    }
  }
  
  throw lastError;
}

/**
 * Single download attempt
 */
function downloadFileAttempt(url, destination, options = {}) {
  const { onProgress, attempt = 1 } = options;
  
  return new Promise((resolve, reject) => {
    const tempFile = `${destination}.download.${process.pid}.tmp`;
    
    https.get(url, (response) => {
      // Handle redirects
      if (response.statusCode === 301 || response.statusCode === 302) {
        const redirectUrl = response.headers.location;
        if (!redirectUrl) {
          reject(new Error('Redirect location not provided'));
          return;
        }
        downloadFileAttempt(redirectUrl, destination, options)
          .then(resolve)
          .catch(reject);
        return;
      }
      
      if (response.statusCode !== 200) {
        reject(new Error(`HTTP ${response.statusCode}: ${response.statusMessage}`));
        return;
      }
      
      const totalSize = parseInt(response.headers['content-length'], 10);
      let downloadedSize = 0;
      
      const fileStream = fs.createWriteStream(tempFile);
      
      response.on('data', (chunk) => {
        downloadedSize += chunk.length;
        if (onProgress && totalSize) {
          const percentage = Math.round((downloadedSize / totalSize) * 100);
          onProgress(percentage, downloadedSize, totalSize);
        }
      });
      
      response.pipe(fileStream);
      
      fileStream.on('finish', () => {
        fileStream.close(() => {
          // Move temp file to final destination
          fs.renameSync(tempFile, destination);
          resolve();
        });
      });
      
      fileStream.on('error', (err) => {
        fs.unlinkSync(tempFile);
        reject(err);
      });
    }).on('error', (err) => {
      if (fs.existsSync(tempFile)) {
        fs.unlinkSync(tempFile);
      }
      reject(err);
    });
  });
}

/**
 * Extract archive based on file extension
 */
async function extractArchive(archivePath, outputDir) {
  const ext = path.extname(archivePath).toLowerCase();
  
  if (ext === '.gz' || archivePath.endsWith('.tar.gz')) {
    // Extract tar.gz
    await tar.extract({
      file: archivePath,
      cwd: outputDir,
      filter: (path) => path === 'dagu' || path === 'dagu.exe'
    });
  } else if (ext === '.zip') {
    // For Windows, we need a zip extractor
    // Using built-in Windows extraction via PowerShell
    const { execSync } = require('child_process');
    const command = `powershell -command "Expand-Archive -Path '${archivePath}' -DestinationPath '${outputDir}' -Force"`;
    execSync(command);
  } else {
    throw new Error(`Unsupported archive format: ${ext}`);
  }
}

/**
 * Download and verify checksums
 */
async function downloadChecksums(version) {
  const checksumsUrl = `${GITHUB_RELEASES_URL}/v${version}/checksums.txt`;
  const tempFile = path.join(require('os').tmpdir(), `dagu-checksums-${process.pid}.txt`);
  
  try {
    await downloadFile(checksumsUrl, tempFile);
    const content = fs.readFileSync(tempFile, 'utf8');
    
    // Parse checksums file
    const checksums = {};
    content.split('\n').forEach(line => {
      const match = line.match(/^([a-f0-9]{64})\s+(.+)$/);
      if (match) {
        checksums[match[2]] = match[1];
      }
    });
    
    return checksums;
  } finally {
    if (fs.existsSync(tempFile)) {
      fs.unlinkSync(tempFile);
    }
  }
}

/**
 * Verify file checksum
 */
function verifyChecksum(filePath, expectedChecksum) {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash('sha256');
    const stream = fs.createReadStream(filePath);
    
    stream.on('data', (data) => hash.update(data));
    stream.on('end', () => {
      const actualChecksum = hash.digest('hex');
      if (actualChecksum === expectedChecksum) {
        resolve(true);
      } else {
        reject(new Error(`Checksum mismatch: expected ${expectedChecksum}, got ${actualChecksum}`));
      }
    });
    stream.on('error', reject);
  });
}

/**
 * Extract file from npm tarball (aligned with Sentry approach)
 */
function extractFileFromTarball(tarballBuffer, filepath) {
  // Tar archives are organized in 512 byte blocks.
  // Blocks can either be header blocks or data blocks.
  // Header blocks contain file names of the archive in the first 100 bytes, terminated by a null byte.
  // The size of a file is contained in bytes 124-135 of a header block and in octal format.
  // The following blocks will be data blocks containing the file.
  let offset = 0;
  while (offset < tarballBuffer.length) {
    const header = tarballBuffer.subarray(offset, offset + 512);
    offset += 512;

    const fileName = header.toString('utf-8', 0, 100).replace(/\0.*/g, '');
    const fileSize = parseInt(header.toString('utf-8', 124, 136).replace(/\0.*/g, ''), 8);

    if (fileName === filepath) {
      return tarballBuffer.subarray(offset, offset + fileSize);
    }

    // Clamp offset to the upper multiple of 512
    offset = (offset + fileSize + 511) & ~511;
  }
  
  throw new Error(`File ${filepath} not found in tarball`);
}

/**
 * Download binary from npm registry (Sentry-style)
 */
async function downloadBinaryFromNpm(version) {
  const { getPlatformPackage } = require('./platform');
  const platformPackage = getPlatformPackage();
  
  if (!platformPackage) {
    throw new Error('Platform not supported!');
  }
  
  const packageName = platformPackage.replace('@dagu-org/', '');
  const binaryName = process.platform === 'win32' ? 'dagu.exe' : 'dagu';
  
  console.log(`Downloading ${platformPackage} from npm registry...`);
  
  // Download the tarball of the right binary distribution package
  const tarballUrl = `https://registry.npmjs.org/${platformPackage}/-/${packageName}-${version}.tgz`;
  const tarballDownloadBuffer = await makeRequest(tarballUrl);
  const tarballBuffer = zlib.unzipSync(tarballDownloadBuffer);
  
  // Extract binary from package
  const binaryData = extractFileFromTarball(tarballBuffer, `package/bin/${binaryName}`);
  
  return binaryData;
}

/**
 * Main download function
 */
async function downloadBinary(destination, options = {}) {
  const version = options.version || PACKAGE_VERSION;
  const { method = 'auto', useCache = true } = options;
  const platformKey = `${process.platform}-${process.arch}`;
  
  console.log(`Installing Dagu v${version} for ${platformKey}...`);
  
  // Check cache first
  if (useCache) {
    const cachedBinary = getCachedBinary(version, platformKey);
    if (cachedBinary) {
      console.log('✓ Using cached binary');
      fs.copyFileSync(cachedBinary, destination);
      if (process.platform !== 'win32') {
        fs.chmodSync(destination, 0o755);
      }
      // Clean old cache entries
      cleanOldCache();
      return;
    }
  }
  
  try {
    let binaryData;
    
    if (method === 'npm' || method === 'auto') {
      // Try npm registry first (following Sentry's approach)
      try {
        binaryData = await downloadBinaryFromNpm(version);
        console.log('✓ Downloaded from npm registry');
      } catch (npmError) {
        if (method === 'npm') {
          throw npmError;
        }
        console.log('npm registry download failed, trying GitHub releases...');
      }
    }
    
    if (!binaryData && (method === 'github' || method === 'auto')) {
      // Fallback to GitHub releases
      const assetName = getAssetName(version);
      const downloadUrl = `${GITHUB_RELEASES_URL}/v${version}/${assetName}`;
      
      const tempFile = path.join(require('os').tmpdir(), `dagu-${process.pid}-${Date.now()}.tmp`);
      
      try {
        await downloadFile(downloadUrl, tempFile, {
          onProgress: (percentage, downloaded, total) => {
            const mb = (size) => (size / 1024 / 1024).toFixed(2);
            process.stdout.write(`\rProgress: ${percentage}% (${mb(downloaded)}MB / ${mb(total)}MB)`);
          }
        });
        console.log('\n✓ Downloaded from GitHub releases');
        
        // Extract from archive
        const binaryName = process.platform === 'win32' ? 'dagu.exe' : 'dagu';
        
        // All files are .tar.gz now
        const archiveData = fs.readFileSync(tempFile);
        const tarData = zlib.gunzipSync(archiveData);
        binaryData = extractFileFromTarball(tarData, binaryName);
      } finally {
        if (fs.existsSync(tempFile)) {
          fs.unlinkSync(tempFile);
        }
      }
    }
    
    if (!binaryData) {
      throw new Error('Failed to download binary from any source');
    }
    
    // Write binary to destination
    const dir = path.dirname(destination);
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
    
    fs.writeFileSync(destination, binaryData, { mode: 0o755 });
    console.log('✓ Binary installed successfully');
    
    // Cache the binary for future use
    if (useCache) {
      try {
        cacheBinary(destination, version, platformKey);
        console.log('✓ Binary cached for future installations');
      } catch (e) {
        // Caching failed, but installation succeeded
      }
    }
    
  } catch (error) {
    throw new Error(`Failed to download binary: ${error.message}`);
  }
}

module.exports = {
  downloadBinary,
  downloadBinaryFromNpm,
  getAssetName
};