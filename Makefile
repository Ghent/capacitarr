.PHONY: lint format check build build\:frontend build\:backend down clean help

# ─── Code Quality ─────────────────────────────────────────────────────────────

## Run ESLint (auto-fix) + Go vet
lint:
	@echo "→ Linting frontend..."
	cd frontend && pnpm lint:fix
	@echo "→ Linting backend..."
	cd backend && go vet ./...
	@echo "✓ Lint complete"

## Run Prettier (auto-fix)
format:
	@echo "→ Formatting frontend..."
	cd frontend && pnpm format
	@echo "✓ Format complete"

## Verify code quality (no auto-fixes — CI-safe)
check:
	@echo "→ Checking frontend lint..."
	cd frontend && pnpm lint
	@echo "→ Checking frontend format..."
	cd frontend && pnpm format:check
	@echo "→ Ensuring go:embed directory exists..."
	mkdir -p backend/frontend/dist && touch backend/frontend/dist/.gitkeep
	@echo "→ Checking backend..."
	cd backend && go vet ./...
	@echo "✓ All checks passed"

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
	@echo "Code Quality:"
	@echo "  make lint            - Auto-fix lint issues (ESLint + go vet)"
	@echo "  make format          - Auto-format code (Prettier)"
	@echo "  make check           - Verify code quality (CI-safe, no auto-fix)"
	@echo ""
	@echo "Standalone Builds:"
	@echo "  make build:frontend  - Build frontend SPA"
	@echo "  make build:backend   - Build backend binary with embedded frontend"
	@echo ""
	@echo "Docker:"
	@echo "  make build           - Build and start via Docker Compose"
	@echo "  make down            - Stop containers"
	@echo "  make clean           - Remove containers, volumes, and build cache"
	@echo ""
	@echo "Workflow: make lint format → commit → make build"
