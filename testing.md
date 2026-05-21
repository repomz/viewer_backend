```bash
# Миграции:
goose -dir ./internal/sql/migrations postgres "postgres://marat:postgres@localhost:5432/angio_db" reset
goose -dir ./internal/sql/migrations postgres "postgres://marat:postgres@localhost:5432/angio_db" status
goose -dir ./internal/sql/migrations postgres "postgres://marat:postgres@localhost:5432/angio_db" up
goose -dir ./internal/sql/migrations postgres "postgres://marat:postgres@localhost:5432/angio_db" down
```

-- +goose Down
-- +goose StatementBegin
TRUNCATE TABLE studies RESTART IDENTITY;
-- +goose StatementEnd


28adee97-b93f-44a4-a7af-ca195f0b26fd

```bash
# просмотр всех study в базе данных
curl -v -H "Accept: application/json" http://localhost:8080/studies

# получение конкретного study по его uuid
curl -v -H "Accept: application/json" http://localhost:8080/study/d5c6821b-e85a-43e0-9313-7bf8b46993ed

# обновление dicom link у конкретного study по его uuid
curl -v -X PATCH -H "Content-Type: application/json" \
-d '{"dicom_link": "https://new-link.com"}' \
http://localhost:8080/study/28adee97-b93f-44a4-a7af-ca195f0b26fd

# удаление конкретного study по его uuid
curl -v -X DELETE http://localhost:8080/study/28adee97-b93f-44a4-a7af-ca195f0b26fd

# удаление всех study в базе данных
curl -v -X DELETE http://localhost:8080/studies

# ==========================================
# НОВЫЕ: Тестирование фильтрации
# ==========================================

# Поиск по дате
curl -s "http://localhost:8080/studies/search?study_date=2025-05-13" | jq 'length'

# Поиск по хирургу (идрисов)
curl -s "http://localhost:8080/studies/search?surgeon=идрисов"| jq 'length'

# Поиск по типу операции (КАГ)
curl -s "http://localhost:8080/studies/search?study_type=каг" | jq 'length'

# Поиск по дате + хирург
curl -s "http://localhost:8080/studies/search?study_date=2025-05-13&surgeon=идрисов" | jq 'length'

# Поиск по дате + тип операции
curl -s "http://localhost:8080/studies/search?study_date=2025-05-13&study_type=цаг" | jq 'length'

# Поиск по хирургу + тип операции
curl -s "http://localhost:8080/studies/search?surgeon=шпилевой&study_type=стент_кор" | jq 'length'

# Поиск по всем трём фильтрам
curl -s "http://localhost:8080/studies/search?study_date=2025-05-13&surgeon=идрисов&study_type=каг" | jq 'length'

# ==========================================
# Создание нового исследования
# ==========================================
curl -s -X POST -H "Content-Type: application/json" \
-d '{
  "study_id": "STUDY-101",
  "patient": "Иван",
  "age": 65,
  "department": "ОХМДиЛ",
  "name_operation": "Коронарная ангиография",
  "study_type": "каг",
  "descr_operation": "Плановое исследование",
  "time_beginning": "2025-05-13 10:31",
  "time_duration": 45,
  "surgeon": "идрисов",
  "dicom_link": "https://pacs/dicom/223"
}' \
http://localhost:8080/study

```

```bash
# Посмотреть структуру ответа
curl -s http://localhost:8080/studies | jq 'type'

# Если массив - посчитать количество элементов
curl -s http://localhost:8080/studies | jq 'length'

# Посмотреть первый элемент
curl -s http://localhost:8080/studies | jq '.[0]'
```