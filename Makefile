-include .env
export

APP_NAME ?= viewer_backend
BUILD_DIR ?= $(if $(BUILD),$(BUILD),./bin)

.PHONY: dc build run test clean migrate-status migrate-up migrate-down migrate-create lint

# --- Сборка и запуск ---

build: ## Сборка бинарного файла
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd

run:  ## Запуск приложения
	@mkdir -p $(BUILD_DIR) && \
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd && \
	$(BUILD_DIR)/$(APP_NAME)

test: ## Запуск тестов
	go test -race -v ./...

clean: ## Очистка артефактов сборки
	@case "$(BUILD_DIR)" in \
		""|"/"|"."|"./"|".."|"../"|"../"*|"./.."|"./../"*) \
			echo "Refusing unsafe BUILD_DIR=$(BUILD_DIR)"; exit 1 ;; \
	esac
	rm -rf -- "$(BUILD_DIR)"


# --- Создание базы данных и пользователя---

db:
	@test -f $(DB_SCRIPT) || { echo "Ошибка: $(DB_SCRIPT) отсутствует"; exit 1; }
	@chmod +x $(DB_SCRIPT)
	@./$(DB_SCRIPT)

# --- Миграции (Goose) ---

migrate-status: ## Проверить статус миграций
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) $(DB_DSN) status

migrate-up: ## Применить все новые миграции
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) $(DB_DSN) up

migrate-down: ## Откатить последнюю миграцию
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) $(DB_DSN) down

migrate-create: ## Создать новый файл миграции (использование: make migrate-create name=add_users_table)
	@if [ -n "$(name)" ]; then \
		goose -dir "$(MIGRATIONS_DIR)" create "$(name)" sql; \
	else \
		read -p "Название миграции: " migration_name; \
		goose -dir "$(MIGRATIONS_DIR)" create "$$migration_name" sql; \
	fi

# --- Линтеры ---

install-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.57.2

lint:
	golangci-lint run ./...


# --- Помощь ---

help: ## Показать это сообщение
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
