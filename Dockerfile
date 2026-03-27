# ── Stage 1: Frontend build ────────────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend-builder
WORKDIR /app/frontend

RUN npm install -g pnpm@10.32.1

# Copy dependency manifests first for layer caching
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm run build

# ── Stage 2: Backend build ─────────────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS backend-builder
WORKDIR /app

# Copy dependency manifests first for layer caching
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

COPY backend/ ./backend/
COPY --from=frontend-builder /app/frontend/.output/public ./backend/frontend/dist

WORKDIR /app/backend
ARG APP_VERSION=dev
ARG BUILD_DATE=unknown
ARG COMMIT_SHA=unknown
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s -X main.version=${APP_VERSION} -X main.commit=${COMMIT_SHA} -X main.buildDate=${BUILD_DATE}" \
    -o capacitarr .

# ── Stage 3: Runtime (hardened Alpine) ─────────────────────────────────────────
# Digest pinned for reproducible builds. Update periodically or via Renovate Bot.
# To refresh: docker pull alpine:3.21 && docker inspect --format='{{index .RepoDigests 0}}' alpine:3.21
FROM alpine:3.21@sha256:c3f8e73fdb79deaebaa2037150150191b9dcbfba68b4a46d70103204c53f4709
WORKDIR /app

LABEL org.opencontainers.image.title="Capacitarr" \
      org.opencontainers.image.description="Media server capacity management" \
      org.opencontainers.image.source="https://github.com/Ghent/capacitarr"

# Install only what's needed, then remove the package manager to reduce attack
# surface. Busybox wget (built into Alpine) replaces curl for healthchecks.
RUN apk add --no-cache ca-certificates tzdata su-exec \
    && rm -rf /sbin/apk /etc/apk /lib/apk /usr/share/apk /var/cache/apk

COPY --from=backend-builder /app/backend/capacitarr /app/capacitarr
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

RUN mkdir -p /config

# Healthcheck uses busybox wget (always available in Alpine, no extra package).
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO /dev/null "${CAPACITARR_HEALTH_URL:-http://localhost:2187/api/v1/health}" || exit 1

VOLUME /config
EXPOSE 2187

ENTRYPOINT ["/app/entrypoint.sh"]
