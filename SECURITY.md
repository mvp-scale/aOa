# aOa Security

## Before You Install

If you're evaluating aOa, these are the questions we'd ask of any tool before installing it. We think you should ask them of us too.

| Question | Answer |
|----------|--------|
| Does it phone home? | No. aOa makes zero outbound network connections. Ever. |
| Does it collect telemetry? | No. No analytics, no usage tracking, no crash reports sent anywhere. |
| Does it auto-update? | No. No update checks, no background downloads, no silent upgrades. |
| Does it send my code anywhere? | No. Your code is read locally, indexed locally, stored locally. |
| Where does my data live? | In your project directory under `.aoa/`. Nowhere else. |
| Can I delete everything? | Yes. `aoa remove` wipes all aOa data. Or just delete `.aoa/`. |
| Does it run a server? | Yes — a localhost-only daemon for the dashboard. Binds to `127.0.0.1`, not accessible from other machines. |
| Does it open any ports? | One TCP port on localhost (range 19000-19999) for the dashboard, and a Unix socket for CLI-to-daemon IPC. Both local only. |
| Does it need root? | No. |
| Does it modify my source code? | No. aOa is read-only. It indexes your code but never writes to your project files. |
| Who built the binary? | GitHub Actions. The source is compiled in CI, not on a developer's laptop. Every build is traceable to a commit. |
| Who built the grammar parsers? | The open source community. 509 grammars maintained by 346 contributors. We compile and validate them — we don't write them. See [GRAMMAR_REPORT.md](GRAMMAR_REPORT.md) for full attribution. |
| Can I verify this myself? | Yes. The source is right here. Every claim above is verifiable by reading the code or asking an AI agent to audit it for you. |

We built aOa for our own team. The goal is saving 95-99% on tokens by giving AI agents fast, local code intelligence. We use it every day. We'd be concerned installing something like this ourselves, so we're telling you exactly what it does and doesn't do.

---

## What We Scan

These checks run in CI. They're standard Go ecosystem tools — nothing exotic. They don't prove the code is perfect. They prove we checked.

| Check | What it does | What it doesn't do |
|-------|-------------|-------------------|
| **govulncheck** | Scans our dependency tree for known CVEs. Uses call-graph analysis — only flags vulnerabilities in code we actually call. | Doesn't find zero-days or vulnerabilities in our own logic. |
| **gosec** | Scans our source for common security patterns: hardcoded secrets, injection risks, weak crypto, unsafe file permissions (~34 rule categories). | Doesn't catch logic errors or novel attack patterns. |
| **go vet** | Standard Go static analysis. Catches misuse of sync primitives, printf bugs, unreachable code. | Basic. Not a security tool per se. |
| **Network audit** | Verifies zero outbound network patterns in production code: no `http.Client`, no `http.Get`, no `net.Dial` to external addresses. | Can't prove dependencies don't contain dormant network code. We audit those manually. |
| **go version -m** | Every binary self-describes: Go version, all compiled-in dependencies with versions, exact git commit, build settings. You can inspect any release binary yourself. | Shows what's compiled in, not what it does at runtime. |

### What's not in CI yet but on the roadmap

| Check | What it would add |
|-------|------------------|
| **SLSA Level 3 provenance** | Cryptographic proof that the binary was built from the stated source on an isolated builder. Verifiable by anyone. |
| **cosign signing** | Keyless binary signing via GitHub's identity. Logged in a public transparency ledger. Proves the binary came from our CI, not a compromised machine. |

We'll add these when we cut our first versioned release. They're real guarantees, not theater — but they require workflow restructuring that isn't worth doing until the release pipeline is stable.

---

## The Honest Limits

No tool can prove software is secure. Here's what we can't guarantee:

- **Our dependencies could have undiscovered vulnerabilities.** We scan with govulncheck, but zero-days exist. We use well-known, widely-audited Go libraries (bbolt, cobra, fsnotify, tree-sitter bindings).
- **Static analysis has blind spots.** gosec catches common patterns but can't reason about complex logic flows. We mitigate this by keeping the codebase simple — hexagonal architecture, no magic, no reflection-heavy code.
- **The standard build uses CGO** (for tree-sitter grammar loading). This means the binary links against system C libraries, which adds surface area. The light build (`--light`) is pure Go with no CGO — smaller attack surface, but no structural parsing.
- **We're a small team.** We haven't done a formal third-party audit. The code is open for anyone to review, and we welcome security reports.

---

## Architecture

For those who want to verify the claims above, here's where to look:

| Claim | Where to verify |
|-------|----------------|
| No outbound connections | `grep -rn '"net/http"' --include='*.go' .` — only used for localhost server + MIME detection |
| Localhost-only binding | `internal/adapters/web/server.go` — `net.Listen("tcp", "127.0.0.1:...")` |
| Local-only IPC | `internal/adapters/socket/server.go` — `net.Listen("unix", "/tmp/aoa-*.sock")` |
| Project-scoped data | `internal/app/config.go` — all paths under `{projectRoot}/.aoa/` |
| No telemetry | Search the entire codebase for analytics, tracking, or reporting endpoints. There are none. |
| Read-only on project files | `internal/domain/index/filecache.go` — reads files, never writes |
| Clean removal | `cmd/aoa/cmd/wipe.go` — removes `.aoa/` directory |

---

## Reporting Security Issues

If you find a security issue, please email us directly rather than opening a public issue. We'll respond within 48 hours and credit you in the fix.
