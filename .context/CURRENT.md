# Session 84 | 2026-03-01 | L4.4 Phase 3: Pre-built Grammar Distribution (continued)

> **Session**: 84

## Done

- [x] build.sh/deploy.sh: symlink (`ln -sf`) instead of copy for `~/bin/aoa`. Fixed `set -euo pipefail` CI failure (`if/then` form).
- [x] L4.4-3.3: Complete UX rewrite for `aoa init` fresh path -- positive tone, zero-outbound policy, GitHub provenance, .aoa/ portability, two-step, `--update` hint
- [x] L4.4-3.3: Returning path -- count detected/ready/missing, file sizes, GitHub source, SHA verification
- [x] L4.4-3.4: download.sh SHA-256 verification via awk from parsers.json (no sidecar). Inline github/local SHA per grammar.
- [x] L4.4-3.5: `aoa init --update` full sync -- scan, generate download.sh, execute via `sh`, continue to indexing. One command.
- [x] CI green after symlink fix

## Decisions

- No download.sha256 sidecar -- SHAs from parsers.json via awk extraction
- Two UX paths: fresh (educational, trust-building) vs returning (`--update`, one command)
- `os/exec` for `--update` only -- runs generated download.sh, not embedding curl
- Symlink over copy for ~/bin/aoa

## Next

- [ ] L4.4-3.6: End-to-end verify on fresh machine
- [ ] Polish items mentioned by user
- [ ] L4.4 Phase 4: npm platform packages (4.1), release workflow (4.3), e2e test (4.4)
- [ ] L5.Va: per-rule detection validation
