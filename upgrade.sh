#!/bin/sh
# Sailbox upgrade script
# Usage: curl -sSL https://get.sailbox.dev/upgrade | sh
#   or:  sh /opt/sailbox/upgrade.sh
set -e

INSTALL_DIR="/opt/sailbox"
COMPOSE_FILE="$INSTALL_DIR/docker-compose.yml"
ENV_FILE="$INSTALL_DIR/.env"
BACKUP_DIR="$INSTALL_DIR/backups"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()  { printf "${CYAN}[info]${NC}  %s\n" "$1"; }
ok()    { printf "${GREEN}[ok]${NC}    %s\n" "$1"; }
warn()  { printf "${YELLOW}[warn]${NC}  %s\n" "$1"; }
fail()  { printf "${RED}[error]${NC} %s\n" "$1"; exit 1; }

# ── Preflight ───────────────────────────────────────────────────
if [ "$(id -u)" -ne 0 ]; then
    fail "Please run as root"
fi

if [ ! -f "$COMPOSE_FILE" ]; then
    fail "Sailbox not found at $INSTALL_DIR. Run the installer first."
fi

if [ ! -f "$ENV_FILE" ]; then
    fail "Configuration not found: $ENV_FILE"
fi

. "$ENV_FILE"

CURRENT=$(docker inspect sailbox --format '{{.Config.Image}}' 2>/dev/null || echo "unknown")
info "Current image: $CURRENT"

# ── Step 1: Backup database ─────────────────────────────────────
info "Backing up database..."
mkdir -p "$BACKUP_DIR"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/sailbox_pre_upgrade_$TIMESTAMP.sql.gz"

docker exec sailbox-postgres pg_dump -U sailbox sailbox | gzip > "$BACKUP_FILE" 2>/dev/null
if [ -s "$BACKUP_FILE" ]; then
    SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
    ok "Database backup: $BACKUP_FILE ($SIZE)"
else
    rm -f "$BACKUP_FILE"
    warn "Backup is empty — database may be empty (fresh install)"
fi

# ── Step 2: Pull new image ──────────────────────────────────────
TARGET="${SAILBOX_VERSION:-latest}"
info "Pulling ghcr.io/sailboxhq/sailbox:$TARGET..."

if ! docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" pull sailbox 2>&1; then
    fail "Failed to pull new image. Aborting — no changes made."
fi
ok "Image pulled"

NEW_IMAGE=$(docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" config --images 2>/dev/null | grep sailbox | head -1)
info "New image: $NEW_IMAGE"

# ── Step 3: Rolling restart ─────────────────────────────────────
info "Restarting Sailbox..."
docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d --no-deps sailbox

# ── Step 4: Wait for healthy ────────────────────────────────────
info "Waiting for health check..."
SAILBOX_PORT="${SAILBOX_PORT:-3000}"

for i in $(seq 1 60); do
    if curl -sf "http://localhost:${SAILBOX_PORT}/healthz" >/dev/null 2>&1; then
        ok "Sailbox is healthy"
        break
    fi
    sleep 2
done

if ! curl -sf "http://localhost:${SAILBOX_PORT}/healthz" >/dev/null 2>&1; then
    warn "Health check failed — rolling back..."

    # Rollback: restore previous image
    docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" down sailbox 2>/dev/null
    if [ -n "$CURRENT" ] && [ "$CURRENT" != "unknown" ]; then
        # Force use old image
        sed -i.bak "s|image:.*sailbox.*|image: $CURRENT|" "$COMPOSE_FILE"
        docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d sailbox
        mv "$COMPOSE_FILE.bak" "$COMPOSE_FILE"  # restore compose file
        info "Rolled back to $CURRENT"
    fi

    # Restore database if needed
    if [ -s "$BACKUP_FILE" ]; then
        info "Restoring database from backup..."
        gunzip -c "$BACKUP_FILE" | docker exec -i sailbox-postgres psql -U sailbox sailbox >/dev/null 2>&1
        ok "Database restored"
    fi

    fail "Upgrade failed — rolled back to previous version."
fi

# ── Step 5: Cleanup old backups (keep last 5) ───────────────────
ls -t "$BACKUP_DIR"/sailbox_pre_upgrade_*.sql.gz 2>/dev/null | tail -n +6 | xargs rm -f 2>/dev/null

# ── Done ────────────────────────────────────────────────────────
printf "\n"
printf "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
printf "${GREEN}  Upgrade complete!${NC}\n"
printf "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
printf "\n"
printf "  ${BOLD}Backup:${NC}  %s\n" "$BACKUP_FILE"
printf "  ${BOLD}Image:${NC}   %s\n" "$NEW_IMAGE"
printf "  ${BOLD}Status:${NC}  %s\n" "$(curl -sf http://localhost:${SAILBOX_PORT}/healthz)"
printf "\n"
