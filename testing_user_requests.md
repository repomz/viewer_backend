# Проверка пользовательских команд Hospital Agent

Примеры соответствуют текущему контракту backend и агента. Выполняйте их с
компьютера, имеющего доступ к backend:

```bash
BACKEND_URL="http://135.106.130.37:8080"
AGENT_ID=2
```

API пока не имеет аутентификации. Порт `8080` должен быть доступен только из
доверенной больничной сети или VPN.

## Общий жизненный цикл

Создание команды выполняется через `POST /user_requests`. В запросе используется
только поле `command`; `action`, `type` и `request_type` не нужны.

После создания сохраните ID:

```bash
RESPONSE=$(curl -sS --fail-with-body \
  -X POST "$BACKEND_URL/user_requests" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"command\":\"get_report\",\"payload\":{\"period\":1},\"max_attempts\":3}")

REQUEST_ID=$(printf '%s' "$RESPONSE" | jq -r '.id')
printf '%s\n' "$RESPONSE" | jq
echo "Request ID: $REQUEST_ID"
```

Проверка состояния:

```bash
curl -sS --fail-with-body \
  "$BACKEND_URL/user_requests/$REQUEST_ID" |
  jq '{id,status,attempt_count,max_attempts,result,errors}'
```

Ожидание terminal-статуса:

```bash
while true; do
  RESPONSE=$(curl -sS --fail-with-body \
    "$BACKEND_URL/user_requests/$REQUEST_ID")
  printf '%s\n' "$RESPONSE" |
    jq '{status,attempt_count,max_attempts,result,errors}'
  STATUS=$(printf '%s' "$RESPONSE" | jq -r '.status')
  [[ "$STATUS" == "completed" || "$STATUS" == "error" ]] && break
  sleep 3
done
```

Статусы:

- `pending` — команда ожидает агента или повторной попытки;
- `in_progress` — агент забрал команду и выполняет её;
- `completed` — команда выполнена;
- `error` — окончательная ошибка, текст находится в поле `errors`.

Не вызывайте вручную:

```text
GET /user_requests?agent_id=2
```

Этот endpoint не показывает список: он атомарно забирает следующую команду в
работу, увеличивает `attempt_count` и устанавливает lease.

## get_report

Создать отчёт за одно предыдущее дежурство:

```bash
curl -sS --fail-with-body \
  -X POST "$BACKEND_URL/user_requests" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"command\":\"get_report\",\"payload\":{\"period\":1},\"max_attempts\":3}" |
  jq
```

`period` — целое число от 1 до 4. Пути и время передавать нельзя: каталоги берутся
из `agent_config.json`, а граница дежурства всегда равна 08:00. TXT сохраняется
на больничном компьютере в `report_dir`, JSON — на backend через `/reports`.

Просмотр последних отчётов:

```bash
curl -sS --fail-with-body "$BACKEND_URL/reports?limit=20" | jq
```

Получение конкретного отчёта:

```bash
REPORT_FILE=$(curl -sS --fail-with-body "$BACKEND_URL/reports?limit=1" |
  jq -r '.[0].filename')
curl -sS --fail-with-body "$BACKEND_URL/reports/$REPORT_FILE" | jq
```

## find_ct и find_xa

Поиск выполняется в локальном больничном PACS по фамилии и периоду. Допустимы
`today`, `yesterday`, `week`, `month` или конкретная дата `YYYY-MM-DD`.

```bash
curl -sS --fail-with-body \
  -X POST "$BACKEND_URL/user_requests" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"command\":\"find_ct\",\"payload\":{\"patient\":\"Иванов\",\"period\":\"week\"},\"max_attempts\":3}" |
  jq
```

```bash
curl -sS --fail-with-body \
  -X POST "$BACKEND_URL/user_requests" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"command\":\"find_xa\",\"payload\":{\"patient\":\"Иванов\",\"period\":\"2026-07-26\"},\"max_attempts\":3}" |
  jq
```

В `result.studies` после выполнения будут ФИО, дата, описание и
`StudyInstanceUID`. Полученный UID используется в `get_ct` или `get_xa`.

## get_ct и get_xa

Замените UID на значение, выбранное из результата `find_ct`/`find_xa`:

```bash
STUDY_UID="1.2.840.113619.2.55.3.604688123.123.1710000000.1"
```

```bash
curl -sS --fail-with-body \
  -X POST "$BACKEND_URL/user_requests" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"command\":\"get_ct\",\"payload\":{\"study_uid\":\"$STUDY_UID\"},\"max_attempts\":3}" |
  jq
```

Для XA изменяется только команда:

```bash
curl -sS --fail-with-body \
  -X POST "$BACKEND_URL/user_requests" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"command\":\"get_xa\",\"payload\":{\"study_uid\":\"$STUDY_UID\"},\"max_attempts\":3}" |
  jq
```

Агент выполняет прямой C-GET, проверяет полноту исследования, загружает все
DICOM в Yandex и отправляет метаданные на `/ct_studies` или `/xa_studies`.
Backend импортирует каждый файл в remote PACS до перевода команды в
`completed`. Частичная передача завершается ошибкой.

На больничном компьютере должны быть настроены:

```text
YANDEX_ACCESS_KEY_ID
YANDEX_SECRET_ACCESS_KEY
YANDEX_BUCKET
YANDEX_ENDPOINT
```

На backend должны быть настроены `REMOTE_PACS_URL`,
`REMOTE_PACS_USERNAME`, `REMOTE_PACS_PASSWORD` и
`REMOTE_PACS_TIMEOUT_SECONDS`.

## find_study

Поиск стандартизованных протоколов операций по фамилии в настроенных каталогах:

```bash
curl -sS --fail-with-body \
  -X POST "$BACKEND_URL/user_requests" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"command\":\"find_study\",\"payload\":{\"patient\":\"Иванов\"},\"max_attempts\":3}" |
  jq
```

Результат содержит уже сокращённые `name_operation` и `descr_operation`. Локальные
пути DOCX backend не получает.

## Управление CT/XA polling

Включение:

```bash
for COMMAND in ct_polling_on xa_polling_on; do
  curl -sS --fail-with-body \
    -X POST "$BACKEND_URL/user_requests" \
    -H "Content-Type: application/json" \
    -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"command\":\"$COMMAND\",\"payload\":{},\"max_attempts\":3}" |
    jq
done
```

Выключение:

```bash
for COMMAND in ct_polling_off xa_polling_off; do
  curl -sS --fail-with-body \
    -X POST "$BACKEND_URL/user_requests" \
    -H "Content-Type: application/json" \
    -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"command\":\"$COMMAND\",\"payload\":{},\"max_attempts\":3}" |
    jq
done
```

Включённый polling обрабатывает новые исследования от момента включения до
ближайших 08:00, после чего автоматически выключается. Состояние сохраняется в
`agent_config.json`.

## Диагностика очереди в PostgreSQL

Просмотр последних команд:

```bash
docker compose exec -T postgres sh -c \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "
  SELECT id, created_at, agent_id, command, status,
         attempt_count, max_attempts, error_log
  FROM user_requests
  ORDER BY created_at DESC
  LIMIT 50;"'
```

Статистика:

```bash
docker compose exec -T postgres sh -c \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "
  SELECT status, count(*)
  FROM user_requests
  GROUP BY status
  ORDER BY status;"'
```

Удаление только завершённых команд:

```bash
docker compose exec -T postgres sh -c \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "
  DELETE FROM user_requests
  WHERE status IN ('\''completed'\'', '\''error'\'');"'
```

Полная очистка очереди, включая `pending` и `in_progress`:

```bash
docker compose exec -T postgres sh -c \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "
  TRUNCATE TABLE user_requests;"'
```

Последние две операции изменяют данные. Перед запуском убедитесь, что результаты
больше не нужны. HTTP endpoint для удаления очереди отсутствует.
