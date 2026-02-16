# aOa Test-Driven Development

> Tests fail first. Implementation makes them pass. Zero tolerance for behavioral divergence.

---

## Test Structure

```
test/
├─ parity_test.go          # Behavioral parity tests (Go vs Python)
├─ fixtures/               # Test data captured from Python
│  ├─ search/              # Search query → expected results
│  ├─ learner/             # Learner state snapshots
│  └─ observe/             # Signal processing tests
└─ integration/            # End-to-end flow tests
```

---

## Running Tests

```bash
# All tests (will skip unimplemented)
go test ./test/...

# Specific test
go test ./test -run TestSearchParity

# With verbose output
go test ./test/... -v

# With benchmarks
go test ./test/... -bench=.
```

---

## Test Philosophy

**1. Fixtures First**
- Capture actual Python behavior as test fixtures
- Fixtures become acceptance criteria
- Implementation satisfies fixtures, not assumptions

**2. Zero Tolerance**
- Search results must match Python exactly (diff = 0)
- Learner state must match numerically (zero float divergence)
- Output format must be identical (byte-for-byte)

**3. Test-Driven Flow**
1. Write test with fixture
2. Test fails (not implemented)
3. Implement minimum code to pass test
4. Test passes → move to next test

---

## Fixture Creation

**Search fixtures** (`fixtures/search/queries.json`):
```bash
# Run in Python aOa
aoa grep login --json > /tmp/go-fixtures-login.json

# Extract to fixture format
# (manual or scripted conversion to expected schema)
```

**Learner fixtures** (`fixtures/learner/*.json`):
```bash
# Snapshot Redis state at specific points
# Requires running Python aOa and extracting state
# See: CURRENT-ARCHITECTURE.md for all Redis keys
```

**TODO:** Create fixture extraction scripts for both search and learner.

---

## Current Status

**Tests implemented:**
- `TestSearchParity` — validates search results match Python
- `TestAutotuneParity` — validates autotune produces identical state

**Tests skipped:** All (nothing implemented yet)

**Next:** Implement Index domain, tests will start passing.
