# ============================================================================
# Stage 1: Build Go API (native cross-compile, no QEMU needed)
# ============================================================================
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS api-builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src/apps/api

COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

COPY apps/api/ ./
ARG VERSION=dev
ARG TARGETOS TARGETARCH
# VERSION in the RUN command ensures cache busts when version changes
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags "-s -w -X github.com/sailboxhq/sailbox/apps/api/internal/version.Version=${VERSION}" \
    -o /usr/local/bin/sailbox-api ./cmd/server && \
    echo "Built version: ${VERSION}"

# ============================================================================
# Stage 2: Build frontend (platform-independent, build once)
# ============================================================================
FROM --platform=$BUILDPLATFORM oven/bun:1-alpine AS web-builder

WORKDIR /src

COPY package.json bun.lock bunfig.toml ./
COPY apps/web/package.json apps/web/package.json
RUN bun install --frozen-lockfile

COPY apps/web/ apps/web/
ARG VERSION=dev
ENV VERSION=${VERSION}
RUN cd apps/web && bun run build

# ============================================================================
# Stage 3: Production image
# ============================================================================
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata curl docker-cli docker-cli-compose \
    && addgroup -S sailbox && adduser -S sailbox -G sailbox

COPY --from=api-builder /usr/local/bin/sailbox-api /usr/local/bin/sailbox-api
COPY --from=web-builder /src/apps/web/dist /srv/web
COPY --from=caddy:2-alpine /usr/bin/caddy /usr/local/bin/caddy
COPY deploy/Caddyfile /etc/caddy/Caddyfile
COPY deploy/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 3000

HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
    CMD curl -sf http://localhost:3000/healthz || exit 1

ENTRYPOINT ["/entrypoint.sh"]
