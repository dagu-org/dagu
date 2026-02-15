const path = require("path");
const childProcess = require("child_process");

const { getPlatformPackage } = require("./lib/platform");

const binaryName = process.platform === "win32" ? "boltbase.exe" : "boltbase";

function getBinaryPath() {
  try {
    const platformSpecificPackageName = getPlatformPackage();
    return require.resolve(`${platformSpecificPackageName}/bin/${binaryName}`);
  } catch (e) {
    return path.join(__dirname, "..", binaryName);
  }
}

function exitForError(error) {
  if (error && typeof error.status === "number") {
    process.exit(error.status);
  }

  if (error && error.signal) {
    process.kill(process.pid, error.signal);
    return;
  }

  const message = error && typeof error.message === "string" ? error.message : "boltbase execution failed";
  if (message) {
    process.stderr.write(`${message}\n`);
  }

  process.exit(1);
}

module.exports.runBinary = function (...args) {
  try {
    childProcess.execFileSync(getBinaryPath(), args, {
      stdio: "inherit",
    });
  } catch (error) {
    exitForError(error);
  }
};
