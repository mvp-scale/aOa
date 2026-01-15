#!/bin/bash

# Quick test script for quickstart completion output
# Run this to iterate on alignment without rebuilding CLI

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

# =========================================================================
# TRUST-BUILDING INTRO (before processing)
# =========================================================================

echo -e "${CYAN}${BOLD}⚡ aOa Quickstart${NC}"
echo ""
echo -e "  Found ${BOLD}70 files${NC} in your project."
echo ""
echo -e "  ${DIM}•${NC} ${BOLD}Read-only${NC} — no files modified"
echo -e "  ${DIM}•${NC} ${BOLD}Local only${NC} — nothing leaves your machine"
echo -e "  ${DIM}•${NC} ${BOLD}Respects .gitignore${NC} — only your source code"
echo ""
echo -e "  ${DIM}~1 minute for most projects.${NC}"
echo ""
echo -e "  Press ${BOLD}Y${NC} to continue, or ${DIM}N${NC} to skip: ${GREEN}Y${NC}"
echo ""

# [Progress bar would go here]
echo -e "  Processing ${BOLD}70${NC} files (4 workers)"
echo ""
echo -e "  [${GREEN}████████████████████████████████████████${NC}] 100%"
echo ""

# =========================================================================
# COMPLETION OUTPUT (after processing)
# =========================================================================

echo -e "${GREEN}✓${NC} ${BOLD}70 files${NC} semantically compressed"
echo ""
echo -e "${DIM}───────────────────────────────────────────────────────────────────────────────────────${NC}"
echo ""

echo -e "  ${BOLD}What is semantic compression?${NC}"
echo ""
echo -e "  Semantic compression changes how search results are processed to deliver"
echo -e "  faster, targeted responses that AI can read with minimal context."
echo ""

echo -e "  ${DIM}Without aOa:${NC}"
echo ""
echo -e "    grep 'auth' → 12 matches → read 8 files → find method"
echo -e "    Cost: ${RED}${BOLD}50,000 tokens, 30 seconds${NC}"
echo ""

echo -e "  ${CYAN}With aOa:${NC}"
echo ""
echo -e "    ${DIM}\$${NC} aoa grep auth"
echo -e "        ${DIM}↓${NC}"
echo -e "    ${CYAN}⚡ aOa${NC} ${DIM}│${NC} ${BOLD}13${NC} hits ${DIM}│${NC} 6 files ${DIM}│${NC} ${GREEN}2.1ms${NC}"
echo ""
echo -e "    ${GREEN}auth/service.py${NC}:${BOLD}AuthService${NC}().${GREEN}handleAuth${NC}(request)[${CYAN}47-89${NC}]:${YELLOW}52${NC} ${DIM}<${NC}def login():${DIM}>${NC}  ${MAGENTA}@authentication${NC}  ${DIM}#auth #validation${NC}"
echo ""
echo -e "    ${DIM}What does this line mean?${NC}"
echo ""
echo -e "    ${GREEN}auth/service.py${NC}  :  ${BOLD}AuthService${NC}()  .  ${GREEN}handleAuth${NC}(req)  [${CYAN}47-89${NC}]  :${YELLOW}52${NC}  ${DIM}<${NC}def login()${DIM}>${NC}  ${MAGENTA}@authentication${NC}  ${DIM}#auth #validation${NC}"
echo -e "    ${DIM}│                   │                 │                │        │    │              │                │${NC}"
echo -e "    ${DIM}│                   │                 │                │        │    │              │                │${NC}"
echo -e "    ${DIM}└─file              └─class           └─method         └─range  └─ln └─grep         └─domain         └─tags${NC}"
echo ""
echo -e "    AI reads ${CYAN}${BOLD}42 lines${NC}, ${RED}not 8 files${NC}. ${BOLD}Less data. More meaning.${NC} ${CYAN}That's semantic compression.${NC}"
echo ""

echo -e "  ${CYAN}${BOLD}⚡ Signal Angle${NC}"
echo -e "  ${DIM}──────────────────────────────────────────────────────────────────${NC}"
echo -e "  ${BOLD}O(1) indexed search${NC} ${DIM}│${NC} ${GREEN}Results ranked by your intent${NC} ${DIM}│${NC} ${CYAN}aOa enriches${NC} → ${BOLD}Claude decides faster.${NC}"
echo ""
