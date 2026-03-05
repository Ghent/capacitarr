# CI/CD ↔ Local Parity

**Created:** 2026-03-05T12:55Z
**Status:** ✅ Complete

## Problem

The local development tooling (Makefile), CI/CD pipeline (`.gitlab-ci.yml`), and contributor documentation (`CONTRIBUTING.md`) were out of sync. Specific issues:

1. **Frontend formatting** (`pnpm format:check`) was checked locally but not in CI
2. **Vulnerability scanning** (`govulncheck`, `pnpm audit`) was in CI but had no local equivalent
3. **Tests** (`go test`, `pnpm test`) were in CI but had no Makefile target
4. **`golangci-lint`** was referenced everywhere but not installed locally — AI sessions kept trying to invoke it and failing
5. **`CONTRIBUTING.md`** documented `-race` flag for `go test` but CI didn't use it (incompatible with CGO_ENABLED=0 pure-Go build)
6. **Root `package.json`** had stale lint scripts using `npm` instead of `pnpm`
7. **No `.kilocoderules` enforcement** — nothing told AI sessions to use `make ci` or avoid assuming local tool installs

## Solution

Docker-based local CI parity: `make ci` runs the exact same checks, in the exact same Docker images, as the GitLab CI pipeline.

## Changes Made

### 1. Makefile — Added Docker-based CI targets

- [x] `make lint:ci` — runs `golangci-lint` (via `golangci/golangci-lint:latest` Docker image) + ESLint + Prettier format check (via `node:22-alpine`)
- [x] `make test:ci` — runs `go test ./... -count=1` (via `golang:1.25-alpine`) + `pnpm test` (via `node:22-alpine`)
- [x] `make security:ci` — runs `govulncheck` (via `golang:1.25-alpine`) + `pnpm audit` (via `node:22-alpine`)
- [x] `make ci` — chains all three: `lint:ci` → `test:ci` → `security:ci`
- [x] Updated existing `make lint` and `make check` to use Docker for `golangci-lint` instead of assuming local install
- [x] Updated help text and workflow hint

### 2. .gitlab-ci.yml — Added format check to frontend lint

- [x] Added `pnpm format:check` to `lint:frontend` job script (was missing, causing formatting violations to pass CI)

### 3. CONTRIBUTING.md — Unified around `make ci`

- [x] Replaced individual tool commands with `make ci` as the primary verification command
- [x] Removed `-race` flag from `go test` documentation (incompatible with CGO_ENABLED=0)
- [x] Added workflow recommendation: `make lint format → make ci → commit → push`
- [x] Documented individual stage targets (`make lint:ci`, `make test:ci`, `make security:ci`)

### 4. Root package.json — Removed stale scripts

- [x] Removed `lint:frontend`, `lint:backend`, `lint`, and `format` scripts (stale, used `npm` instead of `pnpm`, incomplete)
- [x] Kept only `release` script

### 5. .kilocoderules — Added enforcement rules

- [x] Added rule: "Always verify changes with `make ci` before declaring work complete"
- [x] Added rule: "Never assume `golangci-lint`, `govulncheck`, or other Go tools are installed locally"
- [x] Added rule: "The Makefile is the single source of truth for all checks"

## Verification Matrix

After these changes, all three sources of truth agree:

| Check | Makefile (`make ci`) | CI (`.gitlab-ci.yml`) | Docs (`CONTRIBUTING.md`) |
|---|---|---|---|
| golangci-lint | `lint:ci` (Docker) | `lint:go` | ✅ Documented |
| ESLint | `lint:ci` (Docker) | `lint:frontend` | ✅ Documented |
| Prettier format | `lint:ci` (Docker) | `lint:frontend` | ✅ Documented |
| go test -count=1 | `test:ci` (Docker) | `test:go` | ✅ Documented |
| vitest | `test:ci` (Docker) | `test:frontend` | ✅ Documented |
| govulncheck | `security:ci` (Docker) | `security:govulncheck` | ✅ Documented |
| pnpm audit | `security:ci` (Docker) | `security:pnpm-audit` | ✅ Documented |
