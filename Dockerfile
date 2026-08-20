# syntax=docker/dockerfile:1.7

FROM golang:1.24.3-alpine AS build

WORKDIR /src

ARG VERSION=dev
ARG VCS_REF=unknown

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.revision=${VCS_REF}" \
    -o /out/viewer-backend \
    ./cmd

FROM golang:1.25.7-alpine AS migration-build

ARG GOOSE_VERSION=v3.27.1
RUN CGO_ENABLED=0 GOBIN=/out go install \
    -tags="no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb" \
    github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION}

FROM alpine:3.22 AS runtime-base

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app

WORKDIR /app

FROM runtime-base AS migrations

COPY --from=migration-build /out/goose /usr/local/bin/goose
COPY internal/sql/migrations /app/migrations

USER app

ENTRYPOINT ["/usr/local/bin/goose", "-dir", "/app/migrations"]

FROM runtime-base AS runtime

RUN apk add --no-cache ffmpeg

COPY --from=build /out/viewer-backend /usr/local/bin/viewer-backend
COPY internal/sql/migrations /app/migrations

ARG VERSION=dev
ARG VCS_REF=unknown
LABEL org.opencontainers.image.title="viewer-backend" \
      org.opencontainers.image.description="Backend API for the DICOM viewer and hospital agents" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${VCS_REF}"

ENV HTTP_ADDR=:8080 \
    MIGRATIONS_DIR=/app/migrations

USER app

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/ || exit 1

ENTRYPOINT ["/usr/local/bin/viewer-backend"]
