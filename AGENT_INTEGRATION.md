# Интеграция viewer_backend и hospital_agent

Дата: 19 июля 2026 года.

Проекты:

- backend: `/Users/marat/projects/viewer/viewer_backend`;
- больничный агент: `/Users/marat/projects/viewer/agent`.

## Назначение

Backend хранит очередь команд. Агент с конкретным `agent_id` периодически забирает только своё следующее задание, выполняет разрешённую операцию на больничном компьютере и подтверждает результат.

Поддержанные команды:

- `send_study_to_yandex`;
- `send_dicom_to_mapdr`;
- `generate_operations_report`.

Произвольная shell-команда не поддерживается и не должна добавляться в этот протокол.

## Жизненный цикл

1. Viewer/admin создаёт `POST /user_requests`.
2. Запрос сохраняется как `pending`.
3. Агент вызывает `GET /user_requests?agent_id=2`.
4. PostgreSQL атомарно выбирает одно задание через `FOR UPDATE SKIP LOCKED`, переводит его в `in_process`, увеличивает `attempt_count` и выдаёт lease на 5 минут.
5. Backend добавляет `response_endpoint` в ответ.
6. Агент выполняет команду.
7. Агент отправляет результат в `POST /user_requests/{id}/result`.
8. Успех переводит запрос в `completed`.
9. Ошибка с `retryable=true` возвращает запрос в `pending` на 30 секунд, пока не исчерпан `max_attempts`.
10. Невосстановимая ошибка или исчерпание попыток переводит запрос в `failed`.

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
  "request_type": "execute_command",
  "command": "send_study_to_yandex",
  "payload": {
    "study_uid": "1.2.840.113619.2.55.3.604688435.123"
  },
  "max_attempts": 3
}
```

Backend отклоняет неизвестную команду. Для `send_study_to_yandex` обязателен `payload.study_uid`, для `send_dicom_to_mapdr` — `payload.dicom_path`.

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
  "request_type": "execute_command",
  "command": "send_study_to_yandex",
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
  "error": null
}
```

## Временная ошибка

```json
{
  "agent_id": 2,
  "ok": false,
  "retryable": true,
  "result": {},
  "error": "PACS is temporarily unavailable"
}
```

## Невосстановимая ошибка

```json
{
  "agent_id": 2,
  "ok": false,
  "retryable": false,
  "result": {},
  "error": "study_uid is required"
}
```

## Просмотр состояния

```http
GET /user_requests/11111111-1111-1111-1111-111111111111
```

Ответ содержит `status`, `attempt_count`, `max_attempts`, `result`, `error`, временные метки и lease.

## Протоколы операций

Агент также отправляет разобранные DOCX-протоколы в `/study` или `/studies`. Payload синхронизирован с backend:

- удалены клиентские `id`, `created_at`, `updated_at`;
- используется корректное поле `time_beginning`;
- добавлен обязательный `study_type`;
- хирург нормализуется до фамилии;
- отсутствующие описание/отделение получают допустимое значение.

Все 14 локальных тестовых DOCX успешно преобразованы в backend-контракт без раскрытия данных пациентов в отчёте проверки.

## Перед первым запуском

1. Применить миграцию:

   ```bash
   goose -dir internal/sql/migrations postgres "$DB_DSN" up
   ```

2. Убедиться, что `agent_config.json` содержит числовой `agent_id`, совпадающий с адресатом команд.
3. Запустить backend.
4. Выполнить read-only проверку `GET /user_requests?agent_id=2`.
5. Создать безопасную тестовую команду и проверить переходы `pending → in_process → completed`.

## Обязательные меры безопасности

Текущий backend пока не аутентифицирует viewer и агента. До подключения к недоверенной сети необходимо:

- ввести отдельный credential каждого агента;
- защитить постановку команд ролью admin/operator;
- использовать HTTPS/mTLS либо защищённый VPN;
- ограничить локальные пути, допустимые для `send_dicom_to_mapdr`;
- запретить переопределение MAPDR host/credentials из команды;
- вести аудит постановки и выполнения команд без записи медицинских данных в обычные логи.

Без этих мер endpoint постановки команд нельзя публиковать в интернет или общую больничную сеть.

## Выполненная проверка

В изолированном PostgreSQL-кластере успешно проверено:

- применение всех миграций;
- создание команды;
- выдача команды только `agent_id=2`;
- перенос полей `payload` в контракт агента;
- запись успешного JSON result;
- идемпотентная повторная отправка result;
- отсутствие завершённой команды в следующем polling.

Также проходят Go race-тесты backend, Python unit-тесты агента и преобразование всех 14 тестовых DOCX в актуальный `StudyRequest`.
