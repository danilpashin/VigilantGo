# Настройки интерпретатора shell
SHELL := /usr/bin/env bash -euo pipefail

# Переменные по умолчанию
APP_NAME ?= vigilant-go
MAIN_PATH ?= ./cmd/api
BIN_DIR ?= ./bin

.PHONY: all help fmt vet test test-unit test-race bench run build clean

# Цель по умолчанию: выводит список доступных команд
all: help

## help: Показать список доступных команд
help:
	@echo "Доступные команды:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## fmt: Отформатировать Go-код
fmt:
	@echo "==> Форматирование кода..."
	gofmt -w -s .

## vet: Запустить статический анализатор go vet
vet:
	@echo "==> Анализ кода (go vet)..."
	go vet ./...

## test: Запустить все unit-тесты
test:
	@echo "==> Запуск тестов..."
	go test -count=1 ./...

## test-race: Запустить тесты с детектором гонок
test-race:
	@echo "==> Запуск тестов с детекцией data race..."
	go test -race -count=1 ./...

## test-cover: Запустить тесты с расчетом покрытия
test-cover:
	@echo "==> Расчет покрытия тестами..."
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

## check: Комплексная проверка перед коммитом (fmt, vet, test-race)
check: fmt vet test-race

## build: Собрать бинарный файл
build: clean
	@echo "==> Сборка $(APP_NAME)..."
	CGO_ENABLED=0 go build -trimpath -o $(BIN_DIR)/$(APP_NAME) $(MAIN_PATH)

## run: Запустить приложение локально
run:
	@echo "==> Запуск приложения..."
	go run $(MAIN_PATH)

## clean: Очистить скомпилированные артефакты
clean:
	@echo "==> Очистка артефактов сборки..."
	rm -rf $(BIN_DIR) coverage.out