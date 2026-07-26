# Проверка studies, CT/XA и отчётов

```bash
BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
```

Все записывающие примеры ниже изменяют данные. Используйте тестовый backend или
тестовые идентификаторы.

## Доступность и список

```bash
curl -sS --fail-with-body "$BACKEND_URL/"
curl -sS --fail-with-body "$BACKEND_URL/studies?page=1&page_size=10" | jq
```

`page` должен быть положительным, `page_size` — от 1 до 100.

## Получение по UUID и пациенту

```bash
STUDY_UUID="d5c6821b-e85a-43e0-9313-7bf8b46993ed"
curl -sS --fail-with-body "$BACKEND_URL/studies/$STUDY_UUID" | jq
```

```bash
PATIENT=$(jq -nr --arg value "Иванов И.И." '$value|@uri')
curl -sS --fail-with-body \
  "$BACKEND_URL/studies/patient/$PATIENT" |
  jq
```

## Фильтрация

Фильтры передаются только в `/studies/search`:

```bash
curl -sS --fail-with-body -G "$BACKEND_URL/studies/search" \
  --data-urlencode "study_date=2026-07-26" |
  jq
```

```bash
curl -sS --fail-with-body -G "$BACKEND_URL/studies/search" \
  --data-urlencode "surgeon=идрисов" \
  --data-urlencode "study_type=каг" |
  jq
```

Можно совместно использовать `study_date`, `surgeon` и `study_type`. Хирург и
тип не проверяются по фиксированному справочнику.

## Создание, обновление ссылки и мягкое удаление

```bash
CREATED=$(curl -sS --fail-with-body \
  -X POST "$BACKEND_URL/studies" \
  -H "Content-Type: application/json" \
  -d '{
    "study_id": "TEST-CURL-001",
    "patient": "Тестовый Пациент",
    "age": 50,
    "department": "тест",
    "name_operation": "Тестовая операция",
    "study_type": "тест",
    "descr_operation": "Проверка контракта studies",
    "time_beginning": "2026-07-26T11:13:00Z",
    "time_duration": 15,
    "surgeon": "тестовый",
    "dicom_link": ""
  }' \
  "$BACKEND_URL/studies")

printf '%s\n' "$CREATED" | jq
STUDY_UUID=$(printf '%s' "$CREATED" | jq -r '.id')
```

```bash
curl -sS --fail-with-body \
  -X PATCH "$BACKEND_URL/studies/$STUDY_UUID/dicom-link" \
  -H "Content-Type: application/json" \
  -d '{"dicom_link":"https://example.invalid/dicom/TEST-CURL-001"}' |
  jq
```

```bash
curl -sS --fail-with-body \
  -X DELETE "$BACKEND_URL/studies/$STUDY_UUID" |
  jq
```

Удаление мягкое: запись остаётся в PostgreSQL с `deleted=true`, но перестаёт
возвращаться обычными запросами.

## Входной контракт CT/XA

Эти endpoint’ы предназначены для агента. Они скачивают указанные DICOM и
импортируют их в remote PACS, поэтому пример с фиктивным URL ожидаемо завершится
ошибкой. Для реального теста нужны доступные временные URL DICOM:

```bash
curl -sS --fail-with-body \
  -X POST "$BACKEND_URL/ct_studies" \
  -H "Content-Type: application/json" \
  -d '{
    "study_uid": "1.2.840.10008.1.2.3.4.5",
    "patient": "Тестовый Пациент",
    "age": 50,
    "study_date": "20260726",
    "study_time": "111300",
    "description": "Тестовая КТ",
    "modality": "CT",
    "dicom_link": "s3://bucket/test-study",
    "dicom_files": [
      {
        "name": "1.dcm",
        "size": 12345,
        "url": "https://SIGNED_YANDEX_URL/1.dcm"
      }
    ]
  }' |
  jq
```

Для `/xa_studies` поле `modality` должно быть `XA`.

## Отчёты

```bash
curl -sS --fail-with-body "$BACKEND_URL/reports?limit=20" | jq
```

```bash
REPORT_FILE=$(curl -sS --fail-with-body "$BACKEND_URL/reports?limit=1" |
  jq -r '.[0].filename')
curl -sS --fail-with-body "$BACKEND_URL/reports/$REPORT_FILE" | jq
```

Массовое `DELETE /studies` скрывает все исследования. Его нельзя запускать на
рабочей базе без явного решения администратора.
