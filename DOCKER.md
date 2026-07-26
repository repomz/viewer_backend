# Docker и единый серверный стек

## Полный запуск одной командой

Первичная настройка:

```bash
cd viewer_backend
cp .env.compose.example .env
```

Запуск PostgreSQL, миграций, backend, Orthanc PACS и OHIF:

```bash
docker compose up -d --build --wait --remove-orphans
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

Состояние PostgreSQL, Orthanc и JSON-отчётов хранится в named volumes. Остановка
без удаления данных:

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
docker compose logs -f backend pacs ohif
```

## Образы из registry

В `.env` можно заменить `BACKEND_IMAGE` и `BACKEND_MIGRATIONS_IMAGE` на
адреса опубликованных образов. На сервере с исходниками доступна одна команда
с принудительным получением свежих образов:

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
- каталог отчётов: `REPORTS_DIR`;
- remote PACS: `REMOTE_PACS_URL`, `REMOTE_PACS_USERNAME`,
  `REMOTE_PACS_PASSWORD`, `REMOTE_PACS_TIMEOUT_SECONDS`;
- healthcheck: `GET /`;
- миграции: `/app/migrations`.

## Подключение больничного агента

Hospital agent не запускается на сервере в Docker. Он работает непосредственно
на больничном Windows-компьютере через `pythonw`.

В его `agent_config.json` нужно указать:

```json
{
  "viewer_url": "http://SERVER:8080"
}
```

В `config.json` агента настраивается локальный больничный PACS, из которого
агент выполняет C-FIND/C-GET:

```json
{
  "pacs": {
    "ip": "HOSPITAL_PACS_IP",
    "port": 11112,
    "ae_title": "HOSPITAL_PACS_AE"
  }
}
```

Импорт в серверный Orthanc выполняет backend. Его адрес и учётные данные
задаются в `.env` через `REMOTE_PACS_*`, а не в пользовательской команде.
