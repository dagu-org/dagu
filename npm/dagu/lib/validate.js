const { spawn } = require('child_process');
const fs = require('fs');

/**
 * Validate that the binary is executable and returns expected output
 * @param {string} binaryPath Path to the binary
 * @returns {Promise<boolean>} True if valid, false otherwise
 */
async function validateBinary(binaryPath) {
  // Check if file exists
  if (!fs.existsSync(binaryPath)) {
    return false;
  }
  
  // Check if file is executable (on Unix-like systems)
  if (process.platform !== 'win32') {
    try {
      fs.accessSync(binaryPath, fs.constants.X_OK);
    } catch (e) {
      console.error('Binary is not executable');
      return false;
    }
  }
  
  // Try to run the binary with version subcommand (Dagu uses 'version' not '--version')
  return new Promise((resolve) => {
    const proc = spawn(binaryPath, ['version'], {
      timeout: 5000, // 5 second timeout
      windowsHide: true
    });
    
    let stdout = '';
    let stderr = '';
    
    proc.stdout.on('data', (data) => {
      stdout += data.toString();
    });
    
    proc.stderr.on('data', (data) => {
      stderr += data.toString();
    });
    
    proc.on('error', (error) => {
      console.error('Failed to execute binary:', error.message);
      resolve(false);
    });
    
    proc.on('close', (code) => {
      // Check if the binary executed successfully and returned version info
      // Dagu version command returns version string to stderr like "1.18.0" or "v1.17.4-86-gd5a422f1-250729135655"
      if (code === 0 && stderr.trim().length > 0) {
        resolve(true);
      } else {
        console.error(`Binary validation failed: exit code ${code}`);
        if (stderr) {
          console.error('stderr:', stderr);
        }
        resolve(false);
      }
    });
  });
}

module.exports = {
  validateBinary
};