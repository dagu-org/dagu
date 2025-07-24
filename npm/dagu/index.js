/**
 * Dagu npm package - programmatic interface
 */

const { getBinaryPath } = require('./lib/platform');
const { spawn } = require('child_process');
const path = require('path');

/**
 * Get the path to the Dagu binary
 * @returns {string|null} Path to the binary or null if not found
 */
function getDaguPath() {
  return getBinaryPath();
}

/**
 * Execute Dagu with given arguments
 * @param {string[]} args Command line arguments
 * @param {object} options Child process spawn options
 * @returns {ChildProcess} The spawned child process
 */
function execute(args = [], options = {}) {
  const binaryPath = getDaguPath();
  
  if (!binaryPath) {
    throw new Error('Dagu binary not found. Please ensure Dagu is properly installed.');
  }
  
  return spawn(binaryPath, args, {
    stdio: 'inherit',
    ...options
  });
}

/**
 * Execute Dagu and return a promise
 * @param {string[]} args Command line arguments
 * @param {object} options Child process spawn options
 * @returns {Promise<{code: number, signal: string|null}>} Exit code and signal
 */
function executeAsync(args = [], options = {}) {
  return new Promise((resolve, reject) => {
    const child = execute(args, {
      stdio: 'pipe',
      ...options
    });
    
    let stdout = '';
    let stderr = '';
    
    if (child.stdout) {
      child.stdout.on('data', (data) => {
        stdout += data.toString();
      });
    }
    
    if (child.stderr) {
      child.stderr.on('data', (data) => {
        stderr += data.toString();
      });
    }
    
    child.on('error', reject);
    
    child.on('close', (code, signal) => {
      resolve({
        code,
        signal,
        stdout,
        stderr
      });
    });
  });
}

module.exports = {
  getDaguPath,
  execute,
  executeAsync,
  // Re-export useful functions
  getBinaryPath,
  getPlatformInfo: require('./lib/platform').getPlatformInfo
};