corey@corey:~/production/scout-v1/cloudflare/durable-objects/coordinator$ aoa
                              aOa
                       5 angles. 1 attack.

  O(1) search + pattern learning = Claude reads less, moves faster.

GETTING STARTED
  ./install.sh           Install aOa globally (once)
  aoa init               Enable aOa in current project
  aoa remove             Disable aOa in current project
  aoa projects           List all enabled projects

SEARCH (Unix parity)
  grep <term>            O(1) symbol lookup (<5ms, full codebase)
  grep "a b c"           Multi-term OR search (<10ms, ranked)
  grep -a t1,t2          Multi-term AND search (<10ms, all terms)
  grep -i <term>         Case insensitive search
  egrep <regex>          Regex search (~20ms, working set only)

FILE DISCOVERY (Unix parity)
  find <pattern>         Find files by glob (<10ms)
  find -type py          Find files by language
  tree [dir]             Directory tree (<50ms)
  locate <name>          Fast filename search (<5ms)
  head <file> [n]        Show first n lines (default: 20)
  tail <file> [n]        Show last n lines (default: 20)
  lines <file> M-N       Show specific line range

BEHAVIORAL (aOa unique)
  touched [since]        Files touched in session/time period
  focus                  Current working set from memory

TIME-BASED
  changes [time]         Recent file changes (e.g., 5m, 1h)
  files [pattern]        List indexed files

OUTLINE ANGLE (code structure + semantic tags)
  outline <file>         Code structure (functions, classes, methods)
  outline --pending      Check tagging status (pending/tagged)
  outline --enrich-all   Show files needing tags (detailed)

  To add semantic tags: In Claude Code, say "tag the codebase"
  Then search: aoa grep "#authentication"

INTENT ANGLE (behavioral tracking)
  intent recent [since]  Recent intent records (e.g., 1h, 30m)
  intent tags            All tags with file counts
  intent files <tag>     Files associated with an intent tag
  intent file <path>     Tags associated with a file
  intent stats           Intent index statistics

SESSION
  history [limit]        Recent events
  reset [session|weekly] Reset counters

INTEL ANGLE (external reference)
  repo list              List intel sources
  repo add <name> <url>  Clone and index a git repo
  repo remove <name>     Remove an intel source
  repo <name> search <t> Search in a specific repo

SYSTEM
  start                  Start aOa Docker services
  stop                   Stop aOa Docker services
  health                 Check all angles
  info                   Show indexing config, mounts, registered projects
  metrics                Prediction accuracy and savings
  baseline               Subagent baseline costs and potential savings
  services               Visual service map with live status

EXAMPLES
  # First time setup
  ./install.sh                  # Install globally (once)
  cd ~/my-project && aoa init   # Enable for project

  # Search your project
  aoa grep handleAuth           # Symbol search
  aoa grep "auth token"         # OR search
  aoa grep -a auth,session      # AND search
  aoa egrep "TODO|FIXME"        # Regex search

  # Add reference repos
  aoa repo add flask https://github.com/pallets/flask
  aoa repo flask search Blueprint

ARCHITECTURE
  ~/.aoa/                Global installation
  .claude/hooks/         Per-project hooks (created by aoa init)

  Install once → enable per-project → search anywhere

ALIASES
  search, s    → grep     (deprecated)
  multi, m     → grep -a  (deprecated)
  pattern, p   → egrep    (deprecated)
