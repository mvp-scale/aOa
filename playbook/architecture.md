# Architecture — aoa

*Generated from live source by aOa on 2026-06-10T05:34:27Z. 25 internal packages, 42 dependency edges, 542 third-party modules. Source: AST package index + `go list`. This document regenerates from code; it does not drift.*

## 1. System Overview

`aoa` is a Go application organised as a hexagonal (ports & adapters) architecture across 7 layers: cmd, app, adapters, domain, ports, atlas, other. The contracts hub is **`ports`** (fan-in 12 — most depended-on), and the composition root is **`app`** (fan-out 14 — wires everything together).

## 2. Building Block View — Layer Dependencies (C4 Container level)

```mermaid
flowchart TD
  L_cmd["cmd<br/><small>3 pkg</small>"]
  L_app["app<br/><small>1 pkg</small>"]
  L_adapters["adapters<br/><small>9 pkg</small>"]
  L_domain["domain<br/><small>6 pkg</small>"]
  L_ports["ports<br/><small>1 pkg</small>"]
  L_atlas["atlas<br/><small>1 pkg</small>"]
  L_other["other<br/><small>4 pkg</small>"]
  L_adapters -->|6| L_ports
  L_app -->|6| L_domain
  L_app -->|5| L_adapters
  L_domain -->|4| L_ports
  L_cmd -->|4| L_adapters
  L_cmd -->|3| L_other
  L_cmd -->|2| L_domain
  L_adapters -->|1| L_other
  L_adapters -->|1| L_domain
  L_app -->|1| L_atlas
  L_app -->|1| L_ports
  L_app -->|1| L_other
  L_domain -->|1| L_other
  L_cmd -->|1| L_app
  L_cmd -->|1| L_ports
  classDef ports fill:#3a1d1d,stroke:#e5484d,color:#fff
  classDef domain fill:#16291d,stroke:#30a46c,color:#fff
  class L_ports ports
  class L_domain domain
```

Edge labels = number of package dependencies crossing between layers.

## 3. Component View — Package Dependencies (C4 Component level)

```mermaid
flowchart LR
  subgraph CMD
    n_cmd_aoa_cmd["cmd/aoa/cmd"]
    n_cmd_aoa["cmd/aoa"]
    n_cmd_dim_dump["cmd/dim-dump"]
  end
  subgraph APP
    n_internal_app["app"]
  end
  subgraph ADAPTERS
    n_internal_adapters_bbolt["adapters/bbolt"]
    n_internal_adapters_socket["adapters/socket"]
    n_internal_adapters_claude["adapters/claude"]
    n_internal_adapters_fsnotify["adapters/fsnotify"]
    n_internal_adapters_tailer["adapters/tailer"]
    n_internal_adapters_treesitter["adapters/treesitter"]
    n_internal_adapters_web["adapters/web"]
    n_internal_adapters_ahocorasick["adapters/ahocorasick"]
    n_internal_adapters_recon["adapters/recon"]
  end
  subgraph DOMAIN
    n_internal_domain_analyzer["domain/analyzer"]
    n_internal_domain_enricher["domain/enricher"]
    n_internal_domain_status["domain/status"]
    n_internal_domain_hints["domain/hints"]
    n_internal_domain_index["domain/index"]
    n_internal_domain_learner["domain/learner"]
  end
  subgraph PORTS
    n_internal_ports["ports"]
  end
  subgraph ATLAS
    n_atlas["atlas"]
  end
  subgraph OTHER
    n_internal_peek["peek"]
    n_internal_version["version"]
    n_hooks["hooks"]
    n_recon["recon"]
  end
  n_internal_adapters_bbolt --> n_internal_ports
  n_internal_adapters_claude --> n_internal_adapters_tailer
  n_internal_adapters_claude --> n_internal_ports
  n_internal_adapters_recon --> n_internal_ports
  n_internal_adapters_socket --> n_internal_peek
  n_internal_adapters_socket --> n_internal_ports
  n_internal_adapters_treesitter --> n_internal_domain_analyzer
  n_internal_adapters_treesitter --> n_internal_ports
  n_internal_adapters_web --> n_internal_adapters_socket
  n_internal_adapters_web --> n_internal_ports
  n_internal_app --> n_atlas
  n_internal_app --> n_internal_adapters_bbolt
  n_internal_app --> n_internal_adapters_claude
  n_internal_app --> n_internal_adapters_fsnotify
  n_internal_app --> n_internal_adapters_socket
  n_internal_app --> n_internal_adapters_web
  n_internal_app --> n_internal_domain_analyzer
  n_internal_app --> n_internal_domain_enricher
  n_internal_app --> n_internal_domain_hints
  n_internal_app --> n_internal_domain_index
  n_internal_app --> n_internal_domain_learner
  n_internal_app --> n_internal_domain_status
  n_internal_app --> n_internal_ports
  n_internal_app --> n_internal_version
  n_internal_domain_analyzer --> n_internal_ports
  n_internal_domain_hints --> n_internal_domain_enricher
  n_internal_domain_index -. violates .-> n_internal_peek
  n_internal_domain_index --> n_internal_ports
  n_internal_domain_learner --> n_internal_ports
  n_internal_domain_status --> n_internal_ports
  n_cmd_aoa --> n_cmd_aoa_cmd
  n_cmd_aoa_cmd --> n_hooks
  n_cmd_aoa_cmd --> n_internal_adapters_bbolt
  n_cmd_aoa_cmd --> n_internal_adapters_socket
  n_cmd_aoa_cmd --> n_internal_adapters_treesitter
  n_cmd_aoa_cmd --> n_internal_app
  n_cmd_aoa_cmd --> n_internal_domain_status
  n_cmd_aoa_cmd --> n_internal_ports
  n_cmd_aoa_cmd --> n_internal_version
  n_cmd_dim_dump --> n_internal_adapters_bbolt
  n_cmd_dim_dump --> n_internal_domain_analyzer
  n_cmd_dim_dump --> n_recon
  linkStyle 26 stroke:#e5484d,stroke-width:2px
```

## 4. Layering & Conformance Assessment

Intended dependency direction (hexagonal): `cmd → app → adapters → domain → ports`. A dependency pointing *up* this order is a layering violation.

**Findings (1 violation):**
- `domain/index` → `peek` (domain → other, points up the layers)

## 5. Dependency Inventory

| Package | Layer | Fan-in | Fan-out |
|---|---|---|---|
| `ports` | ports | 12 | 0 |
| `adapters/bbolt` | adapters | 3 | 1 |
| `adapters/socket` | adapters | 3 | 2 |
| `domain/analyzer` | domain | 3 | 1 |
| `domain/enricher` | domain | 2 | 0 |
| `domain/status` | domain | 2 | 1 |
| `peek` | other | 2 | 0 |
| `version` | other | 2 | 0 |
| `atlas` | atlas | 1 | 0 |
| `cmd/aoa/cmd` | cmd | 1 | 8 |
| `hooks` | other | 1 | 0 |
| `adapters/claude` | adapters | 1 | 2 |
| `adapters/fsnotify` | adapters | 1 | 0 |
| `adapters/tailer` | adapters | 1 | 0 |
| `adapters/treesitter` | adapters | 1 | 2 |
| `adapters/web` | adapters | 1 | 2 |
| `app` | app | 1 | 14 |
| `domain/hints` | domain | 1 | 1 |
| `domain/index` | domain | 1 | 2 |
| `domain/learner` | domain | 1 | 1 |
| `recon` | other | 1 | 0 |
| `cmd/aoa` | cmd | 0 | 1 |
| `cmd/dim-dump` | cmd | 0 | 3 |
| `adapters/ahocorasick` | adapters | 0 | 0 |
| `adapters/recon` | adapters | 0 | 1 |

## 6. Provenance

Every element above is derived from the codebase: package nodes and edges from `go list`, layer assignment from path, third-party inventory (see `sbom.cdx.json`, CycloneDX 1.6) from the module graph. No hand-authoring; regenerate on commit.
