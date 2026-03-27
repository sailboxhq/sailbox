# ============================================================================
# Stage 1: Build Go API
# ============================================================================
FROM golang:1.25-alpine AS api-builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src/apps/api

# Cache dependencies
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

# Build
COPY apps/api/ ./
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/sailboxhq/sailbox/apps/api/internal/version.Version=${VERSION}" \
    -o /usr/local/bin/sailbox-api ./cmd/server

# ============================================================================
# Stage 2: Build frontend
# ============================================================================
FROM oven/bun:1-alpine AS web-builder

WORKDIR /src

# Install root deps (workspace)
COPY package.json bunfig.toml ./
COPY apps/web/package.json apps/web/package.json
RUN bun install --frozen-lockfile

# Build
COPY apps/web/ apps/web/
RUN cd apps/web && bun run build

# ============================================================================
# Stage 3: Production image
# ============================================================================
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata curl \
    && addgroup -S sailbox && adduser -S sailbox -G sailbox

# API binary
COPY --from=api-builder /usr/local/bin/sailbox-api /usr/local/bin/sailbox-api

# Frontend static files
COPY --from=web-builder /src/apps/web/dist /srv/web

# Caddy for reverse proxy (static files + API proxy)
COPY --from=caddy:2-alpine /usr/bin/caddy /usr/local/bin/caddy
COPY deploy/Caddyfile /etc/caddy/Caddyfile

# Entrypoint
COPY deploy/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 3000

HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
    CMD curl -sf http://localhost:3000/healthz || exit 1

ENTRYPOINT ["/entrypoint.sh"]
