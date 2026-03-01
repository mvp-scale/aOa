#!/usr/bin/env node
"use strict";

const os = require("os");
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

// Verify the platform binary is available
try {
  require.resolve(`${pkg}/bin/aoa`);
} catch {
  console.error(`aoa: platform package ${pkg} not installed.`);
  console.error("Try reinstalling: npm install @mvpscale/aoa");
  process.exit(1);
}

// Detect if this was a global install (bin goes on PATH automatically)
const isGlobal = process.env.npm_config_global === "true";

console.log("");
console.log("  \x1b[36m\u2713 aOa installed.\x1b[0m");
console.log("");
console.log("  aOa has no embedded downloader \u2014 for security, you control");
console.log("  all downloads using tools you already have.");
console.log("");

if (isGlobal) {
  console.log("  Get started:");
  console.log("    cd your-project && aoa init");
} else {
  console.log("  Get started:");
  console.log("    cd your-project && npx aoa init");
}

console.log("");
console.log("  aoa init will guide you through setting up grammar support.");
console.log("");
