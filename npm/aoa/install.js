#!/usr/bin/env node
"use strict";

const os = require("os");

const PLATFORM_MAP = {
  "linux-x64": "@mvpscale/aoa-linux-x64",
  "linux-arm64": "@mvpscale/aoa-linux-arm64",
  "darwin-x64": "@mvpscale/aoa-darwin-x64",
  "darwin-arm64": "@mvpscale/aoa-darwin-arm64",
};

const key = `${os.platform()}-${os.arch()}`;
const pkg = PLATFORM_MAP[key];

if (!pkg) {
  process.stderr.write(`aoa: unsupported platform ${key}\n`);
  process.stderr.write(`Supported: ${Object.keys(PLATFORM_MAP).join(", ")}\n`);
  process.exit(1);
}

// Verify the platform binary is available
try {
  require.resolve(`${pkg}/bin/aoa`);
} catch {
  process.stderr.write(`aoa: platform package ${pkg} not installed.\n`);
  process.stderr.write("Try reinstalling: npm install @mvpscale/aoa\n");
  process.exit(1);
}

process.stderr.write("\n  \x1b[36maOa installed.\x1b[0m Run \x1b[1maoa init\x1b[0m to get started.\n\n");
