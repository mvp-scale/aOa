#!/usr/bin/env node
"use strict";

const os = require("os");
const fs = require("fs");
const path = require("path");

const PLATFORM_MAP = {
  "linux-x64": "@aoa/recon-linux-x64",
  "linux-arm64": "@aoa/recon-linux-arm64",
  "darwin-x64": "@aoa/recon-darwin-x64",
  "darwin-arm64": "@aoa/recon-darwin-arm64",
};

const key = `${os.platform()}-${os.arch()}`;
const pkg = PLATFORM_MAP[key];

if (!pkg) {
  console.error(`aoa-recon: unsupported platform ${key}`);
  console.error(`Supported: ${Object.keys(PLATFORM_MAP).join(", ")}`);
  process.exit(1);
}

let binPath;
try {
  binPath = require.resolve(`${pkg}/bin/aoa-recon`);
} catch {
  console.error(`aoa-recon: platform package ${pkg} not installed.`);
  console.error("Try reinstalling: npm install -g aoa-recon");
  process.exit(1);
}

const binDir = path.join(__dirname, "bin");
fs.mkdirSync(binDir, { recursive: true });
const dest = path.join(binDir, "aoa-recon");
try { fs.unlinkSync(dest); } catch {}
fs.symlinkSync(binPath, dest);
fs.chmodSync(dest, 0o755);
