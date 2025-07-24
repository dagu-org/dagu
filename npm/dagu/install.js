#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { getBinaryPath, getPlatformPackage, setPlatformBinary, getPlatformInfo } = require('./lib/platform');
const { downloadBinary, downloadBinaryFromNpm } = require('./lib/download');
const { validateBinary } = require('./lib/validate');

async function install() {
  console.log('Installing Dagu...');
  
  try {
    // Check if running in CI or with --ignore-scripts
    if (process.env.npm_config_ignore_scripts === 'true') {
      console.log('Skipping postinstall script (--ignore-scripts flag detected)');
      return;
    }
    
    // Try to resolve platform-specific package
    const platformPackage = getPlatformPackage();
    if (!platformPackage) {
      console.error(`
Error: Unsupported platform: ${process.platform}-${process.arch}

Dagu does not provide pre-built binaries for this platform.
Please build from source: https://github.com/dagu-org/dagu#building-from-source
      `);
      process.exit(1);
    }
    
    console.log(`Detected platform: ${process.platform}-${process.arch}`);
    console.log(`Looking for package: ${platformPackage}`);
    
    // Check for cross-platform scenario
    const { checkCrossPlatformScenario } = require('./lib/platform');
    const crossPlatformWarning = checkCrossPlatformScenario();
    if (crossPlatformWarning) {
      console.warn(`\n${crossPlatformWarning.message}\n`);
    }
    
    // Check if binary already exists from optionalDependency
    const existingBinary = getBinaryPath();
    if (existingBinary && fs.existsSync(existingBinary)) {
      console.log('Using pre-installed binary from optional dependency');
      
      // Validate the binary
      if (await validateBinary(existingBinary)) {
        console.log('✓ Dagu installation complete!');
        return;
      } else {
        console.warn('Binary validation failed, attempting to download...');
      }
    }
  } catch (e) {
    console.log('Optional dependency not found, downloading binary...');
  }
  
  // Fallback: Download binary
  try {
    const binaryPath = path.join(__dirname, 'bin', process.platform === 'win32' ? 'dagu.exe' : 'dagu');
    
    // Create bin directory if it doesn't exist
    const binDir = path.dirname(binaryPath);
    if (!fs.existsSync(binDir)) {
      fs.mkdirSync(binDir, { recursive: true });
    }
    
    // Skip download in development if flag file exists
    if (fs.existsSync(path.join(__dirname, '.skip-install'))) {
      console.log('Development mode: skipping binary download (.skip-install file found)');
      return;
    }
    
    // Download the binary
    await downloadBinary(binaryPath, { method: 'auto' });
    
    // Validate the downloaded binary
    if (await validateBinary(binaryPath)) {
      setPlatformBinary(binaryPath);
      console.log('✓ Dagu installation complete!');
      
      // Print warning about optionalDependencies if none were found
      if (!hasAnyOptionalDependency()) {
        console.warn(`
⚠ WARNING: optionalDependencies may be disabled in your environment.
For better performance and reliability, consider enabling them.
See: https://docs.npmjs.com/cli/v8/using-npm/config#optional
`);
      }
    } else {
      throw new Error('Downloaded binary validation failed');
    }
  } catch (error) {
    console.error('Failed to install Dagu:', error.message);
    console.error(`
Platform details:
${JSON.stringify(getPlatformInfo(), null, 2)}

Please try one of the following:
1. Install manually from: https://github.com/dagu-org/dagu/releases
2. Build from source: https://github.com/dagu-org/dagu#building-from-source
3. Report this issue: https://github.com/dagu-org/dagu/issues
    `);
    process.exit(1);
  }
}

// Check if any optional dependency is installed
function hasAnyOptionalDependency() {
  const pkg = require('./package.json');
  const optionalDeps = Object.keys(pkg.optionalDependencies || {});
  
  for (const dep of optionalDeps) {
    try {
      require.resolve(dep);
      return true;
    } catch (e) {
      // Continue checking
    }
  }
  
  return false;
}

// Handle errors gracefully
process.on('unhandledRejection', (error) => {
  console.error('Installation error:', error);
  process.exit(1);
});

// Run installation
install().catch((error) => {
  console.error('Installation failed:', error);
  process.exit(1);
});