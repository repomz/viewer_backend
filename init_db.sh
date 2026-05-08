#!/bin/bash

# Переменные
DB_NAME="angio_db"
DB_USER="angio_user"
DB_PASS="angio_password"

echo "Создание изолированной базы данных и пользователя..."

# 1. Создаем базу и пользователя через системного администратора
sudo -u postgres psql <<EOF
-- Создаем базу
CREATE DATABASE $DB_NAME;

-- Создаем пользователя
CREATE USER $DB_USER WITH ENCRYPTED PASSWORD '$DB_PASS';

-- Глобальный запрет: отзываем у всех (PUBLIC) право подключаться к базе postgres
REVOKE CONNECT ON DATABASE postgres FROM PUBLIC;

-- Разрешаем пользователю подключаться ТОЛЬКО к его базе
GRANT CONNECT ON DATABASE $DB_NAME TO $DB_USER;
EOF

# 2. Настраиваем права внутри самой базы
sudo -u postgres psql -d $DB_NAME <<EOF
-- Отзываем права у посторонних лиц на схему public в этой базе
REVOKE ALL ON SCHEMA public FROM PUBLIC;

-- Даем полные права на схему public нашему пользователю
GRANT ALL ON SCHEMA public TO $DB_USER;

-- Устанавливаем права по умолчанию: 
-- все, что будет создано в этой базе, будет доступно нашему пользователю
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO $DB_USER;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO $DB_USER;
EOF

echo "Готово! Настройка базы данных '$DB_NAME' завершена успешно! Создан пользователь '$DB_USER', имеет доступ к базе '$DB_NAME'."
