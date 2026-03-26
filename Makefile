.PHONY: lint format check build build\:frontend build\:backend down clean clean\:all help
.PHONY: ci lint\:ci test\:ci security\:ci cache\:clean

# ─── Docker Cache Volumes ──────────────────────────────────────────────────────
# Named volumes persist Go/Node downloads across ephemeral 'docker run --rm'
# containers. First run downloads everything; subsequent runs use cache.
# Run 'make cache:clean' to remove all cached volumes.

GO_CACHE_VOLS  := -v capacitarr-gomod:/go/pkg/mod -v capacitarr-gobuild:/root/.cache/go-build
NODE_CACHE_VOLS := -v capacitarr-pnpm:/root/.local/share/pnpm/store

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
	docker run --rm --pull missing -v $(CURDIR)/backend:/app $(GO_CACHE_VOLS) -w /app \
		golangci/golangci-lint:v2.11.4 sh -c "golangci-lint config verify && golangci-lint run ./..."
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
	@echo "→ Checking frontend types..."
	cd frontend && pnpm typecheck
	@echo "→ Ensuring go:embed directory exists..."
	mkdir -p backend/frontend/dist && touch backend/frontend/dist/.gitkeep
	@echo "→ Checking backend (golangci-lint via Docker)..."
	docker run --rm --pull missing -v $(CURDIR)/backend:/app $(GO_CACHE_VOLS) -w /app \
		golangci/golangci-lint:v2.11.4 sh -c "golangci-lint config verify && golangci-lint run ./..."
	@echo "✓ All checks passed"

# ─── CI-Equivalent Checks (Docker-based, matches CI exactly) ──────────────────

## Lint all code (same Docker images + commands as CI pipeline)
lint\:ci:
	@echo "═══ CI Lint Stage ═══"
	@echo "→ [lint:go] golangci-lint (Docker: golangci/golangci-lint:v2.11.4)..."
	mkdir -p backend/frontend/dist && touch backend/frontend/dist/.gitkeep
	docker run --rm --pull missing -v $(CURDIR)/backend:/app $(GO_CACHE_VOLS) -w /app \
		golangci/golangci-lint:v2.11.4 sh -c "golangci-lint config verify && golangci-lint run ./..."
	@echo "→ [lint:frontend] ESLint + Prettier (Docker: node:22-alpine)..."
	docker run --rm --pull missing -e CI=true -v $(CURDIR)/frontend:/app $(NODE_CACHE_VOLS) -w /app \
		node:22-alpine sh -c "\
			corepack enable && \
			pnpm install --frozen-lockfile && \
			pnpm lint && \
			pnpm format:check && \
			pnpm typecheck"
	@echo "✓ CI lint stage passed"

## Run all tests (same Docker images + commands as CI pipeline)
test\:ci:
	@echo "═══ CI Test Stage ═══"
	@echo "→ [test:go] go test (Docker: golang:1.26-alpine)..."
	mkdir -p backend/frontend/dist && touch backend/frontend/dist/.gitkeep
	docker run --rm --pull missing -v $(CURDIR)/backend:/app $(GO_CACHE_VOLS) -w /app \
		golang:1.26-alpine sh -c "cd /app && go test -v ./... -count=1"
	@echo "→ [test:frontend] vitest (Docker: node:22-alpine)..."
	docker run --rm --pull missing -e CI=true -v $(CURDIR)/frontend:/app $(NODE_CACHE_VOLS) -w /app \
		node:22-alpine sh -c "\
			corepack enable && \
			pnpm install --frozen-lockfile && \
			pnpm test"
	@echo "✓ CI test stage passed"

## Run security scans (same Docker images + commands as CI pipeline)
security\:ci:
	@echo "═══ CI Security Stage ═══"
	@echo "→ [security:govulncheck] (Docker: golang:1.26-alpine)..."
	mkdir -p backend/frontend/dist && touch backend/frontend/dist/.gitkeep
	docker run --rm --pull missing -v $(CURDIR)/backend:/app $(GO_CACHE_VOLS) -w /app \
		golang:1.26-alpine sh -c "\
			go install golang.org/x/vuln/cmd/govulncheck@latest && \
			cd /app && govulncheck ./..."
	@echo "→ [security:pnpm-audit] (Docker: node:22-alpine)..."
	docker run --rm --pull missing -e CI=true -v $(CURDIR)/frontend:/app $(NODE_CACHE_VOLS) -w /app \
		node:22-alpine sh -c "\
			corepack enable && \
			pnpm install --frozen-lockfile && \
			pnpm audit"
	@echo "→ [security:trivy] Filesystem vulnerability scan (Docker: ghcr.io/aquasecurity/trivy:0.69.3)..."
	docker run --rm --pull missing -v $(CURDIR)/backend:/src ghcr.io/aquasecurity/trivy:0.69.3 \
		fs --exit-code 1 --severity HIGH,CRITICAL --scanners vuln /src
	docker run --rm --pull missing -v $(CURDIR)/frontend:/src ghcr.io/aquasecurity/trivy:0.69.3 \
		fs --exit-code 1 --severity HIGH,CRITICAL --scanners vuln /src
	@echo "→ [security:gitleaks] Secret scanning (Docker: zricethezav/gitleaks:v8.30.1)..."
	docker run --rm --pull missing -v $(CURDIR):/src zricethezav/gitleaks:v8.30.1 \
		detect --source /src --config /src/.gitleaks.toml --verbose
	@echo "→ [security:semgrep] SAST scan (Docker: semgrep/semgrep:1.155.0)..."
	docker run --rm --pull missing -v $(CURDIR):/src semgrep/semgrep:1.155.0 \
		semgrep scan --config=auto --error /src
	@echo "✓ CI security stage passed"

## Run OWASP ZAP API scan against a running Capacitarr instance (requires `make build` first)
## Uses the OpenAPI spec to intelligently test all documented endpoints.
## Results saved to zap-report.html in the project root.
security\:zap:
	@echo "═══ OWASP ZAP DAST Scan ═══"
	@echo "→ Ensure Capacitarr is running on localhost:2187 (make build)"
	@echo "→ Running ZAP API scan against http://localhost:2187..."
	mkdir -p $(CURDIR)/zap-out
	chmod 777 $(CURDIR)/zap-out
	docker run --rm --network=host \
		-v $(CURDIR)/docs/api/openapi.yaml:/zap/openapi.yaml:ro \
		-v $(CURDIR)/zap-out:/zap/wrk:rw \
		ghcr.io/zaproxy/zaproxy:stable \
		zap-api-scan.py \
			-t http://localhost:2187/api/v1/ \
			-f openapi \
			-r zap-report.html \
			-w zap-report.md \
			-z "-config rules.cookie.ignorelist=jwt,authenticated"
	-mv $(CURDIR)/zap-out/zap-report.html $(CURDIR)/zap-report.html 2>/dev/null
	-mv $(CURDIR)/zap-out/zap-report.md $(CURDIR)/zap-report.md 2>/dev/null
	-rm -rf $(CURDIR)/zap-out
	@echo "✓ ZAP scan complete — see zap-report.html and zap-report.md"

## Scan the built Docker image for OS-level and binary CVEs (requires prior `make build`)
security\:image:
	@echo "═══ Container Image Scan ═══"
	@echo "→ [security:trivy-image] Scanning Docker image (Docker: ghcr.io/aquasecurity/trivy:0.69.3)..."
	docker run --rm --pull missing -v /var/run/docker.sock:/var/run/docker.sock \
		ghcr.io/aquasecurity/trivy:0.69.3 image --exit-code 1 --severity HIGH,CRITICAL --scanners vuln \
		capacitarr-capacitarr:latest
	@echo "✓ Container image scan passed"

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

## Safe clean: remove containers and build cache (preserves volumes)
clean:
	docker compose down
	docker builder prune -f

## Full clean: remove containers, volumes, and build cache (DESTROYS DATA)
clean\:all:
	docker compose down -v
	docker builder prune -f

## Remove CI cache volumes (Go modules, Go build cache, pnpm store)
cache\:clean:
	@echo "→ Removing CI cache volumes..."
	-docker volume rm capacitarr-gomod capacitarr-gobuild capacitarr-pnpm 2>/dev/null
	@echo "✓ CI cache volumes removed"

# ─── Help ─────────────────────────────────────────────────────────────────────

## Show available targets
help:
	@echo "Capacitarr Development Commands"
	@echo "================================"
	@echo ""
	@echo "CI Pipeline (Docker-based — matches GitHub Actions exactly):"
	@echo "  make ci             - Run full CI pipeline locally (lint + test + security)"
	@echo "  make lint:ci        - Lint all code (golangci-lint + ESLint + Prettier + typecheck)"
	@echo "  make test:ci        - Run all tests (go test + vitest)"
	@echo "  make security:ci    - Run security scans (govulncheck + pnpm audit + trivy + gitleaks + semgrep)"
	@echo "  make security:image - Scan Docker image for CVEs (requires prior make build)"
	@echo "  make security:zap   - Run OWASP ZAP API scan (requires running instance)"
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
	@echo "  make clean          - Remove containers and build cache (safe)"
	@echo "  make clean:all      - Remove containers, volumes, and build cache (DESTROYS DATA)"
	@echo "  make cache:clean    - Remove CI cache volumes (Go modules, pnpm store)"
	@echo ""
	@echo "Workflow: make lint format → make ci → commit → push"
