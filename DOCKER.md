# Docker и единый серверный стек

## Полный запуск одной командой

Первичная настройка:

```bash
cd viewer_backend
cp .env.compose.example .env
```

Запуск PostgreSQL, миграций, backend, Orthanc PACS и frontend:

```bash
docker compose up -d --build --wait --remove-orphans
```

После запуска:

- backend: `http://SERVER:8080`;
- frontend: `http://SERVER/` (также сохранён технический порт `5173`);
- Orthanc Explorer: `http://SERVER:8042`;
- DICOM PACS: AE Title `MAPDR`, порт `4242`;
- Orthanc login: `mapdr`, password: `changestrongpassword`.

Перед публикацией сервера в интернет нужно заменить пароль Orthanc в
`orthanc/orthanc.json`, обновить `PACS_AUTHORIZATION` и закрыть HTTP
TLS reverse proxy.

Состояние PostgreSQL, Orthanc, JSON-отчётов, редактируемого плана и
подготовленных XA-кадров и MP4 cine по сериям хранится в named volumes. Остановка без удаления
данных:

```bash
docker compose down
```

Полная очистка тестовых данных:

```bash
docker compose down --volumes
```

> **Осторожно:** ключ `--volumes` удаляет named volumes PostgreSQL, Orthanc,
> отчётов, плана и серверного XA-кэша. Вместе с ними будут удалены протоколы,
> XA/CT в PACS, сохранённые отчёты и заполненный план. Для обычного обновления
> используйте `docker compose down` без этого ключа.

Просмотр состояния и логов:

```bash
docker compose ps
docker compose logs -f backend pacs frontend
```

После обновления образов XA, уже находящиеся в Orthanc, автоматически
поставятся в очередь фоновой подготовки. Новые XA добавляются в очередь сразу
после импорта. Прогресс виден в логах backend по строкам `XA cine cache`.

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
- идентификатор владельца автоматически формируемых отчётов: `REPORT_AGENT_ID`;
- сервис `reports-init` перед запуском backend назначает каталогу отчётов
  владельца UID/GID `10001`;
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
