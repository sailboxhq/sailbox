#!/bin/sh
# Sailbox upgrade core — shared by manual upgrade.sh and container upgrader.
# Usage: source this file, then call run_upgrade "manual" or run_upgrade "container"
set -e

INSTALL_DIR="${INSTALL_DIR:-/opt/sailbox}"
COMPOSE_FILE="$INSTALL_DIR/docker-compose.yml"
ENV_FILE="$INSTALL_DIR/.env"
BACKUP_DIR="$INSTALL_DIR/backups"
STATUS_FILE="$INSTALL_DIR/upgrade_status.json"
LOG_FILE="$INSTALL_DIR/upgrade.log"
LOCK_FILE="$INSTALL_DIR/upgrade.lock"
ROLLBACK_IMAGE_FILE="$INSTALL_DIR/.rollback_image"

# Shared state across functions
BACKUP_FILE=""

# ── Output helpers ─────────────────────────────────────────────────
_log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') $1" >> "$LOG_FILE"
    if [ "$MODE" = "manual" ]; then
        printf "%s\n" "$1"
    fi
}

write_status() {
    # JSON-safe: escape quotes and newlines in message
    _msg=$(printf '%s' "$2" | sed 's/"/\\"/g' | tr '\n' ' ')
    printf '{"status":"%s","message":"%s"}' "$1" "$_msg" > "$STATUS_FILE"
}

# Health check — works with curl or wget
health_ok() {
    if command -v curl >/dev/null 2>&1; then
        curl -sf http://localhost:3000/healthz >/dev/null 2>&1
    else
        wget -qO /dev/null http://localhost:3000/healthz 2>/dev/null
    fi
}

# ── Lock — file-based, no PID check (works across containers) ─────
acquire_lock() {
    if [ -f "$LOCK_FILE" ]; then
        LOCK_AGE=$(( $(date +%s) - $(stat -c %Y "$LOCK_FILE" 2>/dev/null || stat -f %m "$LOCK_FILE" 2>/dev/null || date +%s) ))
        if [ "$LOCK_AGE" -lt 600 ]; then
            _log "[error] Another upgrade is running (lock age: ${LOCK_AGE}s). If stuck, remove $LOCK_FILE"
            write_status "error" "Another upgrade is already running"
            exit 1
        fi
        _log "[warn] Removing stale lock file (age: ${LOCK_AGE}s)"
        rm -f "$LOCK_FILE"
    fi
    date +%s > "$LOCK_FILE"
}

release_lock() {
    rm -f "$LOCK_FILE"
}

# ── Step functions ─────────────────────────────────────────────────

patch_env() {
    _log "[info] Patching .env to SAILBOX_VERSION=latest"
    write_status "upgrading" "Patching version to latest..."

    if [ ! -f "$ENV_FILE" ]; then
        _log "[error] Cannot find $ENV_FILE"
        write_status "error" "Cannot find .env file"
        return 1
    fi

    # Backup .env for rollback
    cp "$ENV_FILE" "$ENV_FILE.bak"

    if grep -q '^SAILBOX_VERSION=' "$ENV_FILE"; then
        sed -i 's/^SAILBOX_VERSION=.*/SAILBOX_VERSION=latest/' "$ENV_FILE"
    else
        echo "SAILBOX_VERSION=latest" >> "$ENV_FILE"
    fi

    # Ensure SETUP_SECRET exists
    if ! grep -q '^SETUP_SECRET=' "$ENV_FILE"; then
        SECRET=$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)
        echo "SETUP_SECRET=$SECRET" >> "$ENV_FILE"
        _log "[ok] Generated missing SETUP_SECRET"
    fi

    # Ensure docker-compose.yml passes SETUP_SECRET
    if ! grep -q 'SETUP_SECRET' "$COMPOSE_FILE"; then
        sed -i 's/^\(\s*JWT_SECRET:.*\)$/\1\n      SETUP_SECRET: ${SETUP_SECRET}/' "$COMPOSE_FILE"
    fi

    # Ensure docker.sock and /opt/sailbox are mounted
    if ! grep -q 'docker.sock' "$COMPOSE_FILE"; then
        sed -i "s|^\(\s*- /etc/rancher/k3s/k3s.yaml.*\)$|\1\n      - /var/run/docker.sock:/var/run/docker.sock\n      - $INSTALL_DIR:/opt/sailbox|" "$COMPOSE_FILE"
    elif ! grep -q "$INSTALL_DIR:/opt/sailbox" "$COMPOSE_FILE"; then
        sed -i "s|^\(\s*- /etc/rancher/k3s/k3s.yaml.*\)$|\1\n      - $INSTALL_DIR:/opt/sailbox|" "$COMPOSE_FILE"
    fi
}

pull_image() {
    _log "[info] Pulling latest image..."
    write_status "upgrading" "Pulling latest image..."

    # Save current image ID for rollback (digest-based, not tag-based)
    CURRENT_IMAGE_ID=$(docker inspect sailbox --format '{{.Image}}' 2>/dev/null || echo "")
    CURRENT_IMAGE_NAME=$(docker inspect sailbox --format '{{.Config.Image}}' 2>/dev/null || echo "unknown")
    echo "$CURRENT_IMAGE_ID" > "$ROLLBACK_IMAGE_FILE"
    _log "[info] Current image: $CURRENT_IMAGE_NAME ($CURRENT_IMAGE_ID)"

    if ! docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" pull sailbox >> "$LOG_FILE" 2>&1; then
        _log "[error] Failed to pull image"
        write_status "error" "Failed to pull latest image. Check network connectivity."
        return 1
    fi

    _log "[ok] Image pulled successfully"
}

backup_database() {
    _log "[info] Backing up database..."
    write_status "upgrading" "Backing up database..."

    mkdir -p "$BACKUP_DIR"
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    BACKUP_FILE="$BACKUP_DIR/sailbox_pre_upgrade_$TIMESTAMP.sql.gz"

    if docker exec sailbox-postgres sh -c "pg_dump -U sailbox sailbox | gzip" > "$BACKUP_FILE" 2>/dev/null && [ -s "$BACKUP_FILE" ]; then
        _log "[ok] Database backup: $BACKUP_FILE"
    else
        rm -f "$BACKUP_FILE"
        BACKUP_FILE=""
        TABLE_COUNT=$(docker exec sailbox-postgres psql -U sailbox -tAc \
            "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'" sailbox 2>/dev/null || echo "0")
        if [ "$TABLE_COUNT" -gt 0 ]; then
            _log "[error] Database backup failed but database has existing tables. Aborting."
            write_status "error" "Database backup failed with existing data. Aborting to prevent data loss."
            return 1
        fi
        _log "[warn] Backup empty or failed — continuing (fresh install has no data)"
    fi
}

restart_sailbox() {
    _log "[info] Restarting Sailbox..."
    write_status "upgrading" "Restarting with new version..."

    if ! docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d --no-deps sailbox >> "$LOG_FILE" 2>&1; then
        _log "[error] Failed to restart container"
        write_status "error" "Failed to restart Sailbox container"
        return 1
    fi
}

verify_health() {
    _log "[info] Waiting for health check..."
    write_status "upgrading" "Waiting for health check..."

    HEALTHY=false
    for i in $(seq 1 60); do
        if health_ok; then
            HEALTHY=true
            break
        fi
        sleep 2
    done

    if $HEALTHY; then
        # Clean old backups (keep last 5)
        ls -t "$BACKUP_DIR"/sailbox_pre_upgrade_*.sql.gz 2>/dev/null | tail -n +6 | xargs rm -f 2>/dev/null || true
        _log "[ok] Sailbox is healthy — upgrade complete"
        write_status "done" "Upgrade complete"
        return 0
    fi

    _log "[error] Health check failed after 120s"
    return 1
}

rollback() {
    _log "[warn] Rolling back..."
    write_status "upgrading" "Health check failed, rolling back..."

    # Restore .env (restores original SAILBOX_VERSION, not "latest")
    if [ -f "$ENV_FILE.bak" ]; then
        mv "$ENV_FILE.bak" "$ENV_FILE"
        _log "[info] Configuration restored"
    fi

    # Roll back to the exact previous image by digest (not tag)
    OLD_IMAGE_ID=""
    if [ -f "$ROLLBACK_IMAGE_FILE" ]; then
        OLD_IMAGE_ID=$(cat "$ROLLBACK_IMAGE_FILE")
    fi

    if [ -n "$OLD_IMAGE_ID" ]; then
        # Re-tag the old image so docker compose resolves to it
        # Get the image name from compose (e.g. ghcr.io/sailboxhq/sailbox:latest)
        . "$ENV_FILE"
        COMPOSE_IMAGE=$(grep 'image:' "$COMPOSE_FILE" | head -1 | sed 's/.*image:\s*//' | sed "s/\${SAILBOX_VERSION}/${SAILBOX_VERSION}/g")
        if [ -n "$COMPOSE_IMAGE" ]; then
            _log "[info] Re-tagging old image $OLD_IMAGE_ID as $COMPOSE_IMAGE"
            if ! docker tag "$OLD_IMAGE_ID" "$COMPOSE_IMAGE"; then
                _log "[error] Failed to re-tag old image. Rollback cannot guarantee correct version."
                write_status "error" "Upgrade failed and rollback could not restore the exact previous image. Old image ID: $OLD_IMAGE_ID. Manual fix: docker tag $OLD_IMAGE_ID $COMPOSE_IMAGE && docker compose -f $COMPOSE_FILE up -d --no-deps sailbox"
                return
            fi
        else
            _log "[error] Could not determine compose image name for rollback"
            write_status "error" "Upgrade failed and rollback could not determine the image to restore. Check: docker compose -f $COMPOSE_FILE logs sailbox"
            return
        fi
    fi
    docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d --no-deps --pull never sailbox >> "$LOG_FILE" 2>&1

    # Restore database if backup exists
    if [ -n "$BACKUP_FILE" ] && [ -s "$BACKUP_FILE" ]; then
        _log "[info] Restoring database..."
        docker exec sailbox-postgres psql -U sailbox -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;" sailbox >/dev/null 2>&1
        if docker exec -i sailbox-postgres sh -c "gunzip | psql -U sailbox sailbox" < "$BACKUP_FILE" >/dev/null 2>&1; then
            _log "[ok] Database restored"
            docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" restart sailbox >/dev/null 2>&1
        else
            _log "[warn] Database restore failed — backup at: $BACKUP_FILE"
        fi
    fi

    # Verify rollback health
    ROLLED_BACK=false
    for i in $(seq 1 30); do
        if health_ok; then
            ROLLED_BACK=true
            break
        fi
        sleep 2
    done

    # Clean up
    rm -f "$ROLLBACK_IMAGE_FILE"

    if $ROLLED_BACK; then
        _log "[info] Rolled back to previous version"
        write_status "error" "Upgrade failed — rolled back to previous version"
    else
        _log "[error] Rollback health check also failed"
        write_status "error" "Upgrade and rollback both failed. Manual intervention required. Check: docker compose -f $COMPOSE_FILE logs sailbox"
    fi
}

# ── Main orchestrator ──────────────────────────────────────────────

run_upgrade() {
    MODE="${1:-manual}"

    # Truncate log for this run
    : > "$LOG_FILE"
    _log "=== Sailbox upgrade started (mode=$MODE) ==="

    acquire_lock
    trap 'release_lock; rm -f "$ROLLBACK_IMAGE_FILE"' EXIT

    patch_env || { release_lock; exit 1; }
    pull_image || { release_lock; exit 1; }
    backup_database || { release_lock; exit 1; }
    restart_sailbox || { rollback; release_lock; exit 1; }

    if ! verify_health; then
        rollback
        release_lock
        exit 1
    fi

    # Clean up on success
    rm -f "$ENV_FILE.bak" "$ROLLBACK_IMAGE_FILE"
    release_lock
    _log "=== Sailbox upgrade completed ==="
}
