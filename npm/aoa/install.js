#!/usr/bin/env node
"use strict";

const os = require("os");
const fs = require("fs");
const path = require("path");

const PLATFORM_MAP = {
  "linux-x64": "@mvpscale/aoa-linux-x64",
  "linux-arm64": "@mvpscale/aoa-linux-arm64",
  "darwin-x64": "@mvpscale/aoa-darwin-x64",
  "darwin-arm64": "@mvpscale/aoa-darwin-arm64",
};

const key = `${os.platform()}-${os.arch()}`;
const pkg = PLATFORM_MAP[key];

if (!pkg) {
  console.error(`aoa: unsupported platform ${key}`);
  console.error(`Supported: ${Object.keys(PLATFORM_MAP).join(", ")}`);
  process.exit(1);
}

let binPath;
try {
  binPath = require.resolve(`${pkg}/bin/aoa`);
} catch {
  console.error(`aoa: platform package ${pkg} not installed.`);
  console.error("Try reinstalling: npm install -g aoa");
  process.exit(1);
}

// Create bin/ directory and symlink
const binDir = path.join(__dirname, "bin");
fs.mkdirSync(binDir, { recursive: true });
const dest = path.join(binDir, "aoa");
try { fs.unlinkSync(dest); } catch {}
fs.symlinkSync(binPath, dest);
fs.chmodSync(dest, 0o755);

// Guide users to make `aoa` available on PATH
const isGlobal = process.env.npm_config_global === "true";
if (!isGlobal) {
  console.log("");
  console.log("  \x1b[36maOa installed.\x1b[0m Run \x1b[1maoa init\x1b[0m to get started.");
  console.log("");
  console.log("  To add aoa to your PATH, add this to your shell profile (~/.bashrc or ~/.zshrc):");
  console.log("");
  console.log('    export PATH="./node_modules/.bin:$PATH"');
  console.log("");
  console.log("  Or run directly: ./node_modules/.bin/aoa");
  console.log("  Or use: npx @mvpscale/aoa");
  console.log("");
}
