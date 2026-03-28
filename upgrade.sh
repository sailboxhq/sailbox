#!/bin/sh
# Sailbox upgrade script
# Usage: curl -sSL https://get.sailbox.dev/upgrade | sudo sh
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
[ "$(id -u)" -ne 0 ] && fail "Please run as root"
[ ! -f "$COMPOSE_FILE" ] && fail "Sailbox not found. Run the installer first."
[ ! -f "$ENV_FILE" ] && fail "Configuration not found: $ENV_FILE"

. "$ENV_FILE"

CURRENT_IMAGE=$(docker inspect sailbox --format '{{.Config.Image}}' 2>/dev/null || echo "unknown")
info "Current: $CURRENT_IMAGE"

# ── Step 1: Backup database ─────────────────────────────────────
info "Backing up database..."
mkdir -p "$BACKUP_DIR"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/sailbox_pre_upgrade_$TIMESTAMP.sql.gz"

if docker exec sailbox-postgres pg_dump -U sailbox sailbox 2>/dev/null | gzip > "$BACKUP_FILE" && [ -s "$BACKUP_FILE" ]; then
    SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
    ok "Database backup: $BACKUP_FILE ($SIZE)"
else
    rm -f "$BACKUP_FILE"
    BACKUP_FILE=""
    # Check if database has any user data (non-empty install)
    TABLE_COUNT=$(docker exec sailbox-postgres psql -U sailbox -tAc "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'" sailbox 2>/dev/null || echo "0")
    if [ "$TABLE_COUNT" -gt 0 ]; then
        fail "Database backup failed but database has existing tables. Aborting upgrade to prevent data loss."
    fi
    warn "Backup empty or failed — continuing (fresh install has no data)"
fi

# ── Step 2: Pull latest image ───────────────────────────────────
TARGET_IMAGE="ghcr.io/sailboxhq/sailbox:latest"
info "Pulling $TARGET_IMAGE..."

if ! docker pull "$TARGET_IMAGE" 2>&1; then
    fail "Failed to pull image. No changes made."
fi

# Check if already up to date
NEW_DIGEST=$(docker inspect "$TARGET_IMAGE" --format '{{.Id}}' 2>/dev/null)
CURRENT_DIGEST=$(docker inspect sailbox --format '{{.Image}}' 2>/dev/null)

IMAGE_CHANGED=true
if [ "$NEW_DIGEST" = "$CURRENT_DIGEST" ]; then
    ok "Already on the latest image"
    IMAGE_CHANGED=false
fi

# ── Step 3: Update .env ────────────────────────────────────────
# Ensure SAILBOX_VERSION=latest
if sed -i.bak 's/^SAILBOX_VERSION=.*/SAILBOX_VERSION=latest/' "$ENV_FILE" 2>/dev/null && grep -q '^SAILBOX_VERSION=latest' "$ENV_FILE"; then
    ok "Configuration updated to track latest"
else
    warn "Could not update SAILBOX_VERSION in $ENV_FILE — adding it"
    echo "SAILBOX_VERSION=latest" >> "$ENV_FILE"
    ok "SAILBOX_VERSION=latest appended to $ENV_FILE"
fi

# Ensure SETUP_SECRET exists (required since v1.x — older installs won't have it)
if ! grep -q '^SETUP_SECRET=' "$ENV_FILE"; then
    SETUP_SECRET=$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)
    echo "SETUP_SECRET=$SETUP_SECRET" >> "$ENV_FILE"
    ok "Generated SETUP_SECRET for this instance"
fi

# Ensure docker-compose.yml passes SETUP_SECRET to the container
if ! grep -q 'SETUP_SECRET' "$COMPOSE_FILE"; then
    sed -i 's/^\(\s*JWT_SECRET:.*\)$/\1\n      SETUP_SECRET: \${SETUP_SECRET}/' "$COMPOSE_FILE"
    if grep -q 'SETUP_SECRET' "$COMPOSE_FILE"; then
        ok "Added SETUP_SECRET to docker-compose.yml"
    else
        warn "Could not patch docker-compose.yml — add 'SETUP_SECRET: \${SETUP_SECRET}' to sailbox environment manually"
    fi
fi

# ── Step 4: Restart via compose (PG stays running) ──────────────
info "Restarting Sailbox..."
docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d --no-deps sailbox

# ── Step 5: Health check ────────────────────────────────────────
info "Waiting for health check..."
HEALTHY=false
for i in $(seq 1 60); do
    if curl -sf http://localhost:3000/healthz >/dev/null 2>&1; then
        HEALTHY=true
        break
    fi
    sleep 2
done

if $HEALTHY; then
    ok "Sailbox is healthy"

    # Cleanup old backups (keep last 5)
    ls -t "$BACKUP_DIR"/sailbox_pre_upgrade_*.sql.gz 2>/dev/null | tail -n +6 | xargs rm -f 2>/dev/null || true

    printf "\n"
    printf "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
    printf "${GREEN}  Upgrade complete!${NC}\n"
    printf "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
    printf "\n"
    printf "  ${BOLD}Previous:${NC}  %s\n" "$CURRENT_IMAGE"
    printf "  ${BOLD}Current:${NC}   %s\n" "$TARGET_IMAGE"
    printf "  ${BOLD}Backup:${NC}    %s\n" "${BACKUP_FILE:-none}"
    printf "\n"
    exit 0
fi

# ── Rollback ────────────────────────────────────────────────────
warn "Health check failed after 120s — rolling back..."

# Restore .env
if [ -f "$ENV_FILE.bak" ]; then
    mv "$ENV_FILE.bak" "$ENV_FILE"
    info "Configuration restored"
fi

# Restart compose with original image (now .env has old SAILBOX_VERSION)
docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d --no-deps --pull never sailbox

# Wait for rollback health
ROLLED_BACK=false
for i in $(seq 1 30); do
    if curl -sf http://localhost:3000/healthz >/dev/null 2>&1; then
        ROLLED_BACK=true
        break
    fi
    sleep 2
done

if $ROLLED_BACK; then
    info "Rolled back to $CURRENT_IMAGE"
else
    warn "Rollback health check also failed — manual intervention required"
    warn "Try: docker compose -f $COMPOSE_FILE --env-file $ENV_FILE logs sailbox"
fi

# Restore database if backup exists
if [ -n "$BACKUP_FILE" ] && [ -s "$BACKUP_FILE" ]; then
    info "Restoring database..."
    # Drop and recreate to ensure clean state
    docker exec sailbox-postgres psql -U sailbox -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;" sailbox >/dev/null 2>&1
    if gunzip -c "$BACKUP_FILE" | docker exec -i sailbox-postgres psql -U sailbox sailbox >/dev/null 2>&1; then
        ok "Database restored from backup"
        # Restart to pick up restored schema
        docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" restart sailbox >/dev/null 2>&1
        # Final health check after DB restore
        RESTORE_HEALTHY=false
        for i in $(seq 1 30); do
            if curl -sf http://localhost:3000/healthz >/dev/null 2>&1; then
                RESTORE_HEALTHY=true
                break
            fi
            sleep 2
        done
        if $RESTORE_HEALTHY; then
            ok "Service healthy after database restore"
        else
            warn "Service not healthy after database restore — manual check required"
            warn "Try: docker compose -f $COMPOSE_FILE --env-file $ENV_FILE logs sailbox"
        fi
    else
        warn "Database restore failed — backup at: $BACKUP_FILE"
        warn "Manual restore: gunzip -c $BACKUP_FILE | docker exec -i sailbox-postgres psql -U sailbox sailbox"
    fi
fi

fail "Upgrade failed — rolled back to previous version."
