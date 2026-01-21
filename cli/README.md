# aOa CLI - Build Required

**CRITICAL: The `cli/aoa` file is GENERATED. Do not edit it directly.**

## Source Files

All CLI code lives in `cli/src/*.sh`. The numbered prefixes ensure correct concatenation order:

```
src/
  00-header.sh      # Shebang, set -e
  01-constants.sh   # AOA_URL, colors, paths
  02-utils.sh       # Helper functions
  ...
  90-main.sh        # Entry point, command dispatch
```

## Building

After ANY change to files in `cli/src/`, you MUST rebuild:

```bash
./cli/build.sh
```

This concatenates all source files into `cli/aoa`.

## Verification

Test the build works:

```bash
./cli/build.sh test
```

## Common Mistake

If `aoa grep` returns 0 results but `curl` to the API works, **you forgot to rebuild**.

The `cli/aoa` executable won't reflect your source changes until you run `build.sh`.
