Для вашего текущего адреса backend и агента №2:

```bash
BACKEND_URL="http://135.106.130.37:8080"
AGENT_ID=2
```

Важно: порт `8080` должен быть доступен только из доверенной сети/VPN — API пока не имеет авторизации.

## 1. Создать отчёт за предыдущий операционный день

```bash
curl -sS --fail-with-body -X POST "$BACKEND_URL/user_requests" -H "Content-Type: application/json" -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"request_type\":\"execute_command\",\"command\":\"generate_operations_report\",\"payload\":{\"period\":1,\"time\":\"08:00\"},\"max_attempts\":3}" | jq
```

Команда создаёт отчёт от вчерашнего дня `08:00` до текущего момента. Это не строго календарные сутки `00:00–23:59` — текущая реализация агента задаёт начало периода, а концом всегда считает момент выполнения.

Файлы появятся на больничном компьютере в стандартном каталоге:

```text
C:\Users\Angio_hir1\Desktop\План Отчеты\отчеты
```

Если нужны явные пути:

```bash
curl -sS --fail-with-body -X POST "$BACKEND_URL/user_requests" -H "Content-Type: application/json" -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"command\":\"generate_operations_report\",\"payload\":{\"period\":1,\"time\":\"08:00\",\"dir1\":\"C:\\\\Users\\\\Angio_hir1\\\\Desktop\\\\Операции 2026\",\"dir2\":\"C:\\\\Users\\\\Angio_hir1\\\\Desktop\\\\2026 Опер №2\",\"plan_dir\":\"C:\\\\Users\\\\Angio_hir1\\\\Desktop\\\\План Отчеты\",\"report_dir\":\"C:\\\\Users\\\\Angio_hir1\\\\Desktop\\\\План Отчеты\\\\отчеты\"},\"max_attempts\":3}" | jq
```

## 2. Сохранить ID и следить за выполнением

```bash
RESPONSE=$(curl -sS --fail-with-body -X POST "$BACKEND_URL/user_requests" -H "Content-Type: application/json" -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"command\":\"generate_operations_report\",\"payload\":{\"period\":1,\"time\":\"08:00\"},\"max_attempts\":3}")

REQUEST_ID=$(printf '%s' "$RESPONSE" | jq -r '.id')

echo "Request ID: $REQUEST_ID"
printf '%s\n' "$RESPONSE" | jq
```

Проверка результата:

```bash
curl -sS --fail-with-body "$BACKEND_URL/user_requests/$REQUEST_ID" | jq '{id,status,attempt_count,result,error}'
```

Автоматическое ожидание завершения:

```bash
while true; do RESPONSE=$(curl -sS --fail-with-body "$BACKEND_URL/user_requests/$REQUEST_ID"); printf '%s\n' "$RESPONSE" | jq '{status,attempt_count,result,error}'; STATUS=$(printf '%s' "$RESPONSE" | jq -r '.status'); [[ "$STATUS" == "completed" || "$STATUS" == "failed" ]] && break; sleep 3; done
```

Статусы:

- `pending` — ожидает агента;
- `in_process` — агент получил команду;
- `completed` — успешно выполнена;
- `failed` — выполнение окончательно завершилось ошибкой.

Не вызывайте вручную:

```text
GET /user_requests?agent_id=2
```

Этот запрос не показывает список, а забирает следующую команду в работу и устанавливает lease.

## 3. Отчёт за последние 7 дней

```bash
curl -sS --fail-with-body -X POST "$BACKEND_URL/user_requests" -H "Content-Type: application/json" -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"command\":\"generate_operations_report\",\"payload\":{\"period\":7,\"time\":\"08:00\"},\"max_attempts\":3}" | jq
```

## 4. Отправить локальный DICOM с больничного компьютера в Orthanc

Путь относится именно к больничному компьютеру, где работает `pythonw`:

```bash
curl -sS --fail-with-body -X POST "$BACKEND_URL/user_requests" -H "Content-Type: application/json" -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"command\":\"send_dicom_to_mapdr\",\"payload\":{\"dicom_path\":\"C:\\\\DICOM\\\\test-study\",\"mapdr_host\":\"135.106.130.37\",\"mapdr_port\":8042,\"mapdr_username\":\"mapdr\",\"mapdr_password\":\"ПАРОЛЬ_ORTHANC\"},\"max_attempts\":3}" | jq
```

Пароль сохранится в `payload` таблицы `user_requests` в открытом виде. Для production лучше доработать агент так, чтобы он брал учётные данные Orthanc из локального `config.json`, а не из команды.

## 5. Скачать исследование из PACS и отправить в Yandex

Замените UID на настоящий `StudyInstanceUID`:

```bash
curl -sS --fail-with-body -X POST "$BACKEND_URL/user_requests" -H "Content-Type: application/json" -d "{\"user_id\":\"terminal-test\",\"agent_id\":$AGENT_ID,\"command\":\"send_study_to_yandex\",\"payload\":{\"study_uid\":\"1.2.840.113619.2.55.3.604688123.123.1710000000.1\"},\"max_attempts\":3}" | jq
```

На больничном компьютере должны быть доступны PACS и переменные окружения Yandex:

```text
YANDEX_ACCESS_KEY_ID
YANDEX_SECRET_ACCESS_KEY
YANDEX_BUCKET
YANDEX_ENDPOINT
```

## 6. Посмотреть команды непосредственно в PostgreSQL

На сервере:

```bash
cd /opt/viewer/viewer_backend

docker compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT id, created_at, agent_id, command, status, attempt_count, error_log FROM user_requests ORDER BY created_at DESC;"'
```

Статистика:

```bash
docker compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT status, count(*) FROM user_requests GROUP BY status ORDER BY status;"'
```

## 7. Очистить выполненные и ошибочные команды

Сначала убедитесь, что нужные результаты сохранены:

```bash
docker compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "DELETE FROM user_requests WHERE status IN ('\''completed'\'', '\''failed'\'');"'
```

## 8. Полностью очистить все команды

Включая `pending` и `in_process`:

```bash
docker compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "TRUNCATE TABLE user_requests;"'
```

Проверка:

```bash
docker compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT count(*) AS remaining_requests FROM user_requests;"'
```

Через `curl` очистить очередь сейчас нельзя: backend не предоставляет `DELETE /user_requests`. Очистка выполняется непосредственно в PostgreSQL.