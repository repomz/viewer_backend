# Docker image

## Локальная сборка

```bash
docker build \
  --build-arg VERSION=dev \
  --build-arg VCS_REF="$(git rev-parse --short HEAD)" \
  -t viewer-backend:dev .
```

## Запуск

Backend требует доступную PostgreSQL с применёнными миграциями:

```bash
docker run --rm \
  --name viewer-backend \
  --network viewer-e2e \
  -p 8080:8080 \
  -e DB_DSN='postgres://viewer:viewer@postgres:5432/viewer?sslmode=disable' \
  viewer-backend:dev
```

Миграции намеренно не применяются автоматически при старте API. SQL-файлы включены в образ в `/app/migrations`; в CI/E2E их должен применять отдельный migration job.

## Публикация в registry

Один образ:

```bash
docker buildx build \
  --platform linux/amd64 \
  -t ghcr.io/OWNER/viewer-backend:TAG \
  --push .
```

Multi-architecture:

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t ghcr.io/OWNER/viewer-backend:TAG \
  --push .
```

В build context не попадают `.env`, Git history, локальные бинарники, примеры и отчёты.

## Runtime

- процесс работает не от root, UID/GID `10001`;
- порт: `8080`;
- обязательная переменная: `DB_DSN`;
- healthcheck: `GET /`;
- миграции: `/app/migrations`.
