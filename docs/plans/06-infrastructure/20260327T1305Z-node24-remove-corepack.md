# Upgrade Node.js 22→24 and Remove Corepack

**Status:** ✅ Complete
**Created:** 2026-03-27
**Scope:** capacitarr (single repo)

## Summary

Upgrade the project from Node.js 22 to Node.js 24 LTS (Krypton) and remove the
corepack dependency. Corepack provided versioned pnpm management but adds
unnecessary complexity in containerized builds. The CI workflows already bypass
corepack via `pnpm/action-setup@v4`. The Node.js TSC voted to remove corepack
from Node.js 25+, making this a forward-looking change.

Also bumps pnpm from 10.29.3 → 10.32.1 (latest stable).

## Motivation

- **Node.js 24** is the current active LTS (Krypton, entered LTS Oct 2025).
  Node.js 22 remains supported until April 2027, but 24 provides V8 13.6,
  `URLPattern` global, improved `AsyncLocalStorage` via `AsyncContextFrame`,
  undici 7, and npm 11.
- **Corepack** is a package-manager version shim bundled with Node.js. In
  Capacitarr's all-containerized build model (Dockerfile, Makefile docker runs,
  GitHub Actions), each build controls its own pnpm version — corepack adds
  indirection with no benefit. It is being removed from Node.js 25+ (Oct 2026).
- **pnpm 10.32.1** is the latest stable (10.29.3 is current). Minor version
  bump with no breaking changes.

## Changes

### Phase 1: Version Bumps and Corepack Removal

All steps in this phase are independent edits to different files. Each step
specifies the exact file, line(s), and transformation.

#### Step 1: Update `.node-version`

**File:** `.node-version`

```diff
-22
+24
```

#### Step 2: Update `frontend/package.json` — `packageManager` field

**File:** `frontend/package.json` (line 60)

```diff
-  "packageManager": "pnpm@10.29.3",
+  "packageManager": "pnpm@10.32.1",
```

#### Step 3: Update `site/package.json` — `packageManager` field

**File:** `site/package.json` (line 24)

```diff
-  "packageManager": "pnpm@10.29.3",
+  "packageManager": "pnpm@10.32.1",
```

#### Step 4: Update `Dockerfile` — Remove corepack, use npm to install pnpm

**File:** `Dockerfile` (lines 2, 5)

```diff
-FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend-builder
+FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend-builder
 WORKDIR /app/frontend

-RUN corepack enable && corepack prepare pnpm@10.29.3 --activate
+RUN npm install -g pnpm@10.32.1
```

**Rationale:** `npm install -g pnpm@<version>` is the simplest, most portable
approach. npm is always available in the Node.js Docker image. No external
script downloads, no corepack shim.

#### Step 5: Update `Makefile` — Replace `corepack enable` with pnpm install

**File:** `Makefile`

Three targets use `corepack enable` inside ephemeral `docker run` containers.
Additionally, update the Docker image tag from `node:22-alpine` to
`node:24-alpine` in all references (3 targets × docker image tag + echo message,
plus the 3 `corepack enable` replacements).

**`lint:ci` (lines 59–66):**
```diff
-	@echo "→ [lint:frontend] ESLint + Prettier (Docker: node:22-alpine)..."
+	@echo "→ [lint:frontend] ESLint + Prettier (Docker: node:24-alpine)..."
 	docker run --rm --pull missing -e CI=true -v $(CURDIR)/frontend:/app -v /app/node_modules $(NODE_CACHE_VOLS) -w /app \
-		node:22-alpine sh -c "\
-			corepack enable && \
+		node:24-alpine sh -c "\
+			npm install -g pnpm@10.32.1 --silent && \
 			pnpm install --frozen-lockfile && \
```

**`test:ci` (lines 76–81):**
```diff
-	@echo "→ [test:frontend] vitest (Docker: node:22-alpine)..."
+	@echo "→ [test:frontend] vitest (Docker: node:24-alpine)..."
 	docker run --rm --pull missing -e CI=true -v $(CURDIR)/frontend:/app -v /app/node_modules $(NODE_CACHE_VOLS) -w /app \
-		node:22-alpine sh -c "\
-			corepack enable && \
+		node:24-alpine sh -c "\
+			npm install -g pnpm@10.32.1 --silent && \
 			pnpm install --frozen-lockfile && \
```

**`security:ci` (lines 93–98):**
```diff
-	@echo "→ [security:pnpm-audit] (Docker: node:22-alpine)..."
+	@echo "→ [security:pnpm-audit] (Docker: node:24-alpine)..."
 	docker run --rm --pull missing -e CI=true -v $(CURDIR)/frontend:/app -v /app/node_modules $(NODE_CACHE_VOLS) -w /app \
-		node:22-alpine sh -c "\
-			corepack enable && \
+		node:24-alpine sh -c "\
+			npm install -g pnpm@10.32.1 --silent && \
 			pnpm install --frozen-lockfile && \
```

**Note:** The `--silent` flag on `npm install` suppresses npm's noisy output
since pnpm installation is a setup step, not a user-visible action.

#### Step 6: Update CI workflows — Node.js version

**File:** `.github/workflows/ci.yml`

Three `node-version` references (lines 50, 87, 142):

```diff
-          node-version: "22"
+          node-version: "24"
```

**File:** `.github/workflows/release.yml`

One `node-version` reference (line 72):

```diff
-          node-version: "22"
+          node-version: "24"
```

**No corepack changes needed** in CI — `pnpm/action-setup@v4` already installs
pnpm directly from the `packageManager` field without corepack.

#### Step 7: Update `SECURITY.md` — Pinned images table

**File:** `SECURITY.md` (line 315)

```diff
-| `node` | `22-alpine` | Frontend build and test |
+| `node` | `24-alpine` | Frontend build and test |
```

### Phase 2: Lock File Refresh

#### Step 8: Refresh `frontend/pnpm-lock.yaml`

Run `pnpm install` in `frontend/` with the new pnpm version to regenerate the
lock file. This updates the `lockfileVersion` header and may adjust resolution
hashes if pnpm 10.32.1 resolves any packages differently than 10.29.3.

```bash
cd frontend && pnpm install
```

#### Step 9: Refresh `site/pnpm-lock.yaml`

Same for the docs site:

```bash
cd site && pnpm install
```

### Phase 3: Verify

#### Step 10: Run `make ci`

Run the full CI pipeline locally to verify all changes work together:

```bash
make ci
```

This runs lint, test, and security stages using the updated `node:24-alpine`
Docker images with the new pnpm installation approach.

#### Step 11: Run `docker compose up --build`

Verify the production Dockerfile builds and the application starts correctly:

```bash
docker compose up --build
```

Verify:
- Container starts without errors
- Frontend is accessible at `http://localhost:2187`
- Login works with dev credentials (`admin`/`admin`)

## Files Modified

| File | Changes |
|------|---------|
| `.node-version` | `22` → `24` |
| `frontend/package.json` | `packageManager` pnpm 10.29.3 → 10.32.1 |
| `site/package.json` | `packageManager` pnpm 10.29.3 → 10.32.1 |
| `Dockerfile` | `node:22-alpine` → `node:24-alpine`, replace corepack with npm install |
| `Makefile` | `node:22-alpine` → `node:24-alpine` (×3), replace `corepack enable` with `npm install -g pnpm` (×3) |
| `.github/workflows/ci.yml` | `node-version: "22"` → `"24"` (×3) |
| `.github/workflows/release.yml` | `node-version: "22"` → `"24"` (×1) |
| `SECURITY.md` | Pinned images table: `node` version 22 → 24 |
| `frontend/pnpm-lock.yaml` | Regenerated with pnpm 10.32.1 |
| `site/pnpm-lock.yaml` | Regenerated with pnpm 10.32.1 |

## Risks

- **Lock file changes:** pnpm 10.32.1 may resolve packages slightly differently
  than 10.29.3. The `make ci` step (Step 10) catches any regressions.
- **Node.js 24 breaking changes:** V8 13.6 includes `Float16Array` global,
  `url.parse()` runtime deprecation, and `SlowBuffer` runtime deprecation. None
  of these affect Capacitarr's frontend (Vue 3/Nuxt stack). The `pnpm audit`
  step in CI will flag any dependency issues.
- **npm version:** Node.js 24 ships npm 11. The `npm install -g pnpm` command
  uses npm 11, which is fine — we're only using npm as a pnpm bootstrapper.

## Commit

Single commit: `chore(deps): upgrade Node.js 22→24 and remove corepack`

## Execution Notes

- **Steps 1–7:** All file edits applied as planned. No deviations.
- **Steps 8–9:** Lock files were already up to date — pnpm 10.32.1 resolved
  identically to 10.29.3 for both `frontend/` and `site/` dependency trees.
  No lock file changes needed.
- **Step 10:** `make ci` passed all stages (lint, test, security). The
  `npm install -g pnpm@10.32.1 --silent` approach works correctly inside
  `node:24-alpine` ephemeral containers.
- **Step 11:** Docker Compose build succeeded. Health endpoint returned 200.
  Frontend served correctly. Login with dev credentials returned a valid JWT.
