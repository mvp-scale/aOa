# Bitmask Dimensional Analysis — Security Worked Example

> Research artifact. Demonstrates the per-line bitmask technique using tree-sitter + AC pattern matching for security dimensional scoring.

> See also: [AST vs LSP — Viability Assessment](.context/research/asv_vs_lsp.md) for the full comparison of why tree-sitter ASTs are sufficient for early warning dimensional analysis.

---

## How It Works

**Input:** Source code parsed by tree-sitter into a language-unified AST.

**Process:**
1. Define a question set — binary yes/no questions about security concerns (60-80 for security alone)
2. Each question occupies a fixed bit position in the bitmask
3. For each line of code, run two engines against the AST + raw text:
   - **Structural engine** (tree-sitter AST walker) — answers questions requiring code structure (e.g., "is a concatenated string passed to a query call?")
   - **Text engine** (Aho-Corasick + regex) — answers questions requiring pattern matching (e.g., "does this line contain a hardcoded API key pattern?")
4. Each line produces a bitmask: `1` = evidence found, `0` = no evidence
5. At the method/function level, roll up all line bitmasks
6. Compute a severity score from the rollup based on hit rate and question weights

**Output:** A per-method security score and a bitmask fingerprint showing exactly which questions triggered.

---

## Security Question Set (67 questions across 5 categories)

Each question has a fixed bit position. The bitmask is 67 bits wide for the security dimension.

### Category 1: Injection (bits 0-15)

| Bit | Question | Engine | What it looks for |
|-----|----------|--------|-------------------|
| 0 | SQL string concatenation? | Structural | Binary expression containing SQL keyword (`SELECT`, `INSERT`, `UPDATE`, `DELETE`) + identifier concatenation passed to db call |
| 1 | SQL string interpolation? | Structural | Template/f-string containing SQL keyword passed to db call |
| 2 | Raw SQL without parameterization? | Structural | `db.Query(string)` where string is not a literal constant |
| 3 | Command string concatenation? | Structural | `exec.Command()` / `os.system()` / `subprocess` with concatenated argument |
| 4 | Shell invocation with variable? | Structural | `sh -c` or `bash -c` with non-literal argument |
| 5 | XSS: unescaped output to HTML? | Structural | Template render / `innerHTML` / `dangerouslySetInnerHTML` with variable input |
| 6 | XSS: reflected input? | Structural | Request parameter flows to response body without sanitization |
| 7 | LDAP injection? | Text+Struct | LDAP filter string built with concatenation |
| 8 | XML/XXE injection? | Text | XML parser configured without disabling external entities |
| 9 | Regex injection? | Structural | User input passed directly to regex compiler |
| 10 | Header injection? | Structural | Unvalidated input in HTTP header value |
| 11 | Log injection? | Structural | User input written to log without sanitization |
| 12 | Template injection? | Structural | User input passed as template source (not data) |
| 13 | Eval/exec of dynamic code? | Text+Struct | `eval()`, `exec()`, `Function()` with non-literal argument |
| 14 | Deserialization of untrusted data? | Text+Struct | `pickle.loads`, `yaml.load` (unsafe), `ObjectInputStream` on external input |
| 15 | ORM raw query bypass? | Structural | `.raw()`, `.execute()` on ORM with string building |

### Category 2: Secrets (bits 16-28)

| Bit | Question | Engine | What it looks for |
|-----|----------|--------|-------------------|
| 16 | Hardcoded API key? | Text | Variable assignment matching `[A-Za-z0-9]{20,}` near keyword `key`, `api_key`, `apikey` |
| 17 | Hardcoded password? | Text | Assignment to `password`, `passwd`, `pwd` with string literal |
| 18 | Hardcoded secret/token? | Text | Assignment to `secret`, `token`, `bearer` with string literal |
| 19 | Private key material inline? | Text | `BEGIN RSA PRIVATE KEY`, `BEGIN EC PRIVATE KEY`, `BEGIN OPENSSH` |
| 20 | AWS credentials? | Text | Pattern `AKIA[0-9A-Z]{16}` or `aws_secret_access_key` with literal |
| 21 | Connection string with credentials? | Text | `://user:pass@` pattern in string literal |
| 22 | JWT secret inline? | Text | `jwt.sign` / `jwt.encode` with hardcoded string key |
| 23 | Encryption key as literal? | Text+Struct | AES/cipher init with hardcoded byte array or string |
| 24 | .env values committed? | Text | Direct reference to values that look like env defaults with secrets |
| 25 | Disabled TLS verification? | Text | `InsecureSkipVerify: true`, `verify=False`, `NODE_TLS_REJECT_UNAUTHORIZED=0` |
| 26 | Debug credentials? | Text | `admin/admin`, `test/test`, `root/root` in auth context |
| 27 | OAuth client secret inline? | Text | `client_secret` assignment with string literal |
| 28 | Webhook secret inline? | Text | `webhook_secret`, `signing_secret` with string literal |

### Category 3: Auth Gaps (bits 29-42)

| Bit | Question | Engine | What it looks for |
|-----|----------|--------|-------------------|
| 29 | Route without auth middleware? | Structural | HTTP handler registration without auth decorator/middleware in chain |
| 30 | Auth bypass in conditional? | Structural | Auth check in `if` with alternative path that skips validation |
| 31 | Missing CSRF protection? | Structural | POST/PUT/DELETE handler without CSRF token validation |
| 32 | Session without expiry? | Structural | Session creation without `MaxAge`/`Expires` setting |
| 33 | Missing rate limiting? | Structural | Login/auth endpoint without rate limiter middleware |
| 34 | Broken access control? | Structural | Resource access using user-supplied ID without ownership check |
| 35 | Missing input validation? | Structural | Request body/params consumed without validation/schema check |
| 36 | Privilege escalation path? | Structural | Role check that falls through to admin action |
| 37 | Insecure password comparison? | Text+Struct | `==` comparison on password/hash instead of constant-time compare |
| 38 | Missing auth on file upload? | Structural | File upload handler without authentication check |
| 39 | Token in URL/query string? | Structural | Auth token passed as query parameter (logged in access logs) |
| 40 | Permissive CORS? | Text | `Access-Control-Allow-Origin: *` or `AllowAllOrigins: true` |
| 41 | Missing security headers? | Text+Struct | Response without `X-Frame-Options`, `Content-Security-Policy`, `Strict-Transport-Security` |
| 42 | Unvalidated redirect? | Structural | Redirect target from user input without whitelist check |

### Category 4: Cryptography (bits 43-54)

| Bit | Question | Engine | What it looks for |
|-----|----------|--------|-------------------|
| 43 | Weak hash algorithm? | Text | `MD5`, `SHA1` used for security-sensitive hashing (not checksums) |
| 44 | ECB mode? | Text | AES-ECB or block cipher without mode specification (defaults ECB) |
| 45 | Static IV/nonce? | Structural | Cipher initialization with hardcoded IV or nonce |
| 46 | Insufficient key length? | Text+Struct | RSA < 2048, AES < 128, ECDSA < 256 |
| 47 | Missing salt? | Structural | Password hash without salt parameter |
| 48 | Custom crypto implementation? | Text+Struct | Hand-rolled encrypt/decrypt not using standard library |
| 49 | Predictable random for security? | Text+Struct | `math/rand`, `Math.random()`, `random.random()` in auth/token/key context |
| 50 | Deprecated TLS version? | Text | TLS 1.0, TLS 1.1, SSL 3.0 configuration |
| 51 | Weak cipher suite? | Text | RC4, DES, 3DES, NULL cipher in TLS config |
| 52 | Missing certificate validation? | Text+Struct | Custom TLS config that skips cert chain verification |
| 53 | Plaintext storage of sensitive data? | Structural | Writing password/SSN/credit card to DB/file without encryption |
| 54 | Insecure key derivation? | Text | Direct hash as key instead of PBKDF2/scrypt/argon2 |

### Category 5: Path Traversal (bits 55-66)

| Bit | Question | Engine | What it looks for |
|-----|----------|--------|-------------------|
| 55 | Path from user input unsanitized? | Structural | File open/read/write with request parameter in path without sanitization |
| 56 | Directory traversal possible? | Structural | Path join with user input not checked for `..` |
| 57 | Symlink following? | Text+Struct | File operations without `O_NOFOLLOW` or `Lstat` check |
| 58 | Archive extraction without path check? | Structural | Zip/tar extraction without validating entry paths (zip slip) |
| 59 | Unrestricted file upload path? | Structural | Uploaded file saved with original filename without sanitization |
| 60 | File include with user input? | Structural | Dynamic `require()`, `import()`, `include()` with variable path |
| 61 | SSRF via user-controlled URL? | Structural | HTTP request where URL comes from user input without whitelist |
| 62 | File deletion with user path? | Structural | `os.Remove`/`unlink` with user-supplied path |
| 63 | Temp file in predictable location? | Text+Struct | `mktemp` alternative or hardcoded `/tmp/` path for sensitive data |
| 64 | World-readable file permissions? | Text | `0777`, `0666`, `os.ModePerm` on file creation |
| 65 | Path join without clean? | Structural | `filepath.Join` or `os.path.join` with user input without `filepath.Clean` |
| 66 | Unrestricted glob/walk scope? | Structural | `filepath.Walk`/`glob` rooted at user-supplied directory |

---

## Worked Example

### Input: A Go HTTP handler with multiple security issues

```go
func handleTransfer(w http.ResponseWriter, r *http.Request) {       // line 1
    userID := r.URL.Query().Get("user_id")                          // line 2
    amount := r.FormValue("amount")                                 // line 3
    targetAcct := r.FormValue("account")                            // line 4
    password := "admin123"                                          // line 5
    filePath := "/data/exports/" + r.FormValue("filename")          // line 6
    query := "SELECT balance FROM accounts WHERE id='" + userID + "'" // line 7
    rows, _ := db.Query(query)                                      // line 8
    var balance float64                                              // line 9
    rows.Scan(&balance)                                              // line 10
    hash := md5.Sum([]byte(password))                                // line 11
    log.Printf("Transfer by user: %s amount: %s", userID, amount)   // line 12
    os.WriteFile(filePath, []byte(amount), 0777)                     // line 13
    redirect := r.FormValue("next")                                  // line 14
    http.Redirect(w, r, redirect, 302)                               // line 15
}                                                                    // line 16
```

### Tree-sitter AST (simplified, language-unified)

```
function_declaration: handleTransfer
  parameters: (w http.ResponseWriter, r *http.Request)
  body:
    L2:  call_expression: r.URL.Query().Get("user_id")        → assignment: userID
    L3:  call_expression: r.FormValue("amount")                → assignment: amount
    L4:  call_expression: r.FormValue("account")               → assignment: targetAcct
    L5:  short_var_declaration: password = "admin123"           → string_literal
    L6:  binary_expression: "/data/exports/" + r.FormValue()   → assignment: filePath
    L7:  binary_expression: "SELECT..." + userID + "'"         → assignment: query
    L8:  call_expression: db.Query(query)                      → assignment: rows
    L9:  var_declaration: balance float64
    L10: call_expression: rows.Scan(&balance)
    L11: call_expression: md5.Sum([]byte(password))            → assignment: hash
    L12: call_expression: log.Printf(format, userID, amount)
    L13: call_expression: os.WriteFile(filePath, data, 0777)
    L14: call_expression: r.FormValue("next")                  → assignment: redirect
    L15: call_expression: http.Redirect(w, r, redirect, 302)
```

### Per-Line Bitmask Analysis

Each line is evaluated against all 67 questions. Below, only triggered bits (1s) are shown; all others are 0.

```
         Injection (0-15)      Secrets (16-28)    Auth (29-42)     Crypto (43-54)   Path (55-66)
Bit:     0000000000011111  1111111111111  11111111111111  111111111111  111111111111
Pos:     0123456789012345  6789012345678  9012345678901 2  3456789012 34  5678901234 56

Line 1:  0000000000000000  0000000000000  0000000000000 0  0000000000 00  0000000000 00
Line 2:  0000000000000000  0000000000000  0000010000000 0  0000000000 00  0000000000 00
Line 3:  0000000000000000  0000000000000  0000010000000 0  0000000000 00  0000000000 00
Line 4:  0000000000000000  0000000000000  0000010000000 0  0000000000 00  0000000000 00
Line 5:  0000000000000000  0100000000000  0000000000000 0  0000000000 00  0000000000 00
Line 6:  0000000000000000  0000000000000  0000000000000 0  0000000000 00  1100000000 00
Line 7:  1000000000000000  0000000000000  0000000000000 0  0000000000 00  0000000000 00
Line 8:  1010000000000000  0000000000000  0000000000000 0  0000000000 00  0000000000 00
Line 9:  0000000000000000  0000000000000  0000000000000 0  0000000000 00  0000000000 00
Line 10: 0000000000000000  0000000000000  0000000000000 0  0000000000 00  0000000000 00
Line 11: 0000000000000000  0000000000000  0000000000000 0  1000000000 00  0000000000 00
Line 12: 0000000000010000  0000000000000  0000000000000 0  0000000000 00  0000000000 00
Line 13: 0000000000000000  0000000000000  0000000000000 0  0000000000 00  0000000001 00
Line 14: 0000000000000000  0000000000000  0000000000000 0  0000000000 00  0000000000 00
Line 15: 0000000000000000  0000000000000  0000000000001 0  0000000000 00  0000000000 00
Line 16: 0000000000000000  0000000000000  0000000000000 0  0000000000 00  0000000000 00
```

### Detailed Bit Triggers Per Line

| Line | Bits Set | Questions Answered YES |
|------|----------|----------------------|
| 1 | *(none)* | Function signature — no security signal |
| 2 | 35 | **Missing input validation** — `r.URL.Query().Get()` consumed without validation |
| 3 | 35 | **Missing input validation** — `r.FormValue()` consumed without validation |
| 4 | 35 | **Missing input validation** — `r.FormValue()` consumed without validation |
| 5 | 17 | **Hardcoded password** — string literal assigned to `password` variable |
| 6 | 55, 56 | **Path from user input unsanitized** — `r.FormValue()` concatenated into file path; **Directory traversal possible** — no `..` check |
| 7 | 0 | **SQL string concatenation** — `"SELECT...'" + userID + "'"` builds SQL via concat |
| 8 | 0, 2 | **SQL string concatenation** — `query` var carries tainted SQL; **Raw SQL without parameterization** — `db.Query(query)` with non-literal string |
| 9 | *(none)* | Variable declaration — no signal |
| 10 | *(none)* | Scan call — no direct security signal |
| 11 | 43 | **Weak hash algorithm** — `md5.Sum()` used on credential material |
| 12 | 11 | **Log injection** — user-supplied `userID` written to log without sanitization |
| 13 | 64 | **World-readable file permissions** — `0777` on `os.WriteFile` |
| 14 | *(none)* | Value capture — signal triggers on usage (line 15) |
| 15 | 42 | **Unvalidated redirect** — `http.Redirect` with user-supplied URL from `r.FormValue()` |
| 16 | *(none)* | Closing brace |

### Method-Level Rollup

Merge all line bitmasks with bitwise OR:

```
Method bitmask (67 bits):
  Injection:  1 0 1 0 0 0 0 0 0 0 0 1 0 0 0 0   → 3 of 16 bits set
  Secrets:    0 1 0 0 0 0 0 0 0 0 0 0 0           → 1 of 13 bits set
  Auth Gaps:  0 0 0 0 0 0 1 0 0 0 0 0 0 1         → 2 of 14 bits set
  Crypto:     1 0 0 0 0 0 0 0 0 0 0 0              → 1 of 12 bits set
  Path:       1 1 0 0 0 0 0 0 0 1 0 0              → 3 of 12 bits set

Total: 10 of 67 bits set
```

### Severity Scoring

Questions carry weights (critical=3, high=2, medium=1):

| Bit | Finding | Weight |
|-----|---------|--------|
| 0 | SQL concatenation | critical (3) |
| 2 | Raw SQL no params | critical (3) |
| 11 | Log injection | medium (1) |
| 17 | Hardcoded password | critical (3) |
| 35 | Missing input validation | high (2) |
| 42 | Unvalidated redirect | high (2) |
| 43 | Weak hash (MD5) | high (2) |
| 55 | Path from user input | critical (3) |
| 56 | Directory traversal | critical (3) |
| 64 | World-readable perms | medium (1) |

**Raw score:** 3+3+1+3+2+2+2+3+3+1 = **23**
**Max possible:** 67 questions × avg weight ~2.0 = ~134
**Normalized severity:** 23/134 = **0.17** (17% of max)

But density matters more than percentage of max. This method has **10 distinct security findings including 5 criticals**. The weighted score is what surfaces in search results and the Recon tab:

```
handleTransfer()  S: -23  [5 crit, 2 high, 2 med]
```

Negative score = security debt. The higher the magnitude, the more attention needed.

---

## Cross-Language Uniformity

### The core problem

Every language has different syntax, different tree-sitter node names, different idioms. SQL injection in Go looks different from SQL injection in Python looks different from SQL injection in Java. How do we ask 67 questions once and have them work everywhere?

### What tree-sitter actually gives us

Tree-sitter parses source into a **concrete syntax tree** (CST). Every node has:
- A **kind** string (e.g., `call_expression`, `binary_expression`, `string_literal`)
- **Children** (ordered, named or anonymous)
- **Source text** (the exact bytes from the file)
- **Position** (row, column, byte offset)

The node kind names differ per grammar, but they describe the **same structural concepts**. Here's the real mapping from aOa's 28 compiled grammars:

#### Symbol-level nodes (what we already extract today)

| Concept | Go | Python | JavaScript | Rust | Java | C/C++ | C# | Ruby | PHP |
|---------|-----|--------|------------|------|------|-------|-----|------|-----|
| Function | `function_declaration` | `function_definition` | `function_declaration` | `function_item` | — | `function_definition` | — | — | `function_definition` |
| Method | `method_declaration` | *(in class body)* | `method_definition` | *(in impl)* | `method_declaration` | *(in class)* | `method_declaration` | `method` | `method_declaration` |
| Class | — | `class_definition` | `class_declaration` | — | `class_declaration` | `class_specifier` | `class_declaration` | `class` | `class_declaration` |
| Struct | `type_declaration` | — | — | `struct_item` | — | `struct_specifier` | `struct_declaration` | — | — |
| Interface | — | — | `interface_declaration` | `trait_item` | `interface_declaration` | — | `interface_declaration` | `module` | `interface_declaration` |

#### Detection-level nodes (what bitmask analysis needs)

These are the nodes the walker would use to answer questions. They're remarkably consistent:

| Concept | Go | Python | JavaScript | Rust | Java | C | Ruby | PHP |
|---------|-----|--------|------------|------|------|---|------|-----|
| **Function call** | `call_expression` | `call` | `call_expression` | `call_expression` | `method_invocation` | `call_expression` | `call` | `function_call_expression` |
| **String literal** | `interpreted_string_literal` / `raw_string_literal` | `string` / `concatenated_string` | `string` / `template_string` | `string_literal` / `raw_string_literal` | `string_literal` | `string_literal` | `string` | `string` / `encapsed_string` |
| **String concat** | `binary_expression` (+) | `binary_operator` (+) / `concatenated_string` | `binary_expression` (+) / `template_string` | No + for strings (macro) | `binary_expression` (+) | — | `binary` (+) | `binary_expression` (.) |
| **Assignment** | `short_var_declaration` / `assignment_statement` | `assignment` | `variable_declarator` / `assignment_expression` | `let_declaration` | `local_variable_declaration` | `declaration` / `assignment_expression` | `assignment` | `assignment_expression` |
| **If statement** | `if_statement` | `if_statement` | `if_statement` | `if_expression` | `if_statement` | `if_statement` | `if` | `if_statement` |
| **For/loop** | `for_statement` | `for_statement` / `while_statement` | `for_statement` / `while_statement` | `for_expression` / `loop_expression` | `for_statement` / `while_statement` | `for_statement` / `while_statement` | `for` / `while` | `for_statement` / `while_statement` |
| **Return** | `return_statement` | `return_statement` | `return_statement` | `return_expression` | `return_statement` | `return_statement` | `return` | `return_statement` |
| **Import** | `import_declaration` | `import_statement` | `import_statement` | `use_declaration` | `import_declaration` | `#include` (preproc) | `require` (call) | `use_declaration` |
| **Comment** | `comment` | `comment` | `comment` | `line_comment` / `block_comment` | `line_comment` / `block_comment` | `comment` | `comment` | `comment` |
| **Integer literal** | `int_literal` | `integer` | `number` | `integer_literal` | `decimal_integer_literal` | `number_literal` | `integer` | `integer` |

### The lang_map: thin normalization layer

The pattern definitions use **unified concept names**. The `lang_map` translates them to actual node kinds per grammar:

```yaml
# Example lang_map for the "call" concept
call:
  go: call_expression
  python: call
  javascript: call_expression
  rust: call_expression
  java: method_invocation
  c: call_expression
  ruby: call
  php: function_call_expression
  kotlin: call_expression
  csharp: invocation_expression
```

A detection pattern like Bit 0 (SQL concat) references `call` and `string_concat` — the lang_map resolves those to the right node kinds at scan time. The pattern is written once. The map bridges the gap.

### The three detection layers

Each question in the bitmask is answered by one of three detection methods. Here's what each layer can see and the core patterns/regexes it uses:

#### Layer 1: Structural patterns (AST walker)

The walker traverses the tree and matches **shapes** — parent/child/sibling relationships between node kinds.

**Core structural pattern types (reusable across all questions):**

| Pattern | What it matches | Used by |
|---------|----------------|---------|
| `call_with_arg(receiver_contains, arg_type)` | A function call where receiver text matches a set of names and an argument has a specific node type | Bit 0 (SQL concat→query), Bit 3 (cmd concat→exec), Bit 13 (eval with variable) |
| `call_inside_loop(callee_contains)` | A call expression nested inside a for/while/loop body | Performance: N+1, string concat in loop |
| `assignment_with_literal(name_pattern, value_type)` | Assignment where LHS matches a name pattern and RHS is a specific literal type | Bit 17 (password = "..."), Bit 23 (key = literal) |
| `call_without_sibling(callee, expected_sibling)` | A call that should have a companion call nearby but doesn't | Bit 47 (hash without salt), Resource leak (open without close) |
| `nesting_depth(node_type, threshold)` | Count how deep a specific node type nests | Quality: deep nesting, god function |
| `branch_count(scope, threshold)` | Count if/switch/for nodes in a function body | Quality: cyclomatic complexity |
| `literal_in_context(literal_pattern, parent_kinds)` | A literal (string, number) appearing inside a specific structural context | Bit 45 (static IV in cipher init), Bit 64 (0777 in file create) |
| `return_without_wrap(error_var)` | Return statement with bare error variable (Go-specific) | Observability: missing error context |

These ~8-10 core structural pattern types combine to cover the majority of the 67 security questions and extend cleanly to other dimensions. The walker doesn't need a new algorithm per question — it needs parameterized pattern templates.

#### Layer 2: Text patterns (Aho-Corasick automaton)

AC runs once per file, scanning raw source bytes for **literal string matches** simultaneously. One pass, all patterns at once, O(n) in file length.

**Core AC pattern sets:**

| Category | Patterns | Count | Example matches |
|----------|----------|-------|-----------------|
| Dangerous function names | `eval(`, `exec(`, `system(`, `popen(`, `pickle.loads(`, `yaml.load(`, `Function(` | ~30 | Bit 13, 14 |
| Weak crypto identifiers | `MD5`, `SHA1`, `DES`, `RC4`, `ECB`, `SSLv3`, `TLSv1.0` | ~15 | Bit 43, 44, 50, 51 |
| Secret variable names | `password`, `passwd`, `secret`, `api_key`, `apikey`, `token`, `bearer`, `client_secret` | ~20 | Bit 16-18, 27, 28 |
| Credential patterns | `BEGIN RSA PRIVATE KEY`, `BEGIN EC PRIVATE KEY`, `AKIA` | ~10 | Bit 19, 20 |
| Insecure config | `InsecureSkipVerify`, `verify=False`, `NODE_TLS_REJECT_UNAUTHORIZED`, `AllowAllOrigins` | ~15 | Bit 25, 40 |
| Debug/unsafe functions | `fmt.Println`, `console.log`, `print(`, `System.out.println`, `var_dump` | ~15 | Observability: leftover debug |
| Security headers | `X-Frame-Options`, `Content-Security-Policy`, `Strict-Transport-Security` | ~10 | Bit 41 |
| **Total AC patterns** | | **~115** | |

These ~115 literal patterns all fire in a **single pass** through the AC automaton. Adding a new pattern is adding a string to the dictionary — zero performance cost (AC is O(n+m+z) regardless of pattern count).

#### Layer 3: Regex patterns (targeted, post-AC)

Regexes run only when AC triggers a candidate or when the structural walker identifies a node worth deeper inspection. They are NOT run on every line — they're **second-stage confirmation**.

**Core regex patterns:**

| Purpose | Regex | Used by |
|---------|-------|---------|
| AWS access key | `AKIA[0-9A-Z]{16}` | Bit 20 |
| Connection string with creds | `://[^:]+:[^@]+@` | Bit 21 |
| Hardcoded IP/port | `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?\b` | Architecture: hardcoded config |
| PII patterns (email) | `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}` | Compliance: PII in logs |
| PII patterns (SSN) | `\b\d{3}-\d{2}-\d{4}\b` | Compliance: PII in logs |
| File permission octal | `0[0-7]{3}` in file create context | Bit 64 |
| High entropy string | String literal with Shannon entropy > 4.5 and length > 16 | Bit 16 (potential API key) |
| URL in non-config file | `https?://[^\s"']+` outside config/env context | Architecture: hardcoded URLs |

Regex count is small (~15-20) because most text detection is handled by AC. Regexes are reserved for patterns that need character-class matching or quantifiers.

### How it scales: the same question across 5 languages

Take **Bit 0: SQL string concatenation** — the most classic security question. Here's what it looks like in each language and how the **same pattern definition** catches all of them:

**Go:**
```go
query := "SELECT * FROM users WHERE id=" + userID
rows, _ := db.Query(query)
```
AST: `binary_expression` containing `interpreted_string_literal` with "SELECT" → assigned to var → var passed to `call_expression` with receiver text containing "Query"

**Python:**
```python
query = "SELECT * FROM users WHERE id=" + user_id
cursor.execute(query)
```
AST: `binary_operator` containing `string` with "SELECT" → assigned to var → var passed to `call` with receiver text containing "execute"

**JavaScript:**
```javascript
const query = "SELECT * FROM users WHERE id=" + userId;
db.query(query);
```
AST: `binary_expression` containing `string` with "SELECT" → `variable_declarator` → var passed to `call_expression` with receiver text containing "query"

**Java:**
```java
String query = "SELECT * FROM users WHERE id=" + userId;
stmt.executeQuery(query);
```
AST: `binary_expression` containing `string_literal` with "SELECT" → `local_variable_declaration` → var passed to `method_invocation` with name containing "executeQuery"

**Rust:**
```rust
let query = format!("SELECT * FROM users WHERE id={}", user_id);
client.execute(&query, &[]).await?;
```
AST: `macro_invocation` ("format!") containing `string_literal` with "SELECT" + identifier → `let_declaration` → var passed to `call_expression` with receiver text containing "execute"

**One pattern definition:**
```yaml
- id: sql_string_concat
  bit: 0
  severity: critical
  dimension: security
  structural:
    match: call                            # unified concept → lang_map resolves
    receiver_contains: [query, execute, exec, prepare, raw, cursor]
    has_arg:
      # The argument (direct or via variable) traces back to:
      type: [string_concat, format_call, template_string]
      text_contains: ["SELECT", "INSERT", "UPDATE", "DELETE", "DROP"]
  lang_map:
    go: { call: call_expression, string_concat: binary_expression }
    python: { call: call, string_concat: binary_operator }
    javascript: { call: call_expression, string_concat: binary_expression }
    java: { call: method_invocation, string_concat: binary_expression }
    rust: { call: call_expression, format_call: macro_invocation }
```

The walker resolves `call` → the language-specific node kind, walks arguments, checks for the structural shape. **One definition, all languages.** The lang_map is a thin dictionary, not a rewrite of the logic.

### Is it simpler than we think?

Looking at the actual node kinds across tree-sitter's grammars, there's more uniformity than you'd expect:

- **12 of 28 languages** use `call_expression` for function calls
- **10 of 28** use `function_declaration` for top-level functions
- **9 of 28** use `binary_expression` for binary operations
- **14 of 28** use `if_statement` for conditionals
- **12 of 28** use `return_statement` for returns
- **All** use `comment` or `line_comment`/`block_comment`

The lang_map for the ~10 core structural concepts is a table with ~28 rows (one per language) and ~10 columns (one per concept). That's ~280 entries. Many are identical or near-identical. The actual mapping work is a **one-time data entry task**, not an engineering challenge.

**The harder part** is the ~15-20% of questions that are language-specific in their idiom:
- Go error handling (`_, err := f(); if err != nil`) doesn't exist in Python (try/except)
- Rust's ownership model means some patterns (unclosed resources) don't apply
- PHP's string concatenation uses `.` not `+`
- Ruby uses blocks (`do...end`) where other languages use callbacks

For these, the pattern definition includes a `skip_langs` or `lang_override` field. ~80% of questions work identically across all languages. ~20% need per-language variants. That's manageable.

### Bottom line

The cross-language scaling problem reduces to:
1. **~10 core structural pattern types** (parameterized templates, not per-question code)
2. **~115 AC literal patterns** (one scan pass, language-agnostic text matching)
3. **~15-20 regexes** (second-stage confirmation, mostly language-agnostic)
4. **A lang_map table** (~280 entries mapping unified concepts → per-grammar node kinds)
5. **~80% of questions** work identically across all languages; **~20%** need per-language overrides

It is simpler than it looks. The structural patterns are parameterized, not bespoke. The text patterns are language-agnostic by nature. The lang_map is a lookup table, not a translation engine. The question set scales because code structure is more universal than code syntax.

---

## Architecture Summary

```
Source file
    │
    ▼
Tree-sitter parse (already done at index time — zero re-parse cost)
    │
    ├──► Structural engine (AST walker)     ──► bits requiring code structure
    │                                              │
    ├──► Text engine (AC automaton + regex)  ──► bits requiring pattern match
    │                                              │
    ▼                                              ▼
Per-line bitmask (67 bits for security, ~300+ across all 6 tiers)
    │
    ▼
Method-level rollup (bitwise OR of all lines in method)
    │
    ▼
Weighted severity score (sum of triggered bit weights)
    │
    ▼
Stored in bbolt → available to search results + Recon tab
```

---

## What This Proves

1. **Deterministic** — Same code always produces same bitmask. No ML model variance.
2. **Interpretable** — Every set bit maps to a specific yes/no question. You can explain every finding.
3. **Fast** — AC + tree-sitter reuse = ~100ns/line for bitmask computation. A 1000-line file takes ~100μs.
4. **Cross-language** — Tree-sitter's 28 grammars + lang_map = one question set works everywhere.
5. **Extensible** — New question = new bit position + new YAML pattern. No engine changes.
6. **Composable** — Security is one dimension. Performance, Quality, Compliance, Architecture, Observability each get their own bitmask. Stack them for a full dimensional profile.

---

## Next Steps

- [ ] D-01: Formalize the YAML schema for structural + text detection definitions
- [ ] D-02: Build the tree-sitter AST walker engine
- [ ] D-03: Build the AC text scanner engine
- [ ] D-05: Implement bitmask composer + weighted scoring
- [ ] D-06: Define all 67 security questions as YAML patterns
- [ ] Validate against known-vulnerable test projects
