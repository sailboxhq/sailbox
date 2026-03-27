#!/bin/sh
# Sailbox installer — https://github.com/sailboxhq/sailbox
# Usage: curl -sSL https://get.sailbox.dev | sh
set -e

# ── Configuration ───────────────────────────────────────────────
SAILBOX_VERSION="${SAILBOX_VERSION:-latest}"
SAILBOX_PORT="${SAILBOX_PORT:-3000}"
INSTALL_DIR="/opt/sailbox"
COMPOSE_FILE="$INSTALL_DIR/docker-compose.yml"
ENV_FILE="$INSTALL_DIR/.env"

# ── Colors ──────────────────────────────────────────────────────
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
preflight() {
    if [ "$(id -u)" -ne 0 ]; then
        fail "Please run as root: curl -sSL https://get.sailbox.dev | sudo sh"
    fi

    case "$(uname -s)" in
        Linux) ;;
        *) fail "Sailbox requires Linux. Detected: $(uname -s)" ;;
    esac

    ARCH="$(uname -m)"
    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) fail "Unsupported architecture: $ARCH" ;;
    esac

    MEM_KB=$(grep MemTotal /proc/meminfo | awk '{print $2}')
    MEM_MB=$((MEM_KB / 1024))
    if [ "$MEM_MB" -lt 1800 ]; then
        warn "Low memory: ${MEM_MB}MB (recommended 2048MB+)"
    fi

    for port in 80 443; do
        if ss -tlnp 2>/dev/null | grep -q ":${port} " || \
           netstat -tlnp 2>/dev/null | grep -q ":${port} "; then
            fail "Port ${port} is in use. Traefik needs 80/443 for ingress."
        fi
    done

    command -v curl >/dev/null 2>&1 || fail "curl is required"

    ok "Preflight passed (${ARCH}, ${MEM_MB}MB RAM)"
}

# ── Install Docker ──────────────────────────────────────────────
install_docker() {
    if command -v docker >/dev/null 2>&1; then
        ok "Docker already installed"
        return
    fi

    info "Installing Docker (this may take a minute)..."
    curl -fsSL https://get.docker.com | sh >/dev/null 2>&1
    systemctl enable --now docker >/dev/null 2>&1
    ok "Docker installed"
}

# ── Install K3s ─────────────────────────────────────────────────
install_k3s() {
    if command -v k3s >/dev/null 2>&1; then
        ok "K3s already installed"
        return
    fi

    info "Installing K3s..."
    curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server \
        --disable=traefik \
        --disable=servicelb \
        --write-kubeconfig-mode=644" sh -

    info "Waiting for K3s..."
    for i in $(seq 1 60); do
        if k3s kubectl get nodes >/dev/null 2>&1; then break; fi
        sleep 2
    done
    k3s kubectl get nodes >/dev/null 2>&1 || fail "K3s failed to start"
    ok "K3s running"
}

# ── Install Traefik ─────────────────────────────────────────────
install_traefik() {
    export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

    if k3s kubectl get deployment -n traefik traefik >/dev/null 2>&1; then
        ok "Traefik already installed"
        return
    fi

    info "Installing Traefik..."
    if ! command -v helm >/dev/null 2>&1; then
        curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash >/dev/null 2>&1
    fi

    helm repo add traefik https://traefik.github.io/charts >/dev/null 2>&1
    helm repo update >/dev/null 2>&1
    helm install traefik traefik/traefik \
        --namespace traefik --create-namespace \
        --set ports.web.hostPort=80 \
        --set ports.websecure.hostPort=443 \
        --set service.type=ClusterIP \
        --wait --timeout 120s >/dev/null 2>&1
    ok "Traefik installed (80/443)"
}

# ── Generate secrets ────────────────────────────────────────────
generate_secrets() {
    if [ -f "$ENV_FILE" ]; then
        ok "Configuration exists: $ENV_FILE"
        return
    fi

    DB_PASSWORD=$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)
    JWT_SECRET=$(head -c 48 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 48)

    SERVER_IP=$(curl -sf --max-time 5 https://api.ipify.org 2>/dev/null || \
                curl -sf --max-time 5 https://ifconfig.me 2>/dev/null || \
                hostname -I | awk '{print $1}')

    mkdir -p "$INSTALL_DIR"
    cat > "$ENV_FILE" <<EOF
DB_PASSWORD=$DB_PASSWORD
JWT_SECRET=$JWT_SECRET
SERVER_IP=$SERVER_IP
SAILBOX_PORT=$SAILBOX_PORT
SAILBOX_VERSION=$SAILBOX_VERSION
EOF
    chmod 600 "$ENV_FILE"
    ok "Secrets generated"
}

# ── Deploy via Docker Compose ───────────────────────────────────
deploy() {
    mkdir -p "$INSTALL_DIR"

    . "$ENV_FILE"

    cat > "$COMPOSE_FILE" <<COMPOSEFILE
services:
  sailbox:
    image: ghcr.io/sailboxhq/sailbox:${SAILBOX_VERSION}
    container_name: sailbox
    restart: unless-stopped
    ports:
      - "${SAILBOX_PORT}:3000"
    environment:
      DATABASE_URL: postgres://sailbox:\${DB_PASSWORD}@postgres:5432/sailbox?sslmode=disable
      JWT_SECRET: \${JWT_SECRET}
      K8S_IN_CLUSTER: "false"
      KUBECONFIG: /etc/rancher/k3s/k3s.yaml
      APP_URL: http://${SERVER_IP}:${SAILBOX_PORT}
    volumes:
      - /etc/rancher/k3s/k3s.yaml:/etc/rancher/k3s/k3s.yaml:ro
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-sf", "http://localhost:3000/healthz"]
      interval: 10s
      timeout: 3s
      retries: 10
      start_period: 15s

  postgres:
    image: postgres:18-alpine
    container_name: sailbox-postgres
    restart: unless-stopped
    environment:
      POSTGRES_DB: sailbox
      POSTGRES_USER: sailbox
      POSTGRES_PASSWORD: \${DB_PASSWORD}
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U sailbox"]
      interval: 5s
      timeout: 3s
      retries: 10

volumes:
  pgdata:
COMPOSEFILE

    info "Pulling images..."
    docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" pull --ignore-pull-failures >/dev/null 2>&1 || true

    info "Starting Sailbox..."
    docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d

    info "Waiting for Sailbox to be ready..."
    for i in $(seq 1 90); do
        if curl -sf "http://localhost:${SAILBOX_PORT}/healthz" >/dev/null 2>&1; then
            ok "Sailbox is running"
            return
        fi
        sleep 2
    done

    fail "Sailbox failed to start after 180s. Check: docker compose -f $COMPOSE_FILE logs"
}

# ── Summary ─────────────────────────────────────────────────────
summary() {
    . "$ENV_FILE"

    printf "\n"
    printf "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
    printf "${GREEN}  Sailbox is ready!${NC}\n"
    printf "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
    printf "\n"
    printf "  ${BOLD}Panel:${NC}    ${CYAN}http://%s:%s${NC}\n" "$SERVER_IP" "$SAILBOX_PORT"
    printf "  ${BOLD}Config:${NC}   %s\n" "$ENV_FILE"
    printf "  ${BOLD}Logs:${NC}     docker compose -f %s logs -f\n" "$COMPOSE_FILE"
    printf "  ${BOLD}Upgrade:${NC}  docker compose -f %s pull && docker compose -f %s up -d\n" "$COMPOSE_FILE" "$COMPOSE_FILE"
    printf "\n"
    printf "  ${BOLD}Port usage:${NC}\n"
    printf "    :%s  → Sailbox panel\n" "$SAILBOX_PORT"
    printf "    :80    → Traefik HTTP  (your deployed apps)\n"
    printf "    :443   → Traefik HTTPS (your deployed apps)\n"
    printf "    :6443  → K3s API\n"
    printf "\n"
    printf "  Open the panel in your browser to create your admin account.\n"
    printf "\n"
}

# ── Main ────────────────────────────────────────────────────────
main() {
    printf "\n"
    printf "${CYAN}  ⛵ Sailbox Installer${NC}\n"
    printf "${CYAN}  Self-hosted PaaS, powered by Kubernetes${NC}\n"
    printf "\n"

    preflight
    install_docker
    install_k3s
    install_traefik
    generate_secrets
    deploy
    summary
}

main "$@"
