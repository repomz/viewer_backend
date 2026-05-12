# Команды для тестирования в cli

```bash
# просмотр всех study в базе данных
curl -v -H "Accept: application/json" http://localhost:8080/studies

# получение конкретного study по его uuid
curl -v -H "Accept: application/json" http://localhost:8080/study/d5c6821b-e85a-43e0-9313-7bf8b46993ed

# обновление dicom link у конкретного study по его uuid
curl -v -X PATCH -H "Content-Type: application/json" \
-d '{"dicom_link": "https://new-link.com"}' \
http://localhost:8080/study/d5c6821b-e85a-43e0-9313-7bf8b46993ed

# удаление конкретного study по его uuid
curl -v -X DELETE http://localhost:8080/study/d5c6821b-e85a-43e0-9313-7bf8b46993ed

# удаление всех study в базе данных (обычно эндпоинт для тестов)
curl -v -X DELETE http://localhost:8080/studies


```