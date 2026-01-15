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
echo -e "    ${GREEN}auth/service.py${NC}:${BOLD}AuthService${NC}().${GREEN}handleAuth${NC}(request)[${CYAN}47-89${NC}]:${YELLOW}52${NC}  ${MAGENTA}@authentication${NC}  ${DIM}#auth #validation${NC}"
echo ""
echo -e "    ${DIM}What does this line mean?${NC}"
echo ""
echo -e "    ${GREEN}auth/service.py${NC}    :    ${BOLD}AuthService${NC}()    .    ${GREEN}handleAuth${NC}(req)    [${CYAN}47-89${NC}]    :${YELLOW}52${NC}     ${MAGENTA}@authentication${NC}     ${DIM}#auth #validation${NC}"
echo -e "         ${DIM}│${NC}                    ${DIM}│${NC}                      ${DIM}│${NC}                  ${DIM}│${NC}          ${DIM}│${NC}            ${DIM}│${NC}                     ${DIM}│${NC}"
echo -e "         ${DIM}↓${NC}                    ${DIM}↓${NC}                      ${DIM}↓${NC}                  ${DIM}↓${NC}          ${DIM}↓${NC}            ${DIM}↓${NC}                     ${DIM}↓${NC}"
echo -e "       file                class                 method              range        line         domain                  tags"
echo ""
echo -e "    AI reads ${CYAN}42 lines${NC}, not 8 files."
echo ""

echo -e "  ${CYAN}${BOLD}⚡ Signal Angle${NC}"
echo -e "  ${DIM}──────────────────────────────────────────────────────────────────${NC}"
echo -e "  ${BOLD}98% fewer tokens${NC} ${DIM}│${NC} ${GREEN}5ms searches${NC} ${DIM}│${NC} ${CYAN}aOa enriches${NC} → ${BOLD}Claude decides faster.${NC}"
echo ""
