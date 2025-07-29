const path = require("path");
const childProcess = require("child_process");

const { getPlatformPackage } = require("./lib/platform");

function getBinaryPath() {
  try {
    const platformSpecificPackageName = getPlatformPackage();
    // Resolving will fail if the optionalDependency was not installed
    return require.resolve(`${platformSpecificPackageName}/bin/${binaryName}`);
  } catch (e) {
    return path.join(__dirname, "..", binaryName);
  }
}

module.exports.runBinary = function (...args) {
  childProcess.execFileSync(getBinaryPath(), args, {
    stdio: "inherit",
  });
};
