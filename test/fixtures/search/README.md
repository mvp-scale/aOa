# Search Test Fixtures (F-04)

Behavioral parity fixtures for the aOa-go search engine.
Every query defines deterministic expected results against a mock codebase index.
The Go search engine must produce identical results for every fixture.

## Files

| File | Purpose |
|------|---------|
| `index-state.json` | Mock codebase: 13 files, 68 symbols, 8 domains |
| `queries.json` | 26 search queries with expected results |
| `README.md` | This file |

## Mock Codebase Structure

```
services/auth/handler.py          # AuthHandler: login, logout, register, refresh_token
services/auth/middleware.py        # AuthMiddleware: authenticate, validate_token, get_user_from_token
services/api/router.py             # Router: setup_routes, handle_login/register/users/health, log_request
services/api/serializer.py         # UserSerializer, ResponseFormatter: serialize, deserialize, format_*
services/database/models.py        # ORM models: User, Session, Role, Token, AuditLog
services/database/queries.py       # UserQueries, SessionQueries: CRUD + invalidate + cleanup
services/cache/redis_client.py     # RedisCache: get, set, invalidate, invalidate_pattern, flush_all
lib/utils/logger.py                # Logger: setup_logger, log, format_message, get_log_level
lib/utils/config.py                # Config: load_config, get_env, validate_config, get_database_url
tests/test_auth.py                 # TestAuth: 5 test methods + setUp
tests/test_api.py                  # TestAPI: 5 test methods
cmd/server.py                      # create_app, configure_logging, run_server, health_check, shutdown_handler
docker/Dockerfile                  # EXPOSE, CMD directives
```

## Domains (8 total)

| Domain | Terms | Relevant Files |
|--------|-------|---------------|
| `@authentication` | login, session, token, register | handler.py, middleware.py |
| `@api` | handler, route, request, serialize | router.py, serializer.py |
| `@database` | query, model, user, create | models.py, queries.py |
| `@testing` | test, assert, mock, setup | test_auth.py, test_api.py |
| `@caching` | cache, redis, invalidate, ttl | redis_client.py |
| `@logging` | logger, level, format | logger.py |
| `@configuration` | config, env, load, validate | config.py |
| `@deployment` | docker, server, health, shutdown | server.py, Dockerfile |

## Query Coverage Matrix (26 queries)

### Basic Literal Search (5 queries)

| # | Query | Expected Count | Tests |
|---|-------|---------------|-------|
| Q01 | `login` | 5 results | Common term across multiple files |
| Q02 | `invalidate` | 3 results | Unique term, few matches |
| Q03 | `test` | 12 results | Broad term, stress test (G1) |
| Q04 | `xyznonexistent` | 0 results | Zero-result handling |
| Q05 | `a` | 0 results | Below min token length (2) |

### Multi-term OR Search (3 queries)

| # | Query | Expected Count | Tests |
|---|-------|---------------|-------|
| Q06 | `login session` | 10 results | Union, density ranking (G1) |
| Q07 | `auth token session` | 13 results | Broad union, stress test (G1) |
| Q08 | `login authenticate` | 6 results | Same-domain overlap |

### Multi-term AND Search (3 queries)

| # | Query | Flags | Expected Count | Tests |
|---|-------|-------|---------------|-------|
| Q09 | `login,handler` | `and_mode` | 0 results | No symbol has both tokens |
| Q10 | `validate,token` | `and_mode` | 1 result | Narrow intersection |
| Q11 | `login,expose` | `and_mode` | 0 results | Cross-domain empty intersection |

### Regex Search (3 queries)

| # | Query | Mode | Expected Count | Tests |
|---|-------|------|---------------|-------|
| Q12 | `handle.*login` | regex | 1 result | Regex concatenation |
| Q13 | `login\|logout` | regex | 7 results | Regex alternation |
| Q14 | `test_.*login` | regex | 3 results | Pattern for test functions |

### Flag Variations (4 queries)

| # | Query | Flags | Expected Count | Tests |
|---|-------|-------|---------------|-------|
| Q15 | `LOGIN` | `case_insensitive` | 5 results | -i flag (G2) |
| Q16 | `log` | `word_boundary` | 4 results | -w flag: exact token only |
| Q17 | `auth` | `count_only` | count=3 | -c flag: integer output |
| Q18 | `handler` | `include_glob=services/*` | 1 result | --include glob filter |

### Tokenization Edge Cases (4 queries)

| # | Query | Tokenizes To | Expected Count | Tests |
|---|-------|-------------|---------------|-------|
| Q19 | `getUserToken` | get, user, token | 20 results | CamelCase split (G1 stress) |
| Q20 | `app.post` | app, post | 3 results | Dot split |
| Q21 | `tree-sitter` | tree, sitter | 0 results | Hyphen split, no matches |
| Q22 | `resume` (with accents) | N/A | 0 results | Unicode graceful handling |

### Additional Edge Cases (4 queries)

| # | Query | Flags | Expected | Tests |
|---|-------|-------|----------|-------|
| Q23 | `test` | `max_count=3` | 3 results | -m flag truncation |
| Q24 | `config` | `quiet` | exit code 0 | -q with results |
| Q25 | `xyznothing` | `quiet` | exit code 1 | -q without results |
| Q26 | `create` | `exclude_glob=tests/*` | 3 results | --exclude glob filter |

## Tokenization Rules (from SPEC.md)

Split regex: `[/_\-.\\s]+` (slash, underscore, hyphen, dot, whitespace)

CamelCase split: `[A-Z]?[a-z]+|[A-Z]+(?=[A-Z][a-z]|\d|\W|$)|\d+`

- `app.post` -> `["app", "post"]`
- `getUserToken` -> `["get", "user", "token"]`
- `APIKey` -> `["api", "key"]`
- `tree-sitter` -> `["tree", "sitter"]`

Min token length: 2 characters. All tokens lowercased.

## Design Decisions

### Token Exclusion: "self"

Python `self` parameters are excluded from symbol token lists in the index.
Including `self` would add noise to every Python method (appears in ~50 of 68
symbols). The `self` parameter carries no semantic signal for search.

### Domain Assignment

Each search result is assigned the single best-matching domain based on its
token overlap with domain keywords. The `@domain` tag reflects the domain whose
terms contain the most matching tokens from the symbol.

Tags are the top 3 non-"self" tokens from the symbol, ordered by specificity
(rarest tokens first in the inverted index).

### AND Mode (flags.and_mode)

AND mode operates on per-symbol token intersection, not per-file. A symbol
must contain ALL query tokens in its own token list to appear in results.
This is stricter than per-file intersection and matches Python behavior.

### Regex Mode

Regex queries match against symbol names (the `name` field), not against
individual tokens. The pattern is applied to the full symbol name string.

### Result Ordering

- **Literal single-term**: By file path (alphabetical), then by line number
- **Literal multi-term OR**: By match density (symbols matching more query
  terms rank higher), then by file path, then by line number
- **Regex**: By file path, then by line number
- **AND mode**: By file path, then by line number

### Count and Quiet Modes

Q17 uses `expected_count` (integer) instead of `expected` (array).
Q24/Q25 use `expected_exit_code` (0 = found, 1 = not found) instead of `expected`.

The Go test harness should check the presence of these alternative fields
and validate accordingly.

## Go Struct Alignment

The fixture JSON maps to these Go structs in `test/parity_test.go`:

```go
type SearchFixture struct {
    Comment             string        `json:"_comment,omitempty"`
    Query               string        `json:"query"`
    Mode                string        `json:"mode"`
    Flags               SearchFlags   `json:"flags"`
    Expected            []SearchResult `json:"expected,omitempty"`
    ExpectedTokenization []string     `json:"expected_tokenization,omitempty"`
    ExpectedCount       *int          `json:"expected_count,omitempty"`
    ExpectedExitCode    *int          `json:"expected_exit_code,omitempty"`
}

type SearchFlags struct {
    AndMode          bool   `json:"and_mode,omitempty"`
    CaseInsensitive  bool   `json:"case_insensitive,omitempty"`
    WordBoundary     bool   `json:"word_boundary,omitempty"`
    CountOnly        bool   `json:"count_only,omitempty"`
    Quiet            bool   `json:"quiet,omitempty"`
    MaxCount         int    `json:"max_count,omitempty"`
    IncludeGlob      string `json:"include_glob,omitempty"`
    ExcludeGlob      string `json:"exclude_glob,omitempty"`
}

type SearchResult struct {
    File   string   `json:"file"`
    Line   int      `json:"line"`
    Symbol string   `json:"symbol"`
    Range  [2]int   `json:"range"`
    Domain string   `json:"domain"`
    Tags   []string `json:"tags"`
}
```

## Goal Alignment

- **G1 (Performance)**: Q03 (12 results), Q07 (13 results), Q19 (20 results) stress-test ranking
- **G2 (Grep Parity)**: Q15-Q18, Q23-Q26 cover all grep flag variations
- **G5 (Cohesive Architecture)**: Fixture format maps directly to Go structs, loadable by parity_test.go
