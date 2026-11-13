-include backend/.env
export

export PROJECT_ROOT := $(CURDIR)

export UID := $(shell id -u)
export GID := $(shell id -g)

MIGRATIONS_DIR := backend/internal/repository/postgres_migrations
DB_URL = postgresql://${PG_USER}:${PG_PASS}@db:$(or ${PG_PORT},5432)/${PG_DB}?sslmode=disable

.PHONY: env-up env-down env-cleanup migrate-create migrate-up migrate-down migrate-action \
	logs lint-app lint-notify test test-backend test-notify frontend-check \
	compose-config proto-generate run-services dev-rerun run-notify

env-up:
	@docker compose up -d --wait db redis rabbitmq

env-down:
	@docker compose stop db redis rabbitmq
	@docker compose rm -f db redis rabbitmq

env-cleanup:
	@printf "Clear all volume files? Risk of data loss. [y/N]: "; \
	read -r ans; \
	if [ "$$ans" = "y" ] || [ "$$ans" = "Y" ]; then \
	  docker compose down -v && \
	  echo "Containers and volumes completely wiped"; \
	else \
	  echo "Clear canceled"; \
	fi

migrate-create:
	@if [ -z "$(seq)" ]; then \
		echo "Missing <seq> parameter. Example: make migrate-create seq=init"; \
		exit 1; \
	fi
	@mkdir -p "$(MIGRATIONS_DIR)"
	docker compose run --rm --no-deps migrate create \
		-ext sql \
		-dir /migrations \
		-seq "$(seq)"

migrate-up:
	@$(MAKE) migrate-action action=up

migrate-down:
	@$(MAKE) migrate-action action=down

migrate-action:
	@if [ -z "$(action)" ]; then \
		echo "Missing <action> parameter. Example: make migrate-action action=up"; \
		exit 1; \
	fi
	docker compose run --rm migrate \
		-source file:///migrations \
		-database "$(DB_URL)" \
		"$(action)"

logs:
	@if [ -z "$(name)" ]; then \
		echo "Missing <name> parameter. Example: make logs name=split_core_app"; \
		exit 1; \
	fi
	docker logs --tail=100 "$(name)"

lint-app:
	@cd backend && golangci-lint run

lint-notify:
	@cd notifier && golangci-lint run

test-backend:
	@cd backend && go test -race -v ./...

test-notify:
	@cd notifier && go test -race -v ./...

test: test-backend test-notify

frontend-check:
	@cd frontend && corepack enable && pnpm install --frozen-lockfile && pnpm exec tsc -b && pnpm build

compose-config:
	@docker compose config --quiet

proto-generate:
	@cd proto && protoc --go_out=. --go-grpc_out=. notification.proto

run-services:
	@mkdir -p "backend/out/bot"
	@mkdir -p "backend/out/web"
	@docker compose up -d --build

dev-rerun:
	@$(MAKE) env-cleanup
	@$(MAKE) env-up
	@$(MAKE) migrate-up

run-notify:
	@docker compose up -d --build split-notify
