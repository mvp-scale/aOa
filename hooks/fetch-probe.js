// fetch-probe.js — minimal test: does NODE_OPTIONS injection work with Claude Code?
// Just logs when any fetch happens. Writes to a file so we can verify.

const fs = require('fs');
const path = require('path');

const PROBE_LOG = path.join(process.env.HOME || '/tmp', '.aoa-fetch-probe.log');

function log(msg) {
  const ts = new Date().toISOString();
  const line = `[${ts}] ${msg}\n`;
  try { fs.appendFileSync(PROBE_LOG, line); } catch (_) {}
}

log('fetch-probe.js loaded into process: ' + process.argv.join(' '));

const originalFetch = globalThis.fetch;

if (typeof originalFetch === 'function') {
  log('globalThis.fetch exists — wrapping it');

  globalThis.fetch = async function (url, options) {
    const urlStr = (url && url.toString) ? url.toString() : String(url);

    // Log all requests, but highlight usage/oauth ones
    if (urlStr.includes('anthropic') || urlStr.includes('claude')) {
      const method = (options && options.method) || 'GET';
      log(`FETCH ${method} ${urlStr}`);

      // If this looks like a usage endpoint, capture the response
      if (urlStr.includes('usage') || urlStr.includes('oauth')) {
        log(`>>> USAGE ENDPOINT HIT: ${urlStr}`);
        try {
          const response = await originalFetch.apply(this, arguments);
          // Clone so we can read without consuming the body
          const cloned = response.clone();
          cloned.text().then(body => {
            log(`>>> USAGE RESPONSE (${response.status}): ${body.substring(0, 2000)}`);
            // Write the raw response to a file for the daemon to pick up
            const usageFile = path.join(process.env.CLAUDE_PROJECT_DIR || process.cwd(), '.aoa', 'usage.json');
            try {
              const parsed = JSON.parse(body);
              parsed.captured_at = Math.floor(Date.now() / 1000);
              fs.writeFileSync(usageFile, JSON.stringify(parsed, null, 2));
              log(`>>> Wrote usage data to ${usageFile}`);
            } catch (_) {
              log(`>>> Could not parse usage response as JSON`);
            }
          }).catch(err => {
            log(`>>> Error reading usage response: ${err.message}`);
          });
          return response;
        } catch (err) {
          log(`>>> USAGE FETCH ERROR: ${err.message}`);
          throw err;
        }
      }
    }

    return originalFetch.apply(this, arguments);
  };
} else {
  log('WARNING: globalThis.fetch is NOT a function — injection cannot wrap fetch');
}
