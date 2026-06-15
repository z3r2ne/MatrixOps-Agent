const { spawn } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const electronMirror =
  process.env.MATRIXOPS_ELECTRON_MIRROR ||
  "https://npmmirror.com/mirrors/electron/";

function ensureBackendBinaryInBuildDir() {
  if (process.env.MATRIXOPS_SKIP_BACKEND_BUILD === "true") {
    return;
  }
  const frontendDir = process.cwd();
  const projectRoot = path.resolve(frontendDir, "..");
  const buildDir = path.join(projectRoot, "build");
  fs.mkdirSync(buildDir, { recursive: true });

  const exeName = process.platform === "win32" ? "matrixops.exe" : "matrixops";
  const outPath = path.join(buildDir, exeName);
  const legacyPath = path.join(projectRoot, exeName);
  if (fs.existsSync(legacyPath) && !fs.existsSync(outPath)) {
    fs.renameSync(legacyPath, outPath);
  }
}

const env = {
  ...process.env,
  ELECTRON_MIRROR: electronMirror,
  npm_config_electron_mirror: electronMirror,
  NPM_CONFIG_ELECTRON_MIRROR: electronMirror,
};

ensureBackendBinaryInBuildDir();

const child = spawn(
  process.execPath,
  [require.resolve("electron-builder/cli.js"), ...process.argv.slice(2)],
  {
    cwd: process.cwd(),
    env,
    stdio: "inherit",
  }
);

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 1);
});
