.PHONY: lint format check build build\:frontend build\:backend down clean help
.PHONY: ci lint\:ci test\:ci security\:ci

# ─── Code Quality (local tools, auto-fix mode) ────────────────────────────────

## Run ESLint (auto-fix) + golangci-lint (requires local install)
lint:
	@echo "→ Linting frontend (auto-fix)..."
	cd frontend && pnpm lint:fix
	@echo "→ Formatting backend (gofmt)..."
	gofmt -w backend/
	@echo "→ Linting backend (golangci-lint via Docker)..."
	@echo "→ Ensuring go:embed directory exists..."
	mkdir -p backend/frontend/dist && touch backend/frontend/dist/.gitkeep
	docker run --rm -v $(CURDIR)/backend:/app -w /app \
		golangci/golangci-lint:latest golangci-lint run ./...
	@echo "✓ Lint complete"

## Run Prettier (auto-fix)
format:
	@echo "→ Formatting frontend..."
	cd frontend && pnpm format
	@echo "→ Formatting backend (gofmt)..."
	gofmt -w backend/
	@echo "✓ Format complete"

## Verify code quality (no auto-fixes — CI-safe, matches CI pipeline exactly)
check:
	@echo "→ Checking frontend lint..."
	cd frontend && pnpm lint
	@echo "→ Checking frontend format..."
	cd frontend && pnpm format:check
	@echo "→ Ensuring go:embed directory exists..."
	mkdir -p backend/frontend/dist && touch backend/frontend/dist/.gitkeep
	@echo "→ Checking backend (golangci-lint via Docker)..."
	docker run --rm -v $(CURDIR)/backend:/app -w /app \
		golangci/golangci-lint:latest golangci-lint run ./...
	@echo "✓ All checks passed"

# ─── CI-Equivalent Checks (Docker-based, matches CI exactly) ──────────────────

## Lint all code (same Docker images + commands as CI pipeline)
lint\:ci:
	@echo "═══ CI Lint Stage ═══"
	@echo "→ [lint:go] golangci-lint (Docker: golangci/golangci-lint:latest)..."
	mkdir -p backend/frontend/dist && touch backend/frontend/dist/.gitkeep
	docker run --rm -v $(CURDIR)/backend:/app -w /app \
		golangci/golangci-lint:latest golangci-lint run ./...
	@echo "→ [lint:frontend] ESLint + Prettier (Docker: node:22-alpine)..."
	docker run --rm -e CI=true -v $(CURDIR)/frontend:/app -w /app \
		node:22-alpine sh -c "\
			corepack enable && \
			pnpm install --frozen-lockfile && \
			pnpm lint && \
			pnpm format:check"
	@echo "✓ CI lint stage passed"

## Run all tests (same Docker images + commands as CI pipeline)
test\:ci:
	@echo "═══ CI Test Stage ═══"
	@echo "→ [test:go] go test (Docker: golang:1.25-alpine)..."
	mkdir -p backend/frontend/dist && touch backend/frontend/dist/.gitkeep
	docker run --rm -v $(CURDIR)/backend:/app -w /app \
		golang:1.25-alpine sh -c "cd /app && go test -v ./... -count=1"
	@echo "→ [test:frontend] vitest (Docker: node:22-alpine)..."
	docker run --rm -e CI=true -v $(CURDIR)/frontend:/app -w /app \
		node:22-alpine sh -c "\
			corepack enable && \
			pnpm install --frozen-lockfile && \
			pnpm test"
	@echo "✓ CI test stage passed"

## Run security scans (same Docker images + commands as CI pipeline)
security\:ci:
	@echo "═══ CI Security Stage ═══"
	@echo "→ [security:govulncheck] (Docker: golang:1.25-alpine)..."
	mkdir -p backend/frontend/dist && touch backend/frontend/dist/.gitkeep
	docker run --rm -v $(CURDIR)/backend:/app -w /app \
		golang:1.25-alpine sh -c "\
			go install golang.org/x/vuln/cmd/govulncheck@latest && \
			cd /app && govulncheck ./..."
	@echo "→ [security:pnpm-audit] (Docker: node:22-alpine)..."
	docker run --rm -e CI=true -v $(CURDIR)/frontend:/app -w /app \
		node:22-alpine sh -c "\
			corepack enable && \
			pnpm install --frozen-lockfile && \
			pnpm audit" || true
	@echo "✓ CI security stage passed"

## Run the full CI pipeline locally (lint + test + security)
ci: lint\:ci test\:ci security\:ci
	@echo ""
	@echo "════════════════════════════════════════"
	@echo "  ✓ Full CI pipeline passed locally"
	@echo "════════════════════════════════════════"

# ─── Standalone Builds ────────────────────────────────────────────────────────

## Build the frontend SPA (output: frontend/.output/public)
build\:frontend:
	@echo "→ Building frontend..."
	cd frontend && pnpm install --frozen-lockfile && pnpm run build
	@echo "✓ Frontend built → frontend/.output/public"

## Build the backend binary with embedded frontend (output: backend/capacitarr)
build\:backend: build\:frontend
	@echo "→ Copying frontend assets into backend..."
	mkdir -p backend/frontend/dist
	cp -r frontend/.output/public/* backend/frontend/dist/
	@echo "→ Building backend..."
	cd backend && CGO_ENABLED=0 go build \
		-ldflags="-w -s \
		-X main.version=$$(git describe --tags --always) \
		-X main.commit=$$(git rev-parse --short HEAD) \
		-X main.buildDate=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
		-o capacitarr main.go
	@echo "✓ Backend built → backend/capacitarr"

# ─── Docker ───────────────────────────────────────────────────────────────────

## Build and start via Docker Compose
build:
	docker compose up -d --build

## Stop and remove containers
down:
	docker compose down

## Full clean: remove containers, volumes, and build cache
clean:
	docker compose down -v
	docker builder prune -f

# ─── Help ─────────────────────────────────────────────────────────────────────

## Show available targets
help:
	@echo "Capacitarr Development Commands"
	@echo "================================"
	@echo ""
	@echo "CI Pipeline (Docker-based — matches GitLab CI exactly):"
	@echo "  make ci             - Run full CI pipeline locally (lint + test + security)"
	@echo "  make lint:ci        - Lint all code (golangci-lint + ESLint + Prettier)"
	@echo "  make test:ci        - Run all tests (go test + vitest)"
	@echo "  make security:ci    - Run security scans (govulncheck + pnpm audit)"
	@echo ""
	@echo "Code Quality (local, auto-fix mode):"
	@echo "  make lint           - Auto-fix lint issues (ESLint --fix + golangci-lint)"
	@echo "  make format         - Auto-format code (Prettier + gofmt)"
	@echo "  make check          - Verify code quality (no auto-fixes)"
	@echo ""
	@echo "Standalone Builds:"
	@echo "  make build:frontend - Build frontend SPA"
	@echo "  make build:backend  - Build backend binary with embedded frontend"
	@echo ""
	@echo "Docker:"
	@echo "  make build          - Build and start via Docker Compose"
	@echo "  make down           - Stop containers"
	@echo "  make clean          - Remove containers, volumes, and build cache"
	@echo ""
	@echo "Workflow: make lint format → make ci → commit → push"
