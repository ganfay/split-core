include backend/.env
export

export PROJECT_ROOT=$(shell pwd)

export UID=$(shell id -u)
export GID=$(shell id -g)

MIGRATIONS_DIR = backend/internal/repository/postgres_migrations
DB_URL = postgresql://${PG_USER}:${PG_PASS}@db:${PG_PORT}/${PG_DB}?sslmode=disable

env-up:
	@docker compose up -d db redis rabbitmq

env-down:
	@docker compose down db redis rabbitmq

env-cleanup:
	@read -p "Clear all volume files? Risk of data loss. [y/N]: " ans; \
	if [ "$$ans" = "y" ]; then \
	  docker compose down -v db redis && \
	  echo "Containers and volumes completely wiped"; \
	else \
	  echo "Clear canceled"; \
	fi

migrate-create:
	@if [ -z "$(seq)" ]; then \
  		echo "Missing <seq> parameter. Example: make migrate-create seq=init"; \
  		exit 1; \
  	fi; \
  	mkdir -p ${MIGRATIONS_DIR}; \
	docker compose run --rm migrate create \
		-ext sql \
		-dir /migrations \
		-seq $(seq)

migrate-up:
	@make migrate-action action=up

migrate-down:
	@make migrate-action action=down

migrate-action:
	@if [ -z "$(action)" ]; then \
  		echo "Missing <action> parameter. Example: make migrate-action action=up"; \
  		exit 1; \
  	fi; \
	docker compose run --rm migrate \
		-source file:///migrations \
		-database $(DB_URL) \
		"$(action)"

logs:
	@if [ -z "$(name)" ]; then \
  		echo "Missing <name> parameter. Example: make logs name=split_core_app"; \
  		exit 1; \
  	fi; \
	docker logs --tail=100 $(name)

lint-app:
	@cd backend && golangci-lint run

lint-notify:
	@cd notifier && golangci-lint run

proto-generate:
	@cd proto && protoc --go_out=. --go-grpc_out=. notification.proto

run-services:
	@mkdir -p "backend/out/bot"
	@mkdir -p "backend/out/web"
	@docker compose up -d --build

dev-rerun:
	@make env-cleanup
	@make env-up
	sleep 2
	@make migrate-up

run-notify:
	@docker compose up -d --build split-notify
