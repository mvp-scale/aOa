---
name: aoa
description: Fast O(1) code search. Use `aoa grep` instead of Grep/Glob - same syntax, 100x faster.
allowed-tools: Bash
---

# aOa Search

Use `aoa grep` and `aoa egrep` just like Unix grep. Same flags, instant results.

```bash
aoa grep handleAuth         # Find code
aoa egrep "TODO|FIXME"      # Regex search
aoa find "*.py"             # Find files
```

For help on any command:
```bash
aoa help                    # Full command list
aoa grep --help             # Grep options
aoa egrep --help            # Egrep options
```

That's it. The tool documents itself.
