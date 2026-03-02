# ── Stage 1: Frontend build ────────────────────────────────────────────────────
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend

RUN corepack enable && corepack prepare pnpm@10.29.3 --activate

# Copy dependency manifests first for layer caching
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm run build

# ── Stage 2: Backend build ─────────────────────────────────────────────────────
FROM golang:1.25-alpine AS backend-builder
WORKDIR /app
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy dependency manifests first for layer caching
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

COPY backend/ ./backend/
COPY --from=frontend-builder /app/frontend/.output/public ./backend/frontend/dist

WORKDIR /app/backend
ARG APP_VERSION=dev
ARG BUILD_DATE=unknown
ARG COMMIT_SHA=unknown
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-w -s -X main.version=${APP_VERSION} -X main.commit=${COMMIT_SHA} -X main.buildDate=${BUILD_DATE}" \
    -o capacitarr main.go

# ── Stage 3: Runtime ───────────────────────────────────────────────────────────
FROM alpine:3.21
WORKDIR /app

LABEL org.opencontainers.image.title="Capacitarr" \
      org.opencontainers.image.description="Media server capacity management" \
      org.opencontainers.image.source="https://gitlab.com/starshadow/software/capacitarr"

RUN apk add --no-cache ca-certificates tzdata sqlite-libs su-exec curl

COPY --from=backend-builder /app/backend/capacitarr /app/capacitarr
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

RUN mkdir -p /config

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:2187/api/v1/health || exit 1

VOLUME /config
EXPOSE 2187

ENTRYPOINT ["/app/entrypoint.sh"]
