# Короткие команды для установки, запуска и проверки проекта.
GO ?= go
ARGS ?=
BINARY ?= bin/trip-service
MIGRATIONS_DIR ?= migrations

# DATABASE_URL и остальные настройки берутся из .env, который создает tripgoctl.
-include .env

GOOSE = $(GO) tool goose -dir $(MIGRATIONS_DIR)

.DEFAULT_GOAL := help
.PHONY: help install build run format lint test migrate migrate-down migrate-status migrate-create

help: ## Показать список команд
	@grep -hE '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-15s %s\n", $$1, $$2}'

install: ## Скачать зависимости приложения и локальный линтер
	$(GO) mod download

build: ## Собрать исполняемый файл
	$(GO) build -trimpath -o $(BINARY) ./cmd/trip-service

run: ## Запустить приложение, дополнительные параметры передаются через ARGS
	$(GO) run ./cmd/trip-service $(ARGS)

format: ## Применить gofumpt и goimports
	$(GO) tool golangci-lint fmt

lint: ## Проверить go.mod, форматирование и код линтерами
	$(GO) mod tidy -diff
	$(GO) tool golangci-lint config verify
	$(GO) tool golangci-lint fmt --diff
	$(GO) tool golangci-lint run

test: ## Запустить тесты с race detector (требуется CGO) и сформировать отчеты покрытия
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html

migrate: ## Применить все миграции к DATABASE_URL
	$(GOOSE) postgres "$(DATABASE_URL)" up

migrate-down: ## Откатить последнюю миграцию
	$(GOOSE) postgres "$(DATABASE_URL)" down

migrate-status: ## Показать состояние миграций
	$(GOOSE) postgres "$(DATABASE_URL)" status

migrate-create: ## Создать SQL-миграцию, имя передается через NAME
	@test -n "$(NAME)" || { echo "Укажите имя: make migrate-create NAME=create_trips"; exit 1; }
	$(GOOSE) -s create $(NAME) sql

# TODO: tripgoctl environment start / stop
