# Docker и единый серверный стек

## Полный запуск одной командой

Репозитории должны лежать рядом:

```text
viewer/
├── viewer_backend/
└── agent/
```

Первичная настройка:

```bash
cd viewer_backend
cp .env.compose.example .env
```

Запуск PostgreSQL, миграций, backend, hospital agent, Orthanc PACS и OHIF:

```bash
docker compose up -d --build --wait
```

После запуска:

- backend: `http://SERVER:8080`;
- OHIF: `http://SERVER:3000`;
- Orthanc Explorer: `http://SERVER:8042`;
- DICOM PACS: AE Title `MAPDR`, порт `4242`;
- Orthanc login: `mapdr`, password: `changestrongpassword`.

Пароль задан так же, как в исходном `ohif-orthanc`, чтобы стек запускался
без дополнительной генерации конфигов. Перед публикацией сервера в интернет
нужно заменить пароль одновременно в `ohif-orthanc/orthanc.json` и Basic Auth
заголовке `ohif-orthanc/nginx_ohif.conf`, а HTTP закрыть TLS reverse proxy.

Состояние PostgreSQL, Orthanc и агента хранится в named volumes. Остановка без
удаления данных:

```bash
docker compose down
```

Полная очистка тестовых данных:

```bash
docker compose down --volumes
```

Просмотр состояния и логов:

```bash
docker compose ps
docker compose logs -f backend agent pacs ohif
```

## Образы из registry

В `.env` можно заменить `BACKEND_IMAGE`, `BACKEND_MIGRATIONS_IMAGE` и
`AGENT_IMAGE` на адреса опубликованных образов. На сервере с исходниками
доступна одна команда с принудительным получением свежих образов:

```bash
docker compose up -d --pull always --wait
```

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

В едином Compose миграции применяет отдельный одноразовый сервис `migrations`.
Backend стартует только после его успешного завершения.

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

## E2E с локальным PACS

Agent подключён к `pacs:4242` с локальным AE Title `HOSPITAL_AGENT`.
В контейнерном профиле включены только heartbeat и обработка `/user_requests`;
сканирование больничных каталогов, CT и XA polling отключены. Тестовые DICOM
можно импортировать через OHIF или REST API Orthanc, затем проверять команды
агента на изолированном PACS без доступа к больничной сети.
