#!/usr/bin/env node
"use strict";

const os = require("os");
const fs = require("fs");
const path = require("path");

const PLATFORM_MAP = {
  "linux-x64": "@aoa/linux-x64",
  "linux-arm64": "@aoa/linux-arm64",
  "darwin-x64": "@aoa/darwin-x64",
  "darwin-arm64": "@aoa/darwin-arm64",
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
