# 🚀 SplitCore — Microservices Telegram Expense Organizer

[![CI Pipeline](https://github.com/GanFay/SplitCore/actions/workflows/ci.yml/badge.svg)](https://github.com/GanFay/SplitCore/actions/workflows/ci.yml)

**SplitCore** is a modern, high-performance Telegram bot ecosystem designed to automate shared expense tracking for groups of friends, travelers, or event organizers. 

The project has evolved from a simple monolithic bot into an **event-driven microservices architecture**. It showcases practical implementations of Clean Architecture, asynchronous message brokering (RabbitMQ), and high-speed internal RPC communication (gRPC).

## 🔥 The Core Idea
Users create "Funds", invite friends via unique deep-links, and record their expenses. The bot automatically calculates the balance: who overpaid and who needs to settle their debt using a **greedy matching algorithm** to minimize the total number of transactions.

### 🎬 Demo
*(Note: For the best experience, view the `.mp4` video directly if the GIF is buffering)*

![SplitCore Demo](demo2.gif)

---

## 🏗 System Architecture & Flow

To ensure high performance and zero blockage of the main Telegram Bot long-polling loop, the system is decoupled into two independent services communicating via **Event-Driven** and **RPC** patterns:

1. **`backend` (The Core Engine):** Handles database transactions, state management (FSM), greedy algorithms, and Telegram API interaction.
2. **`notifier` (The Background Worker):** An asynchronous service that consumes events, formats personalized HTML alerts, and orchestrates notifications.

```text
  [ Telegram Bot User ]
         │
         ▼  (Adds an expense)
┌─────────────────────────────────────────────────────────┐
│                     split-core                          │
│  1. Commits transaction to PostgreSQL database           │
│  2. Publishes "expense_created" JSON event to RabbitMQ  │──┐
└─────────────────────────────────────────────────────────┘  │
         ▲                                     (gRPC)        │ (Async event)
         │                                                   │
         │ (2. gRPC Call: Fetch target users)                ▼
         │ (4. gRPC Call: Send Telegram Message)      ┌──────────────┐
         └────────────────────────────────────────────│ split-notify │
                                                      └──────────────┘
```

### 🔄 The Async Notification Lifecycle:
1. A user logs a new expense in `backend`.
2. `backend` saves the data to **PostgreSQL** and immediately publishes an `expense_created` JSON event to a **RabbitMQ** queue.
3. The background worker `notifier` consumes the event.
4. `notifier` lacks user details, so it makes a synchronous **gRPC** request back to `backend` to fetch the target users (excluding the expense creator).
5. `notifier` generates beautiful, personalized HTML-formatted notification templates.
6. `notifier` calls `backend`'s gRPC server to dispatch the parsed messages back to Telegram asynchronously, keeping the main bot thread perfectly responsive.

---

## 🛠 Tech Stack
* **Language:** Go (Golang) 1.26.2
* **Framework:** [telebot.v4](https://github.com/tucnak/telebot) (Telegram Bot API)
* **Message Broker:** RabbitMQ (Classic Durable Queues via `amqp091-go` for async decoupled events)
* **RPC Framework:** gRPC (via Protocol Buffers v3 for high-performance synchronous calls)
* **Databases:** PostgreSQL (via `pgx/v5` Connection Pool), Redis 8.6.2 (via `go-redis/v9` for persistent FSM state)
* **Infrastructure & DevOps:** Go Workspaces (`go.work`), Docker Multi-stage builds, Docker Compose, GNU Make
* **CI/CD:** Automated testing and static analysis (`golangci-lint`) via GitHub Actions

---

## 📂 Project Structure (Monorepo)

The repository utilizes a strict Monorepo layout with isolated Go modules and shared protobuf contracts:

```text
SplitCore/ (Repository Root)
├── .github/                # GitHub Actions CI/CD workflows
├── proto/                  # gRPC contract definitions (.proto files and generated Go code)
│   └── pb/
├── split-core/             # The main transaction engine & Telegram Bot (Clean Architecture)
│   ├── cmd/bot
│   ├── internal/
│   │   ├── config/         # Config loader (reads environment variables)
│   │   ├── delivery/       # Adapters: Telegram handler & gRPC Server
│   │   ├── repository/     # Data stores: Postgres, Redis & RabbitMQ Publisher
│   │   ├── usecase/        # Core business workflow orchestrators
|   |   ├── pkg/            # Logger and other utils
|   |   └── domain/         # Entities
|   ├─  .env
│   └── Dockerfile
├── split-notify/           # Asynchronous Notification worker service (Layered Architecture)
│   ├── cmd/
│   ├── internal/
│   │   ├── client/         # gRPC client for split-core
│   │   ├── config/         # Config loader (reads environment variables)
│   │   ├── consumer/       # RabbitMQ Event consumer
│   │   └── processor/      # Event orchestrator (unmarshals JSON & routes messages)
│   └── Dockerfile
├── docker-compose.yml      # Orchestrates Postgres, Redis, RabbitMQ, split-core, and split-notify
└── Makefile                # Unified development task automation
```

---

## ⚙️ Getting Started (Local Development)

**Prerequisites:** Docker, Docker Compose, GNU Make.

1. **Clone the repository:**
   ```bash
   git clone https://github.com/GanFay/SplitCore.git
   ```
2. **Set up environment variables:** 
   Copy the example file at the root and fill in your Bot token and DB credentials:
   ```bash
   cp .env.example .env
   ```
3. **Initialize the Go Workspace locally (Optional, for IDE support):**
   ```bash
   go work init ./split-core ./notifier ./proto
   ```
4. **Start the infrastructure services (Postgres, Redis, RabbitMQ):**
   ```bash
   make env-up
   ```
5. **Run PostgreSQL database migrations:**
   ```bash
   make migrate-up
   ```
6. **Build and start all microservices:**
   ```bash
   make run-services
   ```

*(Note: To rebuild and hot-reload only the notification service after modifying its code, you can use `make run-notify`)*.
