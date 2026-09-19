# SplitCore v1.2.0

[![CI Pipeline](https://github.com/GanFay/SplitCore/actions/workflows/ci.yml/badge.svg)](https://github.com/GanFay/SplitCore/actions/workflows/ci.yml)

**SplitCore** is a Telegram-first shared expense platform with a full web dashboard, async notifications, and a clean microservice architecture.

Create a fund, invite friends, log shared expenses, and let SplitCore calculate who owes whom using a greedy settlement algorithm that minimizes the number of transfers.

![SplitCore Demo](demo2.gif)

## What Is New In 1.2.0

- **React + Tailwind web dashboard** served by Nginx on `:3000`.
- **Telegram web login flow** with session polling, JWT access tokens, and refresh cookie support.
- **HTTP API v1** for funds, members, purchases, balances, virtual users, and current user profile.
- **Swagger documentation** regenerated for the updated API surface.
- **Docker Compose frontend service** added next to backend, bot, notifier, Postgres, Redis, and RabbitMQ.
- **Responsive UI** for desktop and mobile: fund sidebar, dashboard metrics, expense history, settlement view, invite codes, theme toggle, and member management.
- **Deploy polish**: frontend build uses `pnpm-lock.yaml`, compose volumes are relative-path friendly, and Makefile lint paths match the current monorepo layout.

## Product Flow

SplitCore has two user surfaces:

- **Telegram Bot**: fast fund creation, invite links, expense logging, and group notifications.
- **Web Dashboard**: visual fund management with balances, members, purchases, invite codes, and settlement details.

The web app authenticates through Telegram:

1. The browser asks the backend for a temporary auth session.
2. The user opens the generated Telegram deep link.
3. The bot confirms the session and binds it to the internal user id.
4. The web app receives JWT tokens and loads the dashboard.

## Architecture

```text
                       ┌────────────────────────────┐
                       │     React + Tailwind UI     │
                       │     Nginx, /api proxy       │
                       └──────────────┬─────────────┘
                                      │ HTTP / Swagger
                                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                            backend                              │
│  HTTP API · Telegram Bot · gRPC server · usecases · repositories │
└──────────────┬───────────────────────────────┬──────────────────┘
               │                               │
               │ PostgreSQL / Redis            │ RabbitMQ event
               ▼                               ▼
      ┌────────────────┐              ┌────────────────┐
      │ Postgres/Redis │              │    notifier    │
      └────────────────┘              │ Rabbit consumer│
                                      │ gRPC client    │
                                      └───────┬────────┘
                                              │ gRPC
                                              ▼
                                      ┌──────────────┐
                                      │ Telegram API │
                                      └──────────────┘
```

### Async Notification Lifecycle

1. A user logs an expense.
2. `backend` writes the purchase to PostgreSQL.
3. `backend` publishes an `expense_created` event to RabbitMQ.
4. `notifier` consumes the event.
5. `notifier` asks `backend` over gRPC for fund members.
6. `backend` sends Telegram notifications asynchronously, keeping the bot responsive.

## Tech Stack

| Area | Tech |
| --- | --- |
| Frontend | React 19, TypeScript, Vite, Tailwind CSS, lucide-react, Nginx |
| Backend | Go 1.26, Clean Architecture-style domain/usecase/repository layers |
| Telegram | `telebot.v4`, deep links, stateful bot flows |
| API | Go `net/http`, JWT auth, Swagger via `swaggo` |
| Async | RabbitMQ, durable event publishing |
| RPC | gRPC, Protocol Buffers |
| Storage | PostgreSQL, Redis |
| DevOps | Docker, Docker Compose, Makefile, multi-stage builds |

## Repository Layout

```text
.
├── backend/                 # Core service: HTTP API, Telegram bot, gRPC server
│   ├── cmd/
│   │   ├── bot/             # Telegram bot entrypoint
│   │   └── web/             # HTTP + Swagger entrypoint
│   ├── docs/                # Generated Swagger docs
│   ├── internal/
│   │   ├── delivery/        # HTTP, Telegram, gRPC adapters
│   │   ├── domain/          # Entities and interfaces
│   │   ├── repository/      # Postgres, Redis, RabbitMQ implementations
│   │   ├── usecase/         # Business logic and settlement algorithm
│   │   └── pkg/             # Logger and utilities
│   ├── Dockerfile.bot
│   └── Dockerfile.web
├── frontend/                # React + Tailwind web dashboard
│   ├── src/
│   │   ├── api/             # Typed HTTP client
│   │   ├── components/      # Reusable UI pieces
│   │   └── lib/             # Formatting helpers
│   ├── Dockerfile
│   └── nginx.conf
├── notifier/                # RabbitMQ consumer and gRPC notification worker
├── proto/                   # Notification service protobuf contract
├── docker-compose.yaml      # Full local stack
├── Makefile                 # Development and deployment shortcuts
└── README.md
```

## Services And Ports

| Service | Port | Description |
| --- | ---: | --- |
| `frontend` | `3000` | Web dashboard, Nginx SPA, `/api` proxy |
| `web` | `8080` | HTTP API and Swagger |
| `bot` | - | Telegram long-polling bot |
| `split-notify` | - | Async notification worker |
| `db` | `5432` | PostgreSQL |
| `redis` | `6379` | FSM/session cache |
| `rabbitmq` | `5672` | AMQP |
| `rabbitmq` management | `15672` | RabbitMQ UI |

Swagger UI:

```text
http://localhost:8080/api/v1/swagger/index.html
```

Frontend:

```text
http://localhost:3000
```

## Environment

Create your backend environment file:

```bash
cp backend/.env.example backend/.env
```

Required values:

```env
TOKEN=YOUR_BOT_TOKEN
BOT_NAME=YOUR_BOT_NAME

JWT_SECRET_ACCESS=your_access_secret
JWT_SECRET_REFRESH=your_refresh_secret

BOT_VER=1.2.0
NOT_VER=1.2.0
ENV=local

PG_USER=USER
PG_PASS=PASSWORD
PG_DB=MYDB
PG_PORT=5432

REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASS=PASS

RMQ_USER=guest
RMQ_PASS=guest

GRPC_PORT=:50001
```

For production, use strong secrets and run the web app behind HTTPS. Refresh tokens are stored in an HTTP-only secure cookie, so HTTPS is the expected deployment mode.

## Run Locally

Start the infrastructure:

```bash
make env-up
```

Run database migrations:

```bash
make migrate-up
```

Build and run the full stack:

```bash
make run-services
```

Open:

```text
Frontend: http://localhost:3000
Swagger:  http://localhost:8080/api/v1/swagger/index.html
RabbitMQ: http://localhost:15672
```

## Development

Backend tests:

```bash
cd backend
go test ./...
```

Frontend development server:

```bash
cd frontend
pnpm install
pnpm dev
```

Frontend production build:

```bash
cd frontend
pnpm build
```

Regenerate Swagger:

```bash
cd backend
go run -mod=mod github.com/swaggo/swag/cmd/swag init -g cmd/web/main.go
```

Generate protobuf code:

```bash
make proto-generate
```

Run only the notifier after changes:

```bash
make run-notify
```

## API Highlights

Authentication:

- `POST /api/v1/auth/telegram/init`
- `POST /api/v1/auth/telegram/status`
- `POST /api/v1/auth/telegram/tokens`
- `POST /api/v1/auth/telegram/access`
- `POST /api/v1/user/me`

Funds:

- `POST /api/v1/fund`
- `DELETE /api/v1/fund`
- `POST /api/v1/fund/list`
- `POST /api/v1/fund/info`
- `POST /api/v1/fund/join`
- `POST /api/v1/fund/balance`
- `POST /api/v1/fund/expense`
- `POST /api/v1/fund/members`
- `POST /api/v1/fund/virtual-users`
- `POST /api/v1/fund/virtual-users/list`
- `POST /api/v1/fund/purchases`
- `DELETE /api/v1/fund/member`

The API keeps selected `GET` routes for compatibility, but browser-facing endpoints use `POST` when a JSON body is required.

## Deployment Notes

- `docker-compose.yaml` includes local `build` blocks for development.
- `docker-compose.prod.yaml` is image-only for VPS deployment with two domains and Caddy-managed HTTPS.
- GitHub Actions builds and pushes GHCR images tagged as `latest` and `1.2.0`.
- The frontend image builds with `pnpm install --frozen-lockfile`.
- Nginx proxies `/api` to the internal `web:8080` service.
- Run `make run-services` for local all-in-one deployment.
- For production, set real environment variables, enable HTTPS, and avoid committing `.env` files.
- If you publish images, tag them with `1.2.0` as well as `latest` for reproducible rollbacks.

Production quick start:

```bash
docker compose --env-file backend/.env -f docker-compose.prod.yaml pull
docker compose --env-file backend/.env -f docker-compose.prod.yaml --profile tools run --rm migrate
docker compose --env-file backend/.env -f docker-compose.prod.yaml up -d
```

## Release Checklist For 1.2.0

- Backend tests pass.
- Frontend production build passes.
- Swagger docs are generated.
- Docker Compose config validates.
- `backend/.env.example` version fields are updated to `1.2.0`.
- README reflects the frontend + HTTP API release.

## License

This project is distributed under the terms of the [LICENSE](LICENSE).
