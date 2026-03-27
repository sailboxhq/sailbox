.PHONY: help dev dev-api dev-web build build-api build-web migrate migrate-create fmt lint test clean docker-up docker-down gen-api

# ============================================================================
# Variables
# ============================================================================

GO := go
GOFLAGS := -v
API_DIR := apps/api
WEB_DIR := apps/web
BIN_DIR := bin

# ============================================================================
# Help
# ============================================================================

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ============================================================================
# Development
# ============================================================================

dev: docker-up ## Start full dev environment
	@$(MAKE) -j2 dev-api dev-web

dev-api: ## Start Go API with hot reload
	cd $(API_DIR) && air

dev-web: ## Start Next.js dev server
	cd $(WEB_DIR) && bun dev

# ============================================================================
# Build
# ============================================================================

build: build-api build-web ## Build all

build-api: ## Build Go API binary
	cd $(API_DIR) && $(GO) build $(GOFLAGS) -o ../../$(BIN_DIR)/sailbox-api ./cmd/server
	cd $(API_DIR) && $(GO) build $(GOFLAGS) -o ../../$(BIN_DIR)/sailbox-worker ./cmd/worker
	cd $(API_DIR) && $(GO) build $(GOFLAGS) -o ../../$(BIN_DIR)/sailbox-migrate ./cmd/migrate

build-web: ## Build web production
	cd $(WEB_DIR) && bun run build

# ============================================================================
# Database
# ============================================================================

migrate: ## Run database migrations
	cd $(API_DIR) && $(GO) run ./cmd/migrate up

migrate-rollback: ## Rollback last migration
	cd $(API_DIR) && $(GO) run ./cmd/migrate rollback

migrate-create: ## Create new migration (usage: make migrate-create name=add_users)
	cd $(API_DIR) && $(GO) run ./cmd/migrate create $(name)

migrate-status: ## Show migration status
	cd $(API_DIR) && $(GO) run ./cmd/migrate status

# ============================================================================
# Format & Lint
# ============================================================================

fmt: ## Format & fix all code (Go + Biome)
	cd $(API_DIR) && gofmt -w -s . && goimports -w . 2>/dev/null || true
	bunx biome check --write $(WEB_DIR)

lint: ## Run all linters
	cd $(API_DIR) && golangci-lint run ./...
	bunx biome check $(WEB_DIR)

test: ## Run all tests
	cd $(API_DIR) && $(GO) test ./...
	cd $(WEB_DIR) && bun test

test-api: ## Run Go tests only
	cd $(API_DIR) && $(GO) test -race -cover ./...

# ============================================================================
# Docker (local dev infrastructure)
# ============================================================================

docker-up: ## Start dev infrastructure (PG, Valkey, K3s)
	docker compose -f deploy/docker-compose.yml up -d

docker-down: ## Stop dev infrastructure
	docker compose -f deploy/docker-compose.yml down

docker-destroy: ## Destroy dev infrastructure and volumes
	docker compose -f deploy/docker-compose.yml down -v

# ============================================================================
# Code Generation
# ============================================================================

gen-api: ## Generate API client from OpenAPI spec
	@echo "TODO: Generate Go server stubs and TS client from packages/spec/openapi.yaml"

# ============================================================================
# Clean
# ============================================================================

clean: ## Clean build artifacts
	rm -rf $(BIN_DIR)
	cd $(WEB_DIR) && rm -rf .next out node_modules/.cache
