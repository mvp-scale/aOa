# Dimensional Taxonomy — Current State & Question Set

> **Date**: 2026-02-23 (Session 67)
> **Source**: `docs/research/bitmask-dimensional-analysis.md`, `internal/adapters/recon/scanner.go`, `internal/adapters/web/static/app.js`
> **Purpose**: Complete inventory of all dimensional views — what questions each dimension asks, which have active detectors, and what remains to be built.

---

## Overview

The recon system is designed around **6 tiers** containing **21 dimensions**. Each dimension asks binary yes/no questions about source code. The answers are stored as bitmasks per method/function.

**Current state**: 113 pattern detectors active across 21 of 21 dimensions. Dashboard renders 21 of 21 dimensions (+ investigated). 29 questions deferred to future AST-based engine.

| Tier | Color | Dimensions | Active Detectors | Dashboard Dims |
|------|-------|:--:|:--:|:--:|
| Security | Red | 5 | 60 | 5 |
| Performance | Yellow | 4 | 14 | 4 |
| Quality | Blue | 4 | 18 | 4 |
| Architecture | Cyan | 3 | 8 | 3 |
| Observability | Green | 2 | 7 | 2 |
| Compliance | Purple | 3 | 8 | 3 |
| **Total** | | **21** | **113** (incl 2 state-only) | **21** |

---

## Tier 1: Security (Red)

> 67 questions across 5 dimensions. Research doc has full bitmask mapping (bits 0-66).
> **Active**: 4 detectors across 3 dimensions. **Missing**: Auth Gaps (0/14), Path Traversal (0/12).

### Dimension: Injection (bits 0-15)

**Dashboard**: Yes | **Active detectors**: 15 of 16 (`command_injection`, `sql_string_concat`, `sql_interpolation`, `raw_sql_no_param`, `shell_invocation_var`, `xss_unescaped`, `xss_reflected`, `ldap_injection`, `xxe_injection`, `regex_injection`, `header_injection`, `log_injection`, `template_injection`, `eval_dynamic`, `unsafe_deserialize`, `orm_raw_query`)

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| 0 | SQL string concatenation? | Structural | | Binary expression with SQL keyword + identifier concat passed to db call |
| 1 | SQL string interpolation? | Structural | | Template/f-string with SQL keyword passed to db call |
| 2 | Raw SQL without parameterization? | Structural | | `db.Query(string)` where string is not a literal constant |
| 3 | Command string concatenation? | Structural | **Yes** | `exec.Command()` / `os.system()` / `subprocess` with concatenated argument |
| 4 | Shell invocation with variable? | Structural | | `sh -c` or `bash -c` with non-literal argument |
| 5 | XSS: unescaped output to HTML? | Structural | | Template render / `innerHTML` / `dangerouslySetInnerHTML` with variable |
| 6 | XSS: reflected input? | Structural | | Request parameter flows to response body without sanitization |
| 7 | LDAP injection? | Text+Struct | | LDAP filter string built with concatenation |
| 8 | XML/XXE injection? | Text | | XML parser without disabling external entities |
| 9 | Regex injection? | Structural | | User input passed directly to regex compiler |
| 10 | Header injection? | Structural | | Unvalidated input in HTTP header value |
| 11 | Log injection? | Structural | | User input written to log without sanitization |
| 12 | Template injection? | Structural | | User input passed as template source (not data) |
| 13 | Eval/exec of dynamic code? | Text+Struct | | `eval()`, `exec()`, `Function()` with non-literal argument |
| 14 | Deserialization of untrusted data? | Text+Struct | | `pickle.loads`, `yaml.load` (unsafe), `ObjectInputStream` on external input |
| 15 | ORM raw query bypass? | Structural | | `.raw()`, `.execute()` on ORM with string building |

### Dimension: Secrets (bits 16-28)

**Dashboard**: Yes | **Active detectors**: 12 of 13 (`hardcoded_secret`, `hardcoded_password`, `hardcoded_token`, `private_key_material`, `aws_credentials`, `connection_string_cred`, `jwt_secret`, `encryption_key_literal`, `env_default_secret`, `disabled_tls_verify`, `debug_credentials`, `oauth_client_secret`, `webhook_secret`)

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| 16 | Hardcoded API key? | Text | **Yes** | Variable assignment matching `[A-Za-z0-9]{20,}` near `key`, `api_key`, `apikey` |
| 17 | Hardcoded password? | Text | | Assignment to `password`, `passwd`, `pwd` with string literal |
| 18 | Hardcoded secret/token? | Text | | Assignment to `secret`, `token`, `bearer` with string literal |
| 19 | Private key material inline? | Text | | `BEGIN RSA PRIVATE KEY`, `BEGIN EC PRIVATE KEY`, `BEGIN OPENSSH` |
| 20 | AWS credentials? | Text | | Pattern `AKIA[0-9A-Z]{16}` or `aws_secret_access_key` with literal |
| 21 | Connection string with credentials? | Text | | `://user:pass@` pattern in string literal |
| 22 | JWT secret inline? | Text | | `jwt.sign` / `jwt.encode` with hardcoded string key |
| 23 | Encryption key as literal? | Text+Struct | | AES/cipher init with hardcoded byte array or string |
| 24 | .env values committed? | Text | | Direct reference to values that look like env defaults with secrets |
| 25 | Disabled TLS verification? | Text | | `InsecureSkipVerify: true`, `verify=False`, `NODE_TLS_REJECT_UNAUTHORIZED=0` |
| 26 | Debug credentials? | Text | | `admin/admin`, `test/test`, `root/root` in auth context |
| 27 | OAuth client secret inline? | Text | | `client_secret` assignment with string literal |
| 28 | Webhook secret inline? | Text | | `webhook_secret`, `signing_secret` with string literal |

### Dimension: Auth Gaps (bits 29-42)

**Dashboard**: Yes | **Active detectors**: 7 of 14 (`missing_csrf`, `session_no_expiry`, `insecure_password_compare`, `token_in_url`, `permissive_cors`, `missing_security_headers`, `unvalidated_redirect`). 7 require AST.

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| 29 | Route without auth middleware? | Structural | | HTTP handler registration without auth decorator/middleware |
| 30 | Auth bypass in conditional? | Structural | | Auth check in `if` with alternative path that skips validation |
| 31 | Missing CSRF protection? | Structural | | POST/PUT/DELETE handler without CSRF token validation |
| 32 | Session without expiry? | Structural | | Session creation without `MaxAge`/`Expires` setting |
| 33 | Missing rate limiting? | Structural | | Login/auth endpoint without rate limiter middleware |
| 34 | Broken access control? | Structural | | Resource access using user-supplied ID without ownership check |
| 35 | Missing input validation? | Structural | | Request body/params consumed without validation/schema check |
| 36 | Privilege escalation path? | Structural | | Role check that falls through to admin action |
| 37 | Insecure password comparison? | Text+Struct | | `==` comparison on password/hash instead of constant-time compare |
| 38 | Missing auth on file upload? | Structural | | File upload handler without authentication check |
| 39 | Token in URL/query string? | Structural | | Auth token passed as query parameter (logged in access logs) |
| 40 | Permissive CORS? | Text | | `Access-Control-Allow-Origin: *` or `AllowAllOrigins: true` |
| 41 | Missing security headers? | Text+Struct | | Response missing `X-Frame-Options`, `CSP`, `HSTS` |
| 42 | Unvalidated redirect? | Structural | | Redirect target from user input without whitelist check |

### Dimension: Cryptography (bits 43-54)

**Dashboard**: Yes | **Active detectors**: 12 of 12 (`weak_hash`, `insecure_random`, `ecb_mode`, `static_iv`, `short_key_length`, `missing_salt`, `custom_crypto`, `deprecated_tls`, `weak_cipher_suite`, `missing_cert_validation`, `plaintext_sensitive_storage`, `insecure_key_derivation`)

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| 43 | Weak hash algorithm? | Text | **Yes** | MD5 or SHA1 used for security-sensitive hashing |
| 44 | ECB mode? | Text | | AES-ECB or block cipher without mode specification |
| 45 | Static IV/nonce? | Structural | | Cipher initialization with hardcoded IV or nonce |
| 46 | Insufficient key length? | Text+Struct | | RSA < 2048, AES < 128, ECDSA < 256 |
| 47 | Missing salt? | Structural | | Password hash without salt parameter |
| 48 | Custom crypto implementation? | Text+Struct | | Hand-rolled encrypt/decrypt not using standard library |
| 49 | Predictable random for security? | Text+Struct | **Yes** | `math/rand`, `Math.random()` in auth/token/key context |
| 50 | Deprecated TLS version? | Text | | TLS 1.0, TLS 1.1, SSL 3.0 configuration |
| 51 | Weak cipher suite? | Text | | RC4, DES, 3DES, NULL cipher in TLS config |
| 52 | Missing certificate validation? | Text+Struct | | Custom TLS config that skips cert chain verification |
| 53 | Plaintext storage of sensitive data? | Structural | | Writing password/SSN/credit card to DB/file without encryption |
| 54 | Insecure key derivation? | Text | | Direct hash as key instead of PBKDF2/scrypt/argon2 |

### Dimension: Path Traversal (bits 55-66)

**Dashboard**: Yes | **Active detectors**: 12 of 12 (`path_from_user_input`, `directory_traversal`, `symlink_following`, `zip_slip`, `unrestricted_upload_path`, `file_include_dynamic`, `ssrf_user_url`, `file_delete_user_path`, `temp_file_predictable`, `world_readable_perms`, `path_join_no_clean`, `unrestricted_glob`)

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| 55 | Path from user input unsanitized? | Structural | | File open/read/write with request parameter in path |
| 56 | Directory traversal possible? | Structural | | Path join with user input not checked for `..` |
| 57 | Symlink following? | Text+Struct | | File operations without `O_NOFOLLOW` or `Lstat` check |
| 58 | Archive extraction without path check? | Structural | | Zip/tar extraction without validating entry paths (zip slip) |
| 59 | Unrestricted file upload path? | Structural | | Uploaded file saved with original filename without sanitization |
| 60 | File include with user input? | Structural | | Dynamic `require()`, `import()`, `include()` with variable path |
| 61 | SSRF via user-controlled URL? | Structural | | HTTP request where URL comes from user input without whitelist |
| 62 | File deletion with user path? | Structural | | `os.Remove`/`unlink` with user-supplied path |
| 63 | Temp file in predictable location? | Text+Struct | | Hardcoded `/tmp/` path for sensitive data |
| 64 | World-readable file permissions? | Text | | `0777`, `0666`, `os.ModePerm` on file creation |
| 65 | Path join without clean? | Structural | | `filepath.Join` with user input without `filepath.Clean` |
| 66 | Unrestricted glob/walk scope? | Structural | | `filepath.Walk`/`glob` rooted at user-supplied directory |

---

## Tier 2: Performance (Yellow)

> 4 dimensions. Questions focus on runtime cost signals detectable from static analysis.
> **Active**: 1 detector in 1 dimension. **Missing**: Concurrency (empty), Query Patterns (not in dashboard), Memory (not in dashboard).

### Dimension: Resource Leaks

**Dashboard**: Yes | **Active detectors**: 5 of 5 (`defer_in_loop`, `open_without_close`, `listener_no_shutdown`, `context_no_cancel`, `unclosed_channel`)

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| P1 | Defer inside loop? | Structural | **Yes** | `defer` statement inside `for`/`range` loop body |
| P2 | Open without close in same scope? | Structural | | File/connection/handle opened without matching close/defer |
| P3 | Listener/server without shutdown? | Structural | | `net.Listen` or `http.Serve` without graceful shutdown path |
| P4 | Context without cancel? | Structural | | `context.WithCancel`/`WithTimeout` without calling cancel |
| P5 | Unclosed channel? | Structural | | Buffered channel created without matching close |

### Dimension: Concurrency

**Dashboard**: Yes | **Active detectors**: 3 of 5 (`mutex_no_unlock`, `goroutine_leak`, `unbuffered_channel_select`). 2 require AST (race condition, WaitGroup misuse).

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| P6 | Race condition on shared state? | Structural | | Variable written in goroutine without mutex/atomic |
| P7 | Mutex lock without unlock? | Structural | | `Lock()` call without matching `Unlock()` in scope |
| P8 | Goroutine leak (no exit path)? | Structural | | `go func()` without context cancellation or done channel |
| P9 | Unbuffered channel in select? | Structural | | Select with unbuffered channel and no default case |
| P10 | WaitGroup misuse? | Structural | | `Add()` after `Wait()`, or `Done()` count mismatch |

### Dimension: Query Patterns

**Dashboard**: Yes | **Active detectors**: 2 of 4 (`n_plus_one_query`, `unbounded_result_set`). 2 require AST (missing index hint, repeated identical query).

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| P11 | N+1 query in loop? | Structural | | Database call inside for/range loop body |
| P12 | Unbounded result set? | Structural | | `SELECT` without `LIMIT` or pagination in query |
| P13 | Missing index hint? | Text | | Query on large table without index annotation/comment |
| P14 | Repeated identical query? | Structural | | Same query call in tight scope without caching |

### Dimension: Memory

**Dashboard**: Yes | **Active detectors**: 4 of 5 (`unbounded_append`, `string_concat_in_loop`, `byte_to_string_in_loop`, `allocation_in_hot_path`). 1 requires AST (large struct copy).

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| P15 | Unbounded slice/map growth? | Structural | | `append()` or map insert inside loop without cap/limit |
| P16 | Large struct copy? | Structural | | Value receiver on struct > N fields, or pass-by-value to function |
| P17 | String concatenation in loop? | Structural | | `+` or `fmt.Sprintf` on string inside loop (vs `strings.Builder`) |
| P18 | Byte slice to string conversion in loop? | Structural | | `string(bytes)` inside tight loop |
| P19 | Allocation in hot path? | Structural | | `make()`/`new()` inside handler or frequently-called function |

---

## Tier 3: Quality (Blue)

> 4 dimensions. Questions focus on code maintainability and correctness signals.
> **Active**: 3 detectors across 2 dimensions. **Missing**: Dead Code, Conventions.

### Dimension: Complexity

**Dashboard**: Yes | **Active detectors**: 5 of 6 (`long_function`, `nesting_depth`, `too_many_params`, `too_many_returns`, `large_switch`). `cyclomatic_complexity` placeholder wired but fires via state machine heuristic.

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| Q1 | Function exceeds 100 lines? | Structural | **Yes** | Line count from function open brace to close brace |
| Q2 | Cyclomatic complexity > 10? | Structural | | Count of if/switch/for/case/&& nodes in function body |
| Q3 | Nesting depth > 4? | Structural | | Nested if/for/switch exceeds 4 levels |
| Q4 | Too many parameters (> 5)? | Structural | | Function parameter list length |
| Q5 | Too many return values (> 3)? | Structural | | Go-specific: function return tuple length |
| Q6 | Switch/match with > 15 cases? | Structural | | Case count in switch/select statement |

### Dimension: Error Handling

**Dashboard**: Yes | **Active detectors**: `ignored_error`, `panic_in_lib` (2)

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| Q7 | Error assigned to blank identifier? | Structural | **Yes** | `_ = func()` or `_, _ = func()` where func returns error |
| Q8 | Panic in library package? | Text+Struct | **Yes** | `panic()` call in non-main package |
| Q9 | Unchecked type assertion? | Structural | | `x.(Type)` without comma-ok form |
| Q10 | Error not checked after call? | Structural | | Function returning error where return is not captured |
| Q11 | Empty catch/recover block? | Structural | | `recover()` or `catch` with empty body |
| Q12 | Error message without context? | Structural | | `return err` without `fmt.Errorf("...: %w", err)` wrapping |

### Dimension: Dead Code

**Dashboard**: No | **Active detectors**: None

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| Q13 | Unreachable code after return? | Structural | | Statements following unconditional return/panic/os.Exit |
| Q14 | Unused exported function? | Structural | | Exported symbol with no references in project |
| Q15 | Dead branch (always true/false)? | Structural | | Condition that can be statically determined |
| Q16 | Commented-out code block? | Text | | Multi-line comment containing function/variable syntax |
| Q17 | Unused import? | Structural | | Import declaration with no reference in file |

### Dimension: Conventions

**Dashboard**: No | **Active detectors**: None

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| Q18 | Inconsistent naming style? | Text | | Mixed camelCase/snake_case in same package |
| Q19 | Magic number without constant? | Structural | | Numeric literal > 1 used directly (not in const declaration) |
| Q20 | Mutable package-level variable? | Structural | | `var` at package level that isn't const-eligible |
| Q21 | Init function with side effects? | Structural | | `func init()` that does IO, network calls, or global mutation |
| Q22 | Inconsistent error variable naming? | Text | | `e`, `ex`, `error` instead of `err` in Go context |

---

## Tier 4: Architecture (Cyan)

> 3 dimensions. Questions focus on structural health of the codebase.
> **Active**: 1 detector in 1 dimension. **Missing**: Import Health, API Surface.

### Dimension: Anti-patterns

**Dashboard**: Yes | **Active detectors**: `global_state` (1)

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| A1 | Mutable global variable? | Structural | **Yes** | `var` at package level with non-const type |
| A2 | God object (struct > 15 fields)? | Structural | | Struct declaration with excessive field count |
| A3 | Circular method calls? | Structural | | Method A calls B calls A in same package |
| A4 | Service locator pattern? | Structural | | Global registry / singleton accessor for dependencies |
| A5 | Hard-coded configuration? | Text | | IP addresses, URLs, ports as string literals outside config |

### Dimension: Import Health

**Dashboard**: No | **Active detectors**: None

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| A6 | Circular import? | Structural | | Package A imports B imports A (direct or transitive) |
| A7 | Banned/deprecated import? | Text | | Import of known deprecated or unsafe packages |
| A8 | Internal package leak? | Structural | | Package in `internal/` imported from outside its tree |
| A9 | Excessive import count (> 15)? | Structural | | Import block with too many direct dependencies |
| A10 | Domain imports adapter? | Structural | | Code in `domain/` importing from `adapters/` (hexagonal violation) |

### Dimension: API Surface

**Dashboard**: No | **Active detectors**: None

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| A11 | Exported function without doc comment? | Structural | | Uppercase function/type without preceding `//` comment |
| A12 | Leaking internal type in public API? | Structural | | Exported function returning unexported type |
| A13 | Unstable interface (> 10 methods)? | Structural | | Interface declaration with excessive method count |
| A14 | Breaking change in exported signature? | Structural | | Requires diff analysis — parameter/return type changes |

---

## Tier 5: Observability (Green)

> 2 dimensions. Questions focus on debug hygiene and failure visibility.
> **Active**: 2 detectors in 1 dimension. **Missing**: Silent Failures.

### Dimension: Debug Artifacts

**Dashboard**: Yes | **Active detectors**: `print_statement`, `todo_fixme` (2)

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| O1 | Debug print left in code? | Text | **Yes** | `fmt.Println`, `console.log`, `print(`, `System.out.println`, `var_dump` |
| O2 | TODO/FIXME/HACK marker? | Text | **Yes** | `TODO`, `FIXME`, `HACK`, `XXX` in comments |
| O3 | Debug endpoint exposed? | Text | | `/debug/`, `/pprof/`, `/__debug__` in route registration |
| O4 | Verbose logging in production code? | Text+Struct | | `log.Debug` or trace-level logging outside test files |

### Dimension: Silent Failures

**Dashboard**: No | **Active detectors**: None

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| O5 | Empty catch/recover block? | Structural | | `recover()` or `catch {}` with empty body — error swallowed |
| O6 | Recovered panic without logging? | Structural | | `recover()` in defer without log/error output |
| O7 | Context cancellation ignored? | Structural | | `ctx.Done()` channel not checked in long-running loop |
| O8 | Channel never read? | Structural | | Buffered channel written to but never consumed |
| O9 | Goroutine fire-and-forget? | Structural | | `go func()` without error propagation or logging |

---

## Tier 6: Compliance (Purple)

> 3 dimensions. Entirely unimplemented — not in dashboard, no detectors.
> Requires new `RECON_TIERS` entry in `app.js`.

### Dimension: CVE Patterns

**Dashboard**: No | **Active detectors**: None

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| C1 | Known vulnerable function call? | Text | | Calls to functions with known CVEs (e.g., `xml.NewDecoder` without entity disable) |
| C2 | Deprecated standard library API? | Text | | Usage of deprecated stdlib functions (e.g., `ioutil.ReadAll` in Go) |
| C3 | Unsafe defaults? | Text+Struct | | Security-sensitive constructor without explicit safe config |
| C4 | Known insecure dependency pattern? | Text | | Import of packages with known security advisories |

### Dimension: Licensing

**Dashboard**: No | **Active detectors**: None

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| C5 | Missing license header? | Text | | Source file without SPDX or license comment in first 10 lines |
| C6 | GPL contamination in permissive project? | Text | | GPL-licensed dependency imported into MIT/Apache project |
| C7 | License file missing? | Text | | No LICENSE/COPYING file in project root |

### Dimension: Data Handling

**Dashboard**: No | **Active detectors**: None

| # | Question | Engine | Active | What It Looks For |
|---|----------|--------|:------:|-------------------|
| C8 | PII in log output? | Text+Struct | | Email, SSN, phone patterns in log/print arguments |
| C9 | Unencrypted storage of sensitive data? | Structural | | Writing credentials/tokens to file/DB without encryption |
| C10 | Missing data sanitization? | Structural | | User input passed to storage without sanitization |
| C11 | Sensitive data in error message? | Structural | | Error response containing internal paths, stack traces, credentials |

---

## Implementation Notes

### Files to modify per dimension

Every new dimension requires changes in 3 files:

1. **`internal/adapters/recon/scanner.go`** — Add detector patterns to `buildPatterns()`
2. **`internal/adapters/web/recon.go`** — Add rule→tier/dimension mappings to `inferTierDim()`
3. **`internal/adapters/web/static/app.js`** — Add dimension entries to `RECON_TIERS`

### Detection engine types

| Engine | When It Runs | What It Matches | Cost |
|--------|-------------|-----------------|------|
| **Structural** | Index time (AST walker) | Parent/child/sibling node relationships | ~0.1-0.5ms/file |
| **Text** (Aho-Corasick) | Index time (single pass) | Literal string patterns, all simultaneously | ~0.01ms/file |
| **Text+Struct** | Index time (AC flags, walker confirms) | AC finds candidates, structural walker validates context | ~0.02ms/file |
| **Regex** | Index time (on AC candidates only) | Complex character-class patterns | ~0.01ms/file |

### Current interim scanner vs future bitmask engine

The current scanner in `scanner.go` uses **regex-based pattern matching** against raw source lines. This is the interim approach. The full bitmask engine (described in the research doc) would use tree-sitter AST walking + Aho-Corasick for much higher precision and cross-language uniformity. The interim scanner is sufficient for the patterns currently implemented but will need replacement as structural questions (Auth Gaps, most of Injection) require AST awareness.

### Question numbering

- **Security**: Bits 0-66 (from research doc, stable)
- **Performance**: P1-P19 (proposed in this document)
- **Quality**: Q1-Q22 (proposed in this document)
- **Architecture**: A1-A14 (proposed in this document)
- **Observability**: O1-O9 (proposed in this document)
- **Compliance**: C1-C11 (proposed in this document)
- **Total**: 67 + 19 + 22 + 14 + 9 + 11 = **142 questions**

### Board tasks

| Task | Scope |
|------|-------|
| L5.7 | Performance tier — fill P2-P19 |
| L5.8 | Quality tier — fill Q2-Q6, Q9-Q22 |
| L5.16 | Security expansion — fill bits 4-15, 17-28, 29-42, 55-66 |
| L5.17 | Architecture expansion — fill A6-A14 |
| L5.18 | Observability expansion — fill O5-O9 |
| L5.19 | Compliance tier — fill C1-C11 |
