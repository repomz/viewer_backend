# Интеграция viewer_backend и hospital_agent

Дата актуализации: 26 июля 2026 года.

Проекты:

- backend: `/Users/marat/projects/viewer/viewer_backend`;
- больничный агент: `/Users/marat/projects/viewer/agent`.

## Назначение

Backend хранит очередь команд. Агент с конкретным `agent_id` периодически забирает только своё следующее задание, выполняет разрешённую операцию на больничном компьютере и подтверждает результат.

Поддержанные команды:

- `get_report`;
- `find_study`, `find_xa`, `find_ct`;
- `get_xa`, `get_ct`;
- `xa_polling_on`, `xa_polling_off`;
- `ct_polling_on`, `ct_polling_off`.

Произвольная shell-команда не поддерживается и не должна добавляться в этот протокол.
Имя команды определяется только по полю `command`; поля `action` и `type`
не являются альтернативами.

## Жизненный цикл

1. Viewer/admin создаёт `POST /user_requests`.
2. Запрос сохраняется как `pending`.
3. Агент вызывает `GET /user_requests?agent_id=2`.
4. PostgreSQL атомарно выбирает одно задание через `FOR UPDATE SKIP LOCKED`, переводит его в `in_progress`, увеличивает `attempt_count` и выдаёт lease на 5 минут.
5. Backend добавляет `response_endpoint` в ответ.
6. Агент выполняет команду.
7. Агент отправляет результат в `POST /user_requests/{id}/result`.
8. Успех переводит запрос в `completed`.
9. Ошибка с `retryable=true` возвращает запрос в `pending` на 30 секунд, пока не исчерпан `max_attempts`.
10. Невосстановимая ошибка или исчерпание попыток переводит запрос в `error`.

Если агент завершил команду, но не получил подтверждение callback, он сохраняет terminal result в локальном `state_file`. При повторной выдаче задания агент сначала повторяет callback и не выполняет команду второй раз.

Повторная отправка уже записанного результата идемпотентна.

## Создание команды

```http
POST /user_requests
Content-Type: application/json
```

```json
{
  "user_id": "operator-42",
  "agent_id": 2,
  "command": "get_ct",
  "payload": {
    "study_uid": "1.2.840.113619.2.55.3.604688435.123"
  },
  "max_attempts": 3
}
```

Backend отклоняет неизвестную команду. Для `get_xa`/`get_ct` обязателен
`payload.study_uid`; для команд поиска — `payload.patient`. `get_report` принимает
только `payload.period` от 1 до 4.

## Получение команды агентом

```http
GET /user_requests?agent_id=2
Accept: application/json
```

Пример ответа:

```json
{
  "id": "11111111-1111-1111-1111-111111111111",
  "request_id": "11111111-1111-1111-1111-111111111111",
  "agent_id": 2,
  "command": "get_ct",
  "study_uid": "1.2.840.113619.2.55.3.604688435.123",
  "attempt_count": 1,
  "max_attempts": 3,
  "response_endpoint": "/user_requests/11111111-1111-1111-1111-111111111111/result"
}
```

Когда заданий нет, backend возвращает HTTP 200 и пустой объект `{}`.

## Успешный результат

```http
POST /user_requests/11111111-1111-1111-1111-111111111111/result
Content-Type: application/json
```

```json
{
  "agent_id": 2,
  "ok": true,
  "retryable": false,
  "result": {
    "uploaded_files": 120,
    "uploaded_bytes": 987654321
  },
  "errors": null
}
```

## Временная ошибка

```json
{
  "agent_id": 2,
  "ok": false,
  "retryable": true,
  "result": {},
  "errors": "PACS is temporarily unavailable"
}
```

## Невосстановимая ошибка

```json
{
  "agent_id": 2,
  "ok": false,
  "retryable": false,
  "result": {},
  "errors": "study_uid is required"
}
```

## Просмотр состояния

```http
GET /user_requests/11111111-1111-1111-1111-111111111111
```

Ответ содержит `status`, `attempt_count`, `max_attempts`, `result`, `errors`,
временные метки и lease.

## Протоколы операций

Агент отправляет разобранные DOCX-протоколы только в `/studies`. Payload синхронизирован с backend:

- удалены клиентские `id`, `created_at`, `updated_at`;
- используется корректное поле `time_beginning`;
- добавлен обязательный `study_type`;
- хирург нормализуется до фамилии;
- отсутствующие описание/отделение получают допустимое значение.

## Перед первым запуском

1. Применить миграцию:

   ```bash
   goose -dir internal/sql/migrations postgres "$DB_DSN" up
   ```

2. Убедиться, что `agent_config.json` содержит числовой `agent_id`, совпадающий с адресатом команд.
3. Запустить backend.
4. Проверить доступность backend через `GET /`. Не использовать claim endpoint
   как read-only проверку.
5. Создать безопасную тестовую команду и проверить переходы `pending → in_progress → completed`.

## Обязательные меры безопасности

Текущий backend пока не аутентифицирует viewer и агента. До подключения к недоверенной сети необходимо:

- ввести отдельный credential каждого агента;
- защитить постановку команд ролью admin/operator;
- использовать HTTPS/mTLS либо защищённый VPN;
- не принимать локальные пути и адрес remote PACS из пользовательской команды;
- задавать remote PACS и его credential только окружением backend;
- вести аудит постановки и выполнения команд без записи медицинских данных в обычные логи.

Без этих мер endpoint постановки команд нельзя публиковать в интернет или общую больничную сеть.

## Выполненная проверка

Автоматическими тестами проверены контракты новых команд, статусы и поле `errors`,
строгий импорт всех DICOM в remote PACS до сохранения записи, отказ без настройки
remote PACS и хранение/чтение JSON-отчётов. Реальные PACS и Yandex требуют отдельной
интеграционной проверки в больничной сети.
