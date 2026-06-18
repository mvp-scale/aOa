#!/usr/bin/env bash
#
# Generative session-conformance harness (L20.2-gen).
#
# Drives `claude -p` to emit controlled sessions for the CURRENTLY INSTALLED
# Claude Code version, then validates the resulting JSONL against the contract
# in ../claude-session/. This converts alignment from passive observation
# ("wait for an organic session to emit the surface") into a repeatable
# experiment ("generate the surface on demand, assert what aOa would extract").
#
# It is the new-version onboarding test: install a new Claude Code, run this,
# and get a per-surface pass/fail in minutes — from a fresh session, by design.
#
# Usage:
#   ./run.sh                 # auto scenarios, low-cost only
#   ./run.sh --all           # include high-cost (subagent) scenarios
#   ./run.sh --scenario bash-tool
#   ./run.sh --keep          # keep scratch sessions for inspection
#   AOA_CONF_MODEL=claude-opus-4-8 ./run.sh   # pin a model
#
# Requires: claude (in PATH), jq, go. Scenarios run in /tmp sandboxes with
# bypassPermissions — keep prompts trivial and self-contained.
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
CATALOG="$HERE/scenarios.json"
WORK="${AOA_CONF_WORK:-/tmp/aoa-conf}"
MODEL="${AOA_CONF_MODEL:-}"
INCLUDE_HIGH=0
KEEP=0
ONLY=""

while [ $# -gt 0 ]; do
  case "$1" in
    --all) INCLUDE_HIGH=1 ;;
    --keep) KEEP=1 ;;
    --scenario) ONLY="$2"; shift ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
  shift
done

command -v claude >/dev/null || { echo "FATAL: claude not in PATH"; exit 2; }
command -v jq >/dev/null || { echo "FATAL: jq not in PATH"; exit 2; }

INSTALLED_VER="$(claude --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)"
BASELINE_VER="$(ls -d "$REPO"/compliance/claude-session/versions/v*-observed 2>/dev/null \
  | sed -E 's#.*/v(.*)-observed#\1#' | sort -t. -k1,1n -k2,2n -k3,3n | tail -1)"

echo "═══════════════════════════════════════════════════════════════════"
echo " aOa generative session-conformance"
echo "   installed Claude Code: v${INSTALLED_VER:-unknown}"
echo "   contract baseline:     v${BASELINE_VER:-none}"
echo "   model:                 ${MODEL:-<default>}"
echo "═══════════════════════════════════════════════════════════════════"

mkdir -p "$WORK"
declare -A RESULT   # scenario -> PASS|FAIL|WARN|SKIP
declare -A NOTE     # scenario -> detail
SEEN_TYPES_FILE="$WORK/.seen_types"; : > "$SEEN_TYPES_FILE"

# project dir for a given scratch cwd
projdir() { echo "$HOME/.claude/projects/$(echo "$1" | sed 's#/#-#g')"; }

run_scenario() {
  local id="$1" trigger="$2" cost="$3" pmode="$4" prompt="$5" checks="$6"

  if [ "$trigger" = "manual" ]; then
    RESULT[$id]=SKIP; NOTE[$id]="manual — see README"; return
  fi
  if [ "$cost" = "high" ] && [ "$INCLUDE_HIGH" -eq 0 ]; then
    RESULT[$id]=SKIP; NOTE[$id]="high-cost — pass --all to run"; return
  fi

  local cwd="$WORK/$id"; local pd; pd="$(projdir "$cwd")"
  rm -rf "$cwd" "$pd"; mkdir -p "$cwd"

  echo; echo "── $id ──────────────────────────────────────────"
  echo "   surface: $(jq -r --arg i "$id" '.scenarios[]|select(.id==$i).surface' "$CATALOG")"

  local modelflag=(); [ -n "$MODEL" ] && modelflag=(--model "$MODEL")
  local resfile="$WORK/$id.result.json"
  ( cd "$cwd" && timeout 240 claude -p "$prompt" \
      --permission-mode "$pmode" --output-format json "${modelflag[@]}" ) > "$resfile" 2>"$WORK/$id.err" \
    || { RESULT[$id]=FAIL; NOTE[$id]="claude -p failed: $(tail -1 "$WORK/$id.err")"; return; }

  local f; f="$(ls -t "$pd"/*.jsonl 2>/dev/null | head -1)"
  [ -z "$f" ] && { RESULT[$id]=FAIL; NOTE[$id]="no session JSONL produced"; return; }

  # accumulate observed top-level types for the coverage matrix
  jq -r '.type' "$f" 2>/dev/null >> "$SEEN_TYPES_FILE"

  local ok=1 detail=""
  while IFS= read -r c; do
    [ -z "$c" ] && continue
    case "$c" in
      schema)
        local out; out="$( cd "$REPO/compliance/claude-session" && \
          AOA_COMPLIANCE_SESSION_DIR="$pd" go test -tags compliance \
          -run 'TestPass3_Schema' 2>&1 )"
        if echo "$out" | grep -q 'DRIFT (new'; then
          ok=0; detail+="SCHEMA-DRIFT($(echo "$out" | grep -oE 'DRIFT \(new[^:]*' | head -1)); "
        fi ;;
      usage_match)
        local rb inf
        rb="$(jq -c '{i:.usage.input_tokens,o:.usage.output_tokens,cr:.usage.cache_read_input_tokens,cc:.usage.cache_creation_input_tokens}' "$resfile" 2>/dev/null)"
        inf="$(jq -c 'select(.type=="assistant").message.usage|{i:.input_tokens,o:.output_tokens,cr:.cache_read_input_tokens,cc:.cache_creation_input_tokens}' "$f" 2>/dev/null | tail -1)"
        if [ "$rb" != "$inf" ]; then ok=0; detail+="usage_mismatch rb=$rb file=$inf; "
        else detail+="usage_match✓ $rb; "; fi ;;
      surface:*)
        local need="${c#surface:}"
        IFS='|' read -ra WANT <<< "$(echo "$need" | tr ',' '|')"
        for w in "${WANT[@]}"; do
          if ! jq -e --arg t "$w" 'select(.type==$t)' "$f" >/dev/null 2>&1; then
            ok=0; detail+="missing-type:$w; "
          fi
        done ;;
      block:*)
        local need="${c#block:}"
        IFS='|' read -ra WANT <<< "$(echo "$need" | tr ',' '|')"
        for w in "${WANT[@]}"; do
          if jq -e --arg t "$w" 'select(.message.content|type=="array").message.content[]|select(.type==$t)' "$f" >/dev/null 2>&1; then
            detail+="block:$w✓; "
          else ok=0; detail+="missing-block:$w; "; fi
        done ;;
      toolresult:*)
        local tool="${c#toolresult:}" sig=""
        case "$tool" in
          Bash) sig="stdout";; Edit) sig="structuredPatch";; Agent) sig="agentId";;
        esac
        if jq -e --arg k "$sig" 'select(.toolUseResult[$k]!=null)' "$f" >/dev/null 2>&1; then
          detail+="toolresult:$tool✓; "
        else ok=0; detail+="toolresult:$tool-missing; "; fi ;;
      iterations_populated)
        if jq -e 'select(.type=="assistant").message.usage.iterations|select(length>0)' "$f" >/dev/null 2>&1; then
          detail+="iterations✓; "
        else detail+="iterations-empty(single-turn?); "; fi ;;
      agent_tokens)
        local sub real est claimed
        real=0; est=0
        for sf in "$pd"/*/subagents/agent-*.jsonl; do
          [ -f "$sf" ] || continue
          local s; s=$(jq -s '[.[]|select(.type=="assistant").message.usage|select(.!=null)|(.input_tokens+.output_tokens+.cache_creation_input_tokens+.cache_read_input_tokens)]|add // 0' "$sf" 2>/dev/null)
          real=$((real + ${s:-0}))
        done
        claimed=$(jq -s '[.[]|select(.toolUseResult.agentId!=null)|.toolUseResult.totalTokens // 0]|add // 0' "$f" 2>/dev/null)
        local chars; chars=$(jq -r 'select(.toolUseResult.agentId!=null)|(.toolUseResult.content|if type=="array" then (map(.text//"")|join("")) elif type=="string" then . else "" end)|length' "$f" 2>/dev/null | paste -sd+ | bc 2>/dev/null)
        est=$(( ${chars:-0} / 4 ))
        if [ "${real:-0}" -gt 0 ] && [ "$est" -gt 0 ]; then
          local ratio; ratio=$(awk "BEGIN{printf \"%.0f\", $real/$est}")
          detail+="agent_tokens: aoa_est=$est claude=$claimed real=$real (${ratio}x under); "
          [ "$ratio" -ge 10 ] && { RESULT[$id]=WARN; }
        else detail+="agent_tokens: no subagent usage captured; "; fi ;;
    esac
  done < <(echo "$checks" | jq -r '.[]')

  [ "${RESULT[$id]:-}" = "WARN" ] || { [ "$ok" -eq 1 ] && RESULT[$id]=PASS || RESULT[$id]=FAIL; }
  NOTE[$id]="$detail"
  echo "   → ${RESULT[$id]}: $detail"
}

# ---- iterate the catalog ----
COUNT=$(jq '.scenarios|length' "$CATALOG")
for i in $(seq 0 $((COUNT-1))); do
  id=$(jq -r ".scenarios[$i].id" "$CATALOG")
  [ -n "$ONLY" ] && [ "$ONLY" != "$id" ] && continue
  trigger=$(jq -r ".scenarios[$i].trigger" "$CATALOG")
  cost=$(jq -r ".scenarios[$i].cost" "$CATALOG")
  pmode=$(jq -r ".scenarios[$i].permission_mode" "$CATALOG")
  prompt=$(jq -r ".scenarios[$i].prompt" "$CATALOG")
  checks=$(jq -c ".scenarios[$i].checks" "$CATALOG")
  run_scenario "$id" "$trigger" "$cost" "$pmode" "$prompt" "$checks"
done

# ---- coverage matrix: contract surfaces vs what we exercised ----
echo; echo "═══ Coverage: contract event types exercised ═══════════════════════"
RECOGNIZED="user assistant system permission-mode attachment last-prompt ai-title"
KNOWN_DROP="mode queue-operation progress file-history-snapshot"
SEEN_LIST=" $(sort -u "$SEEN_TYPES_FILE" 2>/dev/null | tr '\n' ' ') "
seen() { case "$SEEN_LIST" in *" $1 "*) echo "✓";; *) echo "·";; esac; }
for t in $RECOGNIZED; do printf "   [%s] %-18s (recognized)\n" "$(seen "$t")" "$t"; done
for t in $KNOWN_DROP; do printf "   [%s] %-18s (known-drop)\n" "$(seen "$t")" "$t"; done
echo "   (· = not exercised by an auto scenario; some need manual/interactive runs)"

# ---- summary ----
echo; echo "═══ Summary ════════════════════════════════════════════════════════"
pass=0; fail=0; warn=0; skip=0
for id in "${!RESULT[@]}"; do :; done
ORDER=$(jq -r '.scenarios[].id' "$CATALOG")
for id in $ORDER; do
  [ -n "$ONLY" ] && [ "$ONLY" != "$id" ] && continue
  r="${RESULT[$id]:-?}"
  printf "   %-16s %s  %s\n" "$id" "$r" "${NOTE[$id]:-}"
  case "$r" in PASS) pass=$((pass+1));; FAIL) fail=$((fail+1));; WARN) warn=$((warn+1));; SKIP) skip=$((skip+1));; esac
done
echo "───────────────────────────────────────────────────────────────────"
echo "   PASS=$pass  WARN=$warn  FAIL=$fail  SKIP=$skip"
if [ "${INSTALLED_VER:-}" != "${BASELINE_VER:-}" ]; then
  echo "   ⚠ installed v$INSTALLED_VER ≠ baseline v$BASELINE_VER — recapture baseline (see ../RUNBOOK.md)"
fi

if [ "$KEEP" -eq 0 ]; then
  rm -rf "$WORK"/*/ 2>/dev/null
  # remove the isolated scratch project dirs claude created under ~/.claude/projects/
  for id in $ORDER; do rm -rf "$(projdir "$WORK/$id")" 2>/dev/null; done
fi
[ "$fail" -eq 0 ]
