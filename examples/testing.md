```bash
# Миграции:
goose -dir ./internal/sql/migrations postgres "postgres://marat:postgresd@localhost:5432/angio_db?sslmode=disable" status
goose -dir ./internal/sql/migrations postgres "postgres://marat:postgresd@localhost:5432/angio_db?sslmode=disable" up
goose -dir ./internal/sql/migrations postgres "postgres://marat:postgresd@localhost:5432/angio_db?sslmode=disable" down
```

```bash
# Запуск API:
HTTP_ADDR=:8080 \
DB_DSN="postgres://marat:postgresd@localhost:5432/angio_db?sslmode=disable" \
DEBUG_ERRORS=1 \
go run ./cmd/main.go
```

```bash
# Проверка, что API жив
curl -sS http://localhost:8080/

# Просмотр всех studies в базе данных, ожидается 30 активных записей
curl -sS http://localhost:8080/studies | jq 'length'

# Получение конкретного study по uuid
curl -sS http://localhost:8080/studies/b28ca40a-6943-4404-b797-33b4b43e5c4e \
  | jq '{id, patient, surgeon, study_type}'

# Получение study по пациенту. Для кириллицы и пробелов используем URL-encoding.
patient=$(jq -nr --arg v 'Николаев П.С.' '$v|@uri')
curl -sS "http://localhost:8080/studies/patient/$patient" \
  | jq '{id, patient, surgeon, study_type}'
```

```bash
# Фильтрация studies

# Поиск по дате, ожидается 3
curl -sS -G http://localhost:8080/studies \
  --data-urlencode 'date=2025-05-13' \
  | jq 'length'

# Поиск по хирургу, ожидается 4
curl -sS -G http://localhost:8080/studies \
  --data-urlencode 'surgeon=идрисов' \
  | jq 'length'

# Поиск по типу операции, ожидается 4
curl -sS -G http://localhost:8080/studies \
  --data-urlencode 'type=каг' \
  | jq 'length'

# Поиск по дате + хирург
curl -sS -G http://localhost:8080/studies \
  --data-urlencode 'date=2025-05-13' \
  --data-urlencode 'surgeon=идрисов' \
  | jq 'length'

# Поиск по дате + тип операции
curl -sS -G http://localhost:8080/studies \
  --data-urlencode 'date=2025-05-13' \
  --data-urlencode 'type=стент_кор' \
  | jq 'length'

# Поиск по хирургу + тип операции
curl -sS -G http://localhost:8080/studies \
  --data-urlencode 'surgeon=идрисов' \
  --data-urlencode 'type=стент_кор' \
  | jq 'length'

# Поиск по всем трём фильтрам, ожидается 1
curl -sS -G http://localhost:8080/studies \
  --data-urlencode 'date=2025-05-13' \
  --data-urlencode 'surgeon=идрисов' \
  --data-urlencode 'type=стент_кор' \
  | jq 'length'
```

```bash
# Создание временного исследования, обновление dicom_link и удаление только этой временной записи
created=$(curl -sS -X POST -H "Content-Type: application/json" \
  -d '{
    "study_id": "TEST-CURL-001",
    "patient": "Тестовый Пациент",
    "age": 50,
    "department": "тест",
    "name_operation": "Тестовая операция",
    "study_type": "каг",
    "descr_operation": "Проверка curl",
    "time_beginning": "2026-06-12T11:13:00Z",
    "time_duration": 15,
    "surgeon": "идрисов",
    "dicom_link": "https://pacs/dicom/test-curl-001"
  }' \
  http://localhost:8080/studies)

echo "$created" | jq '{id, study_id, patient, dicom_link}'
id=$(echo "$created" | jq -r '.id')

curl -sS -X PATCH -H "Content-Type: application/json" \
  -d '{"dicom_link": "https://new-link.com"}' \
  "http://localhost:8080/studies/$id/dicom-link" \
  | jq '{id, dicom_link}'

curl -sS -X DELETE "http://localhost:8080/studies/$id" | jq

# После удаления временной записи снова ожидается 30
curl -sS http://localhost:8080/studies | jq 'length'
```

```bash
# Посмотреть структуру ответа
curl -sS http://localhost:8080/studies | jq 'type'

# Посмотреть первый элемент
curl -sS http://localhost:8080/studies | jq '.[0]'
```

```bash
# ОПАСНО: удаление всех studies в базе данных.
# Не запускать при проверке заполненной базы на 30 пациентов.
curl -sS -X DELETE http://localhost:8080/studies | jq
```


```bash
sqlc generate -f sqlc/sqlc.yaml
./bin/rest app generate

go test ./...
go vet ./...
```
