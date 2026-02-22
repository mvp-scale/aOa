#!/usr/bin/env node
"use strict";

const os = require("os");

const PLATFORM_MAP = {
  "linux-x64": "@mvpscale/aoa-recon-linux-x64",
  "linux-arm64": "@mvpscale/aoa-recon-linux-arm64",
  "darwin-x64": "@mvpscale/aoa-recon-darwin-x64",
  "darwin-arm64": "@mvpscale/aoa-recon-darwin-arm64",
};

const key = `${os.platform()}-${os.arch()}`;
const pkg = PLATFORM_MAP[key];

if (!pkg) {
  process.stderr.write(`aoa-recon: unsupported platform ${key}\n`);
  process.stderr.write(`Supported: ${Object.keys(PLATFORM_MAP).join(", ")}\n`);
  process.exit(1);
}

try {
  require.resolve(`${pkg}/bin/aoa-recon`);
} catch {
  process.stderr.write(`aoa-recon: platform package ${pkg} not installed.\n`);
  process.stderr.write("Try reinstalling: npm install @mvpscale/aoa-recon\n");
  process.exit(1);
}

process.stderr.write("\n  \x1b[36maOa Recon installed.\x1b[0m\n\n");
