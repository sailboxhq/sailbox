#!/bin/sh
# Sailbox upgrade script — manual entry point
# Usage: curl -sSL https://get.sailbox.dev/upgrade | sudo sh
set -e

INSTALL_DIR="/opt/sailbox"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ── Preflight (manual-only checks) ────────────────────────────────
[ "$(id -u)" -ne 0 ] && { printf "${RED}[error]${NC} Please run as root\n"; exit 1; }
[ ! -f "$INSTALL_DIR/docker-compose.yml" ] && { printf "${RED}[error]${NC} Sailbox not found. Run the installer first.\n"; exit 1; }
[ ! -f "$INSTALL_DIR/.env" ] && { printf "${RED}[error]${NC} Configuration not found: $INSTALL_DIR/.env\n"; exit 1; }

printf "\n"
printf "${CYAN}  ⛵ Sailbox Upgrade${NC}\n"
printf "\n"

CURRENT_IMAGE=$(docker inspect sailbox --format '{{.Config.Image}}' 2>/dev/null || echo "unknown")
printf "  ${BOLD}Current:${NC} %s\n\n" "$CURRENT_IMAGE"

# ── Download latest upgrade-lib.sh if available, else use bundled ──
LIB_URL="https://raw.githubusercontent.com/sailboxhq/sailbox/main/deploy/upgrade-lib.sh"
LIB_FILE="$INSTALL_DIR/upgrade-lib.sh"

if curl -sSL --max-time 10 "$LIB_URL" -o "$LIB_FILE.tmp" 2>/dev/null && [ -s "$LIB_FILE.tmp" ]; then
    mv "$LIB_FILE.tmp" "$LIB_FILE"
elif [ -f "$LIB_FILE" ]; then
    printf "${YELLOW}[warn]${NC}  Could not download latest upgrade library, using existing\n"
else
    printf "${RED}[error]${NC} Cannot download upgrade library and no local copy exists\n"
    exit 1
fi

chmod +x "$LIB_FILE"

# ── Run the shared upgrade logic ──────────────────────────────────
. "$LIB_FILE"
run_upgrade "manual"

# ── Print summary ─────────────────────────────────────────────────
NEW_IMAGE=$(docker inspect sailbox --format '{{.Config.Image}}' 2>/dev/null || echo "unknown")
printf "\n"
printf "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
printf "${GREEN}  Upgrade complete!${NC}\n"
printf "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
printf "\n"
printf "  ${BOLD}Previous:${NC}  %s\n" "$CURRENT_IMAGE"
printf "  ${BOLD}Current:${NC}   %s\n" "$NEW_IMAGE"
printf "  ${BOLD}Logs:${NC}      %s\n" "$INSTALL_DIR/upgrade.log"
printf "\n"
