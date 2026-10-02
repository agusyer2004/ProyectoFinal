ifeq ($(OS),Windows_NT)
SHELL := C:/Program Files/Git/bin/sh.exe
endif

COMPOSE ?= docker compose -f deploy/docker-compose.yml
GO_MODULES := gateway sink

.PHONY: up up-dev down test lint topics migrate smoke

deploy/.env:
	cp deploy/.env.example deploy/.env

up: deploy/.env
	$(COMPOSE) --profile core up -d --wait

up-dev: deploy/.env
	$(COMPOSE) --profile core --profile dev up -d --wait

down:
	$(COMPOSE) --profile core --profile obs --profile dev down

test:
	@for m in $(GO_MODULES); do (cd $$m && go test ./...) || exit 1; done
	cd simulator && uv run pytest tests ../deploy/tests

lint:
	@for m in $(GO_MODULES); do (cd $$m && golangci-lint run ./...) || exit 1; done
	cd simulator && uv run ruff check . ../deploy/tests && uv run ruff format --check . ../deploy/tests

topics: deploy/.env
	$(COMPOSE) --profile core --profile init run --rm kafka-init

# Los siguientes objetivos se implementan en bloques posteriores.
migrate:
	@echo "pendiente: Bloque 3.1"; exit 1

smoke:
	@echo "pendiente: Bloque 3.4"; exit 1
