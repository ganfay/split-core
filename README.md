# SplitCore v1.2.0

[![CI Pipeline](https://github.com/GanFay/SplitCore/actions/workflows/ci.yml/badge.svg)](https://github.com/GanFay/SplitCore/actions/workflows/ci.yml)

**SplitCore** is a Telegram-first platform for managing shared group expenses.

Create funds, invite members, record expenses, track balances, and calculate settlements using a greedy algorithm that minimizes the number of required transfers.

### Live

**Web Dashboard:** `https://thesplitcore.tech`
**API:** `https://ganfay.me`

The web dashboard is the main visual interface, while Telegram provides fast expense tracking, fund management, invites, and notifications.

![SplitCore Demo](demo2.gif)

## Backend

The backend is the core application service and provides:

* HTTP API
* Telegram bot
* gRPC server
* Business use cases
* Domain entities and interfaces
* PostgreSQL and Redis repositories
* RabbitMQ event publishing
* Settlement calculation

The codebase follows a **Clean Architecture-style separation** between delivery, domain, use cases, and infrastructure.

```text
React + Tailwind
       │
       │ HTTP
       ▼
    Backend
   ┌───┼───────────────┐
   │   │               │
   ▼   ▼               ▼
Postgres Redis       RabbitMQ
                       │
                       ▼
                    Notifier
                       │
                       │ gRPC
                       ▼
                    Backend
                       │
                       ▼
                 Telegram API
```

## Settlement Algorithm

SplitCore calculates the final debt graph from the current balances.

The settlement step uses a **greedy debt-settlement algorithm**:

1. Calculate each member's net balance.
2. Separate creditors and debtors.
3. Match the largest outstanding debtor with the largest outstanding creditor.
4. Transfer the smaller of the two remaining amounts.
5. Repeat until all balances are settled.

This reduces the number of transfers required to settle the group compared with simply paying every recorded expense individually.

## Tech Stack

| Layer        | Technology                                                |
| ------------ | --------------------------------------------------------- |
| Frontend     | React 19, TypeScript, Vite, Tailwind CSS, lucide-react    |
| Web Server   | Nginx                                                     |
| Backend      | Go 1.26                                                   |
| Architecture | Clean Architecture-style domain/usecase/repository layers |
| Telegram     | `telebot.v4`, deep links, stateful bot flows              |
| HTTP API     | Go `net/http`, JWT, Swagger via `swaggo`                  |
| Messaging    | RabbitMQ                                                  |
| RPC          | gRPC + Protocol Buffers                                   |
| Database     | PostgreSQL                                                |
| Cache / FSM  | Redis                                                     |
| Containers   | Docker, Docker Compose                                    |
| Tooling      | Makefile, GitHub Actions, GHCR                            |

## Services

| Service               |    Port | Purpose                          |
| --------------------- | ------: | -------------------------------- |
| `frontend`            |  `3000` | React dashboard + Nginx          |
| `web`                 |  `8080` | HTTP API + Swagger               |
| `bot`                 |       — | Telegram long-polling bot        |
| `split-notify`        |       — | Asynchronous notification worker |
| `db`                  |  `5432` | PostgreSQL                       |
| `redis`               |  `6379` | FSM and session storage          |
| `rabbitmq`            |  `5672` | AMQP broker                      |
| `rabbitmq management` | `15672` | RabbitMQ management UI           |

### Local URLs

**Frontend**

```text
http://localhost:3000
```

**Swagger**

```text
http://localhost:8080/api/v1/swagger/index.html
```

**RabbitMQ**

```text
http://localhost:15672
```

## Environment

Create the backend environment file:

```bash
cp backend/.env.example backend/.env
cp backend/.env .env
```

Required configuration:

```env
#SettingsTG
TOKEN=YOUR_BOT_TOKEN
BOT_NAME=YOUR_BOT_NAME

#Deploy
IMAGE_TAG=latest
FRONTEND_DOMAIN=split.example.com
BACKEND_DOMAIN=api.split.example.com
ACME_EMAIL=you@example.com

#HTTP
JWT_SECRET_ACCESS=your_secret_key
JWT_SECRET_REFRESH=your_secret_key

#Settings
BOT_VER=1.2.0
NOT_VER=1.2.0
ENV=local

#PostgreSQL
PG_USER=USER
PG_PASS=PASSWORD
PG_DB=MYDB
PG_PORT=5432

#Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASS=PASS

#RabbitMQ
RMQ_PASS=guest
RMQ_USER=guest

#gRpc Port
GRPC_PORT=:50001
```

For production:

* use strong, unique secrets;
* keep `.env` files out of version control;
* run the web application behind HTTPS;
* keep refresh tokens in secure HTTP-only cookies;
* use versioned container tags for reproducible deployments.

## Run Locally

Start the infrastructure:

```bash
make env-up
```

Run database migrations:

```bash
make migrate-up
```

Start the full stack:

```bash
make run-services
```

Then open:

```text
Frontend → http://localhost:3000
Swagger  → http://localhost:8080/api/v1/swagger/index.html
RabbitMQ → http://localhost:15672
```

## Development

### Backend tests

```bash
cd backend
go test ./...
```

### Frontend development

```bash
cd frontend
pnpm install
pnpm dev
```

### Frontend production build

```bash
cd frontend
pnpm build
```

### Regenerate Swagger

```bash
cd backend
go run -mod=mod github.com/swaggo/swag/cmd/swag init -g cmd/web/main.go
```

### Generate protobuf code

```bash
make proto-generate
```

### Run the notifier

```bash
make run-notify
```

## API

The current API is versioned under `/api/v1`.

### Authentication

```text
POST /api/v1/auth/telegram/init
POST /api/v1/auth/telegram/status
POST /api/v1/auth/telegram/tokens
POST /api/v1/auth/telegram/access
POST /api/v1/user/me
```

### Funds

```text
POST   /api/v1/fund
DELETE /api/v1/fund

POST /api/v1/fund/list
POST /api/v1/fund/info
POST /api/v1/fund/join
POST /api/v1/fund/balance
POST /api/v1/fund/expense
POST /api/v1/fund/members
POST /api/v1/fund/virtual-users
POST /api/v1/fund/virtual-users/list
POST /api/v1/fund/purchases

DELETE /api/v1/fund/member
```

Selected `GET` endpoints remain available for compatibility. Browser-facing endpoints that require JSON request bodies currently use `POST`.

Full API documentation is available through Swagger:

```text
http://localhost:8080/api/v1/swagger/index.html
```

## Deployment

The repository provides separate Compose configurations for development and production.

### Development

`docker-compose.yaml` builds services locally and is intended for development.

### Production

`docker-compose.prod.yaml` uses pre-built container images and is intended for VPS deployment.

The production stack includes:

* frontend served by Nginx;
* internal `/api` proxy to the backend;
* GHCR-hosted images;
* Caddy-managed HTTPS;
* database migrations through the `tools` profile.

Production quick start:

```bash
docker compose \
  --env-file backend/.env \
  -f docker-compose.prod.yaml \
  pull
```

Run migrations:

```bash
docker compose \
  --env-file backend/.env \
  -f docker-compose.prod.yaml \
  --profile tools run --rm migrate
```

Start the stack:

```bash
docker compose \
  --env-file backend/.env \
  -f docker-compose.prod.yaml \
  up -d
```

### Container Tags

Production images are published with both:

```text
latest
1.2.0
```

Versioned tags should be preferred when reproducible deployments or rollbacks are required.

## CI/CD

GitHub Actions runs the CI pipeline and builds the project's container images.

The production workflow publishes images to **GitHub Container Registry (GHCR)** using both the release version and `latest` tags.

## Release Checklist — 1.2.0

* [x] Backend tests pass
* [x] Frontend production build passes
* [x] Swagger documentation regenerated
* [x] Docker Compose configuration validated
* [x] `backend/.env.example` updated
* [x] README updated for the web + API release
* [x] Production deployment verified

## License

This project is distributed under the terms of the [LICENSE](LICENSE).
