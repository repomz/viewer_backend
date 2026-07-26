# Локальная проверка viewer_backend

## Миграции и запуск

```bash
export DB_DSN="postgres://viewer:viewer-local-password@localhost:5432/viewer?sslmode=disable"
goose -dir ./internal/sql/migrations postgres "$DB_DSN" status
goose -dir ./internal/sql/migrations postgres "$DB_DSN" up
```

```bash
HTTP_ADDR=:8080 DB_DSN="$DB_DSN" go run ./cmd
```

В другом терминале:

```bash
BACKEND_URL="http://localhost:8080"
curl -sS --fail-with-body "$BACKEND_URL/"
```

## Чтение и фильтрация исследований

```bash
curl -sS --fail-with-body \
  "$BACKEND_URL/studies?page=1&page_size=10" |
  jq
```

```bash
curl -sS --fail-with-body -G "$BACKEND_URL/studies/search" \
  --data-urlencode "study_date=2026-07-26" \
  --data-urlencode "surgeon=идрисов" \
  --data-urlencode "study_type=каг" |
  jq
```

```bash
PATIENT=$(jq -nr --arg value "Иванов И.И." '$value|@uri')
curl -sS --fail-with-body \
  "$BACKEND_URL/studies/patient/$PATIENT" |
  jq
```

## Безопасный CRUD-тест одной временной записи

```bash
CREATED=$(curl -sS --fail-with-body \
  -X POST "$BACKEND_URL/studies" \
  -H "Content-Type: application/json" \
  -d '{
    "study_id":"TEST-EXAMPLE-001",
    "patient":"Тестовый Пациент",
    "age":50,
    "department":"тест",
    "name_operation":"Тестовая операция",
    "study_type":"тест",
    "descr_operation":"Локальная проверка API",
    "time_beginning":"2026-07-26T11:13:00Z",
    "time_duration":15,
    "surgeon":"тестовый",
    "dicom_link":""
  }' \
  "$BACKEND_URL/studies")

printf '%s\n' "$CREATED" | jq
STUDY_UUID=$(printf '%s' "$CREATED" | jq -r '.id')
```

```bash
curl -sS --fail-with-body \
  -X PATCH "$BACKEND_URL/studies/$STUDY_UUID/dicom-link" \
  -H "Content-Type: application/json" \
  -d '{"dicom_link":"https://example.invalid/test"}' |
  jq
```

```bash
curl -sS --fail-with-body \
  -X DELETE "$BACKEND_URL/studies/$STUDY_UUID" |
  jq
```

## Проверка исходников

```bash
sqlc generate -f sqlc.yaml
go test ./...
go test -race ./...
go vet ./...
go build ./cmd
docker compose config -q
git diff --check
```

Примеры очереди агента находятся в `testing_user_requests.md`, а расширенные
примеры исследований и отчётов — в `testing_studies.md`.
